package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"subpage/internal/api/middleware"
	"subpage/internal/config"
	"subpage/internal/pkg/subpage"
	"subpage/web"
)

// ignoredHeaders are stripped when proxying a raw subscription response
// back to the client, mirroring IGNORED_HEADERS in ignored-headers.constant.ts.
var ignoredHeaders = map[string]bool{
	"accept-encoding": true, "alt-svc": true, "authorization": true, "cache-control": true,
	"cf-access-client-id": true, "cf-access-client-secret": true, "cf-cache-status": true,
	"cf-connecting-ip": true, "cf-ray": true, "connection": true, "content-length": true,
	"content-security-policy": true, "cross-origin-opener-policy": true,
	"cross-origin-resource-policy": true, "expires": true, "fastly-client-ip": true,
	"forwarded": true, "forwarded-for": true, "host": true, "keep-alive": true, "nel": true,
	"origin-agent-cluster": true, "pragma": true, "proxy-authenticate": true,
	"proxy-authorization": true, "report-to": true, "server": true, "te": true,
	"trailer": true, "transfer-encoding": true, "true-client-ip": true, "upgrade": true,
	"x-api-key": true, "x-client-ip": true, "x-cluster-client-ip": true, "x-forwarded": true,
	"x-forwarded-for": true, "x-forwarded-proto": true, "x-forwarded-scheme": true,
	"x-real-ip": true, "x-remnawave-client-type": true, "x-remnawave-real-ip": true,
	"x-subpage-version": true,
}

var requestTemplateTypeValues = map[string]bool{
	"stash": true, "singbox": true, "mihomo": true, "json": true, "v2ray-json": true, "clash": true,
}

// AppConfigRoute is the path served at /assets/.app-config-v2.json, per
// APP_CONFIG_ROUTE_WO_LEADING_PATH in @remnawave/subscription-page-types@0.4.0.
const AppConfigRoute = "/assets/.app-config-v2.json"

// Subpage groups the handlers for the subscription page: app-config
// (behind the session cookie), the shortUuid catch-all that serves either
// the rendered HTML page or a raw subscription payload, and the built
// frontend's static assets.
type Subpage struct {
	cfg         *config.Config
	panel       *subpage.PanelClient
	configStore *subpage.ConfigStore
}

func NewSubpage(cfg *config.Config, panel *subpage.PanelClient, configStore *subpage.ConfigStore) *Subpage {
	return &Subpage{cfg: cfg, panel: panel, configStore: configStore}
}

// AppConfigHandler serves the decrypted subpage raw config to the SPA.
// Requires WithAssetsCookieCheck to have already verified the session
// cookie and populated the `su` (encrypted subpage-config uuid) claim.
func (h *Subpage) AppConfigHandler(c *fiber.Ctx) error {
	claims := middleware.SessionClaims(c)
	if claims == nil {
		return middleware.KillConnection(c)
	}
	raw, ok := h.configStore.RawConfigJSON(claims.Su)
	if !ok {
		slog.Error("subpage config not found for su claim")
		return middleware.KillConnection(c)
	}
	c.Set("Content-Type", "application/json")
	return c.Send(raw)
}

// SubpageHandler serves GET /{shortUuid} and GET /{shortUuid}/{clientType}:
// either a rendered HTML page (browser clients) or a raw subscription
// payload (VPN client apps), mirroring RootController + RootService.
//
// Built frontend assets (hashed .js/.css/favicons at the dist root) are
// requested as single-segment paths too, so a real static file always
// takes priority over the shortUuid interpretation.
func (h *Subpage) SubpageHandler(c *fiber.Ctx) error {
	if strings.HasPrefix(c.Path(), "/assets") || strings.HasPrefix(c.Path(), "/locales") {
		return middleware.KillConnection(c)
	}

	clientType := c.Params("clientType")
	if clientType == "" {
		if f, err := web.StaticFS.Open(c.Path()); err == nil {
			_ = f.Close()
			// Every file under the dist root is content-hashed by the frontend
			// build (index-<hash>.js, index-<hash>.css, favicon-<hash>.png, ...),
			// so it's safe to cache aggressively: a config/code change always
			// produces a new URL rather than mutating this one.
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
			return filesystem.SendFile(c, web.StaticFS, c.Path())
		}
	}

	shortUUID := c.Params("shortUuid")
	if clientType != "" && !requestTemplateTypeValues[clientType] {
		slog.Error("invalid client type", "clientType", clientType)
		return middleware.KillConnection(c)
	}

	return h.serveSubscriptionPage(c, shortUUID, clientType)
}

func (h *Subpage) serveSubscriptionPage(c *fiber.Ctx, shortUUID, clientType string) error {
	userAgent := c.Get("User-Agent")
	clientIP := middleware.ClientIP(c)

	if subpage.IsGenericPath(c.Path()) {
		return middleware.KillConnection(c)
	}

	if userAgent != "" && subpage.IsBrowser(userAgent) {
		return h.returnWebpage(c, shortUUID)
	}

	resp, err := h.panel.GetSubscription(shortUUID, clientType, clientIP, headersFromFiber(c))
	if err != nil || resp == nil {
		return middleware.KillConnection(c)
	}
	for k, vs := range resp.Headers {
		if ignoredHeaders[strings.ToLower(k)] {
			continue
		}
		for _, v := range vs {
			c.Response().Header.Add(k, v)
		}
	}
	return c.Status(resp.Status).Send(resp.Body)
}

func (h *Subpage) returnWebpage(c *fiber.Ctx, shortUUID string) error {
	clientIP := middleware.ClientIP(c)

	infoResp, err := h.panel.GetSubscriptionInfo(shortUUID, clientIP)
	if err != nil || !infoResp.OK {
		return middleware.KillConnection(c)
	}

	subpageResp, err := h.panel.GetSubpageConfig(shortUUID, clientIP, headersFromFiber(c))
	if err != nil || !subpageResp.OK {
		return middleware.KillConnection(c)
	}
	var subpageConfig struct {
		Response struct {
			SubpageConfigUUID *string `json:"subpageConfigUuid"`
			WebpageAllowed    bool    `json:"webpageAllowed"`
		} `json:"response"`
	}
	if err := json.Unmarshal(subpageResp.Body, &subpageConfig); err != nil {
		return middleware.KillConnection(c)
	}
	if !subpageConfig.Response.WebpageAllowed {
		slog.Info("webpage not allowed for this subpage config")
		return middleware.KillConnection(c)
	}

	perRequestUUID := ""
	if subpageConfig.Response.SubpageConfigUUID != nil {
		perRequestUUID = *subpageConfig.Response.SubpageConfigUUID
	}
	baseSettings := h.configStore.GetBaseSettings(perRequestUUID)

	var subscriptionData map[string]any
	if err := json.Unmarshal(infoResp.Body, &subscriptionData); err != nil {
		return middleware.KillConnection(c)
	}
	if !baseSettings.ShowConnectionKeys {
		if respField, ok := subscriptionData["response"].(map[string]any); ok {
			respField["links"] = []string{}
			respField["ssConfLinks"] = map[string]string{}
		}
	}

	sessionID := subpage.NewSessionID()
	encryptedUUID, err := h.configStore.EncryptedUUID(perRequestUUID)
	if err != nil {
		return middleware.KillConnection(c)
	}
	sessionJWT, err := subpage.NewSessionJWT(sessionID, encryptedUUID, h.cfg.InternalJWTSecret)
	if err != nil {
		return middleware.KillConnection(c)
	}
	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    sessionJWT,
		HTTPOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   1800,
	})

	panelDataJSON, err := json.Marshal(subscriptionData)
	if err != nil {
		return middleware.KillConnection(c)
	}
	panelData := base64.StdEncoding.EncodeToString(panelDataJSON)

	var buf bytes.Buffer
	err = web.IndexTemplate.Execute(&buf, struct {
		MetaTitle       string
		MetaDescription string
		PanelData       string
	}{
		MetaTitle:       baseSettings.MetaTitle,
		MetaDescription: baseSettings.MetaDescription,
		PanelData:       panelData,
	})
	if err != nil {
		slog.Error("failed to render index template", "err", err)
		return middleware.KillConnection(c)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).Send(buf.Bytes())
}

// headersFromFiber converts the incoming fasthttp request headers into a
// net/http.Header, since PanelClient's signature (and the panel's own
// requestHeaders JSON body) predates the framework switch.
func headersFromFiber(c *fiber.Ctx) http.Header {
	h := make(http.Header)
	c.Request().Header.VisitAll(func(k, v []byte) {
		h.Add(string(k), string(v))
	})
	return h
}

// Package middleware holds the fiber.Handler chain shared by every route:
// logging, anti-robots headers, reverse-proxy enforcement, real-IP
// extraction, and the assets session-cookie check.
package middleware

import (
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"subpage/internal/config"
	"subpage/internal/pkg/subpage"
)

type Middleware struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Middleware {
	return &Middleware{cfg: cfg}
}

const localKeySessionClaims = "sessionClaims"

// WithLogger logs method, path, status and duration for every request.
func (m *Middleware) WithLogger(c *fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	slog.Info("request",
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"took", time.Since(start),
	)
	return err
}

// KillConnection aborts the TCP connection with no HTTP response, matching
// the Nest original's res.socket?.destroy() anti-probing behavior used on
// every rejected/unmatched request.
func KillConnection(c *fiber.Ctx) error {
	c.Context().HijackSetNoResponse(true)
	c.Context().Hijack(func(conn net.Conn) {
		_ = conn.Close()
	})
	return nil
}

// WithNoRobots sets x-robots-tag on every response. Mirrors no-robots.middleware.ts.
func (m *Middleware) WithNoRobots(c *fiber.Ctx) error {
	c.Set("x-robots-tag", "noindex, nofollow, noarchive, nosnippet, noimageindex")
	return c.Next()
}

// WithProxyCheck requires the request to arrive over a trusted reverse
// proxy with HTTPS termination, mirroring proxy-check.middleware.ts.
// Skipped in debug/dev mode. c.Secure() reads X-Forwarded-Proto because the
// app is configured with EnableTrustedProxyCheck (see server.go).
func (m *Middleware) WithProxyCheck(c *fiber.Ctx) error {
	if m.cfg.Debug {
		return c.Next()
	}
	isProxy := c.Get("X-Forwarded-For") != ""
	if !c.Secure() || !isProxy {
		slog.Error("reverse proxy and HTTPS are required")
		return KillConnection(c)
	}
	return c.Next()
}

// ClientIP returns the request's real client IP. Fiber resolves this
// itself from X-Forwarded-For once EnableTrustedProxyCheck/ProxyHeader are
// configured on the app (see server.go), the way TRUST_PROXY did in the
// Node original, so no dedicated middleware is needed here.
func ClientIP(c *fiber.Ctx) string {
	return c.IP()
}

// WithAssetsCookieCheck verifies the "session" JWT cookie for requests
// under /assets and /locales (which includes the app-config route,
// /assets/.app-config-v2.json) and stores the claims in locals. Mirrors
// check-assets-cookie.middleware.ts.
func (m *Middleware) WithAssetsCookieCheck(c *fiber.Ctx) error {
	if !strings.HasPrefix(c.Path(), "/assets") && !strings.HasPrefix(c.Path(), "/locales") {
		return c.Next()
	}
	if m.cfg.InternalJWTSecret == "" {
		slog.Error("INTERNAL_JWT_SECRET is not configured")
		return KillConnection(c)
	}
	cookie := c.Cookies("session")
	if cookie == "" {
		slog.Debug("no session cookie")
		return KillConnection(c)
	}
	claims, err := subpage.VerifySessionJWT(cookie, m.cfg.InternalJWTSecret)
	if err != nil {
		slog.Debug("invalid session cookie", "err", err)
		return KillConnection(c)
	}
	c.Locals(localKeySessionClaims, claims)
	return c.Next()
}

// SessionClaims returns the claims stashed by WithAssetsCookieCheck.
func SessionClaims(c *fiber.Ctx) *subpage.SessionClaims {
	if cl, ok := c.Locals(localKeySessionClaims).(*subpage.SessionClaims); ok {
		return cl
	}
	return nil
}

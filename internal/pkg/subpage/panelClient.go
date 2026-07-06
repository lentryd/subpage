package subpage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PanelClient talks to the Remnawave panel REST API. Endpoint paths come
// from the public @remnawave/backend-contract package (api/routes.js):
// ROOT="/api", SUBSCRIPTION_CONTROLLER="sub", SUBSCRIPTIONS_CONTROLLER=
// "subscriptions", SUBSCRIPTION_PAGE_CONFIGS_CONTROLLER=
// "subscription-page-configs".
type PanelClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewPanelClient(baseURL, token string) *PanelClient {
	return &PanelClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// PanelResponse wraps a result from the panel API along with pass-through
// HTTP headers, mirroring the AxiosService return shape used by root.service.ts.
type PanelResponse struct {
	OK      bool
	Status  int
	Headers http.Header
	Body    []byte
}

func (c *PanelClient) do(method, path, clientIP string, extraHeaders http.Header, body []byte) (*PanelResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	for k, vs := range extraHeaders {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	// The panel's own ProxyCheckMiddleware (same anti-probing design as this
	// app's) requires every request to look like it arrived through a
	// trusted reverse proxy: rejects (socket close, no HTTP response) if
	// X-Forwarded-For or X-Forwarded-Proto: https are missing.
	if req.Header.Get("X-Forwarded-Proto") == "" {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	if req.Header.Get("X-Forwarded-For") == "" {
		if clientIP == "" {
			clientIP = "127.0.0.1"
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &PanelResponse{
		OK:      resp.StatusCode >= 200 && resp.StatusCode < 300,
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    respBody,
	}, nil
}

// GetSubscriptionInfo -> GET /api/sub/{shortUuid}/info
func (c *PanelClient) GetSubscriptionInfo(shortUUID, clientIP string) (*PanelResponse, error) {
	return c.do(http.MethodGet, fmt.Sprintf("/api/sub/%s/info", shortUUID), clientIP, nil, nil)
}

// GetSubscription -> GET /api/sub/{shortUuid}[/clientType], forwarding
// selected request headers upstream (as root.service.ts does for
// getSubscription(clientIp, shortUuid, headers, ...)).
func (c *PanelClient) GetSubscription(shortUUID, clientType, clientIP string, forwardHeaders http.Header) (*PanelResponse, error) {
	path := fmt.Sprintf("/api/sub/%s", shortUUID)
	if clientType != "" {
		path = fmt.Sprintf("/api/sub/%s/%s", shortUUID, clientType)
	}
	return c.do(http.MethodGet, path, clientIP, forwardHeaders, nil)
}

// GetSubpageConfig -> GET /api/subscriptions/subpage-config/{shortUuid}.
// Despite being a GET, GetSubpageConfigByShortUuidCommand.RequestBodySchema
// requires a JSON body of {"requestHeaders": {...}} (the incoming request's
// headers, flattened to one value each) — the panel 400s with a Zod
// "Required" error at the root path if it's omitted.
func (c *PanelClient) GetSubpageConfig(shortUUID, clientIP string, requestHeaders http.Header) (*PanelResponse, error) {
	flat := make(map[string]string, len(requestHeaders))
	for k, vs := range requestHeaders {
		if len(vs) > 0 {
			flat[strings.ToLower(k)] = vs[0]
		}
	}
	body, err := json.Marshal(map[string]any{"requestHeaders": flat})
	if err != nil {
		return nil, err
	}
	return c.do(http.MethodGet, fmt.Sprintf("/api/subscriptions/subpage-config/%s", shortUUID), clientIP, nil, body)
}

// GetSubscriptionPageConfigList -> GET /api/subscription-page-configs
func (c *PanelClient) GetSubscriptionPageConfigList() (*PanelResponse, error) {
	return c.do(http.MethodGet, "/api/subscription-page-configs", "", nil, nil)
}

// GetSubscriptionPageConfigByUUID -> GET /api/subscription-page-configs/{uuid}
func (c *PanelClient) GetSubscriptionPageConfigByUUID(uuid string) (*PanelResponse, error) {
	return c.do(http.MethodGet, fmt.Sprintf("/api/subscription-page-configs/%s", uuid), "", nil, nil)
}

// UnmarshalResponse JSON-decodes resp.Body into v.
func UnmarshalResponse(resp *PanelResponse, v any) error {
	return json.Unmarshal(resp.Body, v)
}

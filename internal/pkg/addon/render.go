package addon

import (
	"log/slog"

	"gopkg.in/yaml.v3"
)

// StaticConfig is a ready-made share link plus the final remark it gets in
// the subscription.
type StaticConfig struct {
	Link string
	Name string
}

// proxies parses Configs, giving each Proxy its final Name. Links that fail to
// parse are skipped (LoadConfig already validated them).
func (a Addition) proxies() []Proxy {
	var out []Proxy
	for _, c := range a.Configs {
		p, err := ParseLink(c.Link)
		if err != nil {
			slog.Warn("add-on link skipped", "err", err)
			continue
		}
		p.Name = c.Name
		out = append(out, p)
	}
	return out
}

// ---------- mihomo ----------

func newMap() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode} }

func put(m *yaml.Node, key string, v any) {
	vn, ok := v.(*yaml.Node)
	if !ok {
		vn = &yaml.Node{}
		_ = vn.Encode(v)
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, vn)
}

func putIf(m *yaml.Node, key, v string) {
	if v != "" {
		put(m, key, v)
	}
}

// Mihomo renders the proxy as a mihomo proxies entry, name first.
func (p Proxy) Mihomo() *yaml.Node {
	q := p.Params
	m := newMap()
	put(m, "name", p.Name)
	put(m, "type", p.Scheme)
	put(m, "server", p.Server)
	put(m, "port", p.Port)
	if p.Scheme == "hysteria2" {
		put(m, "password", p.Secret)
		putIf(m, "sni", q.Get("sni"))
		if q.Get("insecure") == "1" {
			put(m, "skip-cert-verify", true)
		}
		putIf(m, "obfs", q.Get("obfs"))
		putIf(m, "obfs-password", q.Get("obfs-password"))
		if alpn := splitComma(q.Get("alpn")); len(alpn) > 0 {
			put(m, "alpn", alpn)
		}
		putIf(m, "ports", q.Get("mport"))
		return m
	}
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}
	put(m, "uuid", p.Secret)
	put(m, "udp", true)
	put(m, "network", network)
	security := q.Get("security")
	if security == "tls" || security == "reality" {
		put(m, "tls", true)
		putIf(m, "servername", q.Get("sni"))
		putIf(m, "client-fingerprint", q.Get("fp"))
		if q.Get("allowInsecure") == "1" {
			put(m, "skip-cert-verify", true)
		}
	}
	putIf(m, "flow", q.Get("flow"))
	if security == "reality" {
		ro := newMap()
		put(ro, "public-key", q.Get("pbk"))
		putIf(ro, "short-id", q.Get("sid"))
		put(m, "reality-opts", ro)
	}
	switch network {
	case "ws":
		ws := newMap()
		putIf(ws, "path", q.Get("path"))
		if host := q.Get("host"); host != "" {
			h := newMap()
			put(h, "Host", host)
			put(ws, "headers", h)
		}
		put(m, "ws-opts", ws)
	case "grpc":
		g := newMap()
		putIf(g, "grpc-service-name", q.Get("serviceName"))
		put(m, "grpc-opts", g)
	}
	return m
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// ---------- sing-box ----------

func setIf(m map[string]any, key, v string) {
	if v != "" {
		m[key] = v
	}
}

// Singbox renders the proxy as a sing-box outbound tagged p.Name.
func (p Proxy) Singbox() map[string]any {
	q := p.Params
	o := map[string]any{"type": p.Scheme, "tag": p.Name, "server": p.Server, "server_port": p.Port}
	if p.Scheme == "hysteria2" {
		o["password"] = p.Secret
		tls := map[string]any{"enabled": true}
		setIf(tls, "server_name", q.Get("sni"))
		if q.Get("insecure") == "1" {
			tls["insecure"] = true
		}
		if alpn := splitComma(q.Get("alpn")); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		o["tls"] = tls
		if obfs := q.Get("obfs"); obfs != "" {
			o["obfs"] = map[string]any{"type": obfs, "password": q.Get("obfs-password")}
		}
		return o
	}
	o["uuid"] = p.Secret
	setIf(o, "flow", q.Get("flow"))
	security := q.Get("security")
	if security == "tls" || security == "reality" {
		tls := map[string]any{"enabled": true}
		setIf(tls, "server_name", q.Get("sni"))
		if q.Get("allowInsecure") == "1" {
			tls["insecure"] = true
		}
		fp := q.Get("fp")
		if fp == "" && security == "reality" {
			fp = "chrome" // sing-box refuses reality without uTLS
		}
		if fp != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		}
		if security == "reality" {
			r := map[string]any{"enabled": true, "public_key": q.Get("pbk")}
			setIf(r, "short_id", q.Get("sid"))
			tls["reality"] = r
		}
		o["tls"] = tls
	}
	switch q.Get("type") {
	case "ws":
		tr := map[string]any{"type": "ws"}
		setIf(tr, "path", q.Get("path"))
		if host := q.Get("host"); host != "" {
			tr["headers"] = map[string]any{"Host": host}
		}
		o["transport"] = tr
	case "grpc":
		tr := map[string]any{"type": "grpc"}
		setIf(tr, "service_name", q.Get("serviceName"))
		o["transport"] = tr
	}
	return o
}

// ---------- xray-json ----------

// Xray renders the proxy as an xray-json subscription entry named p.Name. It
// reports false for protocols xray-json output does not cover (hysteria2).
func (p Proxy) Xray() (map[string]any, bool) {
	if p.Scheme != "vless" {
		return nil, false
	}
	q := p.Params
	user := map[string]any{"id": p.Secret, "encryption": "none"}
	setIf(user, "flow", q.Get("flow"))
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := q.Get("security")
	if security == "" {
		security = "none"
	}
	ss := map[string]any{"network": network, "security": security}
	switch security {
	case "tls":
		tls := map[string]any{}
		setIf(tls, "serverName", q.Get("sni"))
		setIf(tls, "fingerprint", q.Get("fp"))
		if q.Get("allowInsecure") == "1" {
			tls["allowInsecure"] = true
		}
		ss["tlsSettings"] = tls
	case "reality":
		r := map[string]any{"publicKey": q.Get("pbk")}
		setIf(r, "serverName", q.Get("sni"))
		setIf(r, "fingerprint", q.Get("fp"))
		setIf(r, "shortId", q.Get("sid"))
		ss["realitySettings"] = r
	}
	switch network {
	case "ws":
		ws := map[string]any{}
		setIf(ws, "path", q.Get("path"))
		if host := q.Get("host"); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		ss["wsSettings"] = ws
	case "grpc":
		g := map[string]any{}
		setIf(g, "serviceName", q.Get("serviceName"))
		ss["grpcSettings"] = g
	}
	return map[string]any{
		"remarks": p.Name,
		"outbounds": []any{map[string]any{
			"tag":      "proxy",
			"protocol": "vless",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": p.Server, "port": p.Port, "users": []any{user},
			}}},
			"streamSettings": ss,
		}},
	}, true
}

package addon

import (
	"bytes"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// providerLink is a supported config (vless/hysteria2) found in the usage
// subscription; Name is its display name and Link a share link to it. For
// base64/plain subscriptions Link is the original line, for xray-json,
// sing-box and mihomo it is rebuilt from the config's fields.
type providerLink struct {
	Link, Name string
}

// parseProviderLinks extracts the vless/hysteria2 configs of a subscription
// body in any of the formats Detect knows. Other protocols are ignored.
func parseProviderLinks(body []byte) []providerLink {
	format, err := Detect(body)
	if err != nil {
		return nil
	}
	switch format {
	case FormatLinksBase64, FormatLinksPlain:
		return linksOf(body, format)
	case FormatXrayJSON:
		return fromXray(body)
	case FormatSingbox:
		return fromSingbox(body)
	case FormatMihomo:
		return fromMihomo(body)
	}
	return nil
}

func linksOf(body []byte, format Format) []providerLink {
	lines, err := splitLinks(format, body)
	if err != nil {
		return nil
	}
	var out []providerLink
	for _, l := range lines {
		if p, err := ParseLink(l); err == nil {
			out = append(out, providerLink{Link: l, Name: p.Name})
		}
	}
	return out
}

// ---------- building a link from a Proxy ----------

// Link renders the proxy as a share link; ParseLink reads it back.
func (p Proxy) Link() string {
	u := url.URL{
		Scheme:   p.Scheme,
		Host:     net.JoinHostPort(p.Server, strconv.Itoa(p.Port)),
		RawQuery: p.Params.Encode(),
		Fragment: p.Name,
	}
	if user, pass, ok := strings.Cut(p.Secret, ":"); ok {
		u.User = url.UserPassword(user, pass)
	} else {
		u.User = url.User(p.Secret)
	}
	return u.String()
}

// emit appends p to out, keeping only proxies that survive a Link/ParseLink round trip.
func emit(out []providerLink, p Proxy) []providerLink {
	if p.Server == "" || p.Port == 0 || p.Secret == "" {
		return out
	}
	link := p.Link()
	if _, err := ParseLink(link); err != nil {
		return out
	}
	return append(out, providerLink{Link: link, Name: p.Name})
}

// ---------- tiny dynamic-value helpers ----------

type obj = map[string]any

func asObj(v any) obj { m, _ := v.(map[string]any); return m }

func asList(v any) []any { l, _ := v.([]any); return l }

func str(m obj, k string) string { s, _ := m[k].(string); return s }

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	}
	return false
}

func intOf(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case uint64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

func setQ(q url.Values, k, v string) {
	if v != "" {
		q.Set(k, v)
	}
}

func joinStrings(v any) string {
	var parts []string
	for _, x := range asList(v) {
		if s, ok := x.(string); ok {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ",")
}

// ---------- xray-json ----------

func fromXray(body []byte) []providerLink {
	body = bytes.TrimSpace(body)
	var entries []obj
	if body[0] == '[' {
		if json.Unmarshal(body, &entries) != nil {
			return nil
		}
	} else {
		var e obj
		if json.Unmarshal(body, &e) != nil {
			return nil
		}
		entries = []obj{e}
	}
	var out []providerLink
	for _, e := range entries {
		remarks := str(e, "remarks")
		outbounds := asList(e["outbounds"])
		// Outbounds chained through another one (sockopt.dialerProxy), and the
		// ones used as such a hop, can't be expressed as a standalone link.
		hops := map[string]bool{}
		for _, ob := range outbounds {
			if d := dialerProxy(asObj(ob)); d != "" {
				hops[d] = true
			}
		}
		for _, ob := range outbounds {
			o := asObj(ob)
			tag := str(o, "tag")
			if dialerProxy(o) != "" || hops[tag] {
				continue
			}
			var p Proxy
			var ok bool
			switch str(o, "protocol") {
			case "vless":
				p, ok = xrayVless(o)
			case "hysteria":
				p, ok = xrayHysteria(o)
			}
			if !ok {
				continue
			}
			// Name it like base64/mihomo subscriptions do, so one `match`
			// works for any format: "<remarks> · Hysteria 2 · Salamander ·
			// <tag>". Per-server entries are only distinguishable by tag; a
			// lone "proxy" outbound is just the entry itself.
			parts := []string{remarks}
			if p.Scheme == "hysteria2" {
				parts = append(parts, "Hysteria 2")
				if p.Params.Get("obfs") == "salamander" {
					parts = append(parts, "Salamander")
				}
			}
			if tag != "" && tag != "proxy" {
				parts = append(parts, tag)
			}
			p.Name = strings.Join(parts, " · ")
			out = emit(out, p)
		}
	}
	return out
}

func dialerProxy(o obj) string {
	return str(asObj(asObj(o["streamSettings"])["sockopt"]), "dialerProxy")
}

// xrayHysteria reads a hysteria v2 outbound (xray-core 26+): auth in
// hysteriaSettings, TLS in tlsSettings, Salamander in finalmask.udp.
func xrayHysteria(o obj) (Proxy, bool) {
	s := asObj(o["settings"])
	ss := asObj(o["streamSettings"])
	hs := asObj(ss["hysteriaSettings"])
	if intOf(s["version"]) != 2 && intOf(hs["version"]) != 2 {
		return Proxy{}, false
	}
	p := Proxy{Scheme: "hysteria2", Server: str(s, "address"), Port: intOf(s["port"]), Secret: str(hs, "auth"), Params: url.Values{}}
	t := asObj(ss["tlsSettings"])
	setQ(p.Params, "sni", str(t, "serverName"))
	if truthy(t["allowInsecure"]) {
		p.Params.Set("insecure", "1")
	}
	setQ(p.Params, "alpn", joinStrings(t["alpn"]))
	for _, m := range asList(asObj(ss["finalmask"])["udp"]) {
		if m := asObj(m); str(m, "type") == "salamander" {
			p.Params.Set("obfs", "salamander")
			setQ(p.Params, "obfs-password", str(asObj(m["settings"]), "password"))
		}
	}
	return p, true
}

func xrayVless(o obj) (Proxy, bool) {
	s := asObj(o["settings"])
	p := Proxy{Scheme: "vless", Params: url.Values{}}
	user := s
	if vnext := asList(s["vnext"]); len(vnext) > 0 {
		v := asObj(vnext[0])
		user = nil
		if us := asList(v["users"]); len(us) > 0 {
			user = asObj(us[0])
		}
		p.Server, p.Port = str(v, "address"), intOf(v["port"])
	} else {
		p.Server, p.Port = str(s, "address"), intOf(s["port"])
	}
	p.Secret = str(user, "id")
	setQ(p.Params, "flow", str(user, "flow"))

	ss := asObj(o["streamSettings"])
	network := str(ss, "network")
	if network == "" {
		network = "tcp"
	}
	p.Params.Set("type", network)
	security := str(ss, "security")
	switch security {
	case "tls":
		t := asObj(ss["tlsSettings"])
		p.Params.Set("security", "tls")
		setQ(p.Params, "sni", str(t, "serverName"))
		setQ(p.Params, "fp", str(t, "fingerprint"))
		if truthy(t["allowInsecure"]) {
			p.Params.Set("allowInsecure", "1")
		}
	case "reality":
		r := asObj(ss["realitySettings"])
		p.Params.Set("security", "reality")
		setQ(p.Params, "sni", str(r, "serverName"))
		setQ(p.Params, "fp", str(r, "fingerprint"))
		setQ(p.Params, "pbk", str(r, "publicKey"))
		setQ(p.Params, "sid", str(r, "shortId"))
	}
	switch network {
	case "ws":
		w := asObj(ss["wsSettings"])
		setQ(p.Params, "path", str(w, "path"))
		host := str(asObj(w["headers"]), "Host")
		if host == "" {
			host = str(w, "host")
		}
		setQ(p.Params, "host", host)
	case "grpc":
		setQ(p.Params, "serviceName", str(asObj(ss["grpcSettings"]), "serviceName"))
	}
	return p, true
}

// ---------- sing-box ----------

func fromSingbox(body []byte) []providerLink {
	var doc obj
	if json.Unmarshal(bytes.TrimSpace(body), &doc) != nil {
		return nil
	}
	var out []providerLink
	for _, ob := range asList(doc["outbounds"]) {
		o := asObj(ob)
		p := Proxy{Name: str(o, "tag"), Server: str(o, "server"), Port: intOf(o["server_port"]), Params: url.Values{}}
		tls := asObj(o["tls"])
		switch str(o, "type") {
		case "hysteria2":
			p.Scheme, p.Secret = "hysteria2", str(o, "password")
			setQ(p.Params, "sni", str(tls, "server_name"))
			if truthy(tls["insecure"]) {
				p.Params.Set("insecure", "1")
			}
			setQ(p.Params, "alpn", joinStrings(tls["alpn"]))
			if ob := asObj(o["obfs"]); ob != nil {
				setQ(p.Params, "obfs", str(ob, "type"))
				setQ(p.Params, "obfs-password", str(ob, "password"))
			}
		case "vless":
			p.Scheme, p.Secret = "vless", str(o, "uuid")
			setQ(p.Params, "flow", str(o, "flow"))
			network := "tcp"
			tr := asObj(o["transport"])
			if t := str(tr, "type"); t != "" {
				network = t
			}
			p.Params.Set("type", network)
			if truthy(tls["enabled"]) {
				reality := asObj(tls["reality"])
				if truthy(reality["enabled"]) {
					p.Params.Set("security", "reality")
					setQ(p.Params, "pbk", str(reality, "public_key"))
					setQ(p.Params, "sid", str(reality, "short_id"))
				} else {
					p.Params.Set("security", "tls")
				}
				setQ(p.Params, "sni", str(tls, "server_name"))
				setQ(p.Params, "fp", str(asObj(tls["utls"]), "fingerprint"))
				if truthy(tls["insecure"]) {
					p.Params.Set("allowInsecure", "1")
				}
			}
			switch network {
			case "ws":
				setQ(p.Params, "path", str(tr, "path"))
				setQ(p.Params, "host", str(asObj(tr["headers"]), "Host"))
			case "grpc":
				setQ(p.Params, "serviceName", str(tr, "service_name"))
			}
		default:
			continue
		}
		out = emit(out, p)
	}
	return out
}

// ---------- mihomo ----------

func fromMihomo(body []byte) []providerLink {
	var doc struct {
		Proxies []obj `yaml:"proxies"`
	}
	if yaml.Unmarshal(body, &doc) != nil {
		return nil
	}
	var out []providerLink
	for _, m := range doc.Proxies {
		p := Proxy{Name: str(m, "name"), Server: str(m, "server"), Port: intOf(m["port"]), Params: url.Values{}}
		switch str(m, "type") {
		case "hysteria2", "hy2":
			p.Scheme, p.Secret = "hysteria2", str(m, "password")
			setQ(p.Params, "sni", str(m, "sni"))
			if truthy(m["skip-cert-verify"]) {
				p.Params.Set("insecure", "1")
			}
			setQ(p.Params, "obfs", str(m, "obfs"))
			setQ(p.Params, "obfs-password", str(m, "obfs-password"))
			setQ(p.Params, "alpn", joinStrings(m["alpn"]))
			if ports := m["ports"]; ports != nil {
				setQ(p.Params, "mport", strings.TrimSpace(toString(ports)))
			}
		case "vless":
			p.Scheme, p.Secret = "vless", str(m, "uuid")
			setQ(p.Params, "flow", str(m, "flow"))
			network := str(m, "network")
			if network == "" {
				network = "tcp"
			}
			p.Params.Set("type", network)
			reality := asObj(m["reality-opts"])
			switch {
			case reality != nil:
				p.Params.Set("security", "reality")
				setQ(p.Params, "pbk", str(reality, "public-key"))
				setQ(p.Params, "sid", str(reality, "short-id"))
			case truthy(m["tls"]):
				p.Params.Set("security", "tls")
			}
			if p.Params.Get("security") != "" {
				sni := str(m, "servername")
				if sni == "" {
					sni = str(m, "sni")
				}
				setQ(p.Params, "sni", sni)
				setQ(p.Params, "fp", str(m, "client-fingerprint"))
				if truthy(m["skip-cert-verify"]) {
					p.Params.Set("allowInsecure", "1")
				}
			}
			switch network {
			case "ws":
				ws := asObj(m["ws-opts"])
				setQ(p.Params, "path", str(ws, "path"))
				setQ(p.Params, "host", str(asObj(ws["headers"]), "Host"))
			case "grpc":
				setQ(p.Params, "serviceName", str(asObj(m["grpc-opts"]), "grpc-service-name"))
			}
		default:
			continue
		}
		out = emit(out, p)
	}
	return out
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	}
	return ""
}

package addon

import (
	"fmt"
	"net/url"
	"strconv"
)

// Proxy is a parsed vless:// or hysteria2:// share link.
type Proxy struct {
	Scheme string // "vless" or "hysteria2"
	Name   string // decoded #fragment, later replaced by the final remark
	Server string
	Port   int
	Secret string // uuid (vless) or password (hysteria2)
	Params url.Values
}

// ParseLink parses a vless://, hysteria2:// or hy2:// share link.
func ParseLink(link string) (Proxy, error) {
	u, err := url.Parse(link)
	if err != nil {
		return Proxy{}, fmt.Errorf("invalid link: %w", err)
	}
	p := Proxy{Scheme: u.Scheme, Name: u.Fragment, Server: u.Hostname(), Params: u.Query()}
	switch u.Scheme {
	case "vless", "hysteria2":
	case "hy2":
		p.Scheme = "hysteria2"
	default:
		return Proxy{}, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return Proxy{}, fmt.Errorf("invalid port %q", u.Port())
	}
	p.Port = port
	if p.Server == "" || u.User == nil {
		return Proxy{}, fmt.Errorf("link needs user info and host")
	}
	p.Secret = u.User.Username()
	if pw, ok := u.User.Password(); ok {
		p.Secret += ":" + pw
	}
	return p, nil
}

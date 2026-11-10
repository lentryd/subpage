package addon

import "testing"

func TestProxyLinkRoundTrip(t *testing.T) {
	for _, in := range []string{
		"vless://id@h.example.com:443?flow=xtls-rprx-vision&fp=chrome&pbk=PK&security=reality&sid=ab&sni=s.example.com&type=tcp#Name%20%F0%9F%87%B8",
		"hysteria2://pw@h.example.com:443?obfs=salamander&obfs-password=x&sni=s.example.com#N",
		"hysteria2://user:pw@[2001:db8::1]:8443#N",
	} {
		p, err := ParseLink(in)
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseLink(p.Link())
		if err != nil || back.Name != p.Name || back.Server != p.Server || back.Port != p.Port || back.Secret != p.Secret || back.Params.Encode() != p.Params.Encode() {
			t.Errorf("%s -> %s: %+v %v", in, p.Link(), back, err)
		}
	}
}

// wantLink checks that a config read from a structured format yields the same
// link a user would have pasted.
func wantLink(t *testing.T, got []providerLink, name, link string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("no configs, want %q", name)
	}
	for _, g := range got {
		if g.Name != name {
			continue
		}
		a, err1 := ParseLink(g.Link)
		b, err2 := ParseLink(link)
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		if a.Scheme != b.Scheme || a.Server != b.Server || a.Port != b.Port || a.Secret != b.Secret || a.Params.Encode() != b.Params.Encode() {
			t.Fatalf("%q:\n got  %s\n want %s", name, g.Link, link)
		}
		return
	}
	t.Fatalf("%q not found in %+v", name, got)
}

const (
	pVless = "vless://11111111-1111-1111-1111-111111111111@de.example.com:443?flow=xtls-rprx-vision&fp=chrome&pbk=PUB&security=reality&sid=ab12&sni=de.example.com&type=tcp#DE"
	pWS    = "vless://11111111-1111-1111-1111-111111111111@ws.example.com:443?fp=chrome&host=cdn.example.com&path=%2Fws&security=tls&sni=ws.example.com&type=ws#WS"
	pHy2   = "hysteria2://pw@fi.example.com:443?insecure=1&obfs=salamander&obfs-password=secret&sni=fi.example.com#FI"
)

func TestParseProviderXray(t *testing.T) {
	body := `[{"remarks":"DE","outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"de.example.com","port":443,"users":[{"id":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision","encryption":"none"}]}]},
	"streamSettings":{"network":"tcp","security":"reality","realitySettings":{"publicKey":"PUB","serverName":"de.example.com","fingerprint":"chrome","shortId":"ab12"}}}]},
	{"remarks":"WS","outbounds":[{"protocol":"vless","settings":{"address":"ws.example.com","port":443,"id":"11111111-1111-1111-1111-111111111111"},
	"streamSettings":{"network":"ws","security":"tls","tlsSettings":{"serverName":"ws.example.com","fingerprint":"chrome"},"wsSettings":{"path":"/ws","headers":{"Host":"cdn.example.com"}}}}]},
	{"remarks":"skip","outbounds":[{"protocol":"freedom"}]}]`
	got := parseProviderLinks([]byte(body))
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	wantLink(t, got, "DE", pVless)
	wantLink(t, got, "WS", pWS)
}

func TestParseProviderSingbox(t *testing.T) {
	body := `{"outbounds":[
	{"type":"vless","tag":"DE","server":"de.example.com","server_port":443,"uuid":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision",
	 "tls":{"enabled":true,"server_name":"de.example.com","utls":{"enabled":true,"fingerprint":"chrome"},"reality":{"enabled":true,"public_key":"PUB","short_id":"ab12"}}},
	{"type":"hysteria2","tag":"FI","server":"fi.example.com","server_port":443,"password":"pw","obfs":{"type":"salamander","password":"secret"},"tls":{"enabled":true,"server_name":"fi.example.com","insecure":true}},
	{"type":"direct","tag":"direct"}]}`
	got := parseProviderLinks([]byte(body))
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	wantLink(t, got, "DE", pVless)
	wantLink(t, got, "FI", pHy2)
}

func TestParseProviderMihomo(t *testing.T) {
	body := `proxies:
  - name: DE
    type: vless
    server: de.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: tcp
    tls: true
    servername: de.example.com
    client-fingerprint: chrome
    flow: xtls-rprx-vision
    reality-opts: {public-key: PUB, short-id: ab12}
  - name: WS
    type: vless
    server: ws.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: ws
    tls: true
    servername: ws.example.com
    client-fingerprint: chrome
    ws-opts: {path: /ws, headers: {Host: cdn.example.com}}
  - name: FI
    type: hysteria2
    server: fi.example.com
    port: 443
    password: pw
    sni: fi.example.com
    skip-cert-verify: true
    obfs: salamander
    obfs-password: secret
  - name: ss
    type: ss
    server: x
    port: 1
`
	got := parseProviderLinks([]byte(body))
	if len(got) != 3 {
		t.Fatalf("%+v", got)
	}
	wantLink(t, got, "DE", pVless)
	wantLink(t, got, "WS", pWS)
	wantLink(t, got, "FI", pHy2)
}

func TestParseProviderXrayHysteriaAndChains(t *testing.T) {
	body := `[{"remarks":"SE","outbounds":[
	{"tag":"hop-1","protocol":"hysteria","settings":{"address":"hop.example.com","port":8443,"version":2},
	 "streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"auth":"hopauth","version":2},"tlsSettings":{"serverName":"hop.example.com"}}},
	{"tag":"chain-1","protocol":"vless","settings":{"vnext":[{"address":"se.example.com","port":2087,"users":[{"id":"11111111-1111-1111-1111-111111111111"}]}]},
	 "streamSettings":{"network":"grpc","security":"reality","realitySettings":{"publicKey":"PUB"},"sockopt":{"dialerProxy":"hop-1"}}},
	{"tag":"SE-5-b87bc4","protocol":"hysteria","settings":{"address":"fi.example.com","port":443,"version":2},
	 "streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"auth":"pw","version":2},"tlsSettings":{"serverName":"fi.example.com","allowInsecure":true},
	  "finalmask":{"udp":[{"type":"salamander","settings":{"password":"secret"}}]}}},
	{"tag":"direct","protocol":"freedom"}]}]`
	got := parseProviderLinks([]byte(body))
	if len(got) != 1 {
		t.Fatalf("want only the standalone hysteria config, got %+v", got)
	}
	wantLink(t, got, "SE", pHy2)
}

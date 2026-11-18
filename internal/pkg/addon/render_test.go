package addon

import (
	"encoding/json"
	"reflect"
	"testing"
)

// jsonOf round-trips v so numbers and slices have the types encoding/json gives.
func jsonOf(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func xrayOutbound(t *testing.T, link, tag string) map[string]any {
	t.Helper()
	p, err := ParseLink(link)
	if err != nil {
		t.Fatal(err)
	}
	ob, ok := p.XrayOutbound(tag)
	if !ok {
		t.Fatalf("%s: not rendered", link)
	}
	return jsonOf(t, ob)
}

func TestXrayOutboundHysteria2(t *testing.T) {
	got := xrayOutbound(t, "hysteria2://pw@h.example:8443?sni=hy.example#x", "proxy-addon-1")
	want := jsonOf(t, map[string]any{
		"tag": "proxy-addon-1", "protocol": "hysteria",
		"settings": map[string]any{"address": "h.example", "port": 8443, "version": 2},
		"streamSettings": map[string]any{
			"network": "hysteria", "security": "tls",
			"hysteriaSettings": map[string]any{"auth": "pw", "version": 2},
			"tlsSettings": map[string]any{
				"alpn": []any{"h3"}, "enableSessionResumption": false, "serverName": "hy.example",
			},
			"finalmask": map[string]any{"quicParams": map[string]any{"congestion": "bbr", "debug": false}},
		},
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestXrayOutboundHysteria2Salamander(t *testing.T) {
	got := xrayOutbound(t, "hysteria2://pw@h.example:8443?obfs=salamander&obfs-password=op#x", "proxy")
	fm := got["streamSettings"].(map[string]any)["finalmask"].(map[string]any)
	want := []any{map[string]any{"type": "salamander", "settings": map[string]any{"password": "op"}}}
	if !reflect.DeepEqual(fm["udp"], want) {
		t.Fatalf("%v", fm)
	}
	tls := got["streamSettings"].(map[string]any)["tlsSettings"].(map[string]any)
	if _, ok := tls["serverName"]; ok {
		t.Fatalf("serverName must be omitted without sni: %v", tls)
	}
}

func TestXrayOutboundHysteria2InsecureAlpn(t *testing.T) {
	got := xrayOutbound(t, "hysteria2://pw@h.example:8443?insecure=1&alpn=h3,h2#x", "proxy")
	tls := got["streamSettings"].(map[string]any)["tlsSettings"].(map[string]any)
	if tls["allowInsecure"] != true || !reflect.DeepEqual(tls["alpn"], []any{"h3", "h2"}) {
		t.Fatalf("%v", tls)
	}
}

func TestXrayOutboundVlessKeepsShape(t *testing.T) {
	got := xrayOutbound(t, staticVless, "proxy-addon-2")
	if got["tag"] != "proxy-addon-2" || got["protocol"] != "vless" {
		t.Fatalf("%v", got)
	}
	p, _ := ParseLink(staticVless)
	cfg, ok := p.Xray()
	if !ok {
		t.Fatal("vless must still render a config")
	}
	ob := jsonOf(t, cfg)["outbounds"].([]any)[0].(map[string]any)
	ob["tag"] = "proxy-addon-2"
	if !reflect.DeepEqual(ob, got) {
		t.Fatalf("Xray() and XrayOutbound differ:\n%v\n%v", ob, got)
	}
}

func TestXrayHysteria2Config(t *testing.T) {
	p, _ := ParseLink("hysteria2://pw@h.example:8443#x")
	p.Name = "Named"
	cfg, ok := p.Xray()
	if !ok || cfg["remarks"] != "Named" {
		t.Fatalf("%v %v", cfg, ok)
	}
	if tag := jsonOf(t, cfg)["outbounds"].([]any)[0].(map[string]any)["tag"]; tag != "proxy" {
		t.Fatalf("tag %v", tag)
	}
}

func TestXrayOutboundUnsupportedScheme(t *testing.T) {
	if _, ok := (Proxy{Scheme: "ss"}).XrayOutbound("proxy"); ok {
		t.Fatal("unsupported scheme must report false")
	}
}

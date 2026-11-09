package addon

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	staticVless = "vless://11111111-1111-1111-1111-111111111111@v.example:443?type=ws&path=%2Fws&host=cdn.example&security=reality&sni=sni.example&fp=chrome&pbk=PUBKEY&sid=ab12&flow=xtls-rprx-vision#orig"
	staticHy2   = "hysteria2://pw@h.example:8443?sni=hy.example&insecure=1&obfs=salamander&obfs-password=op&alpn=h3#orig2"
)

func staticAdd(names ...string) Addition {
	links := []string{staticVless, staticHy2}
	var cfgs []StaticConfig
	for i, n := range names {
		cfgs = append(cfgs, StaticConfig{Link: links[i%2], Name: n})
	}
	return Addition{Configs: cfgs}
}

func TestMergeStaticLinksBase64(t *testing.T) {
	main := base64.StdEncoding.EncodeToString([]byte("vless://u@m:1#DE"))
	out, n, err := Merge([]byte(main), staticAdd("⭐ Extra 7", "hy2 7"))
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	dec, _ := base64.StdEncoding.DecodeString(string(out))
	lines := strings.Split(string(dec), "\n")
	if len(lines) != 3 || lines[0] != "vless://u@m:1#DE" {
		t.Fatalf("%q", lines)
	}
	for i, want := range []string{"⭐ Extra 7", "hy2 7"} {
		l := lines[i+1]
		frag, _ := url.PathUnescape(l[strings.Index(l, "#")+1:])
		if frag != want {
			t.Errorf("fragment %q want %q", frag, want)
		}
	}
	if !strings.Contains(lines[1], "pbk=PUBKEY") || !strings.HasPrefix(lines[2], "hysteria2://pw@h.example:8443") {
		t.Fatalf("link params lost: %q", lines)
	}
}

const mihomoMain = "proxies:\n  - name: DE\n    type: vless\n    server: m\n    port: 443\nproxy-groups:\n  - name: Proxy\n    type: select\n    proxies: [DE]\n  - name: Final\n    type: select\n    proxies: [Proxy, DIRECT]\n"

func TestMergeStaticMihomo(t *testing.T) {
	out, n, err := Merge([]byte(mihomoMain), staticAdd("⭐ Extra", "hy2"))
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	if !strings.Contains(string(out), "⭐ Extra") {
		t.Fatalf("emoji escaped: %s", out)
	}
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
		Groups  []struct {
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Proxies) != 3 {
		t.Fatalf("%s", out)
	}
	v, h := cfg.Proxies[1], cfg.Proxies[2]
	for k, want := range map[string]any{
		"name": "⭐ Extra", "type": "vless", "server": "v.example", "port": 443,
		"uuid": "11111111-1111-1111-1111-111111111111", "network": "ws", "tls": true,
		"flow": "xtls-rprx-vision", "servername": "sni.example", "client-fingerprint": "chrome",
	} {
		if v[k] != want {
			t.Errorf("vless %s = %v want %v", k, v[k], want)
		}
	}
	ro, _ := v["reality-opts"].(map[string]any)
	wo, _ := v["ws-opts"].(map[string]any)
	if ro["public-key"] != "PUBKEY" || ro["short-id"] != "ab12" || wo["path"] != "/ws" {
		t.Errorf("reality/ws opts: %v %v", ro, wo)
	}
	for k, want := range map[string]any{
		"name": "hy2", "type": "hysteria2", "server": "h.example", "port": 8443, "password": "pw",
		"sni": "hy.example", "skip-cert-verify": true, "obfs": "salamander", "obfs-password": "op",
	} {
		if h[k] != want {
			t.Errorf("hysteria2 %s = %v want %v", k, h[k], want)
		}
	}
	if g := cfg.Groups[0].Proxies; len(g) != 3 || g[1] != "⭐ Extra" || g[2] != "hy2" {
		t.Fatalf("group not updated: %v", g)
	}
	if len(cfg.Groups[1].Proxies) != 2 {
		t.Fatalf("group without servers touched: %v", cfg.Groups[1].Proxies)
	}
}

func TestMergeStaticDuplicateNames(t *testing.T) {
	out, _, err := Merge([]byte(mihomoMain), staticAdd("same", "same"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Proxies) != 3 || cfg.Proxies[1]["name"] != "same" || cfg.Proxies[2]["name"] != "same #2" {
		t.Fatalf("%s", out)
	}
}

func TestMergeStaticNoFragment(t *testing.T) {
	add := Addition{Configs: []StaticConfig{{Link: "vless://id@h:443?type=tcp", Name: ""}}}
	if _, n, err := Merge([]byte(mihomoMain), add); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestMergeStaticSingbox(t *testing.T) {
	main := `{"outbounds":[{"type":"selector","tag":"final","outbounds":["proxy","direct"]},{"type":"selector","tag":"proxy","outbounds":["DE"]},{"type":"urltest","tag":"auto","outbounds":["DE"]},{"type":"vless","tag":"DE"},{"type":"direct","tag":"direct"}]}`
	out, n, err := Merge([]byte(main), staticAdd("Extra", "hy2"))
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Outbounds) != 7 {
		t.Fatalf("%s", out)
	}
	v, h := cfg.Outbounds[5], cfg.Outbounds[6]
	for k, want := range map[string]any{"type": "vless", "tag": "Extra", "server": "v.example", "server_port": float64(443),
		"uuid": "11111111-1111-1111-1111-111111111111", "flow": "xtls-rprx-vision"} {
		if v[k] != want {
			t.Errorf("vless %s = %v want %v", k, v[k], want)
		}
	}
	tls, _ := v["tls"].(map[string]any)
	reality, _ := tls["reality"].(map[string]any)
	utls, _ := tls["utls"].(map[string]any)
	tr, _ := v["transport"].(map[string]any)
	if tls["enabled"] != true || tls["server_name"] != "sni.example" || reality["public_key"] != "PUBKEY" ||
		reality["short_id"] != "ab12" || reality["enabled"] != true || utls["fingerprint"] != "chrome" ||
		tr["type"] != "ws" || tr["path"] != "/ws" {
		t.Errorf("vless tls/transport: %v", v)
	}
	htls, _ := h["tls"].(map[string]any)
	obfs, _ := h["obfs"].(map[string]any)
	if h["type"] != "hysteria2" || h["tag"] != "hy2" || h["password"] != "pw" || h["server_port"] != float64(8443) ||
		htls["server_name"] != "hy.example" || htls["insecure"] != true || obfs["type"] != "salamander" || obfs["password"] != "op" {
		t.Errorf("hysteria2: %v", h)
	}
	list := func(i int) []any { l, _ := cfg.Outbounds[i]["outbounds"].([]any); return l }
	if l := list(1); len(l) != 3 || l[1] != "Extra" || l[2] != "hy2" {
		t.Errorf("selector: %v", l)
	}
	if l := list(2); len(l) != 3 {
		t.Errorf("urltest: %v", l)
	}
	if l := list(0); len(l) != 2 {
		t.Errorf("group without servers touched: %v", l)
	}
}

func TestMergeStaticXray(t *testing.T) {
	main := `[{"remarks":"DE","outbounds":[]}]`
	out, n, err := Merge([]byte(main), staticAdd("Extra", "hy2"))
	if err != nil || n != 1 {
		t.Fatalf("hysteria2 must be skipped for xray: n=%d err=%v", n, err)
	}
	var got []struct {
		Remarks   string           `json:"remarks"`
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Remarks != "Extra" || len(got[1].Outbounds) != 1 {
		t.Fatalf("%s", out)
	}
	ob := got[1].Outbounds[0]
	if ob["protocol"] != "vless" || ob["tag"] != "proxy" {
		t.Fatalf("%v", ob)
	}
	vnext := ob["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)
	user := vnext["users"].([]any)[0].(map[string]any)
	ss := ob["streamSettings"].(map[string]any)
	rs := ss["realitySettings"].(map[string]any)
	ws := ss["wsSettings"].(map[string]any)
	if vnext["address"] != "v.example" || vnext["port"] != float64(443) || user["id"] != "11111111-1111-1111-1111-111111111111" ||
		user["flow"] != "xtls-rprx-vision" || user["encryption"] != "none" || ss["network"] != "ws" || ss["security"] != "reality" ||
		rs["serverName"] != "sni.example" || rs["publicKey"] != "PUBKEY" || rs["shortId"] != "ab12" || rs["fingerprint"] != "chrome" ||
		ws["path"] != "/ws" {
		t.Fatalf("%s", out)
	}
}

func TestSingboxRealityWithoutFingerprintEnablesUTLS(t *testing.T) {
	p, err := ParseLink("vless://id@h:443?security=reality&pbk=K&sni=s#n")
	if err != nil {
		t.Fatal(err)
	}
	tls, _ := p.Singbox()["tls"].(map[string]any)
	utls, _ := tls["utls"].(map[string]any)
	if utls["enabled"] != true || utls["fingerprint"] != "chrome" {
		t.Fatalf("sing-box reality requires uTLS: %v", tls)
	}
}

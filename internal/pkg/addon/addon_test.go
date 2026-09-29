package addon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"subpage/internal/pkg/subpage"
)

const gib = 1 << 30

func testConfig() Config {
	return Config{Timeout: 2 * time.Second, Addons: []Addon{{
		Name:            "white",
		Prefix:          "white_",
		Remark:          "⚪ White · {remark} · осталось {remaining} из {limit} GB",
		RemarkUnlimited: "⚪ White · {remark} · безлимит",
		Stubs: map[string]string{
			"LIMITED": "⚪ White: лимит исчерпан — докупите в боте",
			"EXPIRED": "⚪ White: срок истёк — продлите в боте",
		},
	}}}
}

// fakePanel serves main user "alice" (short "main") and optionally
// white_alice (short "wht") with the given status/traffic.
func fakePanel(t *testing.T, whiteStatus string, whiteCode int, limit, used float64) *httptest.Server {
	t.Helper()
	user := func(short, name, status string) []byte {
		b, _ := json.Marshal(map[string]any{"response": map[string]any{
			"shortUuid": short, "username": name, "status": status,
			"trafficLimitBytes": limit, "userTraffic": map[string]any{"usedTrafficBytes": used},
		}})
		return b
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/by-short-uuid/main":
			_, _ = w.Write(user("main", "alice", StatusActive))
		case "/api/users/by-username/white_alice":
			if whiteCode != 0 {
				w.WriteHeader(whiteCode)
				return
			}
			_, _ = w.Write(user("wht", "white_alice", whiteStatus))
		case "/api/sub/wht":
			_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte("vless://u@w.example:443?security=reality#NL%20White"))))
		default:
			http.NotFound(w, r)
		}
	}))
}

func resolve(t *testing.T, srv *httptest.Server) Result {
	t.Helper()
	s := NewService(subpage.NewPanelClient(srv.URL, "tok"), testConfig())
	username, results, err := s.Resolve(context.Background(), "main", "", "1.2.3.4", nil)
	if err != nil || username != "alice" || len(results) != 1 {
		t.Fatalf("resolve: %q %v %v", username, results, err)
	}
	return results[0]
}

const mainB64Plain = "vless://u@m.example:443#DE%20Main"

func mergeB64(t *testing.T, res Result) []string {
	t.Helper()
	main := []byte(base64.StdEncoding.EncodeToString([]byte(mainB64Plain)))
	if res.Addition == nil {
		return []string{mainB64Plain}
	}
	out, _, err := Merge(main, *res.Addition)
	if err != nil {
		t.Fatal(err)
	}
	dec, _ := base64.StdEncoding.DecodeString(string(out))
	return strings.Split(string(dec), "\n")
}

func fragment(link string) string {
	_, f, _ := strings.Cut(link, "#")
	s, _ := url.PathUnescape(f)
	return s
}

func TestResolveNotFound(t *testing.T) {
	srv := fakePanel(t, "", http.StatusNotFound, 0, 0)
	defer srv.Close()
	res := resolve(t, srv)
	if res.Found || res.Addition != nil || res.Err != nil {
		t.Fatalf("unexpected %+v", res)
	}
}

func TestResolveActiveLimited10GB(t *testing.T) {
	srv := fakePanel(t, StatusActive, 0, 10*gib, 3.4*gib)
	defer srv.Close()
	links := mergeB64(t, resolve(t, srv))
	if len(links) != 2 || links[0] != mainB64Plain {
		t.Fatalf("links %v", links)
	}
	if got, want := fragment(links[1]), "⚪ White · NL White · осталось 6.6 из 10 GB"; got != want {
		t.Fatalf("remark %q, want %q", got, want)
	}
}

func TestResolveActiveUnlimited(t *testing.T) {
	srv := fakePanel(t, StatusActive, 0, 0, 5*gib)
	defer srv.Close()
	links := mergeB64(t, resolve(t, srv))
	if got := fragment(links[1]); got != "⚪ White · NL White · безлимит" {
		t.Fatalf("remark %q", got)
	}
}

func TestResolveStubs(t *testing.T) {
	for status, want := range map[string]string{
		"LIMITED": "⚪ White: лимит исчерпан — докупите в боте",
		"EXPIRED": "⚪ White: срок истёк — продлите в боте",
	} {
		srv := fakePanel(t, status, 0, 10*gib, 10*gib)
		links := mergeB64(t, resolve(t, srv))
		srv.Close()
		if len(links) != 2 || fragment(links[1]) != want || !strings.Contains(links[1], "@0.0.0.0:1") {
			t.Fatalf("%s: links %v", status, links)
		}
	}
}

func TestResolveDisabled(t *testing.T) {
	srv := fakePanel(t, "DISABLED", 0, 10*gib, 0)
	defer srv.Close()
	res := resolve(t, srv)
	if !res.Found || res.Addition != nil {
		t.Fatalf("unexpected %+v", res)
	}
}

func TestResolvePanelError(t *testing.T) {
	srv := fakePanel(t, "", http.StatusInternalServerError, 0, 0)
	defer srv.Close()
	res := resolve(t, srv)
	if res.Err == nil || res.Addition != nil {
		t.Fatalf("unexpected %+v", res)
	}
}

func rename(r string) string { return "W " + r }

func TestMergeXray(t *testing.T) {
	main := `[{"remarks":"DE","outbounds":[{"protocol":"vless","port":443}]}]`
	white := `[{"remarks":"NL","outbounds":[{"protocol":"vless","port":8443}]}]`
	out, n, err := Merge([]byte(main), Addition{Body: []byte(white), Rename: rename})
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1]["remarks"] != "W NL" {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(string(out), "8443") {
		t.Fatalf("numbers lost: %s", out)
	}
}

func TestMergeSingbox(t *testing.T) {
	main := `{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["DE","auto"]},{"type":"urltest","tag":"auto","outbounds":["DE"]},{"type":"vless","tag":"DE"},{"type":"direct","tag":"direct"}]}`
	white := `{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["NL"]},{"type":"vless","tag":"NL","server":"w"},{"type":"direct","tag":"direct"}]}`
	out, n, err := Merge([]byte(main), Addition{Body: []byte(white), Rename: rename})
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	var cfg struct {
		Outbounds []struct {
			Type      string   `json:"type"`
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Outbounds) != 5 || cfg.Outbounds[4].Tag != "W NL" {
		t.Fatalf("%s", out)
	}
	if sel := cfg.Outbounds[0].Outbounds; sel[len(sel)-1] != "W NL" {
		t.Fatalf("selector not updated: %v", sel)
	}
	if len(cfg.Outbounds[1].Outbounds) != 1 {
		t.Fatalf("urltest must stay untouched: %v", cfg.Outbounds[1].Outbounds)
	}
}

func TestMergeMihomo(t *testing.T) {
	main := "mixed-port: 7890\nproxies:\n  - name: DE\n    type: vless\n    server: m\n    port: 443\nproxy-groups:\n  - name: Proxy\n    type: select\n    proxies: [DE]\n  - name: Auto\n    type: url-test\n    proxies: [DE]\n"
	white := "proxies:\n  - name: NL\n    type: vless\n    server: w\n    port: 443\n"
	for _, add := range []Addition{{Body: []byte(white), Rename: rename}, {StubRemark: "stub"}} {
		out, n, err := Merge([]byte(main), add)
		if err != nil || n != 1 {
			t.Fatal(n, err)
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
		want := "W NL"
		if add.Body == nil {
			want = "stub"
		}
		if len(cfg.Proxies) != 2 || cfg.Proxies[1]["name"] != want {
			t.Fatalf("%s", out)
		}
		if g := cfg.Groups[0].Proxies; g[len(g)-1] != want || len(cfg.Groups[1].Proxies) != 1 {
			t.Fatalf("groups %+v", cfg.Groups)
		}
	}
}

func TestMergeStubsAllFormats(t *testing.T) {
	mains := map[Format]string{
		FormatXrayJSON:   `[{"remarks":"DE","outbounds":[]}]`,
		FormatSingbox:    `{"outbounds":[{"type":"vless","tag":"DE"}]}`,
		FormatLinksPlain: "vless://u@m:1#DE\n",
	}
	for f, main := range mains {
		if got, _ := Detect([]byte(main)); got != f {
			t.Fatalf("detect %s: got %s", f, got)
		}
		out, n, err := Merge([]byte(main), Addition{StubRemark: "stub"})
		if err != nil || n != 1 || !strings.Contains(string(out), "stub") || !strings.Contains(string(out), "0.0.0.0") {
			t.Fatalf("%s: %d %v %s", f, n, err, out)
		}
	}
}

func TestRenameVmess(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{"ps":"NL","add":"w","port":"443"}`))
	got := renameLink("vmess://"+payload, rename)
	dec, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
	if !strings.Contains(string(dec), `"ps":"W NL"`) {
		t.Fatalf("%s", dec)
	}
}

func TestFormatGB(t *testing.T) {
	for in, want := range map[float64]string{10 * gib: "10", 6.6 * gib: "6.6", 0: "0", 1.25 * gib: "1.3"} {
		if got := FormatGB(in); got != want {
			t.Errorf("FormatGB(%v)=%q want %q", in, got, want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "addons.yml")
	data := "timeout: 3s\naddons:\n  - prefix: white_\n    stubs:\n      limited: lim\n  - suffix: _eu\n    remark: \"EU {remark}\"\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 3*time.Second || len(cfg.Addons) != 2 {
		t.Fatalf("%+v", cfg)
	}
	w, eu := cfg.Addons[0], cfg.Addons[1]
	if w.Name != "white_" || w.Remark != "{remark}" || w.Stubs["LIMITED"] != "lim" || w.username("bob") != "white_bob" {
		t.Fatalf("%+v", w)
	}
	if eu.RemarkUnlimited != "EU {remark}" || eu.username("bob") != "bob_eu" {
		t.Fatalf("%+v", eu)
	}

	if cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yml")); err != nil || len(cfg.Addons) != 0 {
		t.Fatalf("missing file: %+v %v", cfg, err)
	}
	_ = os.WriteFile(path, []byte("addons:\n  - remark: x\n"), 0o600)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error without prefix/suffix")
	}
}

func TestExampleConfig(t *testing.T) {
	cfg, err := LoadConfig("../../../addons.example.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Addons) != 1 || cfg.Addons[0].username("alice") != "premium_alice" || cfg.Addons[0].Stubs["LIMITED"] == "" {
		t.Fatalf("%+v", cfg)
	}
	if cfg, err := LoadConfig(t.TempDir()); err != nil || len(cfg.Addons) != 0 {
		t.Fatalf("directory: %+v %v", cfg, err)
	}
}

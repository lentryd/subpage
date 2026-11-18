package addon

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"
)

const intoTargetConfig = `{"remarks":"🏳️ Whitelist auto","outbounds":[
  {"tag":"proxy","protocol":"hysteria"},{"tag":"proxy-2","protocol":"hysteria"},
  {"tag":"direct","protocol":"freedom"},{"tag":"block","protocol":"blackhole"}],
  "routing":{"balancers":[{"tag":"auto","selector":["proxy"]}]},
  "burstObservatory":{"subjectSelector":["proxy"]}}`

const intoOther = `{"remarks":"🇫🇮 Finland","outbounds":[{"tag":"proxy","protocol":"hysteria"},{"tag":"direct","protocol":"freedom"}]}`

func intoMain(configs ...string) []byte {
	out := "["
	for i, c := range configs {
		if i > 0 {
			out += ","
		}
		out += c
	}
	return []byte(out + "]")
}

func intoAdd(re string, names ...string) Addition {
	a := staticAdd(names...)
	a.XrayInto = regexp.MustCompile(re)
	a.XrayRemark = "{remark} · 5 of 35 GB"
	return a
}

func decodeConfigs(t *testing.T, out []byte) []map[string]any {
	t.Helper()
	var got []map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return got
}

func outboundTags(c map[string]any) []string {
	var tags []string
	for _, o := range c["outbounds"].([]any) {
		tags = append(tags, o.(map[string]any)["tag"].(string))
	}
	return tags
}

func TestMergeXrayIntoAppendsOutbounds(t *testing.T) {
	main := intoMain(intoOther, intoTargetConfig)
	out, n, err := Merge(main, intoAdd("Whitelist auto$", "a", "b"))
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := decodeConfigs(t, out)
	if len(got) != 2 {
		t.Fatalf("no separate configs expected: %s", out)
	}
	want := []string{"proxy", "proxy-2", "proxy-addon-1", "proxy-addon-2", "direct", "block"}
	if tags := outboundTags(got[1]); !reflect.DeepEqual(tags, want) {
		t.Fatalf("tags %v", tags)
	}
	if got[1]["remarks"] != "🏳️ Whitelist auto · 5 of 35 GB" {
		t.Fatalf("remarks %v", got[1]["remarks"])
	}
	if !reflect.DeepEqual(got[0], decodeConfigs(t, intoMain(intoOther))[0]) {
		t.Fatalf("other config changed: %v", got[0])
	}
	if p := got[1]["outbounds"].([]any)[2].(map[string]any)["protocol"]; p != "vless" {
		t.Fatalf("first add-on link is vless, got %v", p)
	}
	if p := got[1]["outbounds"].([]any)[3].(map[string]any)["protocol"]; p != "hysteria" {
		t.Fatalf("second add-on link is hysteria2, got %v", p)
	}
}

func TestMergeXrayIntoSelectorWithDash(t *testing.T) {
	target := `{"remarks":"T","outbounds":[{"tag":"proxy-2"},{"tag":"direct"}],"routing":{"balancers":[{"selector":["proxy-"]}]}}`
	out, _, err := Merge(intoMain(target), intoAdd("^T$", "a"))
	if err != nil {
		t.Fatal(err)
	}
	if tags := outboundTags(decodeConfigs(t, out)[0]); !reflect.DeepEqual(tags, []string{"proxy-2", "proxy-addon-1", "direct"}) {
		t.Fatalf("%v", tags)
	}
}

func TestMergeXrayIntoFirstMatchOnly(t *testing.T) {
	second := `{"remarks":"Whitelist auto 2","outbounds":[{"tag":"proxy"},{"tag":"direct"}],"routing":{"balancers":[{"selector":["proxy"]}]}}`
	out, _, err := Merge(intoMain(intoTargetConfig, second), intoAdd("Whitelist", "a"))
	if err != nil {
		t.Fatal(err)
	}
	got := decodeConfigs(t, out)
	if len(outboundTags(got[0])) != 5 || len(outboundTags(got[1])) != 2 {
		t.Fatalf("%v %v", outboundTags(got[0]), outboundTags(got[1]))
	}
}

func TestMergeXrayIntoContinuesNumbering(t *testing.T) {
	target := `{"remarks":"T","outbounds":[{"tag":"proxy"},{"tag":"proxy-addon-1"},{"tag":"direct"}],"routing":{"balancers":[{"selector":["proxy"]}]}}`
	out, _, err := Merge(intoMain(target), intoAdd("^T", "a"))
	if err != nil {
		t.Fatal(err)
	}
	if tags := outboundTags(decodeConfigs(t, out)[0]); !reflect.DeepEqual(tags, []string{"proxy", "proxy-addon-1", "proxy-addon-2", "direct"}) {
		t.Fatalf("%v", tags)
	}
}

func TestMergeXrayIntoTwoAddons(t *testing.T) {
	out, _, err := Merge(intoMain(intoTargetConfig), intoAdd("Whitelist auto", "a"))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err = Merge(out, intoAdd("Whitelist auto", "b"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"proxy", "proxy-2", "proxy-addon-1", "proxy-addon-2", "direct", "block"}
	if tags := outboundTags(decodeConfigs(t, out)[0]); !reflect.DeepEqual(tags, want) {
		t.Fatalf("%v", tags)
	}
}

func TestMergeXrayIntoObservatoryFallback(t *testing.T) {
	target := `{"remarks":"T","outbounds":[{"tag":"node"},{"tag":"direct"}],"burstObservatory":{"subjectSelector":["node"]}}`
	out, _, err := Merge(intoMain(target), intoAdd("^T$", "a"))
	if err != nil {
		t.Fatal(err)
	}
	if tags := outboundTags(decodeConfigs(t, out)[0]); !reflect.DeepEqual(tags, []string{"node", "node-addon-1", "direct"}) {
		t.Fatalf("%v", tags)
	}
}

func TestMergeXrayIntoNoSelectorFallsBack(t *testing.T) {
	target := `{"remarks":"T","outbounds":[{"tag":"proxy"},{"tag":"direct"}]}`
	out, n, err := Merge(intoMain(target), intoAdd("^T$", "a", "b"))
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := decodeConfigs(t, out)
	if len(got) != 3 || got[0]["remarks"] != "T" || len(outboundTags(got[0])) != 2 {
		t.Fatalf("expected separate configs: %s", out)
	}
}

func TestMergeXrayIntoNoMatchFallsBack(t *testing.T) {
	out, n, err := Merge(intoMain(intoTargetConfig), intoAdd("nothing", "a"))
	if err != nil || n != 1 || len(decodeConfigs(t, out)) != 2 {
		t.Fatalf("n=%d err=%v %s", n, err, out)
	}
}

func TestMergeXrayIntoEmptyConfigsKeepsTarget(t *testing.T) {
	add := Addition{XrayInto: regexp.MustCompile("Whitelist"), XrayRemark: "{remark} · ?"}
	out, n, err := Merge(intoMain(intoTargetConfig), add)
	if n != 0 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := decodeConfigs(t, out)
	if len(got) != 1 || got[0]["remarks"] != "🏳️ Whitelist auto" || len(outboundTags(got[0])) != 4 {
		t.Fatalf("%s", out)
	}
}

func TestMergeXrayIntoStub(t *testing.T) {
	add := Addition{StubRemark: "limit reached", XrayInto: regexp.MustCompile("Whitelist")}
	out, n, err := Merge(intoMain(intoTargetConfig), add)
	if n != 0 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := decodeConfigs(t, out)
	if len(got) != 1 || got[0]["remarks"] != "🏳️ Whitelist auto · limit reached" || len(outboundTags(got[0])) != 4 {
		t.Fatalf("%s", out)
	}
}

func TestMergeXrayIntoStubNoTargetAddsStubConfig(t *testing.T) {
	add := Addition{StubRemark: "limit reached", XrayInto: regexp.MustCompile("nothing")}
	out, n, err := Merge(intoMain(intoTargetConfig), add)
	if err != nil || n != 1 || len(decodeConfigs(t, out)) != 2 {
		t.Fatalf("n=%d err=%v %s", n, err, out)
	}
}

func TestMergeXrayIntoSingleObjectBody(t *testing.T) {
	out, n, err := Merge([]byte(intoTargetConfig), intoAdd("Whitelist", "a"))
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := decodeConfigs(t, out); len(got) != 1 || len(outboundTags(got[0])) != 5 {
		t.Fatalf("%s", out)
	}
}

func TestMergeXrayIntoUnsupportedLinkSkipped(t *testing.T) {
	add := intoAdd("Whitelist", "a")
	add.Configs = append(add.Configs, StaticConfig{Link: "ss://x@h.example:1#x", Name: "b"})
	out, n, err := Merge(intoMain(intoTargetConfig), add)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := decodeConfigs(t, out); len(outboundTags(got[0])) != 5 {
		t.Fatalf("%s", out)
	}
}

func TestMergeXrayIntoEmptyRemarkTemplateKeepsRemarks(t *testing.T) {
	add := intoAdd("Whitelist", "a")
	add.XrayRemark = ""
	out, _, err := Merge(intoMain(intoTargetConfig), add)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeConfigs(t, out); got[0]["remarks"] != "🏳️ Whitelist auto" {
		t.Fatalf("%v", got[0]["remarks"])
	}
}

func TestMergeXrayIntoTwoAddonsRewrittenRemarks(t *testing.T) {
	first := intoAdd("^🏳️ Whitelist", "a")
	second := intoAdd("^🏳️ Whitelist", "b")
	second.XrayRemark = "{remark} · more"
	out, _, err := Merge(intoMain(intoTargetConfig), first)
	if err != nil {
		t.Fatal(err)
	}
	out, n, err := Merge(out, second)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := decodeConfigs(t, out)
	if len(got) != 1 || got[0]["remarks"] != "🏳️ Whitelist auto · 5 of 35 GB · more" || len(outboundTags(got[0])) != 6 {
		t.Fatalf("%s", out)
	}
}

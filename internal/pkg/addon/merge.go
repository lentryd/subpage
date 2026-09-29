package addon

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format is the shape of a subscription payload returned by /api/sub. The
// panel picks it from the clientType path segment or from the User-Agent,
// so it is detected from the body rather than from the request.
type Format int

const (
	FormatUnknown Format = iota
	FormatLinksBase64
	FormatLinksPlain
	FormatXrayJSON
	FormatSingbox
	FormatMihomo
)

func (f Format) String() string {
	return [...]string{"unknown", "base64", "plain", "xray-json", "singbox", "mihomo"}[f]
}

// stubUUID and stubAddr make an intentionally unusable config: it only
// carries a remark telling the user why the add-on configs are missing.
const (
	stubUUID = "00000000-0000-0000-0000-000000000000"
	stubHost = "0.0.0.0"
	stubPort = 1
)

// Addition describes what gets appended to the main subscription: either
// the add-on user's payload (same format as main) with remarks rewritten by
// Rename, or a single stub config named StubRemark.
type Addition struct {
	Body       []byte
	Rename     func(remark string) string
	StubRemark string
}

// Merge appends add to main and returns the merged payload plus the number
// of configs added. On any error the caller should serve main unchanged.
func Merge(main []byte, add Addition) ([]byte, int, error) {
	format, err := Detect(main)
	if err != nil {
		return nil, 0, err
	}
	if add.Body == nil && add.StubRemark == "" {
		return main, 0, nil
	}
	switch format {
	case FormatLinksBase64, FormatLinksPlain:
		return mergeLinks(format, main, add)
	case FormatXrayJSON:
		return mergeXray(main, add)
	case FormatSingbox:
		return mergeSingbox(main, add)
	case FormatMihomo:
		return mergeMihomo(main, add)
	}
	return nil, 0, errors.New("unknown subscription format")
}

// Detect guesses the payload format.
func Detect(body []byte) (Format, error) {
	t := bytes.TrimSpace(body)
	if len(t) == 0 {
		return FormatUnknown, errors.New("empty body")
	}
	switch t[0] {
	case '[':
		return FormatXrayJSON, nil
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(t, &obj); err != nil {
			return FormatUnknown, err
		}
		if _, ok := obj["remarks"]; ok {
			return FormatXrayJSON, nil
		}
		if _, ok := obj["outbounds"]; ok {
			return FormatSingbox, nil
		}
		return FormatUnknown, errors.New("unrecognized json subscription")
	}
	if decoded, ok := decodeBase64(string(t)); ok && strings.Contains(decoded, "://") {
		return FormatLinksBase64, nil
	}
	var probe map[string]any
	if err := yaml.Unmarshal(t, &probe); err == nil {
		if _, ok := probe["proxies"]; ok {
			return FormatMihomo, nil
		}
	}
	if strings.Contains(string(t), "://") {
		return FormatLinksPlain, nil
	}
	return FormatUnknown, errors.New("unrecognized subscription")
}

func decodeBase64(s string) (string, bool) {
	s = strings.Join(strings.Fields(s), "")
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return string(b), true
		}
	}
	return "", false
}

// uniqueNamer returns names not yet present in taken, suffixing " #2", " #3"...
type uniqueNamer map[string]bool

func (u uniqueNamer) name(n string) string {
	candidate := n
	for i := 2; u[candidate]; i++ {
		candidate = fmt.Sprintf("%s #%d", n, i)
	}
	u[candidate] = true
	return candidate
}

// ---------- links (base64 / plain) ----------

func splitLinks(format Format, body []byte) ([]string, error) {
	text := string(body)
	if format == FormatLinksBase64 {
		decoded, ok := decodeBase64(strings.TrimSpace(text))
		if !ok {
			return nil, errors.New("invalid base64 subscription")
		}
		text = decoded
	}
	var links []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			links = append(links, l)
		}
	}
	return links, nil
}

func mergeLinks(format Format, main []byte, add Addition) ([]byte, int, error) {
	links, err := splitLinks(format, main)
	if err != nil {
		return nil, 0, err
	}
	var extra []string
	if add.Body != nil {
		addFormat, err := Detect(add.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("add-on body: %w", err)
		}
		if addFormat != FormatLinksBase64 && addFormat != FormatLinksPlain {
			return nil, 0, fmt.Errorf("add-on body format %s differs from main %s", addFormat, format)
		}
		addLinks, err := splitLinks(addFormat, add.Body)
		if err != nil {
			return nil, 0, err
		}
		for _, l := range addLinks {
			extra = append(extra, renameLink(l, add.Rename))
		}
	} else {
		extra = append(extra, fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=none#%s",
			stubUUID, stubHost, stubPort, url.PathEscape(add.StubRemark)))
	}
	out := strings.Join(append(links, extra...), "\n")
	if format == FormatLinksBase64 {
		out = base64.StdEncoding.EncodeToString([]byte(out))
	}
	return []byte(out), len(extra), nil
}

// renameLink rewrites a share link's remark: the "ps" field for vmess
// (base64 JSON), the #fragment for everything else.
func renameLink(link string, rename func(string) string) string {
	if payload, ok := strings.CutPrefix(link, "vmess://"); ok {
		if decoded, ok := decodeBase64(payload); ok {
			var obj map[string]any
			if json.Unmarshal([]byte(decoded), &obj) == nil {
				ps, _ := obj["ps"].(string)
				obj["ps"] = rename(ps)
				if b, err := json.Marshal(obj); err == nil {
					return "vmess://" + base64.StdEncoding.EncodeToString(b)
				}
			}
		}
	}
	base, frag, _ := strings.Cut(link, "#")
	orig, err := url.PathUnescape(frag)
	if err != nil {
		orig = frag
	}
	return base + "#" + url.PathEscape(rename(orig))
}

// ---------- xray-json ----------

func decodeJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xrayConfigs(b []byte) ([]map[string]any, error) {
	t := bytes.TrimSpace(b)
	if len(t) > 0 && t[0] == '{' {
		var one map[string]any
		if err := decodeJSON(t, &one); err != nil {
			return nil, err
		}
		return []map[string]any{one}, nil
	}
	var many []map[string]any
	if err := decodeJSON(t, &many); err != nil {
		return nil, err
	}
	return many, nil
}

func mergeXray(main []byte, add Addition) ([]byte, int, error) {
	configs, err := xrayConfigs(main)
	if err != nil {
		return nil, 0, err
	}
	names := uniqueNamer{}
	for _, c := range configs {
		if r, ok := c["remarks"].(string); ok {
			names[r] = true
		}
	}
	var extra []map[string]any
	if add.Body != nil {
		addConfigs, err := xrayConfigs(add.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("add-on body: %w", err)
		}
		for _, c := range addConfigs {
			r, _ := c["remarks"].(string)
			c["remarks"] = names.name(add.Rename(r))
			extra = append(extra, c)
		}
	} else {
		extra = append(extra, map[string]any{
			"remarks": names.name(add.StubRemark),
			"outbounds": []any{map[string]any{
				"tag":      "proxy",
				"protocol": "vless",
				"settings": map[string]any{"vnext": []any{map[string]any{
					"address": stubHost,
					"port":    stubPort,
					"users":   []any{map[string]any{"id": stubUUID, "encryption": "none"}},
				}}},
			}},
		})
	}
	out, err := encodeJSON(append(configs, extra...))
	return out, len(extra), err
}

// ---------- sing-box ----------

// singboxNonProxy are outbound types that are routing helpers, not servers.
var singboxNonProxy = map[string]bool{"selector": true, "urltest": true, "direct": true, "block": true, "dns": true}

func mergeSingbox(main []byte, add Addition) ([]byte, int, error) {
	var cfg map[string]any
	if err := decodeJSON(main, &cfg); err != nil {
		return nil, 0, err
	}
	outbounds, _ := cfg["outbounds"].([]any)
	names := uniqueNamer{}
	for _, o := range outbounds {
		if m, ok := o.(map[string]any); ok {
			if tag, ok := m["tag"].(string); ok {
				names[tag] = true
			}
		}
	}

	var extra []any
	var tags []any
	if add.Body != nil {
		var addCfg map[string]any
		if err := decodeJSON(add.Body, &addCfg); err != nil {
			return nil, 0, fmt.Errorf("add-on body: %w", err)
		}
		addOutbounds, _ := addCfg["outbounds"].([]any)
		for _, o := range addOutbounds {
			m, ok := o.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); singboxNonProxy[t] {
				continue
			}
			tag, _ := m["tag"].(string)
			m["tag"] = names.name(add.Rename(tag))
			extra = append(extra, m)
			tags = append(tags, m["tag"])
		}
	} else {
		tag := names.name(add.StubRemark)
		extra = append(extra, map[string]any{
			"type": "vless", "tag": tag, "server": stubHost, "server_port": stubPort, "uuid": stubUUID,
		})
		tags = append(tags, tag)
	}

	// Only manual selectors get the new tags: auto (urltest) groups must not
	// silently route traffic through paid, traffic-limited add-on configs.
	for _, o := range outbounds {
		if m, ok := o.(map[string]any); ok && m["type"] == "selector" {
			list, _ := m["outbounds"].([]any)
			m["outbounds"] = append(list, tags...)
		}
	}
	cfg["outbounds"] = append(outbounds, extra...)
	out, err := encodeJSON(cfg)
	return out, len(extra), err
}

// ---------- mihomo / clash / stash ----------

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(m *yaml.Node, key, value string) {
	if v := mappingValue(m, key); v != nil {
		v.Kind, v.Tag, v.Value, v.Style = yaml.ScalarNode, "!!str", value, 0
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func yamlRoot(b []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("yaml root is not a mapping")
	}
	return &doc, nil
}

func mergeMihomo(main []byte, add Addition) ([]byte, int, error) {
	doc, err := yamlRoot(main)
	if err != nil {
		return nil, 0, err
	}
	root := doc.Content[0]
	proxies := mappingValue(root, "proxies")
	if proxies == nil || proxies.Kind != yaml.SequenceNode {
		return nil, 0, errors.New("mihomo: proxies is not a list")
	}
	names := uniqueNamer{}
	for _, p := range proxies.Content {
		if n := mappingValue(p, "name"); n != nil {
			names[n.Value] = true
		}
	}

	var extra []*yaml.Node
	if add.Body != nil {
		addDoc, err := yamlRoot(add.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("add-on body: %w", err)
		}
		if addProxies := mappingValue(addDoc.Content[0], "proxies"); addProxies != nil {
			for _, p := range addProxies.Content {
				if p.Kind != yaml.MappingNode {
					continue
				}
				orig := ""
				if n := mappingValue(p, "name"); n != nil {
					orig = n.Value
				}
				setMappingValue(p, "name", names.name(add.Rename(orig)))
				extra = append(extra, p)
			}
		}
	} else {
		var stub yaml.Node
		stubYAML := fmt.Sprintf("{type: vless, server: %q, port: %d, uuid: %q, network: tcp, udp: false}", stubHost, stubPort, stubUUID)
		if err := yaml.Unmarshal([]byte(stubYAML), &stub); err != nil {
			return nil, 0, err
		}
		m := stub.Content[0]
		m.Style = 0
		setMappingValue(m, "name", names.name(add.StubRemark))
		// Put name first, as clients list it that way.
		m.Content = append(m.Content[len(m.Content)-2:], m.Content[:len(m.Content)-2]...)
		extra = append(extra, m)
	}
	proxies.Content = append(proxies.Content, extra...)

	// As with sing-box, only manual "select" groups receive the new proxies.
	if groups := mappingValue(root, "proxy-groups"); groups != nil && groups.Kind == yaml.SequenceNode {
		for _, g := range groups.Content {
			if t := mappingValue(g, "type"); t == nil || t.Value != "select" {
				continue
			}
			list := mappingValue(g, "proxies")
			if list == nil || list.Kind != yaml.SequenceNode {
				continue
			}
			for _, p := range extra {
				list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mappingValue(p, "name").Value})
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, 0, err
	}
	_ = enc.Close()
	return buf.Bytes(), len(extra), nil
}

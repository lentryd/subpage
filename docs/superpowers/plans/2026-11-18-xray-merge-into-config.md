# Xray JSON: merge add-on servers into an existing config — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An add-on with static `links` can append its servers as extra outbounds of an existing xray-json config chosen by a regexp on `remarks`, so the client's balancer picks the best server by ping.

**Architecture:** A new per-add-on option `xray.into` (RE2 on the target config's `remarks`) travels with `Addition` to `mergeXray`. `mergeXray` finds the target config, derives the tag prefix from its balancer selector, inserts `<prefix>-addon-<n>` outbounds, and rewrites its `remarks` with the quota. `Proxy` learns to render a `hysteria2` link as an xray `hysteria` outbound. Without a usable target it falls back to today's separate configs, with a warning.

**Tech Stack:** Go 1.26, `internal/pkg/addon`, encoding/json, yaml.v3, standard `testing`.

**Spec:** `docs/superpowers/specs/2026-11-18-xray-merge-into-config-design.md`

## Global Constraints

- Conventional commits (`feat(addon): ...`, `test(addon): ...`, `docs: ...`), body in English; no `Co-Authored-By` or any other attribution lines.
- Before every commit that changes code: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, `golangci-lint run`.
- Nothing about names, flags, countries, providers or lentryd in code, tests or tracked docs; use neutral examples (`example.com`, `🏳️ Whitelist auto`).
- Real `addons.yml` is gitignored and must not be touched or committed.
- sing-box, mihomo, base64/plain output must not change; existing tests keep passing except the one that asserts hysteria2 is skipped in xray-json (Task 2).
- `xray.into` is valid only for add-ons with `links`; it is ignored for add-ons that use the add-on user's panel subscription (`LoadConfig` rejects the combination).

## Review Focus

- Target balancer selector ends with a dash (`proxy-`) → tag must be `proxy-addon-1`, never `proxy--addon-1`.
- Target `remarks` regexp matches several configs → only the first is used.
- Provider unavailable or nothing matched → `Configs` is empty: `remarks` and `outbounds` stay untouched, no duplicate configs, no warning flood.
- Two add-ons target the same config → tag numbers continue, no duplicate tags.
- Main body is a single JSON object instead of an array (`xrayConfigs` accepts it) → output stays valid.
- hy2 link with and without `sni`, `insecure=1`, `alpn`, `obfs=salamander` → outbound matches the panel's own `hysteria` shape.
- LIMITED/EXPIRED stub while target exists → target `remarks` gets the stub text; no placeholder config is appended.

---

### Task 1: `xray` options in the add-on config

**Files:**
- Modify: `internal/pkg/addon/addon.go` (Addon struct near line 72-90, `LoadConfig` validation loop)
- Test: `internal/pkg/addon/addon_test.go`

**Interfaces:**
- Produces:
  ```go
  // XrayOptions control how a static add-on is merged into xray-json output.
  type XrayOptions struct {
      Into   string `yaml:"into"`   // RE2 on the target config's remarks
      Remark string `yaml:"remark"` // template for the target's remarks; {remark},{remaining},{limit},{used}
      intoRe *regexp.Regexp
  }
  // Addon gets: Xray XrayOptions `yaml:"xray"`
  ```
  `Remark` defaults to `"{remark}"` (leave the target name unchanged) when `Into` is set and `Remark` is empty.

- [ ] **Step 1: Write failing tests** in `addon_test.go`: `TestLoadConfigXrayInto` (valid `xray: {into: "Auto$", remark: "{remark} · {remaining} of {limit} GB"}` on an add-on with `links` loads, `Xray.intoRe` compiled, `Remark` kept); `TestLoadConfigXrayIntoDefaultRemark` (empty remark becomes `"{remark}"`); `TestLoadConfigXrayIntoInvalidRegexp` (error mentions `addons[0].xray.into`); `TestLoadConfigXrayIntoNeedsLinks` (add-on without `links` and with `into` → error mentioning `xray.into requires links`).
- [ ] **Step 2: Run** `go test ./internal/pkg/addon -run TestLoadConfigXray -v` → FAIL (unknown field / no validation).
- [ ] **Step 3: Implement** the struct and validation in `LoadConfig` after the `links` loop; compile `into` with `regexp.Compile`, store in `intoRe`.
- [ ] **Step 4: Run** the same command → PASS; then `go test ./internal/pkg/addon` → PASS.
- [ ] **Step 5: Commit** `feat(addon): add xray.into option for static add-ons`.

---

### Task 2: Render `hysteria2` as an xray `hysteria` outbound

**Files:**
- Modify: `internal/pkg/addon/render.go` (`Proxy.Xray` near line 199)
- Test: `internal/pkg/addon/merge_static_test.go` (`TestMergeStaticXray` currently expects hy2 skipped), `internal/pkg/addon/render_test.go` (create)

**Interfaces:**
- Produces:
  ```go
  // XrayOutbound renders the proxy as one xray outbound with the given tag.
  // It reports false for unsupported schemes.
  func (p Proxy) XrayOutbound(tag string) (map[string]any, bool)
  ```
  `Proxy.Xray()` keeps its signature and becomes `{"remarks": p.Name, "outbounds": []any{XrayOutbound("proxy")}}`; it now returns true for hysteria2 too.
- Required hysteria shape (copied from a real panel response; secrets removed):
  ```json
  {"tag":"<tag>","protocol":"hysteria",
   "settings":{"address":"<server>","port":<port>,"version":2},
   "streamSettings":{"network":"hysteria","security":"tls",
     "hysteriaSettings":{"auth":"<secret>","version":2},
     "tlsSettings":{"alpn":["h3"],"enableSessionResumption":false,"serverName":"<sni>"},
     "finalmask":{"quicParams":{"congestion":"bbr","debug":false}}}}
  ```
  With `obfs=salamander&obfs-password=X` add `finalmask.udp = [{"type":"salamander","settings":{"password":"X"}}]`. `alpn` comes from the link's `alpn` (comma list) and defaults to `["h3"]`; `serverName` is omitted when `sni` is empty; `insecure=1` adds `tlsSettings.allowInsecure: true`.

- [ ] **Step 1: Write failing tests** in `render_test.go`: `TestXrayOutboundHysteria2` (plain: asserts exact map above via `reflect.DeepEqual` after JSON round-trip), `TestXrayOutboundHysteria2Salamander`, `TestXrayOutboundHysteria2InsecureAlpn`, `TestXrayOutboundVlessKeepsShape` (vless output equals what `Xray()` produced before for the existing test link), `TestXrayOutboundUnsupportedScheme` (false). Update `TestMergeStaticXray` so the hy2 link now yields a config with a `hysteria` outbound instead of being skipped.
- [ ] **Step 2: Run** `go test ./internal/pkg/addon -run 'TestXrayOutbound|TestMergeStaticXray' -v` → FAIL.
- [ ] **Step 3: Implement** `XrayOutbound` (move the current vless body of `Xray()` into it, tag parameterised) and rewrite `Xray()` on top of it; update the comment that says hysteria2 is not covered.
- [ ] **Step 4: Run** `go test ./internal/pkg/addon` → PASS.
- [ ] **Step 5: Commit** `feat(addon): render hysteria2 links as xray hysteria outbounds`.

---

### Task 3: Merge into a target config in `mergeXray`

**Files:**
- Modify: `internal/pkg/addon/merge.go` (`Addition` near line 50; `mergeXray` near line 255)
- Test: `internal/pkg/addon/merge_xray_into_test.go` (create)

**Interfaces:**
- Consumes: `Proxy.XrayOutbound(tag string) (map[string]any, bool)` from Task 2; `Addition.proxies()` and `StaticConfig` from `render.go`.
- Produces: two new fields on `Addition`:
  ```go
  XrayInto   *regexp.Regexp // when set, merge static configs into the xray config whose remarks match
  XrayRemark string         // template for the target's remarks; {remark} stays literal until merge
  ```
  and these helpers in `merge.go`:
  ```go
  // intoTarget returns the index of the first config whose remarks match re and its tag prefix.
  func intoTarget(configs []map[string]any, re *regexp.Regexp) (idx int, prefix string, ok bool)
  // mergeXrayInto appends add's proxies to configs[idx]; returns outbounds added.
  func mergeXrayInto(configs []map[string]any, idx int, prefix string, add Addition) int
  ```
- Behaviour pinned by the spec:
  - Prefix = first selector of the first balancer in `routing.balancers`; if none, `burstObservatory.subjectSelector[0]`; if neither, not suitable. Trailing `-` is trimmed. Tag = `<prefix>-addon-<n>`, `n` = highest existing `<prefix>-addon-<k>` + 1, starting at 1.
  - New outbounds are inserted after the last outbound whose tag has the prefix and before the rest (`direct`, `block`); if none has the prefix, before the first outbound whose tag is `direct` or `block`, else appended.
  - Only when `add.Body == nil && len(add.Configs) > 0`. Hysteria2 and vless links both go in; unsupported ones are skipped with `slog.Warn`.
  - `remarks` of the target = `XrayRemark` with `{remark}` replaced by the old remarks (empty `XrayRemark` leaves it unchanged); changed only if at least one outbound was added.
  - Stub (`add.Body == nil && len(add.Configs) == 0`, `StubRemark != ""`) with a suitable target: target remarks = `<old> · <StubRemark>`, nothing appended, `n = 0`.
  - No matching or unsuitable target → existing code path (separate configs/stub config) plus `slog.Warn("add-on xray.into target not usable", ...)`.

- [ ] **Step 1: Write failing tests** in `merge_xray_into_test.go` (table of small JSON bodies; target config modelled on the panel's: `outbounds` proxy/proxy-2/direct/block, `routing.balancers[0].selector=["proxy"]`, `burstObservatory.subjectSelector=["proxy"]`): `TestMergeXrayIntoAppendsOutbounds` (tags `proxy-addon-1..2`, inserted before `direct`, `remarks` rewritten with `{remark} · 5 of 35 GB`, other configs untouched, count 2), `TestMergeXrayIntoSelectorWithDash` (selector `["proxy-"]` → `proxy-addon-1`), `TestMergeXrayIntoFirstMatchOnly`, `TestMergeXrayIntoContinuesNumbering` (target already has `proxy-addon-1`), `TestMergeXrayIntoTwoAddons` (two sequential `Merge` calls, no duplicate tags), `TestMergeXrayIntoObservatoryFallback` (no balancer, observatory selector used), `TestMergeXrayIntoNoSelectorFallsBack` (separate configs appended), `TestMergeXrayIntoNoMatchFallsBack`, `TestMergeXrayIntoEmptyConfigsKeepsTarget` (`Configs` empty, no stub: nothing changes, returns 0), `TestMergeXrayIntoStub`, `TestMergeXrayIntoSingleObjectBody`, `TestMergeXrayIntoUnsupportedLinkSkipped`.
- [ ] **Step 2: Run** `go test ./internal/pkg/addon -run TestMergeXrayInto -v` → FAIL (undefined fields/functions).
- [ ] **Step 3: Implement** the two fields, `intoTarget`, `mergeXrayInto`, and the branch at the top of the `add.Body == nil` logic in `mergeXray`; unmarshalled JSON numbers/arrays are `[]any`/`map[string]any`, so work on those types.
- [ ] **Step 4: Run** `go test ./internal/pkg/addon` → PASS.
- [ ] **Step 5: Commit** `feat(addon): merge static add-on servers into an existing xray config`.

---

### Task 4: Wire the option through the service

**Files:**
- Modify: `internal/pkg/addon/addon.go` (`resolveOne` near line 215, `staticConfigs` near line 244)
- Test: `internal/pkg/addon/addon_test.go` (existing tests use a fake panel via httptest; follow the helper used by the static-links resolve tests)

**Interfaces:**
- Consumes: `Addon.Xray.intoRe` (Task 1), `Addition.XrayInto`/`XrayRemark` (Task 3).
- Produces: `staticConfigs` now returns `([]StaticConfig, string)` where the string is the target remarks template with numbers filled and `{remark}` left literal (use `staticRemark(a.Xray.Remark, "{remark}", info, known)`); `resolveOne` sets `Addition{Configs: cfgs, XrayInto: a.Xray.intoRe, XrayRemark: tmpl}` and, for the stub branch, `Addition{StubRemark: stub, XrayInto: a.Xray.intoRe}` — only when `a.Xray.intoRe != nil`.

- [ ] **Step 1: Write failing tests**: `TestResolveStaticSetsXrayInto` (add-on with `xray.into` and links, ACTIVE user → `Result.Addition.XrayInto` non-nil and `XrayRemark` contains the formatted quota and the literal `{remark}`), `TestResolveStubSetsXrayInto` (LIMITED user), `TestResolveWithoutXrayLeavesFieldsEmpty`, and an end-to-end `TestMergeXrayIntoFromResolve` feeding the Addition into `Merge` with a captured-style xray body.
- [ ] **Step 2: Run** `go test ./internal/pkg/addon -run 'TestResolve.*Xray|TestMergeXrayIntoFromResolve' -v` → FAIL.
- [ ] **Step 3: Implement** the signature change and wiring; update the one existing caller of `staticConfigs`.
- [ ] **Step 4: Run** `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat(addon): pass xray.into from add-on config to the merge`.

---

### Task 5: Docs and final checks

**Files:**
- Modify: `addons.example.yml`, `README.md`, `docs/superpowers/specs/2026-11-18-xray-merge-into-config-design.md` (one line: `xray.remark` is its own field, defaulting to `{remark}`; spec previously said it reuses `remark`)

- [ ] **Step 1:** In `addons.example.yml` add under the static add-on a commented `xray:` block with `into: "Whitelist auto$"` and `remark: "{remark} · {remaining} of {limit} GB left"`, explaining tags (`<balancer prefix>-addon-N`), fallback to separate configs, stub behaviour, and that it applies only to add-ons with `links`. Replace the sentence "hysteria2 is skipped in xray-json output" with the new behaviour.
- [ ] **Step 2:** Mirror the same in the README add-ons section (English, neutral names).
- [ ] **Step 3:** Run `gofmt -l .` (empty), `go vet ./...`, `go test ./...`, `golangci-lint run`, `grep -rIn -i 'lentryd' --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=dist . ` (only the public image name `ghcr.io/lentryd/subpage` allowed).
- [ ] **Step 4: Commit** `docs: document xray.into`.
- [ ] **Step 5:** Squash the branch into one commit (`git reset --soft main`, one conventional commit `feat(addon): merge static add-on servers into an xray config`), per the user's rule; do not push.

# Template Engine / Rendering

Mooncake uses a Pongo2-flavored template engine (`{{ var | filter }}`,
`{% if %}`, `{% for %}`). Most of it works; a few corners surprise.

---

## #16 — ✅ FIXED (or originally mis-reported) — Template engine HTML-escape

**Repro**:
```yaml
- log:
    msg: "host_specific={{ undefined | default:'<unset>' }}"
```

JSON output:
```json
{"type":"print.message","data":{"message":"host_specific=&lt;unset&gt;"}}
```

`<` and `>` rendered as `&lt;` / `&gt;`. Appears in BOTH terminal
output AND the JSON `print.message` event payload — happens at
render time, not display time.

**Why it matters**: `log:` is the canonical "show me a value"
surface. Users debugging will quote literal angle brackets (`"<no
value>"`, `"# <changes>"`), or render YAML/HTML, and see escaped
output that doesn't match the input. The JSON channel also leaks the
escape — wrong for any downstream consumer that isn't a browser.

**Likely cause**: pongo2 / template engine has `autoescape: true` on
by default. Should be `false` for Mooncake's surface (or scoped to
actions that emit to HTML, of which there are none).

**Round 26 verification** (2026-05-15 later):
```
# /work/cfg.yml: file.write content "literal <value> here"
$ cat /tmp/html-test.txt
literal <value> here              ← raw angle brackets, no escape

$ apply --output-format json
"message":"literal angle test: <unset>"
```

File content: ✅ literal angle brackets. JSON output: `<` is the
standard Go json.Marshal escape for `<` (decoder restores it to `<`).
Decode the JSON and you get back the literal string `<unset>`.

**Either the bug was fixed silently** between the original report and
round 26, **or my original observation conflated** JSON-Unicode-escape
(correct) with HTML-entity-escape (would be wrong). Marking
✅ resolved — no action needed.

---

## #79 — `read.json` floats integer values; `read.yaml` preserves them — LOW (consistency)

**Repro**:
```yaml
# /work/data.json: {"port": 8080}
- read.json: { path: /work/data.json }
  as: jc
- log: { msg: "json port = {{ jc.value.port }}" }

# /work/data.yaml: port: 8080
- read.yaml: { path: /work/data.yaml }
  as: yc
- log: { msg: "yaml port = {{ yc.value.port }}" }
```

Output:
```
json port = 8080.000000   ← JSON numbers come back as float
yaml port = 8080          ← YAML preserves integer
```

JSON's spec uses a single Number type and Go's `encoding/json`
decodes all numbers as float64. YAML preserves int vs. float
distinction. So the same logical value "8080" round-trips differently
depending on which read.* you use.

**Why LOW**: arithmetic still works; cosmetic difference. But
`{{ cfg.value.port }}` in templates ends up with `.0` suffix for
JSON-sourced configs, which is ugly in config files and command
strings (`--port 8080.000000` would fail).

**Fix**: in read.json, detect whether a JSON number has a fractional
part and emit as int when not.

---

## #82 — No way to access env vars in templates — LOW (DX gap)

**Repro**:
```yaml
- log: { msg: "env={{ env.MY_ENV }}" }      # → "env=" (empty)
- log: { msg: "env={{ environ.MY_ENV }}" }  # → "env=" (empty)
- shell: echo MY_ENV=$MY_ENV                 # → "MY_ENV=hello" (works)
```

Shell commands inherit the parent process env (bash does that
naturally). But there's no template path to env vars. The variable
namespaces `env.*` and `environ.*` both silently render empty.

Common patterns this blocks:
- `path: "{{ env.HOME }}/.config/app"`  — needs facts.user_home
  instead (works) but only for HOME-like values; arbitrary vars are
  inaccessible
- `content: "API_KEY={{ env.MY_API_KEY }}"` — must work around via
  `shell: echo $MY_API_KEY` + `as:` registered var

**Fix**: add an `env` (or `environ`) variable namespace in template
context, populated from `os.Environ()` at plan time. Document that
secrets-in-env should flow through MOONCAKE_SECRET_* or a sealed
vars file, not direct env access.

(This is also a security/auditability concern — fact-driven and
overlay-driven configs are auditable; env-var-driven aren't. So the
"no env access in templates" stance may be intentional. Worth
documenting either way.)

---

## #17 — Live metrics not available in apply-time templates (despite docs) — MEDIUM

**Repro**:
```yaml
- log:
    msg: "load1={{ load_avg_1m }}"
```

Output: `load1=` (empty).

**Why it matters**: `LLM_GUIDE.md` line 130 explicitly says:
> Use in templates: `{{ cpu_usage_pct }}`, `when: load_avg_1m < 4`

But `load_avg_1m` resolves to empty. Either docs lie or
metrics-in-template wiring isn't connected.

**Resolutions**:
- (a) Wire metrics into the template variable map (matches docs).
- (b) Restrict templates to facts and document that. Provide an
  explicit `{{ metric('load_avg_1m') }}` for on-demand sampling.

(b) is arguably better — apply-time metric rendering has unclear
caching semantics; an explicit accessor is clearer. Pick one and
make docs match.

---

## #18 — Jinja2-style filter args `default('x')` don't parse — LOW (DX surprise)

**Repro**:
```yaml
- log: { msg: "{{ undefined | default('fallback') }}" }
```

Errors:
```
failed to render message: [Error (where: parser) in <string> | Line 1
Col 31 near '('] '}}' expected
```

Works with the Pongo2 colon syntax: `default:'fallback'`.

**Why it matters**: every Ansible / Jinja2 tutorial uses
`default('x')`. Users will type it first, get a cryptic "`}}`
expected" pointing at the open paren, and conclude the engine is
broken.

**Fixes** (pick one):
- (a) Accept both forms.
- (b) Document the colon syntax loudly in the templates doc.
- (c) Improve the error message: "filter argument syntax is `|
  filter:value`; got `(`. Did you mean `default:'fallback'`?".

---

## #5 / #36 — Shell stdout dropped by the text formatter (engine captures it correctly)

**Original observation** (#5): `examples/hello-world/config.yml` does
`shell: echo "Hello from Mooncake!"`. At default log level you only
see step markers; at `-l debug` you see the rendered command but not
its stdout.

**Correction** (#36): the JSON channel surfaces stdout fine.

```
$ mooncake apply -c examples/hello-world/config.yml --output-format json
{"type":"step.stdout","data":{"step_id":"step-0001",
   "stream":"stdout","line":"Hello from Mooncake!","line_number":1}}
{"type":"step.completed","data":{...,"result":{...,
   "stdout":"Hello from Mooncake!\n",...}}}
```

So `shell` emits `step.stdout` events with line numbers, and the
final `step.completed` result includes full stdout. **Only the text
formatter drops it.**

**Fix**: a `--show-output` flag (or simply: if rc==0 and stdout is
non-empty, print it indented under the step). Don't change the
engine; just the renderer.

**Also reveals**: the JSON channel is significantly richer than the
text channel. Worth documenting / promoting the JSON channel as the
"real" output, with text as a friendly summary.

---

## #32 — `text.patch.json` uses `set:` / `delete:` / `merge:` (not JSON Patch RFC 6902) — LOW (DX surprise)

**Repro**:
```
$ mooncake step "text.patch.json: { path: ..., operations: [{op: replace, path: /a, value: 99}] }"
{"error": "validation failed: text.patch.json: at least one of set, delete, or merge is required"}
```

Works with:
```
$ mooncake step "text.patch.json: { path: ..., set: {b: 99, c: \"new\"}, delete: [a] }"
```

**Why it matters**: developers familiar with JSON Patch (RFC 6902 —
`[{op, path, value}]`) will type the standard form first. Mooncake's
schema is a higher-level set/delete/merge structure.

Same DX shape as #18 (`default('x')` vs `default:'x'`).

**Fix** (pick one):
- (a) Support both — accept JSON Patch ops alongside set/delete/merge.
- (b) Reject JSON Patch syntax with a clear hint: "use `set:` /
  `delete:` / `merge:` instead. Mooncake's patch.json is set-oriented,
  not RFC 6902."

---

## Summary table

| # | Sev | Area | Fix |
|---|---|---|---|
| 16 | MEDIUM | autoescape on log: msg: | disable / scope |
| 17 | MEDIUM | metrics in templates | wire OR doc honestly |
| 18 | LOW | `default('x')` syntax | accept + alias OR better error |
| 5/36 | LOW | shell stdout in text output | renderer change only |
| 32 | LOW | `text.patch.json` schema | accept JSON Patch OR better error |

What works (don't regress):
- `{{ var }}`, `{{ name | upper }}`, `{{ items | length }}`
- `{{ count + 1 }}` arithmetic
- `{% if %} / {% else %} / {% endif %}` conditionals
- `{% for i in items %} ... {% endfor %}` loops
- Facts (`{{ os }}`, `{{ arch }}`, `{{ cpu_cores }}`) in templates

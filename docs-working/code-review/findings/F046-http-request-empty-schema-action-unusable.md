---
id: F046
title: `http.request` action ships registered but unusable — missing from schemagen table, so schema.json node is empty and the validator rejects every field
severity: bug
package: internal/schemagen
file: internal/schemagen/generator.go
lines: 580-620
status: open
---

## What

Proposal-16 wave 1 added the `http.request` action — registered in
the action registry, with a Metadata block, a full `HTTPRequest`
config struct, a Run handler, and a Validate method. But the Go
struct → JSON schema generator's lookup table at
`internal/schemagen/generator.go:580-620` does **not** contain a
`"http.request"` entry. Consequence:

```bash
$ mooncake explain http.request    # action visible, but no schema:
action: http.request
  ...
  (no schema: section)

$ cat > p.yml <<'EOF'
- name: GET ok
  http.request:
    url: https://example.com
EOF
$ mooncake validate -c p.yml
Error: p.yml
  Line 3: Unknown field 'url'. Check spelling or remove this field
Found 1 error(s)
❌ Validation failed
```

The internal `schema.json` node confirms the gap:

```python
$ python3 -c '
> import json
> with open("internal/config/schema.json") as f: s=json.load(f)
> hr=s["definitions"]["http.request"]
> print(hr)'
{'type': 'object', 'properties': {}, 'required': [], 'additionalProperties': False}
```

`additionalProperties: false` with no listed properties means
**every field** the user writes — `url`, `method`, `headers`, `body`,
`json`, anything — is rejected by the schema validator before the
step ever reaches the handler.

The action's tests (`internal/actions/http_request/validate_test.go`)
exercise the Go-level `Validate()` directly. None of them go
through the YAML→schema-validate→handler path, so the breakage was
invisible to CI.

## Why it's a bug

The action is in the registry. `mooncake actions list` shows it.
`mooncake explain http.request` resolves it. The wave-1 commit
message advertises "kernel-honest HTTP action with idempotency
contract". But there is no way to *use* it from a playbook —
every YAML invocation fails at validate time with a confusing
"Unknown field 'url'" error that points the user at the wrong
fix (it's not a typo; the schema is empty).

This is a regression of the same shape generator.go's inline
comment (lines 609-612) already calls out:

```go
// Newly-wired up; previously missing from the table which meant
// the schema generator emitted an empty `{type: object,
// additionalProperties: false}` for them — and any YAML plan
// using these actions failed JSON-schema validation at apply time.
"os.firewall":            &config.OsFirewall{},
"os.group":               &config.OsGroup{},
"os.mount":               &config.OsMount{},
"os.systemd":             &config.OsSystemd{},
"text.patch.json":        &config.TextPatchJSON{},
"text.patch.yaml":        &config.TextPatchYAML{},
"windows.firewall_rule":  &config.WindowsFirewallRule{},
```

Six actions previously had this exact bug, all batch-fixed by
adding their entries here. Proposal-16 wave 1 brought the count
back up by one.

## Adjacent — the wider schemagen-table audit

`comm -23` of the registered actions (`action:"..."` tags in
`internal/config/config.go`) against the schemagen keys returns
6 names not in the table:

```
artifact.capture
artifact.validate
http.request   ← the new break
import         ← plan construct, not a real action — different surface
vars           ← pseudo-action — different surface
vars.load      ← pseudo-action — different surface
```

- `import`, `vars`, `vars.load` are plan-level constructs handled
  via a different YAML path. They aren't really actions in the
  schemagen sense; not a bug.
- `artifact.capture` and `artifact.validate` have empty
  `schema.json` nodes for the same root cause. Their YAML
  invocations don't blow up in the same way because the
  YAML→Go-struct unmarshaler rejects unknown fields *first*
  (against the struct's yaml tags), masking the missing schema
  validation. So the artifact.* actions are functionally OK in
  practice but lack their declared validation contract. Weaker
  failure mode; same root cause.

A single fix lands the http.request line plus the artifact.*
lines and closes the entire pattern. Recommend:

```go
"artifact.capture":  &config.ArtifactCapture{},
"artifact.validate": &config.ArtifactValidate{},
"http.request":      &config.HTTPRequest{},
```

## Why `task ci` schema-check missed it

`schema-check` (`Makefile:schema-check`) regenerates `schema.json`
and diffs it against the committed copy:

```
@./out/mooncake schema generate --format json --output .tmp/schema-check/schema.json --strict
@diff -q internal/config/schema.json .tmp/schema-check/schema.json
```

The check verifies "is the file in sync with the generator?" — not
"does the generator know about every registered action?". If both
sides are wrong in the same way (the action missing from the
generator AND the schema.json being empty for that action), the
diff is clean.

This is a coverage gap in the gate, not in the regeneration.
Worth a separate ticket / a second check that audits
`action:"…"` tag coverage against the schemagen table. Out of
scope for the F046 fix itself.

## Suggested fix

Two parts. The narrow fix unblocks http.request:

```go
// internal/schemagen/generator.go, in the action→type map:
"http.request": &config.HTTPRequest{},
```

The wider cleanup (one PR or two; both small):

```go
"artifact.capture":  &config.ArtifactCapture{},
"artifact.validate": &config.ArtifactValidate{},
```

Followed by `task regen` (regenerate `schema.json` + `mooncake.d.ts`
+ docs) and `task schema-check` (now compares populated → populated).

To stop this regression from recurring, add a Go test in
`internal/schemagen/generator_test.go`:

```go
// TestSchemaGen_AllRegisteredActionsHaveStructEntries: every action
// the registry surfaces via `actions list` must have a struct entry
// in generator.go's action→type map. Without one the JSON-schema
// node is empty and YAML plans using the action fail validation.
func TestSchemaGen_AllRegisteredActionsHaveStructEntries(t *testing.T) {
    registered := actions.AllNames()    // or registry.List()
    table := generatorActionTypeMap()   // exposed for testing
    for _, name := range registered {
        if _, ok := skipList[name]; ok { continue } // import / vars / vars.load
        if _, ok := table[name]; !ok {
            t.Errorf("action %q is registered but missing from schemagen table — its schema.json node will be empty and YAML plans using it will fail validation. Add an entry to internal/schemagen/generator.go.", name)
        }
    }
}
```

## Verification

- After fix, `mooncake explain http.request` shows the full schema
  block (url, method, headers, body, json, form, file,
  idempotency_key, creates_when, risk, expect_status, retries,
  retry_on, retry_delay, timeout).
- `mooncake validate -c <playbook-with-http.request>` passes
  when fields are correct, rejects unknown fields with the right
  error message.
- The wider audit test (proposed above) fails before the fix,
  passes after — exactly one assertion failure per missing entry.

## References

- `internal/schemagen/generator.go:580-620` — the action→type
  lookup table this finding is about.
- `internal/config/config.go:1267-1340` — `HTTPRequest` struct
  with full yaml/json tags, ready to be reflected.
- `internal/actions/http_request/handler.go:78-94` —
  Metadata() block; the action is otherwise complete.
- Surfaced during the 2026-05-17 tester pass against master
  @ 90d43a35, immediately after proposal-16 wave 1 landed at
  cdec9bb3.

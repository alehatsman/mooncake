# Plan: `config-json-input` — accept JSON as 2nd config input format

**Branch:** `worktree-config-json-input`
**Worktree:** `/home/aleh/projects/mooncake-config-json-input`
**Claim slug:** `config-json-input`
**Motivation:** LLM-emitted configs (pilot output) cost fewer tokens in compact JSON
than YAML, and pilots make fewer syntax mistakes (no indentation rules). All five
real config-input sites already decode user-supplied bytes; we add format detection
at each boundary. Internal re-encoding paths (e.g. strict-mode revalidation in
`reader.go:265`) stay YAML — they operate on `yaml.Node`, never on source bytes.

---

## Files

| Path | Action | Why |
|---|---|---|
| `internal/config/format.go` | **add** | New `decodeAuto(data []byte, dst any) error` + `decodeAutoNode(data []byte) (*yaml.Node, error)` helpers — sniff first non-ws byte; `{`/`[` → JSON path, else YAML |
| `internal/config/format_test.go` | **add** | Sniff-correctness table tests; JSON/YAML/whitespace/BOM/empty cases |
| `internal/config/reader.go:65` | **modify** | `ReadConfigWithValidation` — read file into bytes, call `decodeAutoNode` instead of direct `yaml.NewDecoder(f).Decode(&rootNode)` |
| `internal/config/reader.go:445` | **modify** | `ReadVariables` — call `decodeAuto(data, &variables)` instead of `yaml.NewDecoder(reader).Decode(&variables)` |
| `internal/presets/loader.go:97` | **modify** | `LoadPreset` — `decodeAuto(data, &preset)` instead of `yaml.Unmarshal(data, &preset)` |
| `internal/presets/loader.go:171` | **modify** | `LoadPresetFromPath` — same swap; also honors `.json` extension for filename-stem default |
| `cmd/step.go:89` | **modify** | Inline `--step` parser — sniff `raw` first; JSON path uses `json.Decoder` with `DisallowUnknownFields()` to preserve MT-83 strict-mode contract |
| `internal/config/reader_test.go` | **modify** | Add a parallel JSON-input case per existing YAML test that covers a public input path (reader + preset loader) |
| `internal/presets/loader_test.go` | **modify** | Add JSON variants for `LoadPreset` / `LoadPresetFromPath` happy-path tests |
| `cmd/step_test.go` | **modify** | Add `--step '{"shell":{"cmd":"echo hi"}}'` case alongside existing YAML cases |
| `docs-next/development/` (one short page) | **add** | One-page "input formats" reference: detection rules, YAML primary / JSON secondary, what `read_yaml` (data action) is *not* |

**Out of scope** (explicit non-goals — reject if scope-creep questions arise):
- `read_yaml` / `text_patch_yaml` user-facing data actions
- TOML, JSON5, HJSON, CUE, Jsonnet, binary formats
- Schema regeneration (`schema.json` validates JSON inputs as-is)
- New CLI flags — detection is automatic
- Pilot's emission format default (separate follow-up, see Open questions §1)

---

## Shape

### Detection rule (single source of truth, lives in `format.go`)

```go
// detectFormat returns "json" if the first non-whitespace byte is '{' or '[',
// "yaml" otherwise. Empty input returns "yaml" so existing empty-file
// diagnostics keep firing.
func detectFormat(data []byte) string {
    for _, b := range data {
        switch b {
        case ' ', '\t', '\n', '\r':
            continue
        case '{', '[':
            return "json"
        default:
            return "yaml"
        }
    }
    return "yaml"
}
```

### Round-trip strategy for typed decodes (preset, variables, step)

JSON path unmarshals into a generic `any`, re-encodes through `yaml.Marshal`,
and re-decodes via `yaml.Unmarshal` into the typed destination. This costs
one extra encode/decode pass but means **zero struct-tag changes** across the
codebase — every existing `yaml:"..."` tag keeps working. Bench cost is
negligible at config-read scale (one-shot, microseconds).

```go
func decodeAuto(data []byte, dst any) error {
    if detectFormat(data) == "json" {
        var generic any
        if err := json.Unmarshal(data, &generic); err != nil {
            return fmt.Errorf("parse JSON: %w", err)
        }
        buf, err := yaml.Marshal(generic)
        if err != nil {
            return fmt.Errorf("re-encode JSON as YAML: %w", err)
        }
        return yaml.Unmarshal(buf, dst)
    }
    return yaml.Unmarshal(data, dst)
}
```

### Node-tree variant for the main config reader

`ReadConfigWithValidation` needs `*yaml.Node` (not a typed struct) because
the location map, secret-tag substitution, and shape detector all walk the
node tree. Same round-trip:

```go
func decodeAutoNode(data []byte) (*yaml.Node, error) {
    if detectFormat(data) == "json" {
        var generic any
        if err := json.Unmarshal(data, &generic); err != nil {
            return nil, fmt.Errorf("parse JSON: %w", err)
        }
        buf, err := yaml.Marshal(generic)
        if err != nil {
            return nil, fmt.Errorf("re-encode JSON as YAML: %w", err)
        }
        var root yaml.Node
        if err := yaml.Unmarshal(buf, &root); err != nil {
            return nil, err
        }
        return &root, nil
    }
    var root yaml.Node
    dec := yaml.NewDecoder(bytes.NewReader(data))
    if err := dec.Decode(&root); err != nil {
        return nil, err
    }
    return &root, nil
}
```

**Source-location fidelity caveat:** JSON inputs produce a `yaml.Node` whose
line/column point into the *re-encoded* YAML buffer, not the user's original
JSON file. Diagnostics from JSON-sourced configs will still report a line
number, but it won't map to the source. Acceptable for v1 — pilot-emitted
JSON is the primary use case and pilots don't read line numbers. Documented
in the new development page.

### Strict mode (MT-83 / MT-44) — preserved

The `KnownFields(true)` strict revalidation at `reader.go:252-294` operates on
the already-built `rootNode` by marshaling it back to YAML bytes. JSON input
flows through `decodeAutoNode` → arrives as a `rootNode` indistinguishable
from a YAML-sourced node → strict mode runs unchanged. Same applies to
`cmd/step.go` — the JSON branch uses `json.Decoder.DisallowUnknownFields()`
to keep the same contract at the inline boundary.

### Detection examples

| Input first chars | Decoded as | Notes |
|---|---|---|
| `- shell:` | YAML | Existing behavior, unchanged |
| `steps:\n  - shell:` | YAML | Existing behavior, unchanged |
| `{"steps":[...]}` | JSON | New — object form |
| `[{"shell":...}]` | JSON | New — array form (matches old top-level list shape) |
| `   {...}` (leading ws) | JSON | Whitespace tolerated |
| `# comment\n- shell:` | YAML | `#` is non-ws non-bracket → YAML |
| `` (empty) | YAML | Existing empty-file diagnostic fires |

---

## Validation

- [ ] `task ci` passes in the worktree (lint + test + schema)
- [ ] `mooncake apply` accepts a `.json` config file end-to-end (manual smoke test)
- [ ] `mooncake apply` still accepts every existing `.yml` example unchanged (regression — run `task test-examples` or equivalent)
- [ ] `mooncake step '{"shell":{"cmd":"echo hi"}}'` runs and prints `hi`
- [ ] `mooncake step '{"shell":{"cmd":"x"},"bogus_field":1}'` errors with "field unknown" (strict-mode preserved)
- [ ] A preset file authored as `presets/foo.json` loads via `use: foo`
- [ ] Variables file authored as `vars.json` loads via `--vars vars.json`
- [ ] Existing test suite green (no regressions in `internal/config/`, `internal/presets/`, `cmd/`)
- [ ] One new doc page in `docs-next/development/` linked from the docs index

---

## Open questions

1. **Pilot emission default** — should `mooncake pilot` switch its output from YAML to compact JSON once this lands? Token math says yes; deferred to a separate story so this PR stays small and reversible. Tracking slug: `pilot-emit-json`.

2. **`.json` extension in preset discovery** — `LoadPreset` currently searches `<name>.yml` and `<name>/preset.yml`. Should it also try `<name>.json` and `<name>/preset.json`? Recommendation: **yes, but in a follow-up** — keeps this PR's diff to the decoder swap and out of the discovery walker. Tracking slug: `preset-discovery-json`.

3. **Source-location fidelity for JSON inputs** — documented as acceptable (caveat above). Revisit only if a user files a real issue about JSON diagnostics being unreadable; the fix (parse JSON → custom node-with-positions walker) is a 200+ LOC project, not worth pre-paying.

4. **Effort estimate** — S (≤200 LOC + tests, single PR).

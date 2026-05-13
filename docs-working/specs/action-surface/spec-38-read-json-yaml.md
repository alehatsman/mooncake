# Spec 38: `read.json` and `read.yaml`

**Status:** Draft
**Epic:** [Read & Report](../../epics/epic-read-and-report.md) — deliverable D1
**Depends on:** [spec-37](spec-37-step-output-capture.md) (`CaptureInPlan` capability)
**Effort:** S (2–3 days)
**Value:** High. Closes the gap that today forces every "read this file
and check / use a value from it" flow to shell out to `jq` / `yq`,
bypassing every audit, agent, redaction, and typed-result guarantee
mooncake exists to provide.

**Design principles:** [action-design-principles.md](../../action-design-principles.md)

---

## Problem

Mooncake has rich write actions for JSON and YAML (spec-25 will add
`text.patch.{json,yaml}` on top of today's `file.template` /
`file.write`). The read counterpart is missing entirely. Consequence:

```yaml
- shell: "jq -r .version ./package.json"
  as: pkg_version
```

…is the load-bearing pattern. That step:

- Carries `Changed=true` semantics it doesn't deserve (it's a read).
- Has no plan-mode preview of the value.
- Has no redaction layer for secret-bearing files.
- Returns a string blob, not a typed value — every consumer re-parses
  with template filters.
- Cannot live in `read.command` or any contracted-read surface (Open Q4
  on the epic punted that out of scope; this spec doesn't reopen it).
- In sandboxed agent mode (Stream 2's future state), `shell:` is gone —
  and the flow above breaks completely.

This spec ships two typed readers — JSON and YAML — that close the
common cases. Other formats (TOML / INI / env / CSV / HTTP / command
output / lines) are explicitly out of scope; the epic's reality-check
already determined they're not real gaps.

---

## Goals

- **G1** Add `read.json` and `read.yaml` as tier-1 actions. Same YAML
  surface, two formats.
- **G2** Optional `query:` field — dotted/indexed path subset
  (`a.b.c`, `a[0]`, `a.b[3].c`). No wildcards, no filters. Shared
  with the future spec-25 `text.patch.{json,yaml}`.
- **G3** Optional `redact:` regex list applied to string leaves of
  the parsed value before publishing.
- **G4** Bounded reads via `max_bytes:` (default 4 MiB). Larger inputs
  fail with a clear error.
- **G5** Read-only by contract: `idempotent: true`, no `Changed=true`,
  no `FilesystemWrite` permissions, no system mutation.
- **G6** `CaptureInPlan: true` so plan-mode runs publish the read
  value into `Scope.Results` — letting downstream `when:` clauses
  branch correctly in the plan preview.

**Out of scope (deferred indefinitely):**

- `read.toml`, `read.ini`, `read.env`, `read.csv`, `read.http`,
  `read.command`, `read.lines`. Each can land later on demonstrated
  demand; speculative breadth is the trap the epic was rewritten to
  avoid.
- A full query DSL (jq, JMESPath, Rego). The path subset stays tiny.
  If jq's surface is genuinely needed, it ships as a tier-2 plugin
  (spec-31), not here.
- Streaming / chunked reads. `max_bytes` bounds in-memory reads;
  anything bigger is the user's problem.
- HTTP-source reads. `read.http` is a separate, deferred decision.
- A `register:`/`outputs:` rename of the `as:` field. Spec-37
  considered and rejected it; `as:` is the canonical capture
  keyword.

---

## Design

Three components, in dependency order. Phase 0 is the prereq the
plan agent flagged; Phase 1 + 2 are the actions themselves.

### Phase 0 — Prereqs

**`internal/pathquery/` (new package).** Top-of-tree so non-action
callers (CLI in spec-39, future spec-25 patch actions) can import
without dragging in the executor. Pure library.

```go
package pathquery

// Extract walks value v along path and returns the addressed subtree.
// Path syntax: dotted segments and integer indices (e.g.
//   "service.port", "tools[0].name", "a.b[3].c").
// No wildcards, filters, or recursion.
//
// Returns (subtree, true, nil) on found, (nil, false, nil) on
// path-miss (not an error — callers decide what to do), and
// (nil, false, err) on malformed syntax or type mismatch (e.g.
// indexing a non-array).
func Extract(v any, path string) (any, bool, error)

// Validate parses path and returns an error for unsupported syntax.
// Used at YAML validate-time so misuse is caught before any IO.
func Validate(path string) error
```

Reject at `Validate` time: `*`, `$`, `[?…]`, leading/trailing dots,
empty segments, unmatched brackets, negative indices. Error message
points at the supported subset.

**`internal/security/redact.go` (extend).** Today the redactor only
takes literal sensitive values via `AddSensitive(value string)`. Add:

```go
// AddPattern compiles and registers a regex pattern. String leaves of
// values passed to RedactValue (and substrings of strings passed to
// Redact) that match the pattern are replaced with [REDACTED].
func (r *Redactor) AddPattern(pattern string) error

// RedactValue walks an arbitrary value (any combination of map, slice,
// string, number, bool, nil) and returns a deep-redacted copy.
// String leaves are passed through Redact; non-string leaves are
// returned unchanged. Map keys are NOT redacted (key-name redaction
// is a footgun: rewriting keys breaks shape).
func (r *Redactor) RedactValue(v any) any
```

Keep the existing `AddSensitive` / `Redact` surface untouched.

### Phase 1 — `read.json`

`internal/actions/read_json/handler.go` — standard tier-1 handler
shape, mirrors `internal/actions/repo_search/handler.go`. Internals
delegate to a shared `internal/actions/read_common/` package so
`read.yaml` is byte-for-byte the same logic with one constant
swapped.

### Phase 2 — `read.yaml`

`internal/actions/read_yaml/handler.go` — sibling. Diff vs Phase 1:
`yaml.Unmarshal` instead of `json.Unmarshal`, action name string,
and the `step.ReadYAML` selector. Everything else (path query,
redaction, max_bytes, plan/apply behavior, error taxonomy) flows
through `read_common`.

### Shared YAML surface

```yaml
- read.json:
    path: ./package.json
    query: version              # optional; pathquery syntax
    max_bytes: 4194304          # optional; default 4 MiB
    redact:                     # optional; regex patterns
      - 'ghp_[A-Za-z0-9]{20,}'
      - '(?i)(token|secret)'
  as: pkg_version

- log:
    msg: "deploying v{{ pkg_version.value }}"
```

`read.yaml` is byte-identical in surface; only the action key differs.

### Result shape

Both readers attach the parsed value to `Result.Data` via `SetData`,
which flows into the registered map under the `as:` name:

```yaml
{{ pkg_version.value }}     # the extracted value (or whole document)
{{ pkg_version.found }}     # bool — false when query missed
{{ pkg_version.path }}      # absolute path that was read
{{ pkg_version.query }}     # the query string (empty if unset)
{{ pkg_version.bytes_read }} # int — bytes actually read
```

Path-miss is **not** an error — `found: false, value: nil`. Lets
downstream `when: pkg_version.found` branch cleanly. (Parse errors,
max_bytes overflows, and file-IO errors *are* errors and fail the
step.)

---

## Key files

| File | New / Mod | Change |
|---|---|---|
| `internal/pathquery/pathquery.go` | New | `Extract` + `Validate`. ~150 LoC. |
| `internal/pathquery/pathquery_test.go` | New | Scalar, nested object, array index, mixed, miss, malformed. |
| `internal/security/redact.go` | Mod | `AddPattern(pattern string) error` + `RedactValue(v any) any` walker. ~50 LoC added. |
| `internal/security/redact_test.go` | Mod | Pattern compilation, walker over map/slice/scalar, no key-rewrite invariant. |
| `internal/actions/read_common/handler.go` | New | `Opts` struct, `Read(ctx, opts) (Output, error)` core. Parser is injected so both siblings reuse the pipeline. |
| `internal/actions/read_common/handler_test.go` | New | `max_bytes` overflow, redact, query miss, parse error taxonomy. |
| `internal/actions/read_json/handler.go` | New | Tier-1 handler. Wires `json.Unmarshal` into `read_common.Read`. |
| `internal/actions/read_json/handler_test.go` | New | Validate + Run (Plan + Apply). |
| `internal/actions/read_yaml/handler.go` | New | Sibling. Wires `gopkg.in/yaml.v3` into `read_common.Read`. |
| `internal/actions/read_yaml/handler_test.go` | New | Validate + Run (Plan + Apply). |
| `internal/config/config.go` | Mod | Add `type ReadFile struct { ... }`; add `ReadJSON *ReadFile` and `ReadYAML *ReadFile` to `Step` (alongside `RepoSearch` at line 738). Update `Clone()` (line 1004 area). |
| `internal/register/register.go` | Mod | Underscore-imports for `read_json` and `read_yaml`. |
| `internal/config/schema.json` | Regen | `make schema-generate`. |
| `internal/config/schema.d`, `mooncake.d.ts` | Regen | Generated. |
| `examples/actions/read.yml` | New | Worked examples for both readers, including query, redact, `as:`, and a plan-mode demo. |
| `internal/actions/actions_test.go` | Mod | Add the two new actions to the registry-completeness assertion (~line should be obvious in-context). |

No changes to `Scope`, `executor`, or `dryrun` — spec-37 already
introduced the `CaptureInPlan` gate the readers will flip on.

---

## Go types

```go
// in internal/config/config.go
type ReadFile struct {
    Path     string   `yaml:"path"      json:"path"               plan:"path"`  // required
    Query    string   `yaml:"query"     json:"query,omitempty"`                  // pathquery syntax
    MaxBytes *int64   `yaml:"max_bytes" json:"max_bytes,omitempty"`              // default 4<<20 when nil
    Redact   []string `yaml:"redact"    json:"redact,omitempty"`                 // regex patterns
}

// in Step (alongside RepoSearch at line 738)
ReadJSON *ReadFile `yaml:"read.json" json:"read.json,omitempty" action:"read.json"`
ReadYAML *ReadFile `yaml:"read.yaml" json:"read.yaml,omitempty" action:"read.yaml"`

// in internal/actions/read_common/handler.go
type Opts struct {
    Path     string
    Query    string
    MaxBytes int64
    Redact   []*regexp.Regexp
    Parse    func([]byte, any) error // injected by the sibling handler
}

type Output struct {
    Path      string `json:"path"`
    Query     string `json:"query,omitempty"`
    Found     bool   `json:"found"`
    Value     any    `json:"value,omitempty"` // scalar | map | slice; nil when !Found
    BytesRead int64  `json:"bytes_read"`
}
```

---

## Action metadata

```go
func (Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:               "read.json",      // or "read.yaml"
        Description:        "Read a JSON file and optionally extract a value by path",
        Category:           actions.CategoryData,
        SupportsDryRun:     true,
        SupportsBecome:     false,
        EmitsEvents:        nil,              // no new event type
        Version:            "1.0.0",
        SupportedPlatforms: []string{},       // all
        RequiresSudo:       false,
        ImplementsCheck:    false,
        CaptureInPlan:      true,             // spec-37: bind value into Scope.Results in plan mode
    }
}
```

---

## Validation rules

At `Validate(step)` time:

- `path: ""` → error: `"read.{json,yaml}: path is required"`.
- `path` containing `..` outside the project root → error
  (reuse existing path-safety helper if there is one; otherwise
  use `filepath.Clean` + ancestor check).
- `max_bytes` set to `<= 0` → error.
- `query: "<malformed>"` → call `pathquery.Validate(query)`; surface
  its error with the field-name prefix.
- Each `redact` pattern compiles with `regexp.Compile` at validate
  time; report the index of the bad pattern.

---

## Plan vs apply behavior

Both handlers expose a single `Run(ctx, step)` (spec-16 contract).

**Plan and apply paths are identical** except for two things:

| Concern | Plan | Apply |
|---|---|---|
| Read the file | Yes | Yes |
| Parse + extract + redact | Yes | Yes |
| Write to `Scope.Results` via `as:` | Yes (gated by `CaptureInPlan: true` — spec-37) | Yes |
| `Result.Changed` | Always false | Always false |
| `Result.Checkable` | true | (n/a) |
| `Result.Reason` | `"would read N bytes from <path>"` (or `"read N bytes from <path>; query path missed"` when applicable) | (n/a) |
| Side-effect-free | Yes (read-only by contract) | Yes |

Reads happen in plan mode by design — they're cheap, deterministic,
and the value is what makes plan output useful for downstream
templating. (Confirms epic Open Q3 in the affirmative.)

---

## Schema regeneration

`make schema-generate`. The reflection generator picks up the new
`ReadFile` struct and the two `Step` fields via their `yaml`/`json`/
`action` tags. CI gate: `make schema-check`.

---

## Tasks

1. **Phase 0** — `internal/pathquery/`. Land alone; pure library;
   unit-test the path grammar thoroughly. Future spec-25 will import
   the same package.
2. **Phase 0** — `redact.AddPattern` + `redact.RedactValue`. Land
   alongside pathquery; ~50 LoC + tests.
3. **Phase 1** — `read.json`. Wire `json.Unmarshal` into a shared
   `read_common.Read(opts)`. Tests cover the full surface.
4. **Phase 2** — `read.yaml`. Wire `gopkg.in/yaml.v3` decoder into
   the same `read_common.Read`. Sibling-package shape.
5. **Phase 3** — `examples/actions/read.yml` and a worked example
   under `LLM_GUIDE.md` showing the `read.json → as: pkg → log` flow.
6. **Phase 4** — schema regen, `make ci` green, commit in a single
   PR so reviewers see the surface delta.

---

## Tests

| Scenario | Layer |
|---|---|
| `pathquery.Extract` — scalar root, dotted, indexed, mixed, miss, malformed | unit |
| `pathquery.Validate` — accepts supported syntax, rejects `*` / `$` / `[?…]` / negative index / empty segments | unit |
| `redact.AddPattern` — bad regex returns error; good regex applied to string leaves | unit |
| `redact.RedactValue` — walks map/slice/scalar; keys untouched; non-string leaves untouched | unit |
| `read.json` validate — missing `path` → error | unit |
| `read.json` validate — bad `query` syntax → error with hint | unit |
| `read.json` validate — bad `redact` regex → error pointing at index | unit |
| `read.json` validate — `max_bytes: 0` → error | unit |
| `read.json` apply — whole document, scalar at root, scalar at nested path, array index | handler |
| `read.yaml` apply — same matrix as JSON | handler |
| `read.json` apply — file > `max_bytes` → structured error with hint to override | handler |
| `read.json` apply — file missing → structured error | handler |
| `read.json` apply — malformed JSON → structured error with line/column | handler |
| `read.json` apply — query path miss → `Found: false, Value: nil`; no error | handler |
| `read.json` apply — `redact:` matches a string leaf → `[REDACTED]`; non-string leaves untouched | handler |
| `read.json` plan — value preview in `Result.Reason`; downstream `when: pkg.found` branches correctly | plan integration |
| `read.json` apply — `as: foo` → `{{ foo.value }}` template substitution works | e2e |
| `read.yaml` plan + apply — equivalent matrix to JSON | plan integration + e2e |
| Both readers via registry — `internal/register/register.go` registers both; `actions_test.go` lists them | registry unit |

---

## Acceptance criteria

1. `make ci` clean (vet / lint / test / schema-check).
2. This YAML, applied twice, reads cleanly both times and `pkg` is
   bound to a typed value with `value`, `found`, `path`, `query`,
   `bytes_read`:
   ```yaml
   - read.json:
       path: ./package.json
       query: version
     as: pkg
   - log:
       msg: "version is {{ pkg.value }}"
   ```
3. `mooncake plan` against the same YAML emits a preview that
   includes the read value, *and* downstream steps' `when: pkg.found`
   branches correctly during plan.
4. A YAML referencing a missing query path runs without error,
   `pkg.found` is false, and downstream `when: pkg.found` skips.
5. A YAML with a `redact: ['ghp_[A-Za-z0-9]{20,}']` pattern and a
   GitHub-PAT-shaped string in the file emits `[REDACTED]` in the
   bound value and in any subsequent log line.
6. `read.yaml` against an equivalent YAML fixture produces
   byte-identical behavior except for the input parser.
7. A file larger than `max_bytes` fails with a structured error
   suggesting an explicit `max_bytes:` override.
8. No regression in spec-37 tests (collision warning, plan-mode
   gate still respects `CaptureInPlan`).

---

## Open questions

1. **`max_bytes` default — 1 MiB or 4 MiB?** Epic Open Q5 punted to
   this spec. `package-lock.json` is routinely 5–20 MB; 1 MiB
   silently breaks the JS ecosystem's most common use case. 4 MiB
   is the proposed default. Anyone with a `package-lock.json` over
   4 MiB sets `max_bytes:` explicitly — and we add a doc example for
   that case. Confirm before merge by sampling a few real lockfiles.
2. **Path safety policy.** Epic + design principles imply path
   reads should be sandboxed, but mooncake has historically allowed
   arbitrary `path:` (it's a system-config tool, not a webapp). For
   v1, accept any path the user writes; rely on the runtime user's
   filesystem permissions. Reopen if Stream 2's sandboxed-agent
   work needs tighter scoping.
3. **YAML multi-document files (`---` separators).** `yaml.v3`
   decodes only the first document by default. Reject multi-doc
   YAML at validate-time, or read all docs and return a list?
   Recommend: reject at parse-time with a structured error pointing
   at the second `---`. Multi-doc support is a separate feature if
   demand appears.
4. **Result `Detail` for plan output.** Spec-16 leaves
   `Result.Detail` available for action-specific plan-mode data.
   Should the readers populate it with the parsed value for richer
   plan-output rendering, beyond just `Reason`? Yes for v1 — but
   bound by the same `max_bytes` so plan output doesn't explode.
   Confirm during implementation what the plan renderer does with
   `Detail`.
5. **Concurrent reads.** Two steps reading the same file in
   parallel (loop / parallel runner). Stateless, no locking needed
   — but verify no shared parser state in `read_common`. Document.
6. **YAML number fidelity.** `yaml.v3` decodes integers as `int`
   and floats as `float64` via `interface{}`. JSON's `json.Number`
   path is also available via `decoder.UseNumber()`. For
   `read.json`, opt out of `UseNumber` (return native `float64`)
   for v1; revisit if a downstream consumer needs big-integer
   fidelity. Document the choice in the action reference.

---

## Risk notes

- **`pathquery` placement.** Top-of-tree (`internal/pathquery/`)
  lets CLI (spec-39) and future spec-25 import it cleanly. Resist
  the urge to colocate it under `internal/actions/read_common/`;
  the CLI would then import an actions package, which forces every
  CLI link to drag the executor in.
- **Redact walker performance.** `RedactValue` deep-copies the
  parsed value. For a 4 MiB JSON with no `redact` patterns, this
  is wasted work. Short-circuit: when the redactor has no patterns
  and no sensitive values, return the input unchanged. Cover with
  a benchmark or a comment.
- **`gopkg.in/yaml.v3` quirks.** Decodes `null` to `interface{}(nil)`
  in some cases and to `(*yaml.Node)(nil)` in others. The decoder
  used here is the `Unmarshal` form which gives `interface{}(nil)` —
  but if `read_common` ever switches to the Node API (e.g. to
  preserve comments, à la spec-25), this can shift under it.
  Document the assumption.
- **Behavior coupling to spec-37.** This spec sets
  `CaptureInPlan: true` and relies on the executor gate. If spec-37
  ships with a bug in that gate, both readers silently fail to
  publish in plan mode. Cover with a cross-spec integration test
  that asserts plan-mode register works end-to-end for `read.json`.

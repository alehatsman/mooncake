---
id: F047
title: `mooncake actions show` strips per-field descriptions and enums — every action's "documentation" output renders as `—`
severity: bug
package: cmd
file: cmd/mooncake.go
lines: 788-857
status: done
fixed: 2026-05-17 — commit `533c15e8 fix(cmd): F047 — actions show now enriches descriptions + enums (and stamp F046 done)`. Two-part change: (a) the one-line option flip the finding called for — `StrictValidation: true` added to the GeneratorOptions literal so the schemagen's description / enum / pattern enrichment runs. (b) extracted the registry-lookup + generator-invocation body into a new helper `loadActionShowDefinition(name) (*ActionMetadata, *Definition, error)` so the regression test can drive the real lookup path. The "Suggested fix" wrapper sketch the finding asked for (`runActionsShow(&buf, "file.write", "json")`) was implemented as this helper rather than as a stdout-capture wrapper — same testability, less plumbing.
verified: 2026-05-17 — confirmed fixed on master @ 533c15e8. (a) Regression test `TestLoadActionShowDefinition_PopulatesDescriptions` exercises the real registry → schemagen pipeline and asserts that at least one of {group, owner, mode} on `file.write` has a non-empty description; with `StrictValidation: true` it passes, without it would fail (the test is asymmetric: any single enriched field passes, so it's tolerant of future schema additions but still catches the bug class). (b) Sibling test `TestLoadActionShowDefinition_UnknownActionErrors` locks in the "unknown action" error message through the same helper. Manual smoke (per the finding's suggestion): `mooncake actions show file.write` now shows "File group (groupname or GID)" / "File owner (username or UID)" on the corresponding parameter rows instead of `—`. The "Adjacent" follow-up the finding flagged (split `StrictValidation` into a separate `EnrichDescriptions` flag, or make enrichment unconditional in the generator so future callers can't hit the same trap) is deferred — explicitly out of scope per the finding itself.
---

## What

DX proposal-04 (the new `mooncake actions show <name>` verb) is
documented as "Show per-action documentation (params, platforms,
capabilities, minimum example)" but the parameter table comes out
with **no descriptions and no enum lists** — every row is rendered
as `name  type  —`:

```
$ mooncake actions show file.write
file.write
──────────
Manage files, directories, links, and permissions
...

Required parameters:
  path               string    —          ← should be "File, directory, or symlink path (required)"

Optional parameters:
  backup             boolean   —
  content            string    —
  force              boolean   —
  group              string    —          ← should be "File group (groupname or GID)"
  mode               string    —          ← should be "File permissions (e.g., '0644', '0755')"
  owner              string    —          ← should be "File owner (username or UID)"
  recurse            boolean   —
  src                string    —
  state              string    —          ← should be the long state-enum description
```

The JSON / YAML formats are missing the same fields:

```
$ mooncake actions show file.write --format json | jq '.properties.group'
{ "type": "string" }                                ← description gone
$ mooncake actions show file.write --format json | jq '.properties.state'
{ "type": "string" }                                ← description + enum list gone
```

Compare with `mooncake schema generate --format json` (default
`--strict=true`) for the same fields:

```
$ mooncake schema generate --format json | jq '.definitions["file.write"].properties.group'
{ "type": "string", "description": "File group (groupname or GID)" }
$ mooncake schema generate --format json | jq '.definitions["file.write"].properties.state'
{ "type": "string", "description": "Desired file state (file/present: ...)",
  "enum": ["file","present","absent","directory","link","hardlink","touch","perms"] }
```

Same generator, same in-memory `Definition` type — different output.

## Why

`actionsShowCommand` in `cmd/mooncake.go:828-833` invokes the
schemagen with **only two options set**:

```go
gen := schemagen.NewGenerator(schemagen.GeneratorOptions{
    IncludeExtensions: true,
    OutputFormat:      "json",
    // StrictValidation:  ← NOT SET (zero value: false)
})
```

`internal/schemagen/generator.go:552-558` gates description /
enum / pattern enrichment behind `StrictValidation`:

```go
// Apply known enums, patterns, and descriptions if enabled
if g.opts.StrictValidation {
    for fieldName, prop := range props {
        applyKnownValidation(meta.Name, fieldName, prop)
        applyEnhancedDescription(meta.Name, fieldName, prop)
    }
}
```

`cmd/schema.go` flips `StrictValidation: strictValidation` from
the `--strict` CLI flag, which **defaults to `true`** — so
`mooncake schema generate` always gets the enriched form, while
`mooncake actions show` never does.

The user-visible effect: the "documentation" subcommand strips
the documentation.

## Verification of root cause

Reproducing `schema generate --strict=false` reproduces the
`actions show` output, confirming `StrictValidation` is the
single toggle:

```
$ mooncake schema generate --format json --strict=false | jq '.definitions["file.write"].properties.group'
{ "type": "string" }                                ← matches actions show
```

## Why CI missed it

The dedicated test file `cmd/actions_show_test.go` covers
`renderActionShowText` and `formatPropertyLine`, but every test
constructs a fake `schemagen.Definition` literal with
descriptions pre-populated:

```go
def := &schemagen.Definition{
    Type:        "object",
    Description: "Copy a single file from source to destination, preserving mode.",
    Properties: map[string]*schemagen.Property{
        "src":  {Type: "string", Description: "Path to source file"},
        "dest": {Type: "string", Description: "Path to destination"},
        "mode": {Type: "string", Description: "File mode (e.g. \"0644\")"},
    },
    Required: []string{"src", "dest"},
}
```

None of the tests call `actionsShowCommand` end-to-end via the
real CLI plumbing, so the generator-option drop is invisible.
The unit tests prove the formatter works *given* a populated
Definition; the real binary never feeds it a populated one.

Same blind-spot pattern as F046 — handler-level tests pass while
the CLI surface is broken.

## Suggested fix

One line, `cmd/mooncake.go:829-833`:

```go
gen := schemagen.NewGenerator(schemagen.GeneratorOptions{
    IncludeExtensions: true,
    StrictValidation:  true,   // F047: needed for enum + description enrichment
    OutputFormat:      "json",
})
```

`StrictValidation: true` is what `schema generate` defaults to and
what `internal/config/schema.json` was generated with, so this
just brings `actions show` in line with the rest of the schema
pipeline.

Consider also: should `StrictValidation` be renamed or split?
The flag's name suggests it controls validation strictness
(oneOf, additionalProperties) — and it *does* — but it also
gates description/enum enrichment, which is documentation,
not validation. That double meaning is the trap that caught
this code. A separate `EnrichDescriptions bool` (or just making
the enrichment unconditional in the generator) would prevent
the same mistake on the next caller. Out of scope for the F047
fix itself; flagged for a separate cleanup.

## Verification (regression test)

Add an end-to-end test in `cmd/actions_show_test.go` that exercises
the full command, not just the formatter:

```go
func TestActionsShowCommand_PopulatesDescriptions(t *testing.T) {
    var buf bytes.Buffer
    err := runActionsShow(&buf, "file.write", "json")  // small wrapper
    if err != nil {
        t.Fatalf("actions show: %v", err)
    }
    var doc map[string]any
    if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
        t.Fatal(err)
    }
    props := doc["properties"].(map[string]any)
    group := props["group"].(map[string]any)
    if group["description"] == nil {
        t.Errorf("group.description was dropped — actions show is not invoking the generator with StrictValidation: true (F047)")
    }
}
```

Manual: after fix, `mooncake actions show file.write` shows
"File, directory, or symlink path (required)" and "File group
(groupname or GID)" in the parameter rows.

## Adjacent — the same generator-option trap

The schemagen generator currently couples six concerns into
one flag (`StrictValidation`): oneOf hoisting, additionalProperties,
enum enrichment, pattern enrichment, description enrichment, and
required-list ordering. Any future caller that wants any one of
those without the others will hit the same trap `actions show` did.
Worth a generator-options-API ticket as a follow-up — out of
scope here.

## References

- `cmd/mooncake.go:788-857` — `actionsShowCommand`; the
  `GeneratorOptions` literal at 829-833 is the bug site.
- `internal/schemagen/generator.go:552-558` — the
  `StrictValidation` gate that controls enrichment.
- `cmd/schema.go:119-125` — the parallel command that sets
  `StrictValidation: strictValidation` correctly.
- `cmd/actions_show_test.go:18-160` — existing tests that pass
  because they pre-populate descriptions instead of going
  through the real path.
- Surfaced during the 2026-05-17 tester pass against master
  @ 8bc5af4c.

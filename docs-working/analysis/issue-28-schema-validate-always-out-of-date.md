# Bug — `mooncake schema validate --schema X` always reports out-of-date, even for freshly-generated X

**Surfaced:** 2026-05-15 during tick-9 of the autonomous test loop —
docs/schema sanity sweep.

## Repro

```
$ mooncake schema generate --output /tmp/schema.json
✓ Generated json schema to /tmp/schema.json

$ mooncake schema validate --schema /tmp/schema.json
✗ Schema is out of date
  Run 'mooncake schema generate' to update
$ echo $?
1
```

Two consecutive invocations of `mooncake schema {generate, validate}` —
no edit, no race, no time gap — and `validate` insists the just-saved
schema is stale.

## Root cause

Two code paths produce the schema bytes; they format differently and
then `validate` byte-compares them.

### Path 1 — `schema generate --output FILE`

`internal/schemagen/writer.go:52`:

```go
func (w *Writer) writeJSON(schema *Schema, out io.Writer) error {
    encoder := json.NewEncoder(out)
    encoder.SetIndent("", "  ")
    encoder.SetEscapeHTML(false)
    return encoder.Encode(schema)
}
```

Produces **pretty-printed JSON** with 2-space indent and HTML escaping
disabled.

```
$ head -c 50 /tmp/schema.json | xxd | head -1
00000000: 7b0a 2020 2224 7363 6865 6d61 223a 2022  {.  "$schema": "
```

→ `{\n  "$schema": "...`

### Path 2 — `schema validate --schema FILE`

`cmd/schema.go:218`:

```go
currentData, err := currentSchema.MarshalJSON()
...
if string(schemaData) == string(currentData) {
    fmt.Println("✓ Schema is up to date")
    return nil
}
fmt.Println("✗ Schema is out of date")
```

And `Schema.MarshalJSON` (`internal/schemagen/generator.go:653`):

```go
func (s *Schema) MarshalJSON() ([]byte, error) {
    type SchemaAlias Schema
    return json.Marshal((*SchemaAlias)(s))  // compact, escapes HTML
}
```

Produces **compact JSON** with HTML-escape on, e.g. `{"$schema":"...`.

Compact bytes never equal pretty bytes, so the byte comparison
guarantees a mismatch.

## Why this matters

`mooncake schema validate` is documented as the way to verify the
embedded `internal/config/schema.json` (used by editors / LSPs for
YAML validation) is in sync with the action registry. In CI, this is
the natural way to catch "someone added a new action field but forgot
to regenerate the schema." With this bug, the check is permanently
broken — it always reports out-of-date, so either:

1. CI is set to ignore the command and the staleness check never
   actually runs (and a stale schema can slip through).
2. CI runs `schema generate && git diff --exit-code` against
   `internal/config/schema.json` instead, in which case `schema
   validate` is dead code.

Either way, the documented command (`mooncake schema validate
--schema schema.json`) does the wrong thing.

## Fix

Three plausible fixes, in order of smallest delta:

### A. Make `validate` use the same writer path

```go
// In validateSchemaAction
var buf bytes.Buffer
writer := schemagen.NewWriter("json")
if err := writer.Write(currentSchema, &buf); err != nil { ... }
currentData := buf.Bytes()
```

Now both halves use `writeJSON` and produce byte-identical output.

### B. Compare parsed JSON, not raw bytes

```go
var lhs, rhs map[string]any
if err := json.Unmarshal(schemaData, &lhs); err != nil { ... }
if err := json.Unmarshal(currentData, &rhs); err != nil { ... }
if reflect.DeepEqual(lhs, rhs) { /* up to date */ }
```

This is more robust to whitespace differences across writer
configurations and would tolerate operators hand-formatting the
schema (e.g. running it through `jq .` for readability).

### C. Show a diff on mismatch

Today the error message is `✗ Schema is out of date / Run 'mooncake
schema generate' to update`. With Option A or B in place, an actual
mismatch (a real out-of-date case) deserves a diff so the operator
can see *what* changed. Today they'd have no signal.

## Test gap

`internal/schemagen/generator_test.go` tests `MarshalJSON` and
`MarshalPrettyJSON` independently but doesn't have an end-to-end
test that the validate-command path matches the generate-command
path. A two-step test would catch this:

```go
// pseudo-test
schema, _ := generator.Generate()
var buf bytes.Buffer
writer.Write(schema, &buf)
saved := buf.Bytes()

current, _ := schema.MarshalJSON()
require.True(t, bytes.Equal(saved, current),
    "validate-compare path must produce same bytes as write path")
```

## Workaround

Operators currently have to do this dance instead:

```bash
# CI staleness check
mooncake schema generate --output /tmp/fresh.json
diff /tmp/fresh.json internal/config/schema.json
```

…and ignore `mooncake schema validate` entirely.

## Related observation

`mooncake docs generate --section platform-matrix` also has a small
output bug — the markdown separator row has a trailing `||` (extra
empty column):

```
| Action | Linux | macOS | Windows | FreeBSD |
|--------|-------|-------|-------|-------||      ← extra | at end
```

`internal/docgen/markdown.go:29`:

```go
write(w, "|--------|%s|\n", strings.Repeat("-------|", len(platforms)))
```

`strings.Repeat("-------|", 4)` already supplies the trailing pipe;
the format-string's own trailing `|` doubles it. Fix: drop the
trailing `|` in the format string. Filing here as a related minor
finding rather than a separate issue — same docs-generator surface,
trivial fix.

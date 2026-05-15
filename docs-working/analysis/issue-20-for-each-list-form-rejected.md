# Bug — `for_each: [list]` inline form rejected by schema; error message generic-empty

**Tracking:** [#20](https://github.com/alehatsman/mooncake/issues/20)
**Surfaced:** 2026-05-15 during the control-flow test sweep.

## Repro

```yaml
# control.yml — fails at plan time
- name: simple
  shell:
    cmd: echo "{{ item }}"
  for_each: [a, b]
```

```
$ mooncake apply -c control.yml
2026/05/15 20:32:24 planner setup failed: failed to build plan:
  configuration validation failed:

Error: control.yml

  Line 4: Step must have exactly one action ()
    for_each: [a, b]
    (in step: simple)

Found 1 error(s)
```

Note the `()` in the error — the list of "valid actions" comes out
empty. Both the rejection AND the error message are problems.

The variable-reference form works fine:

```yaml
- vars:
    items: [a, b, c]
- name: ok
  log:
    msg: "iter {{ item }}"
  for_each: "{{ items }}"     # passes; iterates 3 times
```

## Root cause

Two separate issues stacked.

### Schema mismatch (the rejection)

`config.go:1485` declares `ForEachField` with a custom
`UnmarshalYAML` that accepts both forms:

```go
type ForEachField struct {
    Expr  string         // scalar (template variable expression)
    Items []interface{}  // inline or block sequence
}

func (f *ForEachField) UnmarshalYAML(...) error {
    // tries scalar first, then sequence
}
```

The struct comment even documents the three valid forms:

```yaml
for_each: items_var          # scalar — variable reference
for_each: [a, b, c]          # inline sequence — literal list
for_each:                     # block sequence — literal list
  - a
  - b
```

But the generated `internal/config/schema.json` only declares:

```json
"for_each": {
  "type": "string",
  "description": "Variable expression for iterating over items (universal)"
}
```

No `oneOf` / no array branch. The JSON-schema validator runs
*before* the YAML unmarshal reaches `ForEachField`, so the literal
list form gets bounced at validation.

### Error message regression (the misleading text)

When the validator rejects a step for type-of-a-universal-field
mismatch, the fallback path falls into `formatOneOfError` in
`internal/config/error_messages.go` and renders "Step must have
exactly one action (...)". Recent MT-27 work tightened the
validator vocabulary to track the registry, and apparently the
"valid actions" string is now sourced from a registry that returns
empty in this code path — so the message renders `()` with no
contents.

The user sees a message that says "no action present" when the
real problem is "for_each value isn't a string". Two layers wrong.

## Fix

### Schema (the rejection)

`internal/schemagen/` should emit `for_each` as a `oneOf` covering
both forms:

```json
"for_each": {
  "oneOf": [
    {
      "type": "string",
      "description": "Variable expression rendered through templates and resolved to a list"
    },
    {
      "type": "array",
      "items": { "type": ["string", "number", "boolean", "object"] },
      "description": "Inline literal list of items to iterate"
    }
  ]
}
```

The same treatment likely applies to other dual-form fields —
`for_each_file`, possibly more. A quick `grep -l 'UnmarshalYAML'
internal/config/*.go` followed by reading each custom unmarshaler
will identify the gaps.

### Error message (the misleading text)

The "Step must have exactly one action ()" path needs:

1. To not be reached when the error is "type mismatch on universal
   field" (a more specific error message should win).
2. When it *is* reached, to never render with an empty action
   list. If the registry lookup returns empty, fall back to the
   hand-maintained string in `error_messages.go` so operators see
   *something* useful.

## Workaround

Wrap the literal list in a `vars` step + reference it by name:

```yaml
- vars:
    items: [a, b, c]
- name: ok
  shell:
    cmd: echo "{{ item }}"
  for_each: "{{ items }}"
```

Costs one extra step but otherwise functionally identical.

## Related — `with_items` is documented but doesn't exist

While diagnosing this, I also discovered `with_items` is
referenced as a control-flow primitive in several places:

- `internal/config/config.go:1273` struct comment: `// Optional
  control flow: with_items, with_filetree`
- `docs-next/examples/actions/download.yml` (active example)
- `docs-next/examples/actions/README.md`
- `docs-next/index.md`
- `docs-next/api/config.md`
- `docs-next/about/changelog.md`

But it's NOT a field on the Step struct (only `for_each` and
`for_each_file` are). Plans using `with_items` get a different
validator error ("unknown field `with_items`"). This is a
separate documentation-rot bug. Filing it inline rather than in
its own ticket would muddle the story; recommend a quick
follow-up sweep + grep to mass-replace `with_items` → `for_each`
in docs + struct comments.

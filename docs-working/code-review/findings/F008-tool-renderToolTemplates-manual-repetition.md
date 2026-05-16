---
id: F008
title: tool.renderToolTemplates manually rendering 9 fields with copy-paste error guards
severity: readability
package: internal/actions/tool
file: internal/actions/tool/handler.go
lines: 231-269
status: done
resolved: 2026-05-16 — replaced the 9× copy-paste render-and-wrap block in `renderToolTemplates` with a `{name, src, *dst}` table + `for` loop. Adding a new templatable field is now one struct-literal row instead of an if/wrap/assign paste. Error semantics preserved: each field still surfaces as `<field>: <render err>` in declaration order. `specFromConfig` (lines 208-224) is the assignment-only sibling and is left untouched — no error handling to drop, and turning a 9-line copy block into a 9-line table would be churn without payoff.
verified: 2026-05-16 — table-driven loop replaces 9× if/render/assign blocks; one row per templatable field at handler.go:252-266
---

## What

`renderToolTemplates` (line 231-269) renders 9 string fields off
`*config.Tool` using the same 3-line idiom 9×:

```go
if cp.Name, err = render(t.Name); err != nil {
    return nil, fmt.Errorf("name: %w", err)
}
if cp.Version, err = render(t.Version); err != nil {
    return nil, fmt.Errorf("version: %w", err)
}
// ... 7 more
```

That's 30 lines of boilerplate where the only varying pieces are
the field name + the receiving struct field. Adding a 10th
templatable field means another paste.

## Why it's a `readability` finding (and only that)

It works, it has good error context (field name in the wrap), and
the linear order matches the field order on `config.Tool`. The
smell is purely "could be data-driven."

There's no bug — but the next agent who adds a templatable field
(e.g. `Sha256URL` for a checksum-from-URL feature) will probably
forget one of: the assignment, the error wrap, or both. That's a
real future bug surface.

## Suggested fix

Drive the field list from a table:

```go
fields := []struct {
    name string
    src  string
    dst  *string
}{
    {"name", t.Name, &cp.Name},
    {"version", t.Version, &cp.Version},
    {"url", t.URL, &cp.URL},
    {"repo", t.Repo, &cp.Repo},
    {"asset", t.Asset, &cp.Asset},
    {"tag", t.Tag, &cp.Tag},
    {"checksum", t.Checksum, &cp.Checksum},
    {"bin", t.Bin, &cp.Bin},
    {"mise_tool", t.MiseTool, &cp.MiseTool},
}
for _, f := range fields {
    rendered, err := render(f.src)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", f.name, err)
    }
    *f.dst = rendered
}
return &cp, nil
```

Trade: ~10 LOC saved, plus a future-add becomes a single struct
literal. The downside is a tiny indirection cost (irrelevant in
practice — this runs once per step).

Same shape applies to `specFromConfig` (line 208-224), which has
the same paste-9-fields pattern but without error returns.

## Verification

- `go test ./internal/actions/tool/...`
- Compare error messages on a deliberately-broken template (e.g.
  `version: "{{ .nope"` → should still say `version: ...`).

## References

- Same pattern hides in `internal/config/normalize.go` (long ago
  found) — if that package gets touched, audit there too.

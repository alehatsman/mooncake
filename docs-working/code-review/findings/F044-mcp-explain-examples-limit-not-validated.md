---
id: F044
title: MCP `explain` tool — `examples_limit` is not validated against its declared `inputSchema` bounds (min 0, max 10)
severity: bug
package: internal/mcp
file: internal/mcp/tools.go
lines: 128-139, 456-486
status: fixed
---

## What

The MCP `explain` tool advertises bounded `examples_limit` input in
its `inputSchema` (`internal/mcp/tools.go:133-138`):

```go
"examples_limit": map[string]interface{}{
    "type":        "integer",
    "minimum":     0,
    "maximum":     10,
    "description": "Cap on example excerpts returned for kind:action. Default 3.",
},
```

`HandleExplain` (`internal/mcp/tools.go:461-486`) plumbs the value
through unchanged:

```go
var params struct {
    Noun          string `json:"noun"`
    ExamplesLimit int    `json:"examples_limit"`
}
// ...
result := explain.Resolve(params.Noun, explain.Options{
    ExamplesLimit: params.ExamplesLimit,
})
```

`findExamples` in `internal/explain/examples.go:22-26` treats any
non-positive value as "use default":

```go
limit := opts.ExamplesLimit
if limit <= 0 {
    limit = 3
}
```

Nothing on the path between `tools/call` and `findExamples` enforces
the declared `maximum: 10` or `minimum: 0`. Observed behavior:

| Input `examples_limit` | Examples returned |
| --- | --- |
| `0`  (default, schema-valid) | 3   |
| `-1` (schema-invalid)         | 3   (silently swallowed) |
| `11` (schema-invalid)         | 11  (cap is ignored) |

Reproduced via stdio MCP:

```
{"jsonrpc":"2.0","id":22,"method":"tools/call","params":{"name":"explain","arguments":{"noun":"file.write","examples_limit":11}}}
```

Response payload has `len(action.examples) == 11`.

## Why it's a bug

Three reasons it deserves a fix rather than a docs tweak:

1. **The wire schema is the contract.** The point of advertising
   `inputSchema` to MCP clients is so the client can refuse to send
   bad arguments and so the server promises a bounded response. We
   declare `maximum: 10` but the handler will happily return 100 if
   asked. A well-behaved MCP client that pre-validates against the
   schema will never see the discrepancy; a less strict client (or
   a direct agent call) can blow past the cap.
2. **The advertised cap exists for a reason.** The doc comment in
   `findExamples` says "The cap protects MCP-tool output budget;
   the agent can re-call with a higher limit." Today there is no
   cap and no re-call mechanism — `examples_limit: 1000` works and
   ships 1000 excerpts in one response.
3. **Negative values silently degrade.** `examples_limit: -1` is
   schema-invalid but returns 3 (default), so a client passing a
   sentinel "skip this option" value gets the default it didn't
   ask for. The current `if limit <= 0` branch conflates "unset"
   (zero value from omitempty) with "explicitly negative".

The CLI side has the same gap — `--examples-limit -1` returns 3,
`--examples-limit 999` returns 999 — but it's less load-bearing
there because the CLI doesn't publish a schema.

## Adjacent — no JSON-Schema validation in the MCP dispatcher

The dispatcher (`internal/mcp/dispatch.go`, or wherever
`tools/call` routes) does not run incoming arguments through a
JSON-Schema validator before handing them to the handler. Every
handler does ad-hoc field checks (`if params.Config == ""`,
`if params.Noun == ""`), but range and enum constraints declared
in the schema are advisory only.

Whether to add a generic validator is a bigger call (it changes the
contract for every existing tool); the narrow fix for this finding
is per-handler clamping.

## Suggested fix

Per-handler clamp + reject in `HandleExplain`:

```go
const explainExamplesLimitMax = 10

func HandleExplain(_ context.Context, args json.RawMessage) (string, error) {
    var params struct {
        Noun          string `json:"noun"`
        ExamplesLimit int    `json:"examples_limit"`
    }
    if len(args) > 0 {
        if err := json.Unmarshal(args, &params); err != nil {
            return "", fmt.Errorf("invalid arguments: %w", err)
        }
    }
    if strings.TrimSpace(params.Noun) == "" {
        return "", fmt.Errorf("noun parameter required")
    }
    if params.ExamplesLimit < 0 {
        return "", fmt.Errorf("examples_limit must be >= 0")
    }
    if params.ExamplesLimit > explainExamplesLimitMax {
        params.ExamplesLimit = explainExamplesLimitMax
    }
    // ... rest unchanged
}
```

Two questions for the fixer:

- **Should over-cap be a hard error or a silent clamp?** Hard
  error matches schema semantics ("you asked for something the
  schema forbids"); silent clamp is friendlier to agents that
  optimistically pass `100`. The codebase leans toward hard
  errors for argument validation (see `noun parameter required`
  on the same handler) — recommend reject.
- **Should `0` keep meaning "default 3", or start meaning "zero
  examples please"?** Today `0` → 3 because of the `<= 0` check
  in `findExamples`. If `0` is reserved as "default", the schema
  description should say so; if `0` should mean literally zero,
  `findExamples` needs `if limit < 0 { limit = 3 }`. The schema
  declaring `minimum: 0` implies `0` is a valid request, which
  argues for "zero means zero". Pick one and document it.

A second, smaller cleanup: lift `explainExamplesLimitMax = 10` to
a package-level const and reference it from both the inputSchema
literal and the handler, so the two can't drift.

## Verification

Two table tests in `internal/mcp/tools_test.go`:

```go
func TestHandleExplain_ExamplesLimit_OverMaxRejected(t *testing.T) {
    _, err := HandleExplain(context.Background(),
        json.RawMessage(`{"noun":"file.write","examples_limit":11}`))
    if err == nil || !strings.Contains(err.Error(), "examples_limit") {
        t.Fatalf("expected rejection, got %v", err)
    }
}

func TestHandleExplain_ExamplesLimit_NegativeRejected(t *testing.T) {
    _, err := HandleExplain(context.Background(),
        json.RawMessage(`{"noun":"file.write","examples_limit":-1}`))
    if err == nil || !strings.Contains(err.Error(), "examples_limit") {
        t.Fatalf("expected rejection, got %v", err)
    }
}
```

Plus a manual stdio smoke against the binary to confirm the
`-32000` error round-trips through the dispatcher unchanged.

## References

- `internal/mcp/tools.go:128-139` — schema declaration with the
  unused bounds.
- `internal/mcp/tools.go:461-486` — handler with no clamp.
- `internal/explain/examples.go:22-26` — the `<= 0` default
  branch that swallows negative values.
- Spec-68 §"MCP wave 3" — wave 3's scope was tool registration +
  delegation to `explain.Resolve`; bounds enforcement was
  implicit in the inputSchema but never wired.

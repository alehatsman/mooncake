---
id: F040
title: llm.ClaudeClient — 60s httpClient.Timeout truncates long generations; default model is stale; response body unbounded
severity: smell
package: internal/pilot/llm
file: internal/pilot/llm/claude_client.go
lines: 14-19, 36-39, 42-45, 81-84
status: done
verified: 2026-05-16 — confirmed real on master @ 649c71f4 after agent→pilot rename. internal/pilot/llm/claude_client.go:16 defaultTimeout=60s, :44 model="claude-sonnet-4-20250514" (stale; current is sonnet-4-6 or opus-4-7), :81 io.ReadAll unbounded body read. Paths refreshed
post-fix verified: 2026-05-16 on master @ c328abbd — claude_client.go:60 drops httpClient.Timeout; line 27 defaultModel=claude-sonnet-4-6; line 107 io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
---

## What

Three observations in `claude_client.go`:

### (a) 60-second client-level timeout

```go
const (
    defaultTimeout = 60 * time.Second
    // ...
)

return &ClaudeClient{
    apiKey:   apiKey,
    endpoint: claudeAPIEndpoint,
    httpClient: &http.Client{
        Timeout: defaultTimeout,                    // ← 60s overall request cap
    },
}, nil
```

`http.Client.Timeout` is the **total time** for the request,
including connection setup, request write, response read, and
body. The httpClient sets it to 60s for every request.

For Claude API non-streaming responses, 60s is fine for a 4 KB
prompt that returns a 1 KB plan. But:

- **Sonnet 4.6+ thinking models** can spend most of the budget in
  the `thinking` block; a 60s total cap forces the API to either
  truncate the response or fail with a deadline error.
- **Long-context generations** (e.g. 100+ step plan from a rich
  snapshot) can take 30-90s end-to-end. Cutoff at 60s drops the
  most useful agent runs.
- **The caller already passes a `context.Context`**
  (line 64: `http.NewRequestWithContext(ctx, ...)`). The context
  is the right place for the deadline — the caller knows whether
  it's a quick check or a long generation. The client-level
  timeout double-bounds and the *shorter* of the two wins,
  defeating the per-call decision.

`agent/loop.go:64` calls `GeneratePlan(context.Background(), ...)`
— no deadline. So today the only bound is the 60s
client timeout, and there's no way for a slow generation to
succeed.

### (b) Default model is stale

```go
if model == "" {
    model = "claude-sonnet-4-20250514"
}
```

The hardcoded fallback is `claude-sonnet-4-20250514` — a
2025-05-14 snapshot ID for an older Sonnet 4. CLAUDE.md and
project README reference the **Sonnet 4.6 / Opus 4.7** generation
(see `system` prompt's environment block: "Opus 4.7 (1M context)").

A user who runs the agent loop without specifying `--model`
gets routed to the older snapshot. The difference between
Sonnet 4 and Sonnet 4.6 is meaningful for plan-generation
quality.

### (c) Unbounded response body read

```go
body, err := io.ReadAll(resp.Body)
```

(line 81)

`io.ReadAll` on a Claude API response has no size cap. A
malformed or attack-pretending-to-be-Anthropic server (DNS
hijack, MITM with coerced CA) could return arbitrary GB of
garbage. The agent process OOMs.

The Anthropic API's max response is bounded by `max_tokens`
(4096 in this client, line 17). At 4 tokens-per-byte that's
~16 KB; a malicious response that exceeds 4 MB is clearly
not a Claude response. Bound at e.g. 1 MB to keep the
"valid response shapes work, attacks don't OOM" invariant.

## Why it's `smell` (not `risk` or `bug`)

(a) and (c) are real risks in adversarial / edge conditions but
the agent loop is single-user, runs locally, and the controller-
side trust model for the LLM API is "trust the API, the user
chose this provider." So:

- The 60s cutoff is a usability bug for thinking-model agents,
  not a security one.
- The unbounded body is a defense-in-depth gap; no documented
  exploit today.
- The stale model default is a doc-drift issue that the user
  notices when they get worse plan output without knowing why.

## Suggested fix

### (a) Drop the client-level timeout, drive cancellation via ctx

```go
return &ClaudeClient{
    apiKey:     apiKey,
    endpoint:   claudeAPIEndpoint,
    httpClient: &http.Client{
        Transport: &http.Transport{
            DialContext:           (&net.Dialer{Timeout: 30*time.Second}).DialContext,
            TLSHandshakeTimeout:   30*time.Second,
            ResponseHeaderTimeout: 30*time.Second,
            // no overall timeout — ctx drives end-to-end cancellation
        },
    },
}, nil
```

The 30s dial / TLS / response-headers bounds catch network
problems early; the body-read time (the long part) is bounded
only by ctx. Pattern matches `internal/httputil` (F012 fix).

`agent/loop.go:64` should pass a real context with a generous
deadline:

```go
genCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
defer cancel()
rawPlan, err := client.GeneratePlan(genCtx, systemPrompt, userPrompt, opts.Model)
```

Or even better, expose `--llm-timeout` on the agent loop's CLI.

### (b) Update the default model

```go
if model == "" {
    model = "claude-sonnet-4-6-20250901" // or the project's current latest
}
```

Better: derive the default from a single project-wide constant
(e.g. `internal/llm.DefaultPlannerModel`) so model-update PRs
touch one file. Or — like the system-prompt environment block
already does — read from a config knob.

### (c) Bound the body read

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB
```

Anthropic API non-streaming responses for `max_tokens=4096` are
well under this. If a real future use case needs more, bump it
explicitly; today's value catches malicious responses early.

## Adjacent — internal/httputil should be the canonical client

F012 introduced `internal/httputil.Client` for outbound HTTP.
The Claude client is **another** standalone `&http.Client{}`
that doesn't use it. Migration ask:

```go
import "github.com/alehatsman/mooncake/internal/httputil"

// Use the canonical transport; override only what's Claude-specific:
httpClient: &http.Client{Transport: httputil.Transport()}
```

(Requires `httputil.Transport()` to be exported. Currently
internal-only.)

## Verification

- `go test ./internal/llm/...`
- After (a): bench a long-context prompt against Claude API and
  confirm 90s generations complete instead of erroring at 60s.
- After (b): `mooncake agent <goal>` without `--model` produces
  the same plan-shape as the current latest documented model.
- After (c): point the client at a stub server returning a 10 MB
  body and assert the read errors out cleanly.

## References

- F012 — the canonical-HTTP-client pattern; Claude client should
  align.
- `internal/llm/claude_cli_client.go` — sibling implementation
  that delegates to the `claude` CLI binary; no HTTP plumbing
  needed. F040 is API-client-only.
- CLAUDE.md environment block — references Sonnet 4.6 / Opus 4.7
  as the project's documented model generation; the hardcoded
  fallback predates that.

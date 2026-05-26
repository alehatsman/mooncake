# Plan: spec-67 `S-pilot-openai-shape-provider`

**Story slug:** `pilot-openai-provider` (PICKUP.md row 3, `docs-working/PICKUP.md:25`)
**Spec:** `docs-working/streams/agent/specs/spec-67-mooncake-pilot.md` §5, §6, §8, §12, §16

---

## 1. Files to add / modify

| Path | Status today | Action |
|---|---|---|
| `internal/pilot/llm/openai_shape.go` | Does not exist (`ls` confirms only `claude_*.go`, `client.go`) | **Add.** `OpenAIShapeClient` struct + `NewOpenAIShapeClient(cfg)` + `GeneratePlan`. Mirror `claude_client.go:36-130` structure (httputil transport, no `http.Client.Timeout`, ctx-driven, `LimitReader` cap). |
| `internal/pilot/llm/openai_types.go` | Does not exist | **Add.** `OpenAIRequest`, `OpenAIMessage`, `OpenAIResponse`, `OpenAIChoice`, `OpenAIUsage`, `OpenAIError`. Spec §8 names this as part of `llm/types.go` — recommend a dedicated `openai_types.go` (mirrors `claude_types.go` at `internal/pilot/llm/claude_types.go:1`) so per-shape JSON tags stay localized. |
| `internal/pilot/llm/capability.go` | Does not exist | **Add (minimal).** `ProviderCapabilities{ SupportsToolUse bool; SupportsPromptCache bool }` struct + `(c *OpenAIShapeClient) Capabilities()` method. Per spec §5.2 openai-shape default is `SupportsToolUse: false`, `SupportsPromptCache: false`. Gives downstream stories (`S-pilot-prompt-cache`, `S-pilot-tool-use-spike`) a typed hook. |
| `internal/pilot/llm/client.go` | Exists, 26 lines, `internal/pilot/llm/client.go:13-25` | **Modify.** Replace the two-step Claude-only chain with the §5.1 six-step precedence. Add `NewClient(opts ClientOptions)` taking `Provider`, `Endpoint`, `APIKey`, `Model`. Keep zero-arg `NewClient()` as a thin shim so `loop.go:46` keeps compiling without wider refactor in this story. |
| `internal/pilot/llm/openai_shape_test.go` | Does not exist | **Add.** `httptest.Server` stub mirroring `claude_client_test.go:15-61`. |
| `internal/pilot/loop.go` | Exists, calls `llm.NewClient()` at `internal/pilot/loop.go:46` | **Modify.** Pass `RunOptions.Provider`, `RunOptions.Model`, plus a new `Endpoint` field through to `NewClient(opts)`. |
| `internal/pilot/types.go` | Exists, `internal/pilot/types.go:32-43` defines `RunOptions` | **Modify.** Add `Endpoint string` to `RunOptions`. |
| `cmd/mooncake.go` | Exists, pilot subcommand at `cmd/mooncake.go:1842-1886`, current `--provider` at `:1864` | **Modify.** Add `--endpoint` flag. Pipe `c.String("endpoint")` into `opts.Endpoint` at `:1450-1459`. Drop the hardcoded `if provider == "claude"` branch at `:1461` so `openai-shape` also routes through `RunLoop`. |
| `internal/pilot/config.go` | Does not exist | **Defer to follow-up.** See §4. |

Spec §8 lists `internal/pilot/config.go` and `internal/pilot/llm/types.go`. The story-level DoD (§16) names only the runtime client and the end-to-end CLI flag. **Recommendation:** ship this story flags-only and defer the `pilot.yml` loader to a dedicated follow-up `S-pilot-config-loader`. Rationale: pulling the loader in widens the diff to YAML parsing + `~/.mooncake/pilot.yml` discovery + flag-vs-yml precedence tests, none of which the DoD requires.

---

## 2. Request/response shape

### Request — `POST {endpoint}/chat/completions`

```json
{
  "model": "llama3.1:70b",
  "messages": [
    {"role": "system", "content": "<systemPrompt>"},
    {"role": "user",   "content": "<userPrompt>"}
  ],
  "max_tokens": 4096,
  "temperature": 0.0,
  "stream": false
}
```

Headers:
- `Content-Type: application/json`
- `Authorization: Bearer <api_key>` only when API key resolves non-empty (Ollama against localhost usually has none — empty string must NOT emit the header).

### Response — happy path

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "model": "llama3.1:70b",
  "choices": [
    {
      "index": 0,
      "message": {"role": "assistant", "content": "<yaml plan>"},
      "finish_reason": "stop"
    }
  ],
  "usage": {"prompt_tokens": 1200, "completion_tokens": 450, "total_tokens": 1650}
}
```

`GeneratePlan` returns `resp.Choices[0].Message.Content`. Empty `choices` is an error (matches Claude path's empty-content check at `claude_client.go:125-127`).

### Divergence from Anthropic (Spec §12)

| Anthropic (`claude_types.go:3-13`) | OpenAI-shape |
|---|---|
| Top-level `system` field | System goes in messages as `{"role":"system", ...}` (first element) |
| `cache_control: ephemeral` blocks | **Not supported** — spec §5.2 + §12 say skip. Plain string content only. |
| `content: []Block{Type, Text}` array on response | `choices[].message.content: string` scalar |
| Roles: `user`, `assistant` | Roles: `system`, `user`, `assistant` |
| `max_tokens` required | `max_tokens` optional but always set (4096 default) |
| `x-api-key` + `anthropic-version` headers | `Authorization: Bearer` only |

### `supports_tool_use` flag

v1 default per spec §5.2: free-text YAML, **no** OpenAI `tools[]` / `tool_choice` payload. The capability struct (§1) is populated from a future config field but always reports `false` for v1. The `OpenAIShapeClient.GeneratePlan` body must NOT set `response_format`, `tools`, `tool_choice`, or any other tool-use field in v1; the prompt already instructs the model to emit raw YAML (`internal/pilot/prompt.go:9-14`). When a future story flips `supports_tool_use: true`, the client emits a `tools` array and JSON-Schema-mode `response_format`. Out of scope here.

---

## 3. Provider-selection chain

Per spec §5.1, precedence (first match wins):

1. `--provider <name>` CLI flag → `RunOptions.Provider`
2. `default_provider` in `~/.mooncake/pilot.yml` → **not in v1 of this story** (no config loader)
3. `MOONCAKE_PILOT_PROVIDER` env var
4. `claude` binary on `$PATH` → `AnthropicCLIClient`
5. `CLAUDE_API_KEY` env set → `AnthropicHTTPClient`
6. `MOONCAKE_PILOT_ENDPOINT` env set → `OpenAIShapeClient`
7. Error: `"no LLM provider configured; run mooncake pilot doctor for setup"` (doctor itself is a separate story)

`NewClient(opts)` updated signature (sketch):

```go
type ClientOptions struct {
    Provider string  // "anthropic-cli" | "anthropic-http" | "openai-shape" | ""
    Endpoint string  // openai-shape only; overrides MOONCAKE_PILOT_ENDPOINT
    APIKey   string  // optional; falls back to env
    Model    string  // passed through to GeneratePlan, not used at construction
}
```

When `opts.Provider == "openai-shape"`, the endpoint resolves as `opts.Endpoint` → `MOONCAKE_PILOT_ENDPOINT` → error. Existing `NewClient()` (no args) preserved as a thin shim calling `NewClient(ClientOptions{})` so `loop.go:46` doesn't need rewriting in this story.

Important: today `cmd/mooncake.go:1461` gates the loop on `provider == "claude"`. Drop that gate so `--provider openai-shape` (and bare `--provider ""` with env discovery) route through `RunLoop`.

---

## 4. Config integration

Spec §6.1 defines `pilot.yml`:

```yaml
default_provider: openai-shape
providers:
  openai-shape:
    endpoint: http://gpubox.local:11434/v1
    api_key_env: OLLAMA_API_KEY
    model: llama3.1:70b
    max_tokens: 4096
    timeout: 120s
    supports_tool_use: false
```

`internal/pilot/config.go` **does not exist today** (verified — only `types.go` under `internal/pilot/`).

**Recommendation: defer to a follow-up `S-pilot-config-loader` story.** This story ships flags + env vars only. The §16 DoD does not mention `pilot.yml` — the example CLI invocation passes `--endpoint http://localhost:11434/v1 --model llama3.1:70b` directly. Shipping the YAML loader here doubles the diff with parsing, schema, precedence-merge, and tests that have nothing to do with the OpenAI-shape client itself.

If reviewers push back, the smallest viable shape is: `func LoadConfig() (*Config, error)` reading `$HOME/.mooncake/pilot.yml` if present, populating `default_provider` + `providers.openai-shape.endpoint` + `.model`, returning a zero-value `*Config` with `nil` error when the file is absent.

---

## 5. Test strategy

Mirror `internal/pilot/llm/claude_client_test.go:14-163`.

| Test | Mirrors | What it pins |
|---|---|---|
| `TestOpenAIShapeClient_GeneratePlan_HappyPath` | `claude_client_test.go:15` | `httptest.Server` handler asserts request method `POST`, path suffix `/chat/completions`, body has `model`, `messages[0].role=="system"`, `messages[1].role=="user"`, `max_tokens` present, no `tools`, no `cache_control`. Returns canned 200 JSON; client returns `choices[0].message.content`. |
| `TestOpenAIShapeClient_NonOK_Returns_Error` | (new) | Server returns 500 with a body → client returns wrapped error containing status code. Matches `claude_client.go:112-114`. |
| `TestOpenAIShapeClient_DefaultModelOmitted` | `claude_client_test.go:77` | When caller passes empty model, request body's `model` is whatever default the client picks. (Open question §8.1.) |
| `TestOpenAIShapeClient_BodyIsBounded` | `claude_client_test.go:114` | Same `LimitReader` cap; same unmarshal-error assertion. Defensive against MITM / buggy gateway. |
| `TestOpenAIShapeClient_NoAuthHeader_WhenKeyEmpty` | (new) | Local Ollama against `localhost` has no API key. Confirm `Authorization` header is unset (not `Bearer `, not present at all) when `APIKey == ""`. |

**Parse-failure retry-on-parse path:** Spec §5.2 mentions "free-text + retry on parse failure" for openai-shape. That retry lives at the pilot-loop level (parser/validator already exist; the loop currently has a no-progress check at `loop.go:109-117` but not a parse-retry). **Do not** add retry-on-parse inside `OpenAIShapeClient` for this story — it belongs in the YAML parser / validator path. Flag as open question (§8).

**Eval harness hookup:** `testing-next/pilot-evals/` already exists per spec §14. No changes needed in this story; eval goals use `provider: openai-shape` in their YAML when the operator opts in.

---

## 6. CLI surface

In `cmd/mooncake.go:1842-1886` add to the `pilot run` subcommand flags:

```go
&cli.StringFlag{
    Name:    "endpoint",
    Usage:   "OpenAI-compatible /v1 base URL (e.g. http://localhost:11434/v1). Required for --provider openai-shape unless MOONCAKE_PILOT_ENDPOINT is set.",
},
```

The existing `--provider` flag at `cmd/mooncake.go:1864-1867` already accepts a string; widen the usage string to list the three valid values. The existing `--model` flag at `:1868-1872` has `Value: "sonnet"` as default — change the default to `""` (let the client decide / surface an error for openai-shape).

Wire at `cmd/mooncake.go:1450-1459`: add `Endpoint: c.String("endpoint")` to `pilot.RunOptions`.

Drop the `if provider == "claude"` gate at `cmd/mooncake.go:1461` so `RunLoop` is reachable for `openai-shape` too. The single-shot `pilot.Run` path (no LLM) at `:1480-1500` stays untouched.

---

## 7. DoD from spec §16 — verbatim

> **Goal.** Add `OpenAIShapeClient` in `internal/pilot/llm/`. POSTs to a configurable `/v1/chat/completions` endpoint; handles Ollama, vLLM, LM Studio, llama.cpp server with the same code path. Wire into `NewClient(config)` provider-selection chain.
>
> **DoD.** `mooncake pilot --provider openai-shape --endpoint http://localhost:11434/v1 --model llama3.1:70b "<goal>"` runs end-to-end against a local Ollama server. Test stub uses `httptest.Server` to assert request shape.
>
> **Deps.** `S-pilot-rename`.

### Checklist this plan satisfies

- [x] `OpenAIShapeClient` lives in `internal/pilot/llm/openai_shape.go` (§1)
- [x] POSTs to `{endpoint}/chat/completions` regardless of which server (Ollama / vLLM / LM Studio / llama.cpp) — they all serve the same shape (§2)
- [x] Wired into `NewClient` chain with the six-step precedence (§3)
- [x] CLI invocation `mooncake pilot run --provider openai-shape --endpoint <url> --model <id> --goal "<goal>"` runs end-to-end (§6)
- [x] `httptest.Server` stub in `openai_shape_test.go` asserts request body shape (§5)
- [x] Dep `S-pilot-rename` already shipped (commit `ec02c71f` per spec §16)

Caveat: the spec's example CLI is `mooncake pilot --provider …`, but today's CLI is `mooncake pilot run --provider …` (subcommand `run`, see `cmd/mooncake.go:1844-1847`). The DoD is satisfied with the subcommand form; promoting `pilot <goal>` to a top-level form is a separate UX change tied to broader §7 work.

---

## 8. Resolved decisions

All seven open questions resolved 2026-05-26 (questionary, this session).

1. **Default model for openai-shape** → **Error with actionable message.** When `--model` is empty and `providers.openai-shape.model` is unset, the client refuses to start and prints `openai-shape requires --model or providers.openai-shape.model`. Rationale: different servers expect incompatible id formats (Ollama tags vs HF slugs); a built-in default is wrong for half the user base.
2. **Retry-on-parse-failure ownership** → **Pilot loop.** Parser + validator already live in the loop; one retry path benefits every provider. `OpenAIShapeClient` stays a thin transport. No retry inside the client.
3. **Path joining** → **Client appends `/chat/completions` to the operator's endpoint.** No trailing-slash trimming. Operator passes `http://host:11434/v1`; client posts to `http://host:11434/v1/chat/completions`.
4. **`temperature` default** → **Hardcoded `0.0` for v1.** No flag. YAML emission wants determinism. Surface as config later only if a user case demands it.
5. **Streaming** → **Hardcoded `stream: false`.** Pilot loop consumes responses synchronously (`loop.go:84`). SSE is a separate parse path — out of scope.
6. **`capability.go`** → **Land now with empty/zero-value impl.** Spec §8 names it; ~15 LOC. Provides a typed hook for `S-pilot-prompt-cache` + `S-pilot-tool-use-spike` to slot into without re-spec'ing.
7. **`IterationLog.Provider` value** → **Literal `"openai-shape"`** (not the endpoint URL). Endpoint is config, not identity.

---

## 9. Out of scope

- **Prompt caching** — separate story `S-pilot-prompt-cache`; explicitly excluded per §5.2.
- **Multi-turn thread storage** — `S-pilot-multi-turn`.
- **Tool-use opt-in** — `S-pilot-tool-use-spike`.
- **Pipeline / planner-coder composition** — §5.4 + `S-pilot-planner-coder` (v2).
- **`pilot.yml` config loader** — deferred to `S-pilot-config-loader` (§4).
- **`mooncake pilot doctor` provider diagnostics** — separate story.
- **Top-level `mooncake pilot <goal>` (no `run` subcommand)** — UX change tied to broader §7 work.

---

## 10. Critical files for implementation

- `internal/pilot/llm/client.go`
- `internal/pilot/llm/claude_client.go` (template to mirror)
- `internal/pilot/llm/claude_client_test.go` (test template to mirror)
- `internal/pilot/loop.go`
- `cmd/mooncake.go`

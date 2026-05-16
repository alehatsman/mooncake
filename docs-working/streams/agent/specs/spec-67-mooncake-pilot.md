# Spec 67 — Mooncake Pilot

> **Status:** drafted 2026-05-16. Targets the agent stream.

## 1. Summary

**Pilot is mooncake's in-binary copilot.** A user types a prose goal
(`mooncake pilot "install postgres with TLS"`); pilot routes it
through a configurable LLM (Claude or a local model on the user's
GPU), receives back a mooncake YAML plan, validates it against the
existing schema, shows the typed Diff, asks for confirmation,
executes it inside a transaction, and writes a full audit trail.

Pilot is provider-portable: the same binary drives Anthropic Claude
(CLI or HTTP) or any OpenAI-shape endpoint (Ollama, vLLM, LM
Studio, llama.cpp server). The kernel's typed-mutation guarantees
(Diff, Reverse, Cost, Permissions, transactions, audit) apply
identically regardless of which model wrote the plan.

The bones of this feature ship today as `internal/agent/`. This
spec renames the package to `internal/pilot/`, adds the
shippable-feature UX (plan-confirm gate, transaction wrap,
multi-turn threads), adds OpenAI-shape provider support, and
hardens the prompt pipeline (schema injection, prompt caching,
eval harness).

## 2. Motivation

Two audiences want this:

1. **Model-sovereign operators.** Anyone running a personal fleet
   with a GPU node — and wanting to keep inference on that GPU —
   has no good path today. Claude Code is Anthropic-locked. Cursor
   is cloud-coupled. Codex requires OpenAI auth. Pilot lets a
   local Llama / Qwen / DeepSeek model drive mooncake with the
   same safety story.
2. **Operators who want one binary to do it all.** A single Go
   binary handles config management, fleet apply, MCP server,
   *and* the LLM-driven planning loop. No SaaS dependency, no
   external editor required, no IDE plugin to install.

The MCP server (`internal/mcp/`) covers the parallel path: any
MCP-aware external agent (Claude Code, Cursor, Zed) drives
mooncake from inside the tool the user already uses. The two
paths are complementary; pilot does not displace the MCP path.

## 3. End-to-end user experience

```
$ mooncake pilot "install postgres 16 with TLS on this host"

⠹ planning…

Plan (4 steps) — review before apply:

  1. ✓ reversible    pkg.install            postgresql-16
                     postgresql-contrib-16, postgresql-client-16
  2. ✓ reversible    file.write             /etc/postgresql/16/main/postgresql.conf
                     +12 lines, -2 lines (ssl=on, ssl_cert_file, ...)
  3. ✓ reversible    file.write             /etc/postgresql/16/main/pg_hba.conf
                     +4 lines (hostssl all all 0.0.0.0/0 scram-sha-256)
  4. ✓ reversible    os.service             postgresql state=started

Cost: 4 steps, max-risk=5 (band: moderate). 1 step needs sudo.
Wrapped in transaction: failure auto-reverts.

Apply? [y/N/edit/explain N/abort]: y

⠹ applying step 1 of 4…
⠹ applying step 2 of 4…
⠹ applying step 3 of 4…
⠹ applying step 4 of 4…

✓ done. Audit: ~/.mooncake/pilot/threads/01HV9C7M5K.jsonl
```

Multi-turn:

```
$ mooncake pilot --continue "also configure replication to standby.local"

⠹ planning (resuming thread 01HV9C7M5K)…
…
```

Single-shot execute (no LLM):

```
$ mooncake pilot run --plan ./prepared-plan.yml
$ cat plan.yml | mooncake pilot run --stdin
```

Failure path (transaction rollback):

```
⠹ applying step 3 of 4…
✗ step 3 failed: file.write /etc/postgresql/16/main/pg_hba.conf
  permission denied (need sudo)

⠹ reverting…
✓ step 2 reverted (pkg.install rolled back: postgresql-16 uninstalled is irreversible — see notes)
✓ step 1 reverted
✗ transaction failed; system restored to pre-pilot state.

Audit: ~/.mooncake/pilot/threads/01HV9C7M5K.jsonl
Retry with sudo? [Y/n]:
```

## 4. Architecture

### 4.1 One loop, three orthogonal knobs

Pilot is a single loop:

```
            ┌────────────────────────────────────┐
            │                                    │
            │  user goal + thread history        │
            │           │                        │
            │           ▼                        │
            │  ┌────────────────────┐            │
            │  │  build prompt:     │            │
            │  │  system + schema   │            │
            │  │  + snapshot + goal │            │
            │  │  + last-result     │            │
            │  └─────────┬──────────┘            │
            │            │                       │
            │            ▼                       │
            │     ┌─────────────┐                │
            │     │   provider  │ ◄── plug-in    │
            │     │   client    │     {anthropic-cli,
            │     └──────┬──────┘     anthropic-http,
            │            │            openai-shape-http}
            │            ▼                       │
            │   parse + sanitize + validate      │
            │   against schema.json              │
            │            │                       │
            │            ▼                       │
            │   render typed Diff                │
            │            │                       │
            │            ▼                       │
            │      plan-confirm gate ───── abort ┼──► done
            │            │ y                     │
            │            ▼                       │
            │   wrap in implicit transaction     │
            │            │                       │
            │            ▼                       │
            │   executor.Start(plan)             │
            │            │                       │
            │            ▼                       │
            │   capture result, diffstat,        │
            │   changed files; write iter log    │
            │            │                       │
            └────────────┼───────────────────────┘
                         │
              ┌──────────┴───────────┐
              │                      │
        no changes               error or
        and LLM says         partial completion
        "done"                       │
              │                      │
              ▼                      ▼
            done            loop with feedback
```

Three orthogonal settings on this loop:

| Knob | Range | Set by | v1 default |
|---|---|---|---|
| **Batch size per iteration** | 1 to N actions per emitted plan | The LLM, driven by prompt template | LLM's choice (typically 4–20) |
| **Serialization format** | Free-text YAML vs `tool_use` JSON | Per-provider capability | YAML (universal); `tool_use` opt-in for Anthropic |
| **Conversation state** | Stateless rebuild vs persistent thread | CLI flag (`--continue` / `--thread`) | Stateless single-shot |
| **Interaction style** | "Plan a complete sequence" vs "Propose next action" | `--style {plan,step}` flag | `plan` |

"One command at a time, executing, logging, asking again" is
just `--style step --batch-confirm per-step` — same executor,
different prompt template, different per-step UX. There is no
second loop.

### 4.2 The MCP server is unaffected

Pilot is its own frontend. The MCP server (`internal/mcp/`)
remains the surface external agents (Claude Code, Cursor, Codex)
consume. The two paths share the kernel; they do not share their
loops:

```
                ┌──────────────────────┐
                │   The Kernel         │
                │   typed actions,     │
                │   Diff, Reverse,     │
                │   Cost, transactions │
                └──────────┬───────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
        ┌─────▼─────┐             ┌─────▼─────┐
        │  MCP      │             │  Pilot    │
        │  server   │             │  (this    │
        │           │             │   spec)   │
        └─────┬─────┘             └─────┬─────┘
              │                         │
              ▼                         ▼
        Claude Code,                Operator's
        Cursor, Codex,              own loop +
        Zed (external)              own model
        Anthropic-only              (provider-
        models, cloud               portable)
```

Pilot does not consume the MCP server internally in v1. The
plan-shot loop reads the kernel directly through
`executor.Start`. If a future need ("pilot drives 12 mooncake
peers across the fleet" or "pilot interactive step-by-step over a
remote agentd") demands it, pilot becomes an MCP client at that
point. Not before.

## 5. Provider model

### 5.1 Two protocol shapes, three implementations

| Implementation | Shape | When selected | Local? |
|---|---|---|---|
| `AnthropicCLIClient` | shells out to local `claude` CLI | `claude` binary is in `$PATH` | No |
| `AnthropicHTTPClient` | direct POST to `api.anthropic.com` | `CLAUDE_API_KEY` env var set, no CLI found | No |
| `OpenAIShapeClient` | direct POST to OpenAI-compatible `/v1/chat/completions` | endpoint URL configured | Yes (Ollama, vLLM, LM Studio, llama.cpp) or remote (any OpenAI-API-compatible service) |

Provider selection precedence (default-discovery flow on a fresh
install, in order):

1. `--provider <name>` CLI flag.
2. `default_provider` in `~/.mooncake/pilot.yml`.
3. `MOONCAKE_PILOT_PROVIDER` env var.
4. Anthropic CLI if `claude` is in PATH.
5. Anthropic HTTP if `CLAUDE_API_KEY` is set.
6. OpenAI-shape HTTP if `MOONCAKE_PILOT_ENDPOINT` is set.
7. Error with actionable message: "no LLM provider configured; run
   `mooncake pilot doctor` for setup."

### 5.2 Per-provider capability matrix

| Provider | Tool-use / structured output | Prompt caching | Max tokens default |
|---|---|---|---|
| Anthropic CLI | Whatever the local `claude` does | Managed by the CLI | CLI-managed |
| Anthropic HTTP | `tool_use` block supported | `cache_control: ephemeral` on system + schema blocks | 4096 |
| OpenAI-shape | Capability flag per endpoint: `supports_tool_use: bool`; if false, free-text + retry on parse failure | Not available across the OpenAI-shape ecosystem; skipped | 4096 (configurable) |

Pilot picks the serialization format per-provider. v1 default:
free-text YAML across the board; tool-use enabled for Anthropic
HTTP after the spike (`S-pilot-tool-use-spike`) confirms the
schema-mapping ergonomics. Per-endpoint `supports_tool_use` flag
lets the operator opt into structured output for capable local
models (e.g., Llama 3.1 70B on vLLM).

### 5.3 Provider scope discipline

The provider set is **closed at three implementations and two
protocol shapes for v1**. New shapes (Gemini-native,
Bedrock-converse, etc.) land via the normal spec path one at a
time. No plugin SDK; no provider marketplace; no
LiteLLM-style universal adapter.

## 6. Configuration

### 6.1 `~/.mooncake/pilot.yml`

```yaml
default_provider: anthropic-cli        # one of: anthropic-cli,
                                       # anthropic-http,
                                       # openai-shape
default_model: claude-sonnet-4-7       # provider-specific

providers:
  anthropic-http:
    api_key_env: CLAUDE_API_KEY        # never inline the key
    max_tokens: 4096
    timeout: 60s

  openai-shape:
    endpoint: http://gpubox.local:11434/v1
    api_key_env: OLLAMA_API_KEY        # optional, often unset for local
    model: llama3.1:70b
    max_tokens: 4096
    timeout: 120s                      # local models can be slow
    supports_tool_use: false           # set true per-endpoint when
                                       # the underlying model honors it

defaults:
  style: plan                          # plan | step
  auto_apply: false                    # always prompt by default
  max_iterations: 5
```

### 6.2 CLI flag layering

Precedence (highest wins): CLI flag > env var > pilot.yml > built-in
default.

Relevant flags:

```
--provider <name>             pick provider
--model <id>                  pick model
--endpoint <url>              override openai-shape endpoint
--style {plan,step}           pick prompt template
--auto-apply                  skip plan-confirm gate (DANGEROUS)
--max-iterations N            override default 5
--thread <id>                 resume a specific thread
--continue                    resume the most-recent thread
--no-cache                    disable prompt caching this run
```

## 7. CLI surface

```
mooncake pilot <goal>                  start a new thread; prompt → plan → confirm → apply
mooncake pilot --continue [<goal>]     resume last thread; optional follow-up prompt
mooncake pilot --thread <id> [<goal>]  resume specific thread; optional follow-up

mooncake pilot run --plan <file>       single-shot execute; no LLM call
mooncake pilot run --stdin             single-shot execute from stdin

mooncake pilot threads list            list saved threads
mooncake pilot threads show <id>       print thread transcript + iteration logs
mooncake pilot threads rm <id>         delete thread

mooncake pilot doctor                  diagnose provider + connectivity
mooncake pilot explain providers       list configured providers + capabilities
```

## 8. Package structure (post-rename)

```
internal/pilot/
├── pilot.go                  // Run(opts): single-shot, Loop(opts): iterative
├── types.go                  // RunOptions, LoopOptions, IterationLog,
│                                ThreadState, StopReason
├── prompt.go                 // BuildPrompt + style selection
├── prompt_schema.go          // Trim schema.json into a prompt-fit chunk
├── prompt_styles.go          // "plan" and "step" template variants
├── sanitize.go               // Strip markdown fences from LLM output
├── parser.go                 // YAML extract + schema validate + retry-on-error
├── confirm.go                // Plan-confirm gate UX (y/n/edit/abort/explain)
├── transaction_wrap.go       // Wrap parsed plan in implicit transaction:
├── iteration_store.go        // ~/.mooncake/pilot/iterations/NNNNN.*
├── thread_store.go           // ~/.mooncake/pilot/threads/<ulid>.jsonl
├── config.go                 // pilot.yml load + flag overlay
└── llm/
    ├── client.go             // Client interface, NewClient(config)
    ├── anthropic_cli.go      // shells out to `claude`
    ├── anthropic_http.go     // direct HTTP to api.anthropic.com,
    │                            cache_control on system block
    ├── openai_shape.go       // POST /v1/chat/completions
    ├── capability.go         // per-provider feature flags
    └── types.go              // request/response shapes per provider

cmd/
└── pilot.go                  // `mooncake pilot ...` command wiring
```

## 9. Storage layout

```
~/.mooncake/pilot/
├── threads/
│   └── 01HV9C7M5K…ULID.jsonl     // append-only thread transcript
└── iterations/
    ├── 00001.plan.yml             // saved plan per iteration
    ├── 00001.json                 // IterationLog
    ├── 00002.plan.yml
    └── 00002.json

~/.mooncake/pilot.yml              // operator config (see §6.1)
```

Each thread file is JSONL — one record per turn:

```json
{"ts":"2026-05-16T19:00:00Z","kind":"user_goal","content":"install postgres 16 with TLS"}
{"ts":"2026-05-16T19:00:02Z","kind":"snapshot","branch":"...","head":"...","actions":[...]}
{"ts":"2026-05-16T19:00:03Z","kind":"llm_request","provider":"anthropic-http","model":"claude-sonnet-4-7","prompt_hash":"sha256:..."}
{"ts":"2026-05-16T19:00:05Z","kind":"llm_response","plan_hash":"sha256:...","raw":"..."}
{"ts":"2026-05-16T19:00:05Z","kind":"plan_validated","steps":4,"max_risk":5,"reversible":true}
{"ts":"2026-05-16T19:00:08Z","kind":"user_confirm","response":"y"}
{"ts":"2026-05-16T19:00:11Z","kind":"step_result","step":1,"status":"success","changed_files":[]}
...
{"ts":"2026-05-16T19:00:30Z","kind":"thread_complete","stop_reason":"success"}
```

Migration: hard cut. Old `.mooncake/iterations/` from the
`internal/agent/` era is not migrated. Pre-release sideproject;
no backwards compat shim.

## 10. Plan-confirm gate

After validation but before execution, render to the operator:

- A step-by-step table with reversibility (✓ / ✗ / ⚠), risk band,
  and structural Diff.
- A recap line: "Plan: N steps, max-risk=R (band: B), S steps need
  sudo. Wrapped in transaction: failure auto-reverts."
- A prompt: `Apply? [y/N/edit/explain N/abort]:`

Responses:

| Response | Behavior |
|---|---|
| `y` | Apply |
| `N` (default) | Skip; go to next loop iteration with "user rejected plan" feedback |
| `edit` | Open `$EDITOR` on the plan YAML; re-validate after save |
| `explain N` | Show full Diff + Cost + Permissions for step N |
| `abort` | Stop loop entirely; thread saved with `stop_reason: aborted` |

`--auto-apply` skips the gate. Required for unattended use (CI,
scripted runs). Issues a clear warning at thread start.

For `--style step` runs, the gate applies per-step with extra
options: `approve_next N` ("approve the next N proposed steps
without prompting"), `approve_thread` ("don't prompt again this
thread").

## 11. Transaction wrapping

Every plan pilot apples is wrapped in an implicit `transaction:`
block. The kernel's existing LIFO rollback handles the rest.

Semantics:

- Any step failure during apply → executor begins LIFO reversal of
  already-completed steps using each handler's `Reverse()` method.
- Steps declared irreversible by the handler are surfaced **at
  plan-confirm time**, not at apply time. The plan-confirm gate
  marks them with `✗` and the recap line counts them.
- If apply succeeds, the transaction marker is closed normally.
- If reversal partially fails, the thread is marked
  `rollback_failed` and the audit trail records every reversal
  attempt. The operator sees an exact list of what's still
  applied.

No new YAML keyword. Pilot writes the validated plan with an
implicit `transaction:` wrap to the executor; the wire format is
the same plan the user can inspect.

## 12. Prompt construction

### 12.1 Layout (Anthropic HTTP)

```
system block (cache_control: ephemeral)
├── persona + behavior contract
├── schema chunk (trimmed schema.json — see §12.2)
└── action vocabulary table

[messages thread]
user: snapshot (cache_control: ephemeral on first turn only)
user: goal
assistant: <prior plan if multi-turn>
user: <result + new goal if multi-turn>
...
assistant: <new plan>
```

Two cache-controlled segments: the system block (stable across
the thread) and the snapshot (stable until the operator's
repo state changes meaningfully). Cache hit rate target on the
second iteration onward: 85%+.

### 12.2 Schema chunk

The system prompt's schema vocabulary is generated *from*
`internal/config/schema.json` at prompt-build time. No
hand-written action list in source code (the current
`internal/agent/prompt.go` line 17–46 hand-writes this; that
hand-written copy is dropped). When a new handler lands, pilot
picks up its action without a prompt-source-code edit.

The chunk includes:

- Action verb list with one-line descriptions
- Required + optional field names per verb
- Type hints (string, int, bool, array, map)
- Three short examples per verb (selected from `examples/`)
- Constraints (idempotent, no interactive commands, paths
  absolute or repo-relative)

Format is plain-text, not JSON. The model parses it natively;
JSON-stuffing the system block burns tokens.

### 12.3 Style templates

- **`--style plan`** (v1 default): "Design a complete mooncake
  YAML plan accomplishing this goal. Output the entire plan in a
  single response; we execute the whole plan in one transaction.
  Aim for 4–30 steps; verify with `assert:` where useful."
- **`--style step`**: "Propose the *next single action* needed to
  make progress toward the goal. Output a YAML plan with exactly
  one step. After we execute it and report back, you'll propose
  the next single action. Stop when the goal is reached and emit
  an empty plan."

The executor is the same either way. The prompt is what differs.

## 13. Multi-turn conversations

Threads persist to `~/.mooncake/pilot/threads/<ULID>.jsonl`
(see §9). Resume by ULID or by `--continue` (most-recent).

The model sees:

- Full thread transcript (all prior turns) in `messages` array.
- Cache control on the system block + initial snapshot only.

`mooncake pilot threads list` shows ULID, first-turn goal, last
turn timestamp, status. `threads show <id>` prints a
human-readable transcript. `threads rm <id>` deletes.

Threads have no automatic expiry; operator runs `threads rm`
when they don't want them. Future: optional retention policy in
`pilot.yml`.

## 14. Eval harness

The prompt is the actual moat. The eval harness is what lets
prompt iteration *not* drift the loop's reliability.

Layout:

```
testing-next/pilot-evals/
├── goals/
│   ├── 001-install-package.yml      // (goal, expected-plan-shape) tuples
│   ├── 002-create-systemd-service.yml
│   ├── 003-rewrite-config-with-tls.yml
│   └── ...
├── snapshots/                       // canned snapshots to feed the prompt
│   ├── ubuntu-24-04-clean.json
│   ├── debian-13-with-postgres.json
│   └── ...
├── assertions/                      // plan-shape assertions
│   └── ...
└── run.go                           // the test runner
```

Each goal file:

```yaml
goal: "install postgres 16 with TLS"
snapshot: ubuntu-24-04-clean
provider: anthropic-http             # which provider to test
model: claude-sonnet-4-7

assertions:
  - "plan validates against schema.json"
  - "plan contains at least one pkg.install step"
  - "plan contains a file.write step targeting postgresql.conf"
  - "plan contains an os.service step with state=started"
  - "max-risk <= 7"
  - "all steps reversible OR explicitly noted as irreversible"
```

Runner is gated behind `MOONCAKE_PILOT_EVAL=1` env var so it does
not run on every PR (API tokens cost real money). CI hook: PRs
labeled `needs-pilot-eval` trigger the runner.

Starter set: ~10 goals covering pkg install / service management /
config file editing / repo search-and-replace.

## 15. Non-goals

| Non-goal | Why |
|---|---|
| Fleet-shaped pilot ("apply this prompt across N peers") | Pilot runs locally per-peer. Fleet-aware semantics would require a control-plane abstraction; non-goal #3. |
| Provider plugin SDK | Closed-set discipline. Two shapes, three implementations, new shapes via spec path. Non-goal #2. |
| Pipeline DSL for chaining pilot calls | Pilot takes prose; the bridge to typed plans is the LLM. New YAML keywords for "if-this-then-that-pilot" are not in scope. Non-goal #7. |
| Atomicity claims for the implicit transaction | SAGA, not ACID. Surface partial-rollback states honestly. Non-goal #6. |
| MCP-internal-client architecture in v1 | Not needed; the plan executor handles K=1 to K=N uniformly. If a future need genuinely requires MCP-as-client (e.g., remote agentd target), it lands then. |
| Long-horizon autonomous loops without operator gate | Plan-confirm is the default. `--auto-apply` exists for CI but is explicitly the dangerous mode and warns at start. |
| A second MCP server inside pilot | The existing `internal/mcp/` is the MCP server. Pilot does not become a second one. |

## 16. Implementation plan

Stories in dependency order. Each is self-contained; pick the
next one whose dependencies are all `done`.

### S-pilot-rename

- **Goal.** Rename `internal/agent/` → `internal/pilot/` + all
  import sites; rename CLI verb `mooncake agent` → `mooncake
  pilot`; move `.mooncake/iterations/` writes to
  `.mooncake/pilot/iterations/`.
- **DoD.** `make test-race` and `make ci` green. No remaining
  references to `internal/agent` import path. CLI subcommand
  removed without backwards-compat shim.
- **Deps.** None. **Land first.**

### S-pilot-openai-shape-provider

- **Goal.** Add `OpenAIShapeClient` in `internal/pilot/llm/`.
  POSTs to a configurable `/v1/chat/completions` endpoint; handles
  Ollama, vLLM, LM Studio, llama.cpp server with the same code
  path. Wire into `NewClient(config)` provider-selection chain.
- **DoD.** `mooncake pilot --provider openai-shape --endpoint
  http://localhost:11434/v1 --model llama3.1:70b "<goal>"` runs
  end-to-end against a local Ollama server. Test stub uses
  `httptest.Server` to assert request shape.
- **Deps.** `S-pilot-rename`.

### S-pilot-confirm-gate

- **Goal.** Implement the plan-confirm gate (§10) with full
  response set (y/N/edit/explain N/abort). Default behavior:
  prompt. `--auto-apply` flag preserves current always-apply.
- **DoD.** Manual test of each response. Unit test on the response
  parser. `--auto-apply` warning emitted at thread start.
- **Deps.** `S-pilot-rename`.

### S-pilot-transaction-wrap

- **Goal.** Wrap each iteration's plan in an implicit
  `transaction:` block before sending to the executor (§11).
- **DoD.** Regression test: a plan whose step 3 fails leaves the
  system byte-identical to its pre-pilot state. Confirm-gate
  recap line shows irreversible-step count when any step is
  irreversible.
- **Deps.** `S-pilot-rename`. Independent of confirm-gate.

### S-pilot-schema-injection

- **Goal.** Drop the hand-written schema/action list in
  `prompt.go`; generate a prompt-fit chunk from
  `internal/config/schema.json` at build time (§12.2). Snapshot
  test pins the rendered chunk shape.
- **DoD.** Adding a new in-tree action shows up in pilot's prompt
  vocabulary the same release. No source edit needed in
  `internal/pilot/prompt*.go` to surface new actions.
- **Deps.** `S-pilot-rename`.

### S-pilot-prompt-cache

- **Goal.** Add `cache_control: ephemeral` to the system block
  and the initial snapshot in `AnthropicHTTPClient` requests
  (§12.1). Skip caching for `AnthropicCLIClient` (CLI manages its
  own) and `OpenAIShapeClient` (capability not universally
  supported).
- **DoD.** API response logging shows `cache_read_input_tokens >
  0` from the second turn onward. Per-thread cost reduction
  measurable in eval harness output.
- **Deps.** `S-pilot-schema-injection` (so the cached chunk is
  stable across the thread).

### S-pilot-multi-turn

- **Goal.** Persist threads to
  `~/.mooncake/pilot/threads/<ULID>.jsonl` (§9, §13). Implement
  `--continue`, `--thread <id>`, and `threads list/show/rm`.
- **DoD.** A two-turn flow demo: first turn approved + applied;
  second turn issued with `--continue` references the prior plan
  meaningfully (Claude's response cites it). `threads show`
  produces a readable transcript.
- **Deps.** `S-pilot-rename`. Better with `S-pilot-prompt-cache`
  (cache hit rate matters once threads grow) but not blocking.

### S-pilot-styles

- **Goal.** Implement the `--style plan` (default) and `--style
  step` prompt templates (§12.3) and per-style confirm UX (§10).
- **DoD.** A `--style step` run on the install-postgres goal
  produces an iterative flow: each turn proposes one action,
  confirm-gates per step, executes, feeds result back to next
  prompt. Demo on a real local model (Ollama).
- **Deps.** `S-pilot-rename`, `S-pilot-confirm-gate`.

### S-pilot-eval-harness

- **Goal.** Stand up `testing-next/pilot-evals/` (§14) with 5
  starter goals + a CI hook (env-gated).
- **DoD.** `make pilot-evals` runs locally with a valid provider
  configured. CI runs the harness on PRs labeled
  `needs-pilot-eval`. Five (goal, snapshot, assertions) tuples
  shipped.
- **Deps.** `S-pilot-rename`. Independent of the rest; can start
  in parallel.

### S-pilot-tool-use-spike (gated)

- **Goal.** Spike Anthropic `tool_use` mode for the
  `AnthropicHTTPClient`. Build a side-by-side eval against the
  free-text-YAML path on the harness goals; write up which mode
  ships as v1 default for Anthropic.
- **DoD.** Decision recorded in
  `docs-next/decision-pilot-anthropic-tool-use.md` with eval
  numbers (validation rate, latency, cost) supporting the
  recommendation.
- **Deps.** `S-pilot-eval-harness`. Optional for v1 — defer if
  the YAML path is already reliable enough.

## 17. Out of scope / future

- **MCP-internal-client architecture.** Pilot reads the kernel
  directly today; consuming `internal/mcp/` from inside pilot
  would only earn its weight when pilot needs to drive a remote
  agentd or aggregate across peers. Spec when that need arrives.
- **Fleet-aware pilot.** `mooncake fleet pilot "..."` applying a
  prompt-driven plan across N peers. Conceptually clean; spec
  later if asked.
- **More providers (Gemini-native, Bedrock-converse).** Normal
  spec path one at a time. Closed set holds.
- **Long-horizon autonomous runs.** "Pilot runs for an hour
  without operator gate." Probably needs trust signals (drift
  detection, budget caps) we don't have yet. Out of scope.

## 18. Cross-references

- `internal/agent/` — current code; gets renamed in
  `S-pilot-rename`.
- `internal/llm/` — current LLM client package; gets moved to
  `internal/pilot/llm/` in `S-pilot-rename`.
- `internal/config/schema.json` — source of truth for §12.2.
- `internal/mcp/` — the MCP server; the parallel path for
  external agents.
- `docs-working/vision/kernel.md` — kernel discipline this spec
  rides on (pilot is a rendering, not a new property).
- `docs-working/vision/non_goals.md` — the rails §15 walks.
- `examples/transactions/rollback-demo.yml` — the primitive
  §11 inherits from.
- `docs-working/streams/agent/README.md` — stream this spec
  belongs to.
- `docs-working/streams/agent/proposals/proposal-01..06` —
  parallel work on the MCP-driven external-agent path; pilot
  does not displace it.

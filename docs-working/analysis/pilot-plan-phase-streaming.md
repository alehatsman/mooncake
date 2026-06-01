# Streaming planner progress during pilot's plan phase (#69)

**Status:** exploration / design — no code committed. Feasibility + event-schema
options + recommended phasing for killing the silent gap between turn-start and
`plan.loaded` in `mooncake pilot run --output-format json`.

Filed against moongit#133 (client-side "Planning…" spinner-over-a-void stopgap).

## 1. The gap is real and structural

In JSON mode the NDJSON stream a consumer sees per turn is:

```
turn starts → <dead air for the whole planning latency> → plan.loaded + all step.* burst out
```

Two independent facts cause it:

1. **The planner call is fully buffered.** `ClaudeCLIClient.GeneratePlan`
   (`internal/pilot/llm/claude_cli_client.go:54`) runs the CLI with
   `--print --output-format json` and `cmd.Run()` (`:55`, `:74`) — it blocks
   until the *entire* result envelope is written, then returns the whole plan
   string. Nothing is emitted incrementally. The HTTP (`claude_client.go`) and
   `openai_shape.go` providers are the same shape: one request, await full body.

2. **No event publisher exists during planning.** The per-step NDJSON lines are
   produced by a `ConsoleSubscriber` in JSON mode that is subscribed to the
   executor's publisher — and that publisher is constructed *inside*
   `applyPlanIteration` **after** the plan is generated
   (`internal/pilot/loop.go:~563`, mirrored in `run.go:105`). `plan.loaded`
   itself is emitted by the **executor** (`internal/executor/executor.go:1362`),
   not the pilot loop. So during `RunLoop`'s `client.GeneratePlan(...)` call
   (`loop.go:~160`) there is no publisher and no subscriber writing to stdout —
   structurally there is nothing that *could* emit a planning event today.

`plan.loaded` carries only `RootFile` / `TotalSteps` / `Tags`
(`internal/events/event.go:144`) — no step list, so even after the burst a UI
can't show the *whole* plan, only steps as they execute.

Note: the single-shot `--plan` / `--stdin` path has no planner call (the plan
comes from a file), so this only concerns the LLM-driven `RunLoop` path.

## 2. Provider streaming feasibility

| Provider | Streaming mechanism | Feasible? | Notes |
|---|---|---|---|
| `anthropic-cli` (default) | `--output-format stream-json` + `--include-partial-messages` (both verified present in the installed CLI via `claude --help`) | **Yes** | CLI emits NDJSON events (`message_start`, `content_block_delta` with text/thinking deltas, `message_stop`). Read stdout with a `bufio.Scanner`, forward text deltas as events, accumulate the final result for the existing return value. |
| `anthropic-http` | Messages API SSE (`stream: true`) | Yes | More work: SSE framing + `content_block_delta` parsing. |
| `openai-shape` | `/v1/chat/completions` SSE (`stream: true`) | Yes | Standard OpenAI delta chunks. Local endpoints (Ollama etc.) support it. |

Token-level streaming is feasible on **all three**, cheapest on the default
(`anthropic-cli`) because the CLI already frames the stream as NDJSON.

## 3. Options (matching the issue's three directions)

### Option A — `plan.generating` lifecycle marker (lightweight)

Emit a `plan.generating` event (a `started` bracket) immediately before the
planner call, so a consumer has a **real start event** to bracket the phase
instead of a spinner over silence. The matching "finished" bracket already
exists: `plan.loaded`.

- **Provider changes:** none. Pure pilot-loop change.
- **Plumbing:** still requires a publisher/sink active during planning (see §4),
  but only to emit one event, not a stream.
- **Value:** removes the "is it hung?" ambiguity. Doesn't show *content*.
- **Effort:** small. **Risk:** minimal.

### Option B — `planner.delta` streamed tokens (rich)

Switch the planner call to a streaming mode and forward text/thinking deltas as
`planner.delta` (or `plan.thinking`) events as they arrive.

- **Provider changes:** add a streaming variant to the `Client` interface
  (§4). At minimum implement it for `anthropic-cli`; others can fall back to the
  buffered path (emit one synthetic delta with the full text) until they grow
  real streaming.
- **Value:** highest — live content to render. Matches what a human sees in an
  interactive `claude` session.
- **Effort:** medium. **Risk:** medium (stream parsing, partial-failure modes,
  truncation/back-pressure on the NDJSON sink).

### Option C — surface the step list in `plan.loaded` (orthogonal, cheap)

Add the parsed step names/titles to `PlanLoadedData` (today only `TotalSteps`).
Independent of streaming; lets a UI render the **whole** plan at once instead of
discovering steps as they execute.

- **Provider changes:** none.
- **Back-compat:** additive field on an existing event — existing consumers keep
  reading `total_steps` (and `fleet/multiplex.go:180` keeps reading
  `step_count`); unknown new field is ignored.
- **Effort:** small. **Risk:** minimal. Note: `plan.loaded` is emitted by the
  executor, so this addition lives in the executor, not the pilot.

## 4. Plumbing required (shared by A and B)

The blocker for *any* planning-phase event is that no sink is wired during
planning. Minimal shape:

1. **A planning-phase publisher/sink.** Hoist a publisher to `RunLoop` scope (or
   thread a small `func(events.Event)` delta sink down from
   `cmd/kernel/pilot.go`) so events emitted around/within `GeneratePlan` reach
   the same stdout JSON encoder the executor's `ConsoleSubscriber` uses. Today
   that subscriber is created per-iteration inside `applyPlanIteration`; a clean
   refactor creates the publisher once in `RunLoop` and reuses it for both the
   planning phase and `applyPlanIteration` (passed in rather than constructed
   inside).

2. **A streaming entry point (Option B only).** Extend the `Client` interface
   (`internal/pilot/llm/client.go:10`). Two viable shapes:
   - **Callback:** `GeneratePlanStream(ctx, sys, user, model string, onDelta func(string)) (string, error)`. Providers without streaming call `onDelta(full)` once. Simplest; keeps the return contract (final plan string) identical.
   - **Optional interface:** a separate `StreamingClient` that `RunLoop`
     type-asserts; falls back to `GeneratePlan` when absent. Avoids touching
     non-streaming providers at all.

   The callback shape is recommended — one method, back-compatible return, and
   the loop owns the delta→event translation (so providers stay
   transport-only and don't import `internal/events`).

3. **Output discipline.** Deltas must go *only* to the NDJSON encoder in JSON
   mode and *only* to the operator's terminal in text mode — never raw to stdout
   (would corrupt the event stream). This mirrors the existing
   `captureWriter(outputFormat)` / `consoleLogFormat(outputFormat)` gating in
   `run.go` / `loop.go`.

## 5. Back-compat

All additions are **type-additive** and safe for the documented consumer
contract (moongit keys on the top-level `type` and ignores unknown event types
— stated in the issue):

- New event types `plan.generating` / `planner.delta`: ignored by existing
  consumers; opt-in for new ones.
- New field on `plan.loaded` (step list): existing readers of `total_steps` /
  `step_count` unaffected.
- The terminal `pilot.completed` contract is unchanged.

No existing event is renamed or restructured.

## 6. Recommendation (phased)

1. **Phase 1 (cheap, high signal-to-noise):** Option A (`plan.generating`
   bracket) **+** Option C (step list in `plan.loaded`). Together these turn the
   void into "planning started → here's the whole plan", with no provider
   changes and trivial back-compat risk. This alone retires moongit's
   spinner-over-a-void: the spinner gets a real start/stop bracket and, on
   completion, the full plan to render.

2. **Phase 2 (if live content is still wanted):** Option B (`planner.delta`)
   on the `anthropic-cli` provider via `--output-format stream-json
   --include-partial-messages`, with the callback-shaped interface and
   buffered-fallback for the other two providers.

Phase 1 is the recommended first cut: it removes the perceived hang and unlocks
whole-plan rendering for a fraction of Phase 2's surface area. Phase 2 is a
clean follow-up once the publisher hoist (§4.1) is in place.

## 7. Open questions / risks

- **Publisher hoist scope.** Reusing one publisher across the planning phase and
  `applyPlanIteration` is the cleanest plumbing but touches the per-iteration
  subscriber lifecycle (close/flush ordering — cf. the MT-53 events-drop-on-close
  note in `executor.go`). Needs care so planning-phase deltas don't race the
  per-iteration writer's close.
- **Delta volume / back-pressure.** A long plan can stream a lot of tokens;
  `planner.delta` should be coalesced (e.g. flush per content block or per N
  bytes) rather than one event per token, to keep the NDJSON stream sane for
  forwarders (agentd/fleet/SSE).
- **stream-json envelope drift.** The buffered path already pins the CLI
  envelope shape (`claudeCLIEnvelope`, `claude_cli_client.go:41`) with a dated
  verification comment; the streaming event shape needs the same treatment and
  a parse-failure fallback to the buffered call.
- **`--include-partial-messages` is `--print`-only** and pairs with
  `stream-json` — matches how the provider already invokes the CLI, so no
  interactive-session concerns.

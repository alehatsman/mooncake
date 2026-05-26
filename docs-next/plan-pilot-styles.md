# Plan: spec-67 `S-pilot-styles` — `--style {plan,step}` prompt templates + per-style confirm UX

**Spec:** `docs-working/streams/agent/specs/spec-67-mooncake-pilot.md` §4.1, §10, §12.3, §16
**Story slug:** `pilot-styles`

---

## 1. Files to add / modify

| Path | Action | Purpose |
|---|---|---|
| `internal/pilot/prompt_styles.go` | **add** (spec §8 names it) | Hold the two `Style` constants and the per-style template fragments + a `selectStyleFragment(style)` function. |
| `internal/pilot/prompt.go` | **modify** (`internal/pilot/prompt.go:9-46`) | Replace the hardcoded `promptPreamble` "Generate a Mooncake YAML plan…" tail with a style-aware selector; `buildSystemPrompt` and `BuildPrompt` both take a `Style`. Schema chunk injection unchanged. |
| `internal/pilot/types.go` | **modify** (`internal/pilot/types.go:32-43`) | Add `Style Style` field to `RunOptions`; add `Style` string-typed enum (`StylePlan`, `StyleStep`); add `StopStepDone StopReason = "step_done"` constant. Add `Style` field to `PlanInput`. |
| `internal/pilot/confirm.go` | **modify** (`internal/pilot/confirm.go:24-92` and `:117-164`) | Add `RespApproveNext` (with `N` field reused) and `RespApproveThread` `ResponseKind`s. Extend `ParseResponse` to recognize `approve_next N` and `approve_thread`. Add a `StepGateState` struct (counter + thread-approved bool) carried into `ConfirmPlan` so consecutive step calls share state. Surface the extended responses only in step mode (different prompt string + parser whitelist). |
| `internal/pilot/loop.go` | **modify** (`internal/pilot/loop.go:54-260`) | Propagate `opts.Style` into `BuildPrompt`. After `SanitizePlan` + validation, if `opts.Style == StyleStep` and `len(steps) == 0`, terminate the loop with `StopStepDone`. Allocate one `StepGateState` per `RunLoop` call so `approve_next N` / `approve_thread` persist across iterations within a thread. |
| `cmd/mooncake.go` | **modify** (`cmd/mooncake.go:1842-1886` + `:1433-1459`) | Add `&cli.StringFlag{Name: "style", Value: "plan"}` (validate against `plan|step`); thread into `pilot.RunOptions.Style`. Precedence: CLI > `MOONCAKE_PILOT_STYLE` env > built-in default `plan`. |
| `internal/pilot/prompt_test.go` | **add** (does not exist; `prompt_schema_test.go` covers schema chunk only) | Snapshot the rendered plan-style and step-style system prompts so prompt drift is visible in code review. |
| `internal/pilot/confirm_test.go` | **modify** (`internal/pilot/confirm_test.go:10-47` table) | Add table rows for `approve_next 3`, `approve_thread`, and rejection of those tokens under `--style plan`. Add a state-carrying test for the step-mode gate. |
| `internal/pilot/loop_test.go` | **modify** | Add `TestRunLoop_StyleStep_EmptyPlanStops` using a stub `llm.Client` returning `[]` to verify `StopStepDone`. |

Pilot.yml loader: spec §6.1 shows `defaults.style:` but `config.go` (per §8) doesn't exist yet. **Scope this story to CLI flag + env only.** Pilot.yml loading is a separate story (see Open Questions).

---

## 2. Template text — diffs vs current `prompt.go`

Current behavior (`internal/pilot/prompt.go:9-14` + `:100-106`) emits one fixed instruction. Spec §12.3 defines two style-specific instruction blocks.

**`plan` style** (default; current behavior, naming-only). Replaces the `Generate a Mooncake YAML plan to accomplish the goal.\nOutput ONLY a YAML array of steps…` tail in `BuildPrompt` (`prompt.go:100-106`):

```
TASK STYLE: complete plan
Design a complete mooncake YAML plan accomplishing this goal.
Output the entire plan in a single response; we execute the
whole plan in one transaction. Aim for 4–30 steps; verify with
`assert:` where useful.
```

**`step` style** (new):

```
TASK STYLE: one step at a time
Propose the NEXT SINGLE action needed to make progress toward
the goal. Output a YAML plan containing EXACTLY ONE step.
After we execute it and report back in LAST ITERATION, you
will propose the next single action. When the goal is reached,
emit an empty plan (the YAML literal `[]`) to signal "done".
```

The schema chunk (built by `BuildSchemaChunk()` per `prompt.go:34`), the `OUTPUT REQUIREMENTS`, the `BEST PRACTICES`, and the `CONSTRAINTS` blocks are **identical across styles**. Only the trailing TASK STYLE block changes (and the inline "Example format" hint in `BuildPrompt`'s user-message footer, dropped in favor of the style block).

`BuildPrompt` signature changes minimally: `PlanInput.Style` is populated, and `buildSystemPrompt(style Style) (string, error)` becomes the new internal seam.

---

## 3. Confirm gate per-style

**Style plan** — unchanged. `Apply? [y/N/edit/explain N/abort]:` (`confirm.go:125`). Parser whitelist stays `{y,n,edit,explain N,abort}`.

**Style step** — extended prompt: `Apply? [y/N/edit/explain N/approve_next N/approve_thread/abort]:`. Parser additions:

| Token | Maps to | Behavior |
|---|---|---|
| `approve_next 3` | `Response{Kind: RespApproveNext, N: 3}` | Apply current step; auto-`y` the next 2 calls. |
| `approve_thread` | `Response{Kind: RespApproveThread}` | Apply current step; auto-`y` every subsequent call this loop. |

State lives in a new `StepGateState` value owned by `RunLoop` (not persisted to disk):

```go
type StepGateState struct {
    RemainingAutoApprovals int  // approve_next counter
    ApprovedThread         bool // approve_thread sticky flag
}
```

Threaded into `ConfirmPlan` via a new optional parameter (or a `ConfirmPlanStep(in, out, plan, state *StepGateState)` wrapper to keep the plan-style API stable). When `state.ApprovedThread || state.RemainingAutoApprovals > 0`, `ConfirmPlan` short-circuits to `OutcomeApply` without reading from `in` and decrements the counter.

**Non-persistence is deliberate**: each `mooncake pilot` invocation starts a fresh state. Multi-turn thread resume (separate story) re-prompts — operator intent at session A doesn't carry to session B. Audit log emits `kind: user_confirm` with `response: "approve_next 3"` so the JSONL transcript stays honest.

In `--style plan`, `approve_next`/`approve_thread` tokens map to `RespInvalid`. Pass a `style Style` parameter into `ParseResponse`.

---

## 4. Loop termination for `--style step`

Spec §12.3 defines the done signal: LLM emits empty plan. Detection point in `loop.go`:

1. After `SanitizePlan(rawPlan)` returns `planBytes` (`loop.go:96`).
2. Before `WrapInTransaction` (`loop.go:119`).
3. Call `decodePlan(planBytes)` (already in `transaction_wrap.go:107`); if `opts.Style == StyleStep && len(steps) == 0`, write a success iteration log with `Status: "step_done"` and return:

```go
return &LoopResult{
    Iterations: iterations,
    StopReason: StopStepDone,
    FinalLog:   doneLog,
}, nil
```

New constant: `StopStepDone StopReason = "step_done"` in `types.go` (`internal/pilot/types.go:61-70`).

Validation of an empty plan currently goes through `config.ReadConfigWithValidation` — it's likely treated as valid-but-no-op (zero steps is the JSON-Schema-clean form). The empty-plan check needs to happen **before** `WrapInTransaction` because wrapping `[]` may yield an empty `transaction:` block the executor declines. Verify by snapshot test: `decodePlan([]byte("[]"))` returns `(nil, "", nil, nil)`.

If `opts.Style == StylePlan`, an empty plan continues to behave as it does today (validation / no-progress path).

---

## 5. Test strategy

| Test file | New tests |
|---|---|
| `internal/pilot/prompt_test.go` (new) | `TestBuildSystemPrompt_StylePlan_Snapshot`, `TestBuildSystemPrompt_StyleStep_Snapshot` — compare full system-prompt string against a checked-in golden. Prompt drift surfaces in PR diff. `TestBuildPrompt_StyleStep_UserPromptHasNoMultiStepHint`. |
| `internal/pilot/confirm_test.go` | Extend `TestParseResponse` table with `approve_next 1`, `approve_next 12`, `approve_next 0` (invalid), `approve_thread`. Add `TestConfirmPlan_StepGateAutoApproveCountdown` driving 3 consecutive `ConfirmPlanStep` calls with `approve_next 2` on the first — assert exactly two no-input auto-applies follow. Add `TestConfirmPlan_PlanStyleRejectsStepTokens`. |
| `internal/pilot/loop_test.go` | `TestRunLoop_StyleStep_EmptyPlanReturnsStepDone` with a fake `llm.Client` returning `[]` on iter 1 — assert `StopReason == StopStepDone`, exactly one iteration. `TestRunLoop_StyleStep_FeedsResultBack` with stub returning a single-step plan iter 1, `[]` iter 2 — assert iter 2's `LastIteration.Status == "success"`. |

Inject the LLM client via dependency injection — `llm.NewClient()` is currently called unconditionally at `loop.go:46`. Either add a package-level `var newClient = llm.NewClient` for tests to override, or pass a `Client` factory through `RunOptions`. Prefer the var-override pattern, matching `editorRunner` at `confirm.go:300`.

---

## 6. CLI surface

```
mooncake pilot run --goal "…" --style plan      # default; current behavior, just named
mooncake pilot run --goal "…" --style step      # iterative single-step mode
```

Precedence: CLI `--style` > `MOONCAKE_PILOT_STYLE` env > built-in default `plan`. `pilot.yml` loader does not exist (spec §8 lists `config.go` but only `iteration_store.go` is present today — confirmed via `ls internal/pilot/`). **Out of scope** for this story; flag-only.

Validate the flag value in `pilotRunCommand` (`cmd/mooncake.go:1433-1459`): reject anything not in `{plan, step}` with an actionable error before constructing `RunOptions`.

---

## 7. DoD from spec §16 (verbatim) + checklist

> **Goal.** Implement the `--style plan` (default) and `--style step` prompt templates (§12.3) and per-style confirm UX (§10).
>
> **DoD.** A `--style step` run on the install-postgres goal produces an iterative flow: each turn proposes one action, confirm-gates per step, executes, feeds result back to next prompt. Demo on a real local model (Ollama).
>
> **Deps.** `S-pilot-rename`, `S-pilot-confirm-gate`.

Checklist:

- [ ] `--style plan` accepted by CLI and is default — §1 cmd/mooncake.go change, §6
- [ ] `--style step` accepted by CLI — §1, §6
- [ ] Two distinct system prompts rendered, schema chunk identical — §2, §5 snapshot tests
- [ ] Per-step confirm gate exposes `approve_next N` and `approve_thread` — §3
- [ ] Each step executes, result fed back via existing `LastIteration` plumbing — no change needed; `loop.go:159-260` already does this
- [ ] Empty-plan signal terminates step loop with `StopStepDone` — §4
- [ ] Demoable on Ollama — depends on `S-pilot-openai-shape-provider` (not a hard dep but the demo target). Note in PR description.
- [ ] Deps `S-pilot-rename` (shipped per spec §16) and `S-pilot-confirm-gate` (shipped) — confirmed via spec §16 status markers.

---

## 8. Resolved decisions

All five open questions resolved 2026-05-26 (questionary, this session).

1. **Multi-turn coupling** → **Single-invocation only.** Cross-session step continuation defers to `S-pilot-multi-turn`. Within one `mooncake pilot` invocation, `RunLoop`'s in-memory `lastIteration` already feeds turn-to-turn context. Deps stay rename + confirm-gate only.
2. **LLM emits >1 step under `--style step`** → **Reject + retry.** Post-validation, count `len(steps)`. If `> 1`, treat as a contract violation: fail the iteration with a clear error (`--style step requires exactly one step, got N`) and let the loop's existing `LastIteration` plumbing carry the violation back into the next iteration's prompt. The model self-corrects on retry. No silent truncation. Adds `StopReason` value or just uses `execution_failed` semantics; reuse existing.
3. **Assertion-emission instructions** → **Omit from step prompt.** Assertions inside single-step turns are awkward — the model has no follow-up step to validate against. Flag as spec follow-up if a real need surfaces.
4. **Empty-plan validation path** → **Verify in implementation.** `config.ReadConfigWithValidation` should accept zero-step plans; if it doesn't, move empty-plan detection earlier (right after `decodePlan`). Validation is part of the work, not a separate decision.
5. **`approve_thread` audit visibility** → **stderr line at approval time only.** Print `auto-approving remaining steps this thread` once when the gate flips. JSONL `kind: thread_auto_approve_set` audit lands with `S-pilot-multi-turn`. Operator sees the message in their terminal; that's the audit surface v1 has.

### Codebase update since plan was drafted

The cmd-cli-step1b refactor (`dbd15bf0`, 2026-05-26) moved the pilot action func out of `cmd/mooncake.go` into `cmd/pilot_cmd.go`. The plan's references to `cmd/mooncake.go:1433-1459` and `:1842-1886` are stale; edits land in `cmd/pilot_cmd.go` instead. Find by string (`Name:  "pilot"`, `pilot.RunOptions{`), not line number.

---

## 9. Out of scope

- `S-pilot-multi-turn` thread storage (this plan's `StepGateState` is in-memory only).
- `S-pilot-prompt-cache` cache_control wiring on the new template blocks.
- `S-pilot-openai-shape-provider` (Ollama provider) — referenced as demo target only.
- `S-pilot-tool-use-spike` (Anthropic structured output).
- `S-pilot-planner-coder` pipeline composition.
- `pilot.yml` loader and `defaults.style` config key (deferred to a config-loader story; this story is CLI + env only).

---

## 10. Critical files for implementation

- `internal/pilot/prompt.go`
- `internal/pilot/confirm.go`
- `internal/pilot/loop.go`
- `internal/pilot/types.go`
- `cmd/mooncake.go`

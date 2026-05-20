# Pickup — what to work on, right now

**Last curated:** 2026-05-20 (refresh when the table goes stale).

If you have **<5 minutes**, take **item 1** (small code-review reads).
If you have **more**, scan the table and pick the highest-rank
entry that isn't already claimed.

> **Before you start any item below**, check `~/.mooncake/claims.jsonl`:
> ```
> grep '"task":"<task-slug>"' ~/.mooncake/claims.jsonl | tail -1
> ```
> If empty or last status is `done`/`abandoned` → append a `claimed`
> line and proceed. If `claimed`/`in-progress` from a different
> worktree → STOP, pick a different item.

---

## Top picks

| # | Task | Stream | Effort | Where to read | Claim slug |
|---|---|---|---:|---|---|
| 1 | **Continue the code-review cold-read** — 10 package areas still unread. Top candidates: `internal/agentd/{handlers,jsonl_sink,respond,config*,self_mac,self_shutdown*}`, `internal/presets/registry/{loader,validator,expander}`, `internal/actions/text_*`, `internal/actions/os_*` (darwin parity just landed). Read cold, file `archive/code-review/findings/F<NNN>` if a smell appears. | any | S each | [`code-review/TODO.md`](./code-review/TODO.md) "Still to review" | `review-<pkg>` |
| 2 | **spec-66 waves 6–8** — typed plan diff is 5/8 waves done. Remaining: `render_git` + `render_repo` (wave 6, S), `render_transaction` (wave 7, M), handler audit (wave 8, M). Wave 6 is the smallest entry point. | core | S–M | [`streams/core/specs/spec-66-typed-plan-diff.md`](./streams/core/specs/spec-66-typed-plan-diff.md), `internal/diff/` | `spec-66-w6` |
| 3 | **spec-67 pilot — next story: OpenAI-shape provider** — rename/confirm-gate/transaction-wrap/schema-injection/eval-harness all shipped. The next unblocked story adds `OpenAIShapeClient` so local Ollama/vLLM/llama.cpp can drive pilot. Independent of prompt-cache and multi-turn. | agent | S | [`streams/agent/specs/spec-67-mooncake-pilot.md`](./streams/agent/specs/spec-67-mooncake-pilot.md) §`S-pilot-openai-shape-provider` | `pilot-openai-provider` |
| 4 | **spec-58 fleet drift** — highest-leverage unstarted fleet feature. Turns Mooncake from "config management tool" into "fleet operating system." Start with the `InspectPlan` periodic loop; the typed-diff and transport primitives it needs are all in master. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 5 | **proposal-16 `expect_json_schema`** — the one open piece of http.request (waves 1–3 + `expect_json_keys` shipped). Full draft-07 file-path schema validation. Deferred pending validator-library design decision; pick this up only if you want to drive that conversation. | core | S | [`streams/core/proposals/proposal-16-http-request-action.md`](./streams/core/proposals/proposal-16-http-request-action.md) | `http-expect-schema` |
| 6 | **Draft an agent-safety spec** — policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return and is the last open piece of the unfair-advantage statement. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps" | `agent-spec-<topic>` |

---

## Currently claimed by others

Check live: `tail -50 ~/.mooncake/claims.jsonl`.

No items in the table above were claimed in the last 48 hours.
The list shifts fast — check the live file before starting.

---

## How to refresh this file

Manually. The table is **curated**, not generated — the priority
order reflects human judgment from
[`feedback_priority_bugs_before_arch.md`](../../.claude/projects/-home-aleh-projects-mooncake/memory/feedback_priority_bugs_before_arch.md)
+ the 2026-05-17 stream audit. When you finish an item:

1. Mark the claim `done` in `~/.mooncake/claims.jsonl`.
2. If the table got stale, edit this file before pushing.
3. If a new high-priority item showed up (urgent bug, blocker for
   another stream), prepend it as item 1 and re-rank the rest.

The whole file should fit on one screen. If it doesn't, prune the
bottom rows — they belong in the deeper sources.

---

## Where to look for more (the deeper sources)

| If you want… | Look at |
|---|---|
| Stream feature-state + open gaps | `docs-working/streams/<stream>/README.md` (4 streams: core, fleet, dx, agent) |
| Drafted but unstarted specs | `docs-working/streams/<stream>/specs/` |
| Smaller "not-yet-spec" ideas | `docs-working/streams/<stream>/proposals/` |
| Open code-review findings | `docs-working/code-review/TODO.md` + `archive/code-review/findings/F*.md` |
| Architecture pressure points | `docs-working/arch-report/` (latest dated report) |
| Strategic positioning | `VISION.md`, `docs-working/positioning.md` |
| In-flight work + claims | `~/.mooncake/claims.jsonl` (off-tree, shared across worktrees) |
| Codebase conventions | `CLAUDE.md`, `AGENT.md`, `LLM_GUIDE.md` |

---

## Hard rules (mirrored from CLAUDE.md)

- **Don't commit or push** unless the user explicitly says so.
- **Implementation work happens in a worktree**: `git worktree add ../mooncake-<slug> -b worktree-<slug>`. Doc-only edits can stay on the current checkout.
- **Claim before editing.** See the `grep ~/.mooncake/claims.jsonl` snippet at the top.
- **Soft caps are advisory**, not blocking. `make budget-status` shows current state.

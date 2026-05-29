# Pickup — what to work on, right now

**Last curated:** 2026-05-29 (refresh when the table goes stale).

Scan the table and take the highest-rank entry that isn't already
claimed; smaller (S) items suit short sessions. History of what
already landed lives in `git log` and `~/.mooncake/claims.jsonl` —
this file is forward-looking only.

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
| 1 | **F058 follow-up — centralize cancel classification (choice 2).** The minimal F058 fix stopped the double-count for handlers that return a `*Result` (shell, spec-69 RawRunners). Handlers that return `(nil, err)` never reach `syncResultEnvelope`, so on a mid-flight cancel they still land `Failed`-only (no `Cancelled` bump) → those steps exit 1 instead of 130 on `--timeout`/fleet/MCP. Fix: classify cancel in `ExecuteStep` off `ec.Svc.Ctx.Err()` regardless of handler return shape, and remove the `syncResultEnvelope`-only `Stats.Cancelled` bump so there's exactly one bump site. Touches the F4 design + its unit tests (`sync_envelope_test.go`) and `cancel_classification_test.go`. **Worktree already created: `../mooncake-f058-followup` (branch `worktree-f058-followup`).** Read F058 §"Proposed fix" choice 2 first. | core | M | [`archive/code-review/findings/F058-cancel-double-counts-as-failed.md`](./archive/code-review/findings/F058-cancel-double-counts-as-failed.md) | `fix-f058-classify-centralize` |
| 2 | **Continue the code-review cold-read.** `internal/executor/` is now fully covered (re-reviewed 2026-05-29, F058 closed). Remaining under-reviewed candidates: `internal/plan/` (planner.go covered; the rest unread), `internal/template/` (clean per quick check but no formal pass). | any | S each | [`code-review/TODO.md`](./code-review/TODO.md) "Reviewed (done)" table for what's covered | `review-<pkg>` |
| 3 | **spec-58 fleet drift** — highest-leverage unstarted fleet feature. Turns Mooncake from "config management tool" into "fleet operating system." Start with the `InspectPlan` periodic loop; the typed-diff and transport primitives it needs are all in master. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 4 | **Draft an agent-safety spec** — policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return and is the last open piece of the unfair-advantage statement. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps" | `agent-spec-<topic>` |
| 5 | **Author `docs-next/guide/modules.md`** — `mooncake mod` (add/cache list/cache clean) has no user-facing guide. The retired preset-CLI guide is gone (proposal-07 deletion). `MODULES.md` at repo root has the seed material; flesh it out into a guide and re-add to the mkdocs nav. | dx | S | `MODULES.md`, `cmd/mod/mod.go` for the CLI surface | `modules-guide` |

---

## Currently claimed by others

Claims shift fast and live off-tree — always check before starting:
`tail -50 ~/.mooncake/claims.jsonl`.

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
- **Soft caps are advisory**, not blocking. `mooncake task budget-status` shows current state.

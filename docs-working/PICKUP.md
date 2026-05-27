# Pickup — what to work on, right now

**Last curated:** 2026-05-27 (refresh when the table goes stale).

**Recently landed (2026-05-27):**
cmd/ subsystem-boundary refactor complete. 16 sub-packages now stand
under `cmd/` (agentd, cmdutil, docs, doctor, fleet, history, init,
kernel, mcp, mod, query, schema, snapshot, step, task, tool); cmd/
root went from 56 files to 4 (mooncake.go + 3 test files);
`cmd/mooncake.go` from 2043 LOC to 194. Presets CLI retired
(`2b7eee8e`); orphan packages `internal/recommend/` +
`internal/presets/registry/` + `docs-next/presets/` deleted
(`4db53ad6`, −5629 LOC). Doc-side `mooncake presets` references
catalogued in proposal-07 (see item 3 below).

Code-review cold-read queue **cleared and both findings fixed**
(2026-05-27): 8 package areas read cold; F051 (os_* ctx.TODO)
landed `6ae880da`, F052 (kernel/validate os.Exit) landed
`fa05dbd7`. 51 findings closed total. See `code-review/TODO.md`.

**Spec-66 typed plan diff DONE** (2026-05-27,
`103bcb4d`→`1a698906`): waves 6 (git/repo), 7 (transaction/try),
8 (git.clone, os.ssh_key, os.sysctl, pkg.upgrade + audit). 16
renderers registered. `mooncake plan --diff` now shows typed
output for every common action category.

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
| 1 | **Continue the code-review cold-read** — queue empty as of 2026-05-27. Pick a package not in `code-review/TODO.md`'s "Reviewed (done)" table and read it cold. Top under-reviewed candidates: `internal/executor/` (4003 LOC, only spot-checked), `internal/plan/` (planner.go covered; the rest unread), `internal/template/` (clean per quick check but no formal pass). | any | S each | [`code-review/TODO.md`](./code-review/TODO.md) "Reviewed (done)" table for what's covered | `review-<pkg>` |
| 2 | ~~**spec-66 waves 6–8**~~ — **DONE** 2026-05-27 (`103bcb4d`→`1a698906`). All 8 waves shipped; 16 renderers registered. See `streams/core/specs/spec-66-typed-plan-diff.md`. Drop to a new item next refresh. | core | — | — | — |
| 3 | **proposal-07 presets-CLI docs migration** — ~30 user-facing string + doc references still mention `mooncake presets …` (a command that no longer exists). 5 tiers: delete 3 obsolete guide docs (~2k LOC), rewrite 6 docs with isolated mentions, fix 4 in-binary user-facing strings (doctor fix message, first-run hint, scaffold templates), 1 round of maintainer-facing comment polish. Mostly mechanical; 3 open questions flagged for the executing agent. | dx | S | [`streams/dx/proposals/proposal-07-presets-cli-docs-migration.md`](./streams/dx/proposals/proposal-07-presets-cli-docs-migration.md) | `presets-docs-migration` |
| 4 | **pilot feedback for non-cmd actions** — output capture (`#34`) only surfaces stdout from cmd/shell-family steps into `LastIteration.LastStepStdout`. `file.write`, `file.copy`, `pkg.*`, `os.service`, etc. complete with no signal the LLM can read, so in `--style step` the model has no positive evidence its step succeeded and tends to re-emit the same plan (`StopNoProgress` instead of `StopStepDone`). Surfaced from real Ollama + Claude testing 2026-05-27. Add per-action-type summary lines (e.g. `wrote 16 bytes to <path>`) to `LastIteration`, surface in the prompt. | agent | S | [`internal/pilot/output_capture.go`](../../internal/pilot/output_capture.go), `internal/pilot/loop.go` LastIteration build sites | `pilot-feedback-non-cmd` |
| 5 | **spec-58 fleet drift** — highest-leverage unstarted fleet feature. Turns Mooncake from "config management tool" into "fleet operating system." Start with the `InspectPlan` periodic loop; the typed-diff and transport primitives it needs are all in master. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 6 | **proposal-16 `expect_json_schema`** — the one open piece of http.request (waves 1–3 + `expect_json_keys` shipped). Full draft-07 file-path schema validation. Deferred pending validator-library design decision; pick this up only if you want to drive that conversation. | core | S | [`streams/core/proposals/proposal-16-http-request-action.md`](./streams/core/proposals/proposal-16-http-request-action.md) | `http-expect-schema` |
| 7 | **Draft an agent-safety spec** — policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return and is the last open piece of the unfair-advantage statement. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps" | `agent-spec-<topic>` |

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

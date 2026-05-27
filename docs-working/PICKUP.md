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
| 1 | **Continue the code-review cold-read** — package areas still unread. Top candidates: `internal/agentd/{handlers,jsonl_sink,respond,config*,self_mac,self_shutdown*}`, `internal/actions/text_*`, `internal/actions/os_*` (darwin parity just landed), the new `cmd/kernel/` package (largest cmd/ sub-package after the 2026-05-27 boundary refactor). Read cold, file `archive/code-review/findings/F<NNN>` if a smell appears. (Note: `internal/presets/registry/` listed in the previous revision was deleted in `4db53ad6` — drop from the queue.) | any | S each | [`code-review/TODO.md`](./code-review/TODO.md) "Still to review" | `review-<pkg>` |
| 2 | **spec-66 waves 6–8** — typed plan diff is 5/8 waves done. Remaining: `render_git` + `render_repo` (wave 6, S), `render_transaction` (wave 7, M), handler audit (wave 8, M). Wave 6 is the smallest entry point. | core | S–M | [`streams/core/specs/spec-66-typed-plan-diff.md`](./streams/core/specs/spec-66-typed-plan-diff.md), `internal/diff/` | `spec-66-w6` |
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

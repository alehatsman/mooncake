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
catalogued in proposal-07 (see item 2 below).

Code-review cold-read queue **cleared and both findings fixed**
(2026-05-27): 8 package areas read cold; F051 (os_* ctx.TODO)
landed `6ae880da`, F052 (kernel/validate os.Exit) landed
`fa05dbd7`. 51 findings closed total. See `code-review/TODO.md`.

**Spec-66 typed plan diff DONE** (2026-05-27,
`103bcb4d`→`1a698906`): waves 6 (git/repo), 7 (transaction/try),
8 (git.clone, os.ssh_key, os.sysctl, pkg.upgrade + audit). 16
renderers registered. `mooncake plan --diff` now shows typed
output for every common action category.

**Pilot per-step summaries DONE** (2026-05-27, `e28c4842`):
`pilot-feedback-non-cmd` shipped. `IterationSummary.StepSummaries`
now carries one line per completed step regardless of action type;
`--style step` loops see positive evidence that `file.write` /
`pkg.*` / `os.service` ran. `executor.Result.ToMap()` now exposes
`reason`; new `pilot.summarizeStep()` builds the lines.

**Presets-CLI docs migration DONE** (2026-05-27, `eee2c15f`):
proposal-07 fully executed. 20 files, −2,165 LOC. Deleted 3
obsolete preset guides + orphan CSS + broken mkdocs nav block;
rewrote 4 in-binary user strings, 4 scaffold templates, 6 docs
to point at `mooncake mod` / `mooncake actions list` /
`./presets/ + use:`. Only historical retire-note references to
the retired CLI remain in-tree.

If you have **<5 minutes**, take **item 1** (cold-read of one
under-reviewed package). If you have **more**, scan the table
and pick the highest-rank entry that isn't already claimed.

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
| 2 | **spec-58 fleet drift** — highest-leverage unstarted fleet feature. Turns Mooncake from "config management tool" into "fleet operating system." Start with the `InspectPlan` periodic loop; the typed-diff and transport primitives it needs are all in master. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 3 | **proposal-16 `expect_json_schema`** — the one open piece of http.request (waves 1–3 + `expect_json_keys` shipped). Full draft-07 file-path schema validation. Deferred pending validator-library design decision; pick this up only if you want to drive that conversation. | core | S | [`streams/core/proposals/proposal-16-http-request-action.md`](./streams/core/proposals/proposal-16-http-request-action.md) | `http-expect-schema` |
| 4 | **Draft an agent-safety spec** — policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return and is the last open piece of the unfair-advantage statement. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps" | `agent-spec-<topic>` |
| 5 | **Author `docs-next/guide/modules.md`** — `mooncake mod` (add/cache list/cache clean) has no user-facing guide. The retired preset-CLI guide is gone (proposal-07 deletion). `MODULES.md` at repo root has the seed material; flesh it out into a guide and re-add to the mkdocs nav. | dx | S | `MODULES.md`, `cmd/mod/mod.go` for the CLI surface | `modules-guide` |

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

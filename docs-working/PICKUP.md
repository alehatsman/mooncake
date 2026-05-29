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
| 1 | **spec-58 fleet drift** — highest-leverage unstarted fleet feature. Turns Mooncake from "config management tool" into "fleet operating system." Start with the `InspectPlan` periodic loop; the typed-diff and transport primitives it needs are all in master. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 2 | **Draft an agent-safety spec** — policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return and is the last open piece of the unfair-advantage statement. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps" | `agent-spec-<topic>` |
| 3 | **Author `docs-next/guide/modules.md`** — `mooncake mod` (add/cache list/cache clean) has no user-facing guide. The retired preset-CLI guide is gone (proposal-07 deletion). `MODULES.md` at repo root has the seed material; flesh it out into a guide and re-add to the mkdocs nav. | dx | S | `MODULES.md`, `cmd/mod/mod.go` for the CLI surface | `modules-guide` |
| 4 | **spec-73 `vault:` secret provider** — encrypted-at-rest secrets that live *in* the repo, as an age-backed provider for the existing `!secret` resolver (spec-23 §3). Closes the fleet gap where every secret (sudo pass, dex tokens, service creds) is a hand-placed gitignored `0600` file with no versioning or fan-out. Draft only — needs a read on priority vs the fleet/agent work above. | core | M | [`streams/core/specs/spec-73-secret-vault-provider.md`](./streams/core/specs/spec-73-secret-vault-provider.md) | `spec-73-w1` |
| 5 | **Test-coverage gaps in churned packages** (broad follow-up). The code-review cold-read of `internal/{executor,plan,template}` is now complete; the last open review-queue item is a test-writing pass over packages changed without test catch-up (spec-66 wave 5, proposal-16 wave 3, R2.1c phase 2). Scope with the user first — it's a test pass, not a review pass. | any | M | [`code-review/TODO.md`](./code-review/TODO.md) "Still to review" | `test-coverage-gaps` |

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

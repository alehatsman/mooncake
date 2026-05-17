# Pickup — what to work on, right now

**Last curated:** 2026-05-18 (refresh when the table goes stale).

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
| 1 | **Continue the code-review cold-read** — 10 packages still unread (item-3 in `TODO.md`). Recently merged candidates: `internal/agentd/{handlers,jsonl_sink,respond,config*}.go`, `internal/presets/registry/{loader,validator,expander}.go`, `internal/actions/text_*`. Read cold, file `findings/F<NNN>` if a smell appears. | any | S each | [`code-review/TODO.md`](./code-review/TODO.md) "Still to review" | `review-<pkg>` |
| 2 | **proposal-16 `expect_json_schema`** — closes the last open piece of the http.request proposal. Adds full-JSON-schema response validation (the broader sibling of `expect_json_keys`, which shipped 2026-05-17). Note: deferred pending validator-library + schema-loading design conversation; pick this up only if you want to drive that decision. | core | S | [`streams/core/proposals/proposal-16-http-request-action.md`](./streams/core/proposals/proposal-16-http-request-action.md), `internal/actions/http_request/` | `http-expect-schema` |
| 3 | **spec-58 fleet drift** — drafted, nobody's touching it. README calls it "the single feature that would turn Mooncake from config management tool into fleet operating system." Start with the `InspectPlan` periodic loop. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 4 | **Draft an agent-safety spec** — agent stream has zero un-specced safety primitives. Pick one: policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps", [`VISION.md`](../VISION.md) | `agent-spec-<topic>` |
| 5 | **Audit Reverse() coverage** — `streams/core/README.md` open-gaps still says `os.*`, `pkg.repo`, `pkg.hold`, `os.service` "return refusal stubs"; spot-checks (`pkg_repo/reverse.go`, `service/reverse.go`) show full implementations. Walk every `internal/actions/*/reverse.go`, list the actual gaps (if any), then update or close the open-gap line in `streams/core/README.md`. | core | S | [`streams/core/README.md`](./streams/core/README.md), `internal/actions/*/reverse.go` | `reverse-audit` |

---

## Currently claimed by others

Check live: `tail -50 ~/.mooncake/claims.jsonl`.

Items above marked claimed in the last 24 hours include
`spec-66 wave 5` (typed-diff renderers for cron + mount) and the
darwin-parity wave for `pkg.*` and `os.*`. The list shifts fast —
8–12 merges/work-day cadence as of 2026-05-17.

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
| Open code-review findings | `docs-working/code-review/TODO.md` + `findings/F*.md` |
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

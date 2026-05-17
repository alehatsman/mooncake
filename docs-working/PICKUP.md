# Pickup — what to work on, right now

**Last curated:** 2026-05-17 (refresh when the table goes stale).

If you have **<5 minutes**, do **item 1**. It's a 1-line code change
with a written-up finding.
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
| 1 | **F048 — fleet.yml strict YAML** — one-line: `yaml.Unmarshal` → `yaml.NewDecoder + KnownFields(true)` in `internal/fleet/machine.go:88`. Add a unit test asserting unknown fields are rejected. | code-review | XS | [`docs-working/code-review/findings/F048-fleet-machine-manifest-non-strict-yaml.md`](./code-review/findings/F048-fleet-machine-manifest-non-strict-yaml.md) | `fix-F048` |
| 2 | **`service` handler split** — package is at 1844 LOC (~23% over the 1500 soft cap). Split into `internal/actions/service/{linux,darwin,windows}` sub-packages, mirroring the per-OS pattern already used by `os_user`/`os_group`. | core | M | [`CLAUDE.md` §1](../CLAUDE.md), `make budget-status`, `internal/actions/service/` | `service-split` |
| 3 | **spec-58 fleet drift** — drafted, nobody's touching it. README calls it "the single feature that would turn Mooncake from config management tool into fleet operating system." Start with the `InspectPlan` periodic loop. | fleet | L | [`streams/fleet/specs/spec-58-fleet-drift.md`](./streams/fleet/specs/spec-58-fleet-drift.md) | `spec-58-w1` |
| 4 | **Draft an agent-safety spec** — agent stream has zero un-specced safety primitives. Pick one: policy DSL, plan signing, per-action quotas, sandbox mode, or deterministic replay. Replay has the highest "demoable win" return. | agent | M (draft only) | [`streams/agent/README.md`](./streams/agent/README.md) §"Open gaps", [`VISION.md`](../VISION.md) | `agent-spec-<topic>` |
| 5 | **Continue the code-review cold-read** — 11 packages still unread. `internal/fleet/machine.go` was item 1 → produced F048. Try `internal/agentd/self_mac.go` + `self_shutdown*.go` next (brand-new from today's WoL work; no review yet). | any | S each | [`code-review/TODO.md`](./code-review/TODO.md) "Still to review" | `review-<pkg>` |
| 6 | **proposal-16 `expect_json_schema`** — closes the last open piece of the http.request proposal. Adds full-JSON-schema response validation (the broader sibling of `expect_json_keys`, which shipped today). | core | S | [`streams/core/proposals/proposal-16-http-request-action.md`](./streams/core/proposals/proposal-16-http-request-action.md), `internal/actions/http_request/` | `http-expect-schema` |
| 7 | **Reverse-capture rollout to refusing handlers** — `os.*` family, `pkg.repo`, `pkg.hold`, `os.service` still return refusal stubs from `Reverse()`. Pattern is spec-26 reverse-capture v1 (already done for `git.checkout`/`git.config`); one PR per handler. | core | S each (×13) | [`streams/core/README.md`](./streams/core/README.md) "Open gaps", `internal/actions/git_checkout/` for the reference impl | `reverse-capture-<handler>` |

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

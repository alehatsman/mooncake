# fleet Proposals

Six concrete CLI / DX proposals for the fleet stream, distilled from
walking every fleet subcommand's `--help` after the 2026-05-15 manual
testing pass plus 15 hands-on fleet scenarios.

These are **brainstormed proposals**, not specs. Each cites pain
points from the audit and proposes a focused change.

| # | Proposal | Effort | Value | Why |
|---|---|---|---|---|
| [01](./proposal-01-peer-targeting-unify.md) | Unify `--peers` + `--peer-filter` into one `--peer` DSL | S | High | Halves the surface; every fleet subcommand has both flags today |
| [02](./proposal-02-fleet-kill.md) | `fleet kill <peer> <run-id>` — cancel in-flight runs | S | High | Closes the hole where `fleet ps` shows runs you can't stop |
| [03](./proposal-03-fleet-shell-cp.md) | `fleet shell <peer>` + `fleet cp` | S | Medium | Closes the everyday gap that drives operators back to SSH |
| [04](./proposal-04-global-flags.md) | Hoist `--no-color/--json/--peers-file/--parallel/--timeout` to global fleet flags | XS | Medium | Trivial refactor; cuts ~80 lines of repeated help text |
| [05](./proposal-05-doctor-status-last-applied.md) | `fleet doctor --all` + `fleet status` shows last-applied | S | High | The daily-driver answer to "is the fleet healthy + in sync?" |
| [06](./proposal-06-bootstrap-resume.md) | Idempotent `fleet bootstrap` + `--resume` + `--dry-run` | S | Medium | Recoverable onboarding; matters most for agent-driven flows |

## Recommended order

1. **04 global flags** — XS, mechanical, lowers the bar for the rest
2. **01 peer targeting unify** — S, pairs with 04 (both touch flag surface)
3. **05 doctor --all + last applied** — S, ships the daily-driver question
4. **02 fleet kill** — S, requires agentd lifecycle hooks; pairs with bootstrap-resume
5. **06 bootstrap idempotent + resume** — S, builds on agentd lifecycle work
6. **03 fleet shell + cp** — S, needs PTY streaming; lowest-priority but high-impact

## Cross-cutting themes

### Theme A: One flag, one meaning

Today the fleet CLI has redundant ways to express the same intent:
- Peer selection: `--peers` AND `--peer-filter` (proposal 01)
- Output format: `--json` (subcommand-local) vs core's `--format json` (proposal 04, DX proposal 02)
- Same flag on every subcommand: `--no-color`, `--peers-file` (proposal 04)

The pattern: pick one shape, hoist where global, deprecate the dupes.

### Theme B: Audit-preserving everyday workflow

Mooncake's fleet pitch ("Docker for AI agents") only holds if
operators never need SSH for routine work. Today the gap pulls them
out: cancel a run (proposal 02), interactive triage (proposal 03),
file push (proposal 03 cp), and recoverable install (proposal 06)
are the obvious holes.

Closing these keeps the audit trail intact AND makes the fleet
pitch real for daily use, not just slides.

### Theme C: The daily question deserves one command

"Is the fleet healthy and in sync?" should be answerable by **one**
command, not three. Today it requires `fleet status` + per-peer
`fleet doctor` + `fleet ps`. Proposal 05 collapses these. Combined
with spec-58 (drift detection, planned), it becomes the morning
glance.

## What's NOT in this batch

Things that came up but didn't make the cut:

- **`fleet ssh <peer>`** — convenience wrapper around SSH using
  peers.toml host field. Defeats the audit pitch; if shell access
  matters, do it through agentd (proposal 03).
- **`fleet edit <peer>:<path>`** — download → edit → upload.
  Composes from proposal 03's primitives; defer.
- **TUI mode for `fleet ps` / `watch`** — nice-to-have; out of
  scope for v1 CLI iteration.
- **`fleet observe` consistency with `apply` on `<peer-positional>` vs `--peer`**
  — observe always fans out (no positional), other commands take
  a positional. Editorial decision; small impact.
- **`fleet upgrade --rollback`** — if a peer comes up broken after
  upgrade, revert to previous binary. Composes with proposal 02's
  state machine; defer.
- **`fleet drift`** — covered by spec-58 already.

## Audit receipts

Manual testing rounds that touched fleet:
- Round 30 — fleet status / exec / observe / doctor / pair / bootstrap
- Round 31 — fleet ps / logs / watch / facts
- Round 34 — peer filtering
- Round 37 — fleet bootstrap error paths + presets
- Round 46 — sudo-pass flows on fleet exec

Findings file: `docs-working/analysis/findings-2026-05-15/` —
specifically the fleet-related entries in `cli-and-friction.md` and
`positive-keepers.md`.

The positive-keepers list is long for fleet; the friction list is
short. Most of these proposals reduce friction without redesigning
what already works.

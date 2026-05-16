# dx Proposals

Six concrete DX proposals distilled from the 2026-05-15 manual-tester
audit (59 iterations, ~250 `mooncake apply` invocations, 87 numbered
findings). Each proposal links back to specific findings as receipts.

These are **not specs** — they're brainstormed proposals. Pick the
ones worth promoting to specs (single-PR-able items first); fold the
rest into the broader DX roadmap if/when there's user pull.

| # | Proposal | Effort | Value | Why now |
|---|---|---|---|---|
| [01](./proposal-01-step-name-defaults.md) | Step name defaults from action + content | XS | Medium | Most visible papercut; trivial PR |
| [02](./proposal-02-output-middle-ground.md) | `--output-format readable` (new default) | S | High | Fixes "can't see what happened" without breaking machine API |
| [03](./proposal-03-mooncake-watch.md) | `mooncake watch` hot-reload loop | S | High | Iteration speed for active development |
| [04](./proposal-04-actions-show.md) | `mooncake actions show <name>` | XS | High | Closes the discoverability gap; data already exists |
| [05](./proposal-05-error-recipes.md) | Doctor-style `→ why / → fix` on every error | M | High | Doctor already nails the template; port everywhere |
| [06](./proposal-06-mooncake-lint.md) | `mooncake lint` anti-pattern detection | S | Medium | Catches what schema validation can't |

## Recommended order

If picking from this list, ship in roughly this order:

1. **04 actions show** — XS, no risk, instant DX win
2. **01 step name defaults** — XS, fixes the most visible papercut
3. **05 error recipes** — M, cross-cutting; ship per-PR over time
4. **02 output middle ground** — S; ship behind feature flag, flip default later
5. **03 mooncake watch** — S, new feature, won't break anything
6. **06 mooncake lint** — S, separable; opt-in CLI surface

01 and 04 together cost ~1.5 days and meaningfully change the
first-five-minutes experience for new users.

## Cross-cutting themes

Three patterns emerge across the proposals:

### Theme A: surface what exists; don't add storage

01, 02, 04 all expose information the system already has. The
registry knows action metadata (proposal 04). The runner already
captures stdout (proposal 02). The plan tree already has action
types (proposal 01). These are *render*, not *engine*, changes.

### Theme B: the doctor template generalizes

Doctor's "✓ ℹ ⚠" + "fix: <action>" + "used by: <context>" pattern
is the project's best UX win. Proposal 05 ports it everywhere; the
other proposals' output formats borrow from it.

### Theme C: opinions belong in DX, not core

Proposal 06 (lint) makes this explicit: core enforces invariants;
DX provides guidance. Same separation lets us iterate on error
copy (proposal 05) and output style (proposal 02) without touching
action handlers.

## Out of scope

Worth considering separately, but didn't make this round:

- **`mooncake fmt`** (canonical playbook formatter — adjacent to
  proposal 06)
- **`mooncake repl`** (interactive iterative-build loop — adjacent
  to proposal 03)
- **Editor / IDE schema auto-install** (`mooncake schema install
  --editor vscode|nvim` — adjacent to proposal 04)
- **Per-step duration in the artifact bundle** (already there in
  events.jsonl; surface in `history show <N>`)
- **Stdin support** (`mooncake apply -c -` for piped YAML) — small
  win for the agent-driven case

## Audit receipts

Findings file: `docs-working/analysis/findings-2026-05-15/` —
specifically `cli-and-friction.md` for the inputs that motivated
proposals 01, 02, 04, 05.

Iteration count breakdown:
- ~250 `mooncake apply` invocations (proposal 03 trigger)
- ~50 `grep <field> schema.json` lookups (proposal 04 trigger)
- ~30 cases where stdout was missing in text output (proposals 01, 02)
- ~15 ad-hoc playbooks I wrote that contained obvious anti-patterns
  (proposal 06)
- ~10 confusing-error-message rounds (proposal 05)

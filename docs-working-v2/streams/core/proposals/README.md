# core Proposals

Six proposals for the kernel — action handlers, planner, executor,
the four-method ABI. Distilled from the 2026-05-15 manual-tester
audit (~50 actions tested, dozens of result-shape inconsistencies
catalogued in `findings-2026-05-15/silent-success-bugs.md` and
`cli-and-friction.md`).

These are brainstormed proposals, not specs. The pattern most of
them push toward: **codify what's informally consistent in the wild,
make divergent handlers conform**.

| # | Proposal | Effort | Value | Why |
|---|---|---|---|---|
| [01](./proposal-01-result-schema-conventions.md) | Standardize result schema: 5-field common envelope | M | High | Every action returns its own shape; agents have to know each |
| [02](./proposal-02-recap-counter-discipline.md) | Codify `ok / changed / skipped / failed / reverted / cancelled` | S | High | Counters are the user's exit signal; today their semantics are folklore |
| [03](./proposal-03-step-validator-consistency.md) | `mooncake step` enforces `additionalProperties: false` | XS | Medium | Closes the asymmetry where `step` accepts unknown fields silently |
| [04](./proposal-04-typed-plan-diff.md) | Typed plan diffs (per-action-type, not just file) | M | High | `plan --diff` is the safety story; only file diffs are useful today |
| [05](./proposal-05-action-capability-flags.md) | Surface `Permissions/Diff/Cost/Reverse` capability flags | XS | Medium | spec-22 shipped the methods; make their outputs inspectable |
| [06](./proposal-06-failed-vs-error-distinction.md) | Reconcile `failed: false` + `error: "..."`; query vs. mutation taxonomy | S | Medium | Recurring confusion across observe/wait/os actions (#61 umbrella) |

## Recommended order

1. **03 step validator** — XS, no controversy, fixes #83
2. **05 capability flags** — XS, no controversy, exposes existing data
3. **01 result schema** — M, deprecation window required; ship per-PR
4. **06 failed/error distinction** — S, depends on 01; handler-by-handler
5. **02 recap counter discipline** — S, depends on 01 + 06
6. **04 typed plan diff** — M, biggest scope; ship last

The first two are pure cleanup. The middle three are the disciplined
refactor of the result + recap surface. The last is new functionality
that builds on the disciplined surface.

## Cross-cutting themes

### Theme A: Codify what's informally consistent

Several action handlers (os.group, pkg.hold, pkg.repo, text.line,
os.cron, os.sysctl, os.ssh_key) already emit `operation:
create|update|delete|noop`. Proposal 01 makes this the rule;
proposal 02 turns it into the recap-counter input. Most of the
discipline is already lived in the wild — just not enforced for
new handlers.

### Theme B: Make the ABI inspectable

spec-22 declared four ABI methods per handler
(`Permissions/Diff/Cost/Reverse`). Their existence is invisible
today unless you read source. Proposal 04 surfaces `Diff` output;
proposal 05 surfaces capability flags. The ABI becomes a contract
the user can see and reason about, not just an implementation
convention.

### Theme C: Query-style vs. mutation-style actions

Six families: query (observe.*, read.*, repo.*, wait.*), mutation
(file.*, pkg.*, os.*, text.*, git.*), assert (a special hybrid),
compound (transaction:, try:), informational (log:, vars:), and
shell (the opaque escape hatch). Today `failed:` semantics blur
across families. Proposal 06 sharpens them: a query returning
"absent" is success; a wait timeout is failure; a mutation that
didn't happen is failure.

## What's NOT in this batch

- **for_each variable scope rules** — `as: foo` inside a for_each
  block: visible outside? per-iteration? Today: unclear. Worth a
  proposal but not the highest pain right now.
- **`when:` evaluation in for_each** — per-iteration vs. once?
  Documented assumption needed.
- **`!secret` typed refs** — covered by spec-23 §3 (shipped per
  agent README) but the agent surface still leaks secrets in
  some error paths. Open in agent stream.
- **`retry:` per-action defaults** — some actions (file.download)
  should retry on network blips by default. Currently always
  step-level. Worth a future proposal.
- **Plan-time facts staleness** — `--allow-stale` exists but its
  failure mode for non-trivial fact drift is unclear. Defer.

## Audit receipts

Findings from 2026-05-15 that motivate these proposals:
- **Proposal 01**: #61 (failed/error inconsistency), #70 (.value
  wrapper drift), #22 (step result truncation)
- **Proposal 02**: #2 (creates/unless recap), #45 (transactions
  reverted miscount), #28 (failed_when on assert)
- **Proposal 03**: #83 (step validator gap)
- **Proposal 04**: positive-keepers entry for file-diff; coverage
  gaps for non-file actions
- **Proposal 05**: registry already tracks SUDO/CHECK columns;
  spec-22 phases declared the methods
- **Proposal 06**: #61 again — query/wait/mutation taxonomy is
  the unifier

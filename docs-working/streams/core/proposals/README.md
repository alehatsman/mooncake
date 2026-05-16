# core Proposals

Proposals for the kernel — action handlers, planner, executor, the
four-method ABI, template renderer.

Two source streams feed this folder:

1. **Audit-distilled (01–06)** — from the 2026-05-15 manual-tester
   pass (~50 actions tested, result-shape inconsistencies catalogued
   in `findings-2026-05-15/silent-success-bugs.md` and
   `cli-and-friction.md`). Pattern: **codify what's informally
   consistent in the wild, make divergent handlers conform**.
2. **User-filed feature requests (07–10)** — gaps surfaced while
   migrating real dotfiles from `shell:` blocks to typed actions on
   2026-05-16. Pattern: **a real shell workaround exists; the typed
   action equivalent is one missing field away**.

These are brainstormed proposals, not specs.

| # | Proposal | Effort | Value | Why |
|---|---|---|---|---|
| [01](./proposal-01-result-schema-conventions.md) | Standardize result schema: 5-field common envelope | M | High | Every action returns its own shape; agents have to know each |
| [02](./proposal-02-recap-counter-discipline.md) | Codify `ok / changed / skipped / failed / reverted / cancelled` | S | High | Counters are the user's exit signal; today their semantics are folklore |
| [03](./proposal-03-step-validator-consistency.md) | `mooncake step` enforces `additionalProperties: false` | XS | Medium | Closes the asymmetry where `step` accepts unknown fields silently |
| [04](./proposal-04-typed-plan-diff.md) | Typed plan diffs (per-action-type, not just file) | M | High | `plan --diff` is the safety story; only file diffs are useful today |
| [05](./proposal-05-action-capability-flags.md) | Surface `Permissions/Diff/Cost/Reverse` capability flags | XS | Medium | spec-22 shipped the methods; make their outputs inspectable |
| [06](./proposal-06-failed-vs-error-distinction.md) | Reconcile `failed: false` + `error: "..."`; query vs. mutation taxonomy | S | Medium | Recurring confusion across observe/wait/os actions (#61 umbrella) |
| [07](./proposal-07-pkg-aur-support.md) | `pkg.install: manager: yay` / `paru` for Arch AUR | S | Medium | Today's AUR install is a 20-line `shell: yay -S` block — no idempotency, no plan diff |
| [08](./proposal-08-pkg-brew-taps-and-tolerant-rc.md) | `pkg.repo: manager: brew` for taps; tolerate idempotent non-zero rc | S | Medium | `brew tap` exits 1 on re-tap; cask vs. formula is shell-only today |
| [09](./proposal-09-template-now-filter.md) | Working `now` / `apply_started_at` for timestamped strings | XS | Medium | Pongo2's `{% now %}` silently no-ops; blocks rolling backup patterns |
| [10](./proposal-10-wait-http-post-body.md) | `wait_http`: POST + headers + body | S | Medium | GET-only blocks polling services with no health endpoint (e.g. vLLM embeddings) |

## Recommended order

### Audit-distilled (kernel discipline)

1. **03 step validator** — XS, no controversy, fixes #83
2. **05 capability flags** — XS, no controversy, exposes existing data
3. **01 result schema** — M, deprecation window required; ship per-PR
4. **06 failed/error distinction** — S, depends on 01; handler-by-handler
5. **02 recap counter discipline** — S, depends on 01 + 06
6. **04 typed plan diff** — M, biggest scope; ship last

The first two are pure cleanup. The middle three are the disciplined
refactor of the result + recap surface. The last is new functionality
that builds on the disciplined surface.

### User-filed (independent; ship when motivated)

- **09 template `now`** — XS, unblocks rolling-backup patterns; no
  dependency on the kernel-discipline batch.
- **10 wait_http POST** — S, additive on the existing `wait_http`
  handler; no schema breakage.
- **07 pkg.install yay/paru** — S, additive `manager:` value; reuses
  the pacman driver.
- **08 pkg.repo brew taps + tolerant rc** — S, but pairs with the
  broader question of what `pkg.repo`'s driver framework looks like
  on non-APT/yum managers. Worth sequencing after a pkg-driver
  audit.

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

User-filed receipts (2026-05-16 dotfiles migration):
- **Proposal 07**: `platforms/arch/packages.yml` — 15-package
  `shell: yay -S` block
- **Proposal 08**: `platforms/macos/packages.yml` — `brew tap
  ... 2>&1 | grep -v "already tapped"` workaround
- **Proposal 09**: `components/zsh/index.yml` (and 4 sibling
  components) — `cp ~/.zshrc ~/.dotfiles-backup/.zshrc.$(date +...)`
- **Proposal 10**: `components/mcsearch/server.yml` — 60s polling
  loop for `POST /v1/embeddings`

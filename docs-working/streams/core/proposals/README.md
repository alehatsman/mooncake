# core Proposals

Proposals for the kernel — action handlers, planner, executor, the
four-method ABI, template renderer.

Three source streams feed this folder:

1. **Audit-distilled (01–06)** — from the 2026-05-15 manual-tester
   pass (~50 actions tested, result-shape inconsistencies catalogued
   in `findings-2026-05-15/silent-success-bugs.md` and
   `cli-and-friction.md`). Pattern: **codify what's informally
   consistent in the wild, make divergent handlers conform**.
2. **User-filed feature requests (07–10)** — gaps surfaced while
   migrating real dotfiles from `shell:` blocks to typed actions on
   2026-05-16. Pattern: **a real shell workaround exists; the typed
   action equivalent is one missing field away**.
3. **Agent-runtime expansion (11–15)** — from a 2026-05-17
   brainstorm on which new actions unlock pilot agents, self-healing
   systems, and LAN-fleet composition. Pattern: **the kernel's
   diff/perms/risk wrap is the moat; new actions push complexity
   into their own `args:` block, not onto `Step`** (see CLAUDE.md
   soft cap §2 — 36/40 universal fields used today).

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
| [11](./proposal-11-action-assert-heal.md) | `assert` action + on-fail `heal:` handler | M | High | Self-healing as a primitive; the maintain-loop kernel verb |
| [12](./proposal-12-action-kv-state.md) | `kv` action — typed persistent state in plan/diff | S | High | Agents need memory between runs; today they smuggle it through files |
| [13](./proposal-13-action-process.md) | `process` action — supervised long-running processes | S | Medium-High | Fills the gap between `shell:` (fire-and-forget) and `service:` (OS-installed) |
| [14](./proposal-14-action-watch.md) | `watch` action — reactive triggers for the maintain loop | M | Medium-High | Event-driven heals + agent reflexes; cron replacement |
| [15](./proposal-15-action-plan-recurse.md) | `plan` action — recursive sub-plan execution | M | Medium-High | Composition primitive; unlocks shared heals, conditional flows, per-branch perms |

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

### Agent-runtime expansion (compose; do 11 first)

These five proposals cross-reference each other heavily. 11 is the
keystone; the rest layer on top.

1. **11 `assert` + `heal:`** — the maintain-loop primitive. Touches
   the executor (new counter, new evaluator). Do first.
2. **12 `kv` action** — adds the state surface 11, 14, 15 all want
   for counters, debounce windows, sub-plan handoff.
3. **15 `plan` recurse** — makes `heal:` / `on_event:` / shared
   sub-flows composable. Cheaper to add once than to retrofit.
4. **13 `process`** — independent of the others; ship when a real
   user need surfaces (pilot agent with local model is the obvious
   one).
5. **14 `watch`** — biggest scope (event-source abstraction); the
   payoff is event-driven maintain mode. Ship last in the batch.

Agent stream's proposal-07 (`mcp_tool` action) is the sibling to
this batch and should sequence with it — `mcp_tool` lives in the
agent stream because the MCP integration story is owned there, but
the action itself is a new kernel handler and follows the same
field-budget discipline.

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

Agent-runtime brainstorm receipts (2026-05-17):
- **Proposal 11**: cron + retry + alert glue users hand-build to
  keep a service alive; `agentd` plan that wants to maintain
  invariants without an external scheduler.
- **Proposal 12**: agent loops that write to files just to remember
  "last build SHA"; loss of audit trail when state lives outside
  facts.
- **Proposal 13**: pilot-agent plans that need `ollama serve` for
  the next 5 minutes — too heavy for `service:`, too dangling for
  `shell:`.
- **Proposal 14**: cron-driven `mooncake apply` loops every minute
  to find one drifted thing; event-driven is the latency fix.
- **Proposal 15**: the pattern "this heal should also be these
  three steps" today copy-pastes the steps; composition is the fix.

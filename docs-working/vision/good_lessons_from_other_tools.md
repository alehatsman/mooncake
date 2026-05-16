# Good lessons from other tools

The primitives that survived 30 years of provisioning, orchestration,
and config-management work — and the explicit verdicts on what
Mooncake will actually adopt from each one.

Source brief: [GitHub issue #25](https://github.com/alehatsman/mooncake/issues/25).

---

## Four primitives that survived

These four show up in every system still relevant a decade after
release. Mooncake already embraces all four; restated here so future
proposals can be checked against them.

### 1. Convergence

> Declare desired state; let the runtime decide how to reach it.

**Where it survived:** Puppet (resources), Ansible (idempotency in
modules), Kubernetes (reconcilers), CFEngine (promises stripped of the
marketing), Nix (content-addressed builds).

**Mooncake's expression:** typed actions with idempotent semantics
(`file.write`, `pkg.install`, `os.service`). The handler interface
explicitly distinguishes plan mode (compute would-change) from apply
mode (mutate).

### 2. Plan-before-apply

> Render consequences before mutation.

**Where it survived:** Terraform (`terraform plan`), `kubectl diff`,
Ansible `--check`, Git (`git status` before commit).

**Mooncake's expression:** `mooncake plan --diff` returns structural
`Diff` records per step — what file content changed, what package
version, what service state. Consumed by the MCP server and any AI
agent driving Mooncake to decide whether to proceed.

### 3. Dependency graph + compensating action

> Explicit edges, declared reversibility. Honest SAGA semantics, not
> ACID claims.

**Where it survived:** systemd (`After` / `Before` / `Requires` /
`OnFailure`), SAGA patterns from distributed-systems papers, Erlang/
OTP supervision trees, database write-ahead logging.

**Mooncake's expression:** the four-method handler ABI —
`Permissions`, `Diff`, `Reverse`, `Cost` — declared per handler.
`transaction:` blocks LIFO-revert previously-completed steps when a
later one fails. Some actions (`pkg.upgrade`, `git.clone`) declare
themselves irreversible and the operator sees that at plan time.

### 4. Correlation ID + append-only log

> Every mutation has an ID, an author, and a reason that outlives the
> mutation itself.

**Where it survived:** GitOps' attribution property, distributed
tracing (W3C trace context), database audit logs, kubectl audit policy.

**Mooncake's expression:** JSONL run-log with ULID run IDs, structured
events (`step.started`, `step.completed`, `step.failed`), per-step
diffs persisted as artifacts. Every mutation traceable back to its
plan, its step, its actor.

---

## Adoption decisions

The brief in issue #25 surveys 20 systems. Each is mapped here to a
concrete verdict: **shipped** (permanent rule already in place),
**adopt next** (concrete near-term work), **adopt later** (real but
not urgent), **gated** (tied to another decision), or **rejected**
(violates a non-goal).

### Shipped — permanent rules, restated

| System | Idea | Mooncake state |
|---|---|---|
| **Go** | Single static binary, no interpreter, no module cache | One Go binary including agentd. Install is `curl \| sh`. There is no Python runtime to break. |
| **SQLite** | Local-first, no infrastructure required | Files in `~/.mooncake/`. No mandatory daemon, database, or hub. The personal-fleet layer is opt-in on top. |
| **Nomad** | Constrained operational scope | Permanent non-goal: no control-plane sprawl, no controller architecture. The "fleet" surface is intentionally small. |
| **Make** | Explicit dependency DAGs, no magic inference | `on_change:` chains and `requires:` edges are explicit. We never infer dependencies. |

These four don't need new work; they exist to be defended when a
future proposal would erode them.

### Adopt next — concrete near-term deliverables

These five are the work items the brainstorm surfaced as worth
pursuing on the current trajectory. None of them require new
architectural land — each builds on primitives already shipped.

#### 1. Git — content-addressed plan hashes

> Immutable history, hashed diffs, append-only operations.

**Today:** run-log is append-only JSONL; runs have ULID IDs. Plans
themselves are not content-addressed.

**Adopt:** compute `sha256` of the compiled plan JSON, surface it as
`plan_hash` on every run record. Pair it with the invariant: **once a
run completes, its artifacts are immutable.** This unlocks (a)
reliable "did this exact plan run on this peer?" checks for spec-58
drift, (b) the audit story that a signed plan_hash would later
strengthen for the agent stream, and (c) the foundation for a future
`mooncake replay <plan-hash>` command.

**Cost:** low. The plan is already serialized in `~/.mooncake/runs/`;
hashing is bytes-in → 32 bytes-out. The invariant is mostly assertion:
add a check that artifact directories are not modified after their
parent run reaches a terminal state.

#### 2. Kubernetes — typed run-state vocabulary

> Real systems live in `partially_applied` / `rollback_failed` /
> `drifted` / `recovering`, not just `success` / `failure`.

**Today:** runs have `queued` / `running` / `completed` / `failed`. A
half-applied transaction with a failed rollback collapses into
`failed`; the operator has to read events to learn what happened.

**Adopt:** widen the run-state enum to include explicit states the
real system already produces but doesn't label:
- `partially_applied` — some steps succeeded, the run was halted.
- `rolled_back` — failure reverted cleanly.
- `rollback_failed` — failure reverted incompletely (loudest state).
- `drifted` — applies, then a later plan-conformance check showed
  divergence (lights up once spec-58 lands).

**Cost:** small. Mostly a labeling pass over `internal/runlog/` +
documenting which transitions are legal. The `rollback_failed` state
is the one most worth surfacing — today it hides inside `failed`.

#### 3. ZFS — reversibility as a first-class plan-time column

> Rollback is explicit, dangerous, limited. Never fake ACID.

**Today:** `Reverse()` per handler returns either an inverse Step or
an explicit refusal. The plan output mentions reversibility somewhere
but doesn't lead with it.

**Adopt:** in `mooncake plan` text-mode output, render a `REV` column
per step (✓ reversible / ✗ irreversible / ⚠ best-effort) — the same
information that's in the JSON `Reversible` field, but loud and
visible at glance time. Mirror it in the recap line: "plan: 14 steps,
2 irreversible (steps 7, 11)". For agents, the JSON already carries
the bit; this is purely a text-mode plan UX improvement on already-
typed data.

**Cost:** small. Read `Reverse()` per step at plan time (we already
do); add the column. Risk: convincing operators *they actually
should* skim it. Naming and color matter.

#### 4. Borg + Rust — admission control with loud danger signaling

> Should this mutation even be allowed? Dangerous operations should
> be visible.

**Today:** `Permissions()` declares Sudo + RequiredBinaries; the
executor preflights both. `Cost.Risk` (0–10) is computed per step.
Neither is surfaced loudly in text-mode plan output.

**Adopt:** combine `Permissions` + `Cost.Risk` into a `DANGER` column
in `mooncake plan` text mode: rendered for any step where
`Permissions.Sudo == true` or `Cost.Risk >= 7`. Plan recap aggregates:
"max-risk=9 (band: catastrophic), 3 steps need sudo, 1 needs network
egress". This is the human-facing half of the admission-control gate;
the machine-facing half (a policy DSL on top of these primitives)
lives in the agent stream backlog.

**Cost:** small. Same shape as the ZFS column — read existing typed
data, surface it. Most of the work is UX (where the column sits,
what colors mean, when to default to plan-only vs prompt).

#### 5. Linux kernel — ABI stability contract before plugins

> Stable interfaces matter enormously once an ecosystem exists.

**Today:** four-method handler ABI (Permissions / Diff / Reverse /
Cost) is declared across the priority handler set. No formal stability
guarantee.

**Adopt:** before spec-31 (tier-2 plugins) ships, write the ABI
contract document — what methods are required vs optional for tier-1
vs tier-2, what additions are allowed without major-version bump, what
the deprecation policy is. This lives in the agent stream because it
gates plugin work, but the contract itself is a Core artifact.

**Cost:** documentation, not code. The right time to write it is
*before* a plugin author depends on the current shape.

### Adopt later — concrete, not urgent

| System | Idea | Mooncake adaptation |
|---|---|---|
| **systemd** | `systemctl status` / `journalctl -u` feel immediate and inspectable | `mooncake explain <step|run|resource>` rendering why a step ran/skipped/changed/failed, sourced from existing event log + diff + redaction layers. Replaces ad-hoc grep through JSONL. |
| **tmux** | Process survives disconnect; reattachable | Local `mooncake apply` is foreground-only today; if SSH drops, the run dies. Fleet runs already have SSE reattach via `fleet logs <run-id>`. Bring the same shape to local: `mooncake apply --detach` + `mooncake attach <run-id>`. |
| **CI systems** | Avoid rerunning successful work | Resumable runs at transaction boundaries: `mooncake retry <run-id>` resumes from the last completed transaction. Pairs with the typed-state vocabulary above. |
| **Erlang/OTP** | Supervised subsystems, isolated failure domains | Formalize agentd's subsystem supervision once drift loop (spec-58) lands. Today subsystems coexist; with the drift loop running on a timer, restartability under partial failure matters more. |

These four are real follow-ups but none of them block what's
currently shipping. Worth specs when the timing is right.

### Gated — tied to another decision

| System | Idea | Gate |
|---|---|---|
| **Package managers** (npm, Cargo) | Lockfile: `mooncake.lock` pinning module versions + checksums | Gated on **issue #24** — Git-based versioned module system. The lockfile concept is meaningless until module distribution exists. |
| **OCI / container registries** | Signed, immutable published artifacts | Same gate as above. When modules ship from a registry, immutability and signing must come with them; don't ship a mutable module system and try to retrofit. |

### Rejected — would violate non-goals

| System | Idea | Why we won't adopt |
|---|---|---|
| **Docker / OCI** | Layer reuse / cached execution proofs ("CUDA already installed → reuse prior proof") | Would require a fleet-state cache of "what's already validated on which peer." That cache is exactly the shape of a control-plane drift, which `non_goals.md` §3 forbids. The cheap version — per-step `creates:` and `unless:` — already exists and stays scoped per-step. |
| **Bazel** | Full hermetic execution (sandbox env per step) | Container-level sandboxing is out of L1 scope. Mooncake is *above* the OS; the OS provides the sandbox if one is needed. Adopting hermeticity would either require containerizing every step (heavyweight) or building our own sandbox runtime (NIH). Neither earns its weight. |

### Already covered by other primitives

| System | Idea | Where it lives |
|---|---|---|
| **Redis** | Obvious mental model, minimal CLI surface | Covered by Make + Nomad + systemd lessons above. The constraint is permanent: command names stay boring (`apply` / `plan` / `doctor` / `history` / `fleet`). |
| **Terraform** | Graph visualization, dependency display, blast radius | Covered by Plan-before-apply (primitive #2). The graph-visualization religion (ChangeGraph as Mooncake's core IR) is *not* adopted — see [GitHub issue #8 analysis](https://github.com/alehatsman/mooncake/issues/8): ~60% of the ChangeGraph thesis is already present in spec-22 + spec-30 + spec-58; the rest is over-architecture. |

---

## Modern lessons already in production

Beyond the four core primitives and the new adoption decisions, six
more recent ideas have earned their place. These are shipped today
and exist to be defended:

- **Single static binary** — one Go binary including agentd, mirroring
  the Hashistack distribution model.
- **Typed schema as part of the product** — `internal/schemagen/`
  produces `schema.json` consumed by the IDE, the validator, and the
  MCP server uniformly.
- **Local-first, peer-to-peer** — mDNS discovery + bearer-token-authed
  HTTP+SSE. Personal fleet of up to ~50 nodes without SaaS.
- **Structured errors with suggested fixes** — Rust-compiler-diagnostic
  shape. Error code, message, failing step, *and* a suggested fix.
- **Reconciliation loops at the node** — agentd reconciles its own
  state. The controller never holds fleet-wide state authoritatively.
- **MCP-style typed tool surfaces for LLMs** — agents consume JSON
  Schema'd tool definitions, not parsed CLI output.

---

## Borrow vocabulary, not implementation

A specific corollary: when other tools have already taught operators
a word, **reuse the word**.

- `Diff`, `Reverse`, `Permissions`, `Cost` — chosen because they're
  English. Not "compensator," not "antaction," not "promise."
- Dependency edges follow systemd's vocabulary: `after`, `before`,
  `requires`, `wants`, `conflicts`, `on_failure`. Operators already
  parse these.
- `transaction:` for the LIFO-rollback compound — the word ATM
  customers have known for 40 years.

If there's already a boring name, don't be clever.

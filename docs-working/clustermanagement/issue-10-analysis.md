# Issue #10 — Historical systems research, mapped to Mooncake

**Source:** [#10 Research historical provisioning and cluster-management systems for Mooncake architecture direction](https://github.com/alehatsman/mooncake/issues/10)
**Analyzed against:** master @ `c6f6838` (2026-05-15)
**Companion:** [`issue-11-analysis.md`](./issue-11-analysis.md) (cluster-mgmt capabilities), [`agentic-interface-brainstorm.md`](./agentic-interface-brainstorm.md) (agent-centric primitives)

The issue asks: which ideas from 30 years of provisioning / orchestration /
config-management / distributed-systems work are worth borrowing, adapting,
or avoiding for Mooncake? It lists 14 systems and asks for a comparative
table, a primitives map, and an architecture-direction conclusion.

This document answers all three deliverables — but it does not treat
each system as an isolated lecture. The interesting finding only shows up
when you lay all fourteen side by side: **the systems that survived
share four primitives, the systems that died share three pathologies,
and Mooncake has already chosen sides on every single one.** The work
ahead is mostly to keep choosing well — not to invent something new.

The strategic conclusion at the end (§4) is the punchline; the per-system
analysis (§2) is the receipts.

---

## 1. Headline

Across the 14 systems researched, **four primitives keep working** and
**three pathologies keep killing systems**:

**Primitives that survived:**

1. **Convergence** — declare desired state; let the runtime decide how to
   reach it. (Puppet, Ansible idempotency, K8s reconcilers, CFEngine, Nix.)
2. **Plan-before-apply** — render consequences before mutation. (Terraform,
   `kubectl diff`, Ansible `--check`.)
3. **Dependency graph + compensating action** — explicit edges, declared
   reversibility. (systemd's `After/Before/Requires/OnFailure`, SAGA
   patterns, Erlang/OTP supervision trees.)
4. **Correlation/attribution ID + append-only log** — every mutation has
   an ID, an author, and a reason that outlives the mutation itself.
   (GitOps, observability traces, audit logs.)

**Pathologies that killed systems:**

A. **DSL religion** — the config language grows into a programming
   language, then a runtime, then a culture. (Puppet manifest compilation,
   CFEngine "promises", Jenkinsfile, Chef Ruby DSL, HCL functions, K8s
   CRD ecosystems.)
B. **Provider/plugin marketplace** — the core stays small; the periphery
   becomes 90% of the user's surface area, with no API discipline.
   (Terraform's 3000 providers, K8s operators, Jenkins plugins.)
C. **Control-plane sprawl** — the system that promised to remove
   complexity grows a control plane more complex than the workloads it
   manages. (K8s itself, Puppet masters + PuppetDB + ENC, Chef Server,
   ArgoCD/Flux + their CRDs.)

Mooncake has already taken sides on each of these:

- ✅ Embraces all four surviving primitives (with varying maturity).
- ✅ Explicit non-goals on all three pathologies (LLM_GUIDE.md "Reshape
  freely", issue #11 anti-goals, CLAUDE.md "no over-engineering").

**The remaining question is not "which historical idea should we
borrow?" — it is "which of the surviving primitives still has the most
leverage left to spend?"** §4 argues that's the *dependency graph +
compensator* combination, which is exactly the ChangeGraph thesis in
issue #8.

---

## 2. Per-system map

### At a glance

| System | Surviving idea | Pathology to avoid | Mooncake's current move |
|---|---|---|---|
| Puppet | Declarative resources, idempotency | Manifest compilation, Hiera DSL | ✅ Typed handlers; ❌ no compiler layer |
| Terraform | `plan` as artifact, blast-radius visibility | Provider marketplace, state-file oracle | ✅ DryRun → Plan; ❌ no plugin marketplace |
| Kubernetes | Reconciliation loop | CRD/operator sprawl, control-plane bureaucracy | 📝 spec-58 (drift loop); ❌ no controller architecture |
| Nix / Guix | Content-addressed reproducibility | Purity learning cliff; mutable state boundary | 🟡 plan/run IDs; ❌ no pure-functional system model |
| Ansible | YAML+SSH readability floor | Jinja-as-programming-language | ✅ YAML schema is closed; resist Jinja explosion |
| SaltStack | Event bus as substrate | Master-of-masters complexity | ✅ agentd SSE hub; ❌ no "reactor" layer |
| CFEngine | Autonomous repair + invariants | Cryptic DSL, social rejection | 📝 invariants in spec-58 — call them assertions |
| systemd | `Before/After/Requires/OnFailure` vocabulary | Scope creep (systemd-the-platform) | 🟡 ChangeGraph (#8) — borrow vocabulary verbatim |
| GitOps | Author + reason + commit-shaped history | Git-as-a-database, ArgoCD-as-a-platform | ✅ run-log + artifacts; ❌ no git-coupled control plane |
| HPC cluster mgrs | Accept heterogeneity as the default | Vendor lock-in, image monoculture | ✅ tags/overlays (spec-48/50); stay above the OS |
| Databases (ACID) | Mental model of transactions | ACID-is-a-lie for system mutation | ✅ spec-22 Reverse() = SAGA, not ACID |
| Distributed systems (SAGAs) | Compensating action per step | "Eventually consistent" hand-waving | 🟡 Reverse() + Permissions(); needs UX surfacing |
| CI/CD (canary, waves) | Blast-radius-as-a-knob | Pipeline DSL spaghetti | 🟡 fleet apply phases; 📝 waves as flags, not DSL |
| Observability | Stable IDs threading every artifact | Telemetry sprawl, metric-cardinality blowups | ✅ run_id everywhere; commit to plan_id/step_id/change_id |

Legend: ✅ shipped · 🟡 partial · 📝 drafted · ❌ explicit non-goal.

The rest of §2 walks each entry in detail.

---

### 2.1 Puppet — convergence

**Surviving idea.** "X must exist" as the unit of work, not `apt install X`.
Every resource implements an idempotent converge: detect current state,
compute diff, apply. This is the right shape and Mooncake handlers
already work this way (`Check()` / `Execute()` / `Result.Changed`).

**Why the rest died.** Puppet didn't stay at "declare resources." It
grew a manifest *compiler* that turned `.pp` files into a catalog, a
hiera layer for parameter lookup, exported resources for cross-host
state, environments for promotion, custom types for new resource kinds —
each layer added because the previous one was missing one thing. After
a decade, the puppet runtime is the smaller half of operating a Puppet
shop; the larger half is operating Puppet itself.

**Mooncake position.** Has the good half. The non-goal is the
*compilation pipeline*: no Hiera, no exported resources, no environments,
no manifest-time language features. Variables + templates do the
parameterization work without a separate type system.

**Concrete rule.** Adding a feature that "needs to know about other
hosts' resources at compile time" is the warning sign — that's where
the cathedral started. If a feature requires cross-host reasoning,
push it to a fleet-time operator (e.g. `fleet apply` with waves), not
to a planner-time catalog.

---

### 2.2 Terraform — plan/apply

**Surviving idea.** The `plan` output is an *artifact users learn to
read*. The diff trains the operator's intuition for what's about to
happen. Once you've spent a year reading Terraform plans, you can read
any structured diff. This is the highest-leverage UX innovation in
the entire infrastructure-tools timeline.

**Why the rest broke.** Two failures:

1. **The provider ecosystem.** 3000+ providers, each its own bespoke
   schema, each with its own version skew, breaking changes, and
   "we deprecated `aws_eip` last year, use `aws_eip_v2`." Terraform
   the language is fine; Terraform the *vendored-provider experience*
   is the actual product, and it's a mess.
2. **State-file as oracle.** The .tfstate file is supposed to be the
   record of truth, but it drifts from reality constantly. Tools
   accreted around this (`terraform import`, `terraform taint`,
   `terraform refresh`, Terragrunt) — each a patch for the same
   fundamental mismatch.

**Mooncake position.** Plans are first-class (`internal/plan/`,
`SupportsDryRun: true` per handler), `--check` exists, and the
plan-shape is what `mooncake plan` emits. No state file — facts are
recomputed each run, which is slower but doesn't drift. No provider
marketplace — the closed action set is the closed action set.

**Concrete rule.** Resist "what if users could write their own actions
in a plugin SDK?" That's the slope. The closed action set is a feature.
If a need is real and recurring, it becomes a built-in action; if it's
one-off, it stays in `shell` or `command`. There is no middle layer.

The next move on the Terraform-good-side: make plans into *signed,
hashable, replayable artifacts* (Nix-like reproducibility, ¶2.4). A
plan hash is the foundation for `mooncake explain` (issue #8 §4),
spec-58 drift (issue #11 §10), and any AI-agent attestation story.

---

### 2.3 Kubernetes — reconciliation

**Surviving idea.** The control loop: `observe → diff → reconcile`,
running forever, repairing drift automatically. The *loop itself*
becomes the API surface — you describe desired state, and the loop
figures out the rest. This is genuinely good.

**Why the rest is a tax.** Every part of K8s that isn't "the control
loop" is overhead. The CRD ecosystem (cert-manager, external-dns,
ArgoCD, Linkerd, Flux, ...) each add their own controllers, finalizers,
webhooks, admission policies, and CRDs. Understanding "why didn't my
Pod start?" can mean reading through five operators' code. The control
plane that promised to remove complexity *is* the complexity now.

**Mooncake position.** spec-58 (drift detection) is the reconciliation
loop applied at one resource at a time, locally, on agentd — not a
fleet-wide controller swarm. The clean rule: **loops are a primitive,
not an architecture.** You can run an `InspectPlan` loop without
building a CRD ecosystem around it.

**Concrete rule.** Never build the K8s shape: no "controller" abstraction,
no admission webhooks, no finalizers, no leader election, no CRD-as-a-
data-model. Drift detection runs *in the agentd process* against the
local plan; if the local plan disagrees with reality, agentd repairs
according to the per-plan policy (`drift: notify|reapply|revert|none`
in spec-58). No central reconciler.

The mental test: if a new feature would benefit from a control loop,
ask *where* the loop runs. If the answer is "on agentd, against local
state, with explicit policy" — good. If the answer is "on a controller
that watches the fleet" — stop, that's the K8s slope.

---

### 2.4 Nix / Guix — immutability

**Surviving idea.** Content-addressed artifacts: every build input is
hashed, every output is keyed by the hash of its inputs, so reproducing
a system means replaying the hashes. This is the *only* real solution
to "works on my machine" that the industry has produced.

**Why it stayed niche.** Two reasons:

1. **The purity tax.** To get reproducibility, you give up "just run
   this command." Everything must declare its inputs. Mutable runtime
   state (databases, log files, runtime caches) breaks the model and
   has to be handled as an escape hatch.
2. **The learning cliff.** Nix-the-language is dense; the error
   messages are infamous; the cost to ramp a generalist ops person is
   on the order of weeks, not hours.

**Mooncake position.** A purely-functional system model won't fit
Mooncake's "mutate existing machines" reality. But the *hash + replay*
property can be borrowed at the *plan* level without taking on the
Nix runtime. A plan + the facts it was compiled against + the resolved
actions = a hash. That hash is the foundation for:

- Audit ("this exact plan was applied at 2026-05-15 13:44").
- Replay ("re-apply this plan on a fresh box").
- Drift detection ("the current state's hash diverges from the last
  applied plan's hash").
- AI-agent attestation ("the LLM proposed plan-hash-X; the human
  approved plan-hash-X; the system applied plan-hash-X").

**Concrete rule.** Aim for *hashable plans + recorded effects*, not
*pure functional system semantics*. The 80/20 split is enormous here.

Specifically: every successfully applied plan should be persisted to
`~/.mooncake/runs/<run-id>/{plan.yaml, facts-before.json, facts-after.json,
events.jsonl, plan.hash}`. issue #11 §9 (artifact collection) is the
work item; it's actually a Nix-shaped feature in disguise.

---

### 2.5 Ansible — operational UX

**Surviving idea.** YAML + SSH + tasks was the *readable floor*. The
honest test is: "can I read this playbook during an outage?" Ansible
passed; almost nothing else in this list did. Adoption tracked
readability, not features.

**Why it ages badly.** YAML grew into a programming language. Jinja2
expressions, loops, conditionals, blocks, includes, roles, collections,
strategies, vars precedence (22 levels at last count), tags — each
shipped because someone needed it. The result is YAML files that
require reading the Ansible docs to understand. The readable-floor
property quietly evaporated.

**Mooncake position.** The schema is closed (`internal/config/schema.json`),
which is already 70% of the answer. The remaining 30% is *resisting
Jinja-as-programming-language*. A few rules:

- Prefer typed action fields over `{{ ... }}` substitution. If a
  field needs a value computed from facts, it goes in a typed field
  (e.g. `when:`), not embedded in a template string.
- Resist "include" / "include_tasks" / "import_playbook" sprawl.
  Mooncake's `preset` and `include` already do this; the temptation
  is to add `include_when`, `include_tags`, etc.
- Loops stay simple (`loop:` over a list). No `loop_control`, no
  `with_subelements`, no `with_indexed_items`.

**Concrete rule.** When considering a new YAML feature, ask: "would
this be readable at 3am by someone debugging a failed rollout, who
has not read the docs?" If no, don't ship it.

This is also why issue #8's ChangeGraph proposal explicitly keeps the
rule: *Everything must compile into executable steps. No pure metadata
religion.* That's the Ansible-readability discipline restated.

---

### 2.6 SaltStack — event bus

**Surviving idea.** Events as substrate. Infrastructure emits events
(`config_changed`, `service_failed`, `package_upgraded`); subscribers
can react. This is the right shape — pub/sub at the infra layer
genuinely composes well.

**Why it didn't generalize.** Two things:

1. **Master-of-masters topology.** Salt's recommended scaling story
   added syndic masters, then syndic-of-syndic-of-masters, then
   external job caches. Operationally a fractal of failure modes.
2. **Reactor logic.** Reactors (the consumer side of events) are
   declared in YAML+Jinja and run on the master — debugging "why
   didn't this reactor fire?" is among the worst experiences in
   modern infra tooling.

**Mooncake position.** `internal/agentd/sse_hub.go` is the salt-event-
bus done right: HTTP/SSE transport (not custom ZeroMQ), append-only,
consumers subscribe and act *locally*. No reactor layer on the
controller. issue #11 §8 (`fleet logs --follow`, spec-53 watch) is
the consumer side.

**Concrete rule.** Don't add a "reactor" abstraction. If a consumer
wants to react to events, it's a normal program that subscribes to
the SSE stream and takes action. Mooncake's job is to *emit* events
honestly, not to define what consumers do with them.

The agentic angle: the event stream is what makes Mooncake legible
to LLM agents. An agent that subscribes to `/v1/events` sees the same
fleet truth a human does. issue #8 §7 (`observe.*` actions) is the
mirror of this — actions that *emit* events on demand.

---

### 2.7 CFEngine — autonomous repair

**Surviving idea.** Invariants + autonomous repair. The machine
asserts "service nginx must be running" and the agent fixes it on a
fixed cadence (the original CFEngine cadence was 5 minutes). This is
the right model for self-healing.

**Why it died socially.** Two cultural failures:

1. **The "promise" DSL.** CFEngine called invariants "promises" and
   built a domain-specific language around them. The terminology
   repelled operators ("why is my machine *promising* things?") and
   the DSL was syntactically alien.
2. **Centralized policy evolution.** Updating a CFEngine policy required
   coordination with whoever owned the policy hub — the operational
   loop was slow.

**Mooncake position.** spec-58 (drift) is the autonomous-repair model.
The right move on terminology: **call invariants what they are.** Not
"promises," not "contracts." Call them `assert:`, `healthcheck:`,
`invariant:`, or `constraint:` — words operators already know. The
drift loop's policy options (`notify | reapply | revert | none`) are
the right user-facing surface.

**Concrete rule.** When in doubt about a name, pick the boring one.
"Drift" beats "convergence delta." "Reapply" beats "remediation
action." "Invariant" beats "promise."

---

### 2.8 systemd — dependency graphs

**Surviving idea.** The dependency vocabulary: `After=`, `Before=`,
`Requires=`, `Wants=`, `Conflicts=`, `Condition*=`, `OnFailure=`. This
covers ~95% of the dependency relationships that show up in real
service orchestration, and operators *already know it*.

**Why systemd is also a cautionary tale.** Scope creep. systemd-the-init
became systemd-the-platform: systemd-resolved (DNS), systemd-networkd
(networking), systemd-homed (user mgmt), systemd-boot (bootloader),
systemd-timesyncd (NTP), ... The dependency model is fine; the empire
is the problem.

**Mooncake position.** issue #8's ChangeGraph is *the same idea* —
typed nodes (mutations) and typed edges (dependencies). The cleanest
move is to **borrow systemd's edge vocabulary verbatim**:

- `after: [step-id]` / `before: [step-id]` — ordering
- `requires: [step-id]` — hard dependency, fail if missing
- `wants: [step-id]` — soft dependency, continue if missing
- `conflicts: [step-id]` — mutual exclusion (one fleet at a time)
- `on_failure: [step-id]` — compensating action / rollback trigger

Mooncake already has `dependencies:` between steps in some places;
ChangeGraph would generalize that into a typed edge set. Use names
operators recognize.

**Concrete rule.** Don't invent edge-type vocabulary. If systemd has
a name for the edge, use systemd's name. (This is Hofstadter's law
applied to specs: every project that invents new dependency vocabulary
underestimates how confusing it'll be in three years.)

---

### 2.9 GitOps — auditability

**Surviving idea.** Every mutation has an author, a commit message, a
timestamp, and a reviewer. The git log *is* the audit log. This is
the only audit model that's ever stuck in the infra world.

**Why it's overrated as an architecture.** Two failures:

1. **Git as a database.** Git is slow at high-frequency state, bad at
   structured queries, and the push-vs-pull distinction (ArgoCD vs
   Flux) splits the community for reasons that have nothing to do
   with the audit property.
2. **The tooling ate the property.** ArgoCD and Flux each grew into
   K8s-shaped control planes. The audit property got buried under
   the operator-of-operators tax.

**Mooncake position.** Borrow the *attribution property* (author +
reason + timestamp on every mutation); reject the *git-is-the-DB
coupling*. Run history goes in `~/.mooncake/runs/`, structured as
JSONL — queryable, append-only, no git overhead. issue #8 §4
(`mooncake explain`) is the operator-facing surface; spec-58 (drift)
is the agent-facing one.

**Concrete rule.** Every applied plan records: `author` (CLI user or
agent ID), `reason` (free-form string, optional), `approved_by` (for
human-in-the-loop AI flows), `plan_hash`, `timestamp`. Make `mooncake
runs ls` / `mooncake explain` the surface. Don't push users into a
git repo to get attribution.

The agentic-trust angle: when an LLM proposes a plan, the `author`
field becomes the agent identity and the `approved_by` field is the
human gate. issue #8 §9 (agent negotiation) lives here.

---

### 2.10 HPC / cluster managers — heterogeneity

**Systems researched:** Bright Cluster Manager, xCAT, Warewulf, Rocks,
OpenHPC.

**Surviving idea.** Real fleets are *heterogeneous*. Different OS
versions, different GPU SKUs, different network adapters, different
firmware revisions. The systems that survived (xCAT, Warewulf) accept
heterogeneity as default; the ones that pushed image-monoculture
(Rocks, Bright) lost ground when GPU diversity exploded.

**What didn't transfer.** Image-based provisioning + PXE works for
homogeneous HPC clusters but is overkill for personal AI workstation
fleets. Most users want to keep their existing OS install and have
Mooncake manage *above* the OS.

**Mooncake position.** issue #11 already takes the right stance: tags
+ selectors (spec-48/50) + per-host overlays accept heterogeneity as
the default. The `epic-cluster-management.md` boundary ("Mooncake
manages everything above the OS") is the right call — image
provisioning is a separate concern. Power control (issue #11 §20)
can stay narrow: WoL + SSH shutdown for personal-fleet, IPMI/Redfish
as out-of-tree integrations.

**Concrete rule.** Don't aim for "Bright Cluster for everyone." Aim
for "the boring layer above whatever-distro-you-already-have." The
right marker: a Mooncake fleet should be able to mix Ubuntu 22.04,
Debian 13, Fedora 41, and macOS workstations and still apply the
same plan with per-host overlay differences.

---

### 2.11 Databases — transactions (and the ACID lie)

**Surviving idea.** ACID gave operators a mental model: a transaction
either fully happens or fully doesn't. This is genuinely useful as a
*mental* model.

**Why it's a lie for system mutation.** You cannot atomically install
CUDA, restart Docker, and open a firewall port. Each step is its own
mutation, observable to the rest of the system, and a failure mid-
sequence leaves you in a partially-mutated state that ACID promised
wouldn't exist.

The mistake: every infra tool that tried to fake ACID semantics ended
up *lying to the operator*. Terraform's "partial apply on failure"
leaves state-file divergence; Ansible's "stop on first failure" leaves
half-mutated boxes; Puppet's "this run failed" runs again next cycle
and re-attempts the half-mutated work.

**Mooncake position.** Be honest. spec-22 (transactions with `Reverse()`)
calls this what it is: a *SAGA*, not an ACID transaction. The semantics
are:

- Each step is its own local transaction (idempotent, observable).
- Steps may declare a compensating action (`Reverse()`).
- On failure, Mooncake walks the executed steps backwards, calling
  each compensator.
- Some steps **have no compensator** (e.g. "sent an email", "deleted
  a file with no backup"). These are surfaced *in the plan UI*, before
  execution, with explicit operator acknowledgement.

**Concrete rule.** Never claim atomicity. The right UI surface for
plans is:

```
3 steps, 2 reversible, 1 irreversible (mark sensitive)

  ✓ install nginx           [reversible: uninstall]
  ⚠ delete /var/log/old     [IRREVERSIBLE — confirm]
  ✓ restart nginx           [reversible: previous-state]
```

The honest model is the trustworthy model.

---

### 2.12 Distributed systems — compensating actions (SAGAs)

**Surviving idea.** SAGAs: a sequence of local transactions, each with
a declared compensator, plus an orchestrator that runs forward on
success and backward on failure. Born in microservice papers, now
standard. The right model for "transactional infrastructure mutation"
when ACID isn't available.

**Failure mode.** "Eventually consistent" hand-waving — claiming SAGA
semantics without actually declaring the compensators. The compensator
must be a real, testable action; otherwise it's just a wish.

**Mooncake position.** spec-22's `Reverse()` is the SAGA compensator
made typed. The work-in-progress is:

1. **Coverage** — every mutating action should either declare a
   compensator or *declare itself irreversible*. Both states are
   honest; "we'll figure out rollback later" is not.
2. **UX surfacing** — the plan UI should show reversibility per step
   (per ¶2.11).
3. **Auto-reverse on failure** — when a step in a transaction fails,
   walk backward through completed steps, calling each compensator.
   Stop on first compensator failure and report the partial-rollback
   state. *Never* pretend the rollback succeeded when it didn't.

**Concrete rule.** Compensators are tested code paths, not afterthoughts.
A mutating action lands when its compensator lands; not before.

---

### 2.13 CI/CD — staged rollout (canary, waves, gates)

**Surviving idea.** Blast-radius-as-a-knob. You don't mutate the whole
fleet at once; you mutate one node, run a health gate, expand to a
wave, gate again, expand to the rest. This is the right vocabulary
for fleet-wide mutation.

**Failure mode.** Pipeline DSLs. Jenkinsfile, Github Actions YAML,
CircleCI config — all started as "describe your rollout" and grew into
ad-hoc programming languages with their own variable scoping, secret
handling, conditional logic, and matrix expansion. Every CI/CD
platform has the same regret: the pipeline-DSL ate the company.

**Mooncake position.** issue #11 §6/§7 (waves, canary) are real
upcoming work. The right shape:

- Rollout metadata is *flags on `fleet apply`*, not a new DSL.
- `--canary peer-name` is a wave of size 1 with an explicit target.
- `--wave-size N` chunks the peer list.
- `--health-gate doctor` or `--health-gate <assertion-name>` declares
  the gate.
- If the gate fails, the rollout stops and reports the partial state.
  No automatic continuation, no exponential backoff retry, no
  "self-healing" — just *stop and tell the human*.

**Concrete rule.** Rollout is a property of an apply, not a separate
artifact. The day someone proposes `mooncake pipeline.yml` is the
day to refuse.

---

### 2.14 Observability — correlation IDs

**Surviving idea.** Stable IDs threading every artifact a mutation
produces. Distributed tracing's trace_id is the most-imitated idea in
observability because it works.

**Failure mode.** Telemetry sprawl — emitting metrics for everything,
high-cardinality tags everywhere, and storing it all forever in a
$200k/year SaaS bill. The ID is the primitive; the rest is overhead.

**Mooncake position.** Already has `run_id` for every applied plan.
The right move is to commit to a stable hierarchy:

- `plan_id` — the hash-of-the-compiled-plan (Nix-shaped, ¶2.4).
- `run_id` — one execution of that plan, generated at apply time.
- `step_id` — stable per step within a plan.
- `change_id` — if/when issue #8's ChangeGraph lands, one per typed
  mutation.

Every event, every artifact, every log line tags with the relevant
IDs. `mooncake runs show <run-id>` becomes the universal entrypoint.

**Concrete rule.** IDs are the *spine* of the artifact system. Surface
them in CLI output, persist them in artifacts, accept them as query
arguments. The cost of inconsistent IDs is permanent; the cost of
boring consistency is trivial.

---

## 3. What the issue missed

The issue lists 14 systems but four threads are worth adding:

### 3.1 Erlang/OTP — supervision trees + "let it crash"

The OTP supervisor hierarchy is the cleanest model of autonomous
repair ever shipped. A supervisor watches its children, and on child
failure either restarts the child (with a strategy: one_for_one,
one_for_all, rest_for_one), escalates to its parent, or terminates.
The model is *typed*: restart strategies are declared at the
supervisor level, not hand-rolled per failure.

**Lesson for Mooncake.** The drift-loop policy from spec-58
(`notify | reapply | revert | none`) is a one-for-one supervisor
strategy declared per-plan. The framing extends cleanly to *trees*
of plans: a top-level fleet plan can declare its restart strategy
across child plans. Worth keeping in mind when designing the
fleet-wide drift response.

The OTP slogan — "let it crash, the supervisor will handle it" — is
the right cultural posture for infra mutation: don't bury errors,
escalate them up a typed hierarchy.

### 3.2 NixOS modules (vs Nix-the-build-system)

NixOS *modules* are different from Nix-the-language: they're declarative
system descriptions with strong typing, defaults, and composition. The
module system is closer to Mooncake's preset model than Nix's pure
build-graph is. Specifically, NixOS modules merged from many sources
deterministically — option `services.nginx.enable` can be set in five
modules and the merge rules are explicit.

**Lesson for Mooncake.** When presets compose (issue #11 §3 facts
cache, §4 selectors), the merge semantics need to be explicit. NixOS
solved this with `lib.mkMerge`, `lib.mkForce`, `lib.mkDefault`,
`lib.mkIf`. The names are awkward but the *semantics* are right —
"this value takes precedence" / "this value is a default" / "this
value only applies under condition" should be first-class in the
preset merge.

### 3.3 Tailscale ACLs — declarative policy that worked

Counter-example to "policy DSL religion." Tailscale's ACLs are
declarative JSON: short, readable, narrow-scoped (network access
policy only). They work because the *scope* is constrained.

**Lesson for Mooncake.** issue #11 §18 (policy gates) is real future
work. The Tailscale rule: pick the *narrow* version. A policy DSL
that controls "what actions can run when" is bounded and useful; a
policy DSL that controls "anything you might ever want to express
about anything" is OPA / Cedar / Rego — the same trap CFEngine fell
into. Specifically:

- ✅ `deny: sudo outside maintenance` — narrow, declarative, bounded.
- ❌ "expressive policy language with predicates over arbitrary facts"
  — that's where the religion starts.

### 3.4 Borg / Omega papers — what K8s lost

Google's pre-K8s schedulers (Borg, Omega) had clearer separation of
concerns: a scheduler, a master, and per-task agents. The K8s
generalization added etcd-as-the-API-server, watch-based controllers,
and the CRD ecosystem — which papered over the loss of clarity.

**Lesson for Mooncake.** Read the original papers, not the K8s
implementation. The right separation for a Mooncake fleet:

- *Plans* are the unit of declared work (Borg's job specs).
- *agentd* is the per-node executor (Borg's Borglet).
- The *controller* is a thin coordinator that distributes plans and
  collects results (Borg's BorgMaster — but narrower).
- There is **no scheduler** as a first-class component. Placement
  (issue #11 §14) is a separate, optional concern that can be a
  one-shot recommendation tool, not a long-running scheduler.

---

## 4. Vision conclusion

### What Mooncake actually is

The 14-system audit makes Mooncake's identity sharper than any single
issue does: **Mooncake is the typed mutation runtime + audit substrate
for autonomous agents managing personal AI fleets.**

Decomposed:

- **Typed mutation runtime** — convergence (Puppet), plan/apply
  (Terraform), idempotent handlers (Ansible), declared compensators
  (SAGAs). Already shipping; spec-22's `Reverse()` is the last big
  piece.
- **Audit substrate** — append-only run history (GitOps' attribution
  property, not its git-coupling), correlation IDs (observability),
  hashable plans (Nix's reproducibility property, not its purity).
  Partially shipping (runs/, JSONL events); needs commitment to
  plan_id/step_id/change_id as a spine.
- **For autonomous agents** — the unique bet. None of the 14 systems
  were designed for AI agents. This is the wedge that earns the
  right to exist.
- **Personal AI fleets** — the scope. Not hyperscale ("another K8s"),
  not solo-machine ("another dotfiles tool"). The 1–50 node range
  where GPU workstations and AI inference live. Tags + selectors +
  overlays + agentd is exactly this scope.

### What's missing

Three primitives, in order of leverage:

1. **ChangeGraph (issue #8)** — the dependency-graph + compensator
   combination. Borrow systemd's edge vocabulary (`after`, `before`,
   `requires`, `wants`, `conflicts`, `on_failure`); store the graph
   as the plan's first-class representation. This is the highest-
   leverage missing piece because everything else (rollback ordering,
   risk scoring, explanation, fleet-wide waves) reads from this
   graph.
2. **Typed observability (`observe.*` family)** — the mirror of
   typed mutation. `observe.cpu`, `observe.gpu`, `observe.service`,
   `observe.port` instead of parsing free-form `shell` output.
   Already argued in `agentic-interface-brainstorm.md` §1; reinforced
   here because it's the substrate for §1 (drift), §11 (placement),
   §14 (explain) from issue #11.
3. **Plan hash + artifact lineage** — every plan gets a stable hash;
   every run records the plan-hash it applied; every drift detection
   compares against the last-applied plan-hash. This is the Nix-
   shaped property without the Nix runtime. Cheap to add, opens up
   replay/attestation/audit.

### What to never build

The seven explicit non-goals, each grounded in a specific historical
failure:

1. **No DSL evolution** (Puppet/CFEngine/Jinja-explosion) — the YAML
   schema is closed. New features are typed action fields, not new
   template syntax.
2. **No provider/plugin marketplace** (Terraform) — the closed action
   set is a feature. Out-of-tree integrations are *separate tools
   that produce Mooncake YAML*, not plugins.
3. **No control-plane sprawl** (K8s) — no controllers, no CRDs, no
   admission webhooks. Loops run on agentd, locally, against local
   state, with explicit policy.
4. **No git-coupled audit** (ArgoCD/Flux) — audit is JSONL run-logs +
   plan-hashes, not a git repository acting as the API.
5. **No image monoculture** (Bright/Rocks) — heterogeneity is the
   default; Mooncake stays above the OS.
6. **No ACID claims** (Terraform/Ansible failure modes) — be honest
   about SAGA semantics; surface irreversibility in the plan UI.
7. **No pipeline DSL** (Jenkinsfile/Github Actions) — rollout is
   flags on apply, not a separate artifact language.

### The honest summary

Mooncake's job in 2026 is not to invent. The 14 systems researched
already produced the primitives that work. Mooncake's job is to **pick
the four survivors, combine them in a way none of the historical
systems did (typed mutation + agent-shaped surface), and resist the
three pathologies that killed the rest**.

The historical lesson is unanimous: the systems that died, died from
*growing into something else*. Mooncake's most important architectural
property is **staying what it is**.

---

## 5. Direction conclusion: what becomes graph, what stays Step

The issue's deliverable #3 asked for a concrete partition.

### What stays Step-based (do not graph-ify)

These are *units of execution*. They have inputs, outputs, idempotency
guarantees, and dry-run support. Adding graph semantics to them adds
nothing.

- The action ABI (`Handler` interface, `Result`, `Permissions`,
  `Reverse`). Stays as it is.
- Per-step idempotency (`creates`, `unless`, `changed_when`). Stays.
- The preset model (parameters, base-dir, expansion). Stays.
- Per-action testing. Stays.

### What becomes graph-based (issue #8 ChangeGraph)

These are *relationships between steps*. They cannot be expressed cleanly
in a flat step list:

- Step dependencies (`after`, `before`, `requires`, `wants`,
  `conflicts`, `on_failure`). Borrow systemd's edge vocabulary.
- Rollback ordering on transaction failure. The graph determines
  which compensators run, in what order.
- Risk scoring. The graph's *shape* (depth, breadth, irreversible
  fraction, sudo fraction) feeds the risk score from issue #8 §6.
- Explainability. `mooncake explain nginx` walks the graph backward
  from a resource to the changes that introduced it.

Implementation order: graph as *internal IR* first, derived from the
existing step list at plan time. CLI surface (`mooncake plan --graph`,
`mooncake explain`) comes second. Storage / artifact format third.

### What stays intentionally simple

These are areas where the temptation to over-engineer is strong:

- **Variables and templates.** Pongo2 + a typed `when:` is enough.
  No new template features.
- **Preset composition.** Flat presets only (no nesting). Already
  the rule in LLM_GUIDE.md.
- **Fleet topology.** A flat peer list with tags. No tree, no graph,
  until a real consumer (placement, power-graph) needs it.
- **Policy.** Narrow-scoped (action-quota, maintenance-window, sudo-
  fence). No expressive policy language.
- **Scheduling.** Placement is a recommendation tool, not a scheduler.

### What should never be built

Restated from §4, in the form the issue requested:

| Never build | Because |
|---|---|
| Manifest compilation layer | Puppet's path to complexity |
| Plugin SDK / marketplace | Terraform provider ecosystem trap |
| CRD-shaped controller architecture | Kubernetes sprawl |
| Pure-functional system semantics | Nix usability cliff |
| Jinja-as-programming-language | Ansible YAML decay |
| Reactor / event-DSL layer | Salt complexity |
| "Promise" / proprietary DSL terminology | CFEngine social failure |
| Git-as-the-database | GitOps tax |
| Image-based monoculture provisioning | HPC vendor lock-in |
| ACID-claiming transaction layer | Database lie applied to infra |
| Pipeline DSL | Jenkinsfile et al. |
| Expressive policy DSL | OPA / Rego sprawl |
| Centralized SaaS control plane | Mandatory cloud capture |
| Distributed consensus in the control plane | Operational fragility |

---

## 6. Recommended next reads

- [`issue-11-analysis.md`](./issue-11-analysis.md) — the per-capability
  fleet-mgmt audit; this doc's companion.
- [`agentic-interface-brainstorm.md`](./agentic-interface-brainstorm.md)
  — agent-centric primitives (typed observability, federated MCP,
  reconciliation loops, capability-scoped trust).
- [`../epics/done/epic-personal-fleet.md`](../epics/done/epic-personal-fleet.md)
  — the solo-dev scope where most of this work lives.
- [`../epics/epic-cluster-management.md`](../epics/epic-cluster-management.md)
  — the platform-team scope (boundary: "above the OS").
- Issue [#8 ChangeGraph](https://github.com/alehatsman/mooncake/issues/8)
  — the typed-mutation-IR thesis this doc endorses.
- Issue [#9 killer demo](https://github.com/alehatsman/mooncake/issues/9)
  — the demo arc that proves the differentiator publicly.

---

## 7. Suggested next action

The audit makes the priority obvious: **land ChangeGraph v0 (issue #8)
before anything else strategic.**

Rationale: of the four surviving primitives (¶1), Mooncake already has
convergence (✅), plan-before-apply (✅), and the start of attribution/IDs
(🟡). The missing one — dependency graph + compensator — is the
substrate that *everything else in the roadmap* (drift, explain,
waves, rehearsal, AI agent negotiation) reads from. Adding any of
those before the graph means re-shaping them later when the graph
lands.

Concrete first slice (matching issue #8's "Suggested next concrete
slice"):

1. Internal `Change` / `ChangeGraph` IR wrapping existing planned
   steps. Edges from systemd vocabulary.
2. `mooncake plan --graph` emits the graph as JSON.
3. Risk scoring using graph shape + existing `Permissions()` /
   reversibility metadata.
4. `mooncake explain <resource>` walks the graph backward against
   run history.
5. MCP `plan` / `diff` / `risk` / `apply_approved` surfaces, all
   keyed by plan_hash.
6. One deterministic demo (issue #9): AI proposes plan → Mooncake
   shows risk + reversibility → human approves → execution succeeds
   or rolls back cleanly.

This is the spine that converts Mooncake from "another config tool"
into "the typed mutation substrate for autonomous agents." The
historical research's only real verdict is: *that wedge is open, and
nothing in 30 years has filled it.*

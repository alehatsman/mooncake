# Non-goals

The list of things Mooncake will not become, each grounded in a
specific historical system that died from growing into it.

**When to consult:** every proposed feature, spec, or PR. If a proposal
moves the project toward any line below, the proposal needs a different
shape — not a polished version of the same shape.

For the full historical receipts that produced this list, see
[`bad_lessons_from_other_tools.md`](./bad_lessons_from_other_tools.md).

---

## The seven explicit non-goals

### 1. No DSL evolution

The YAML schema is closed. New features are typed action fields; new
behavior is a new handler. **Never** new template syntax, new
control-flow keywords, new `loop_control` / `with_subelements`
variants.

> Puppet manifest compilation, CFEngine "promises", Ansible Jinja
> explosion, Chef Ruby DSL, HCL functions — every config tool that let
> its DSL grow into a programming language paid for it with readability
> and ramp time.

**Test:** "Could someone debug this at 3am during an outage without
reading the docs?" If no, don't ship it.

### 2. No provider / plugin marketplace

The closed action set is a feature. Out-of-tree integrations are
*separate tools that produce Mooncake YAML*, not plugins inside the
runtime. If a need is real and recurring, it becomes a built-in
action. If it's one-off, it stays in `shell`. **There is no middle
layer.**

> Terraform's 3000 providers, K8s operators, Jenkins plugins — the
> ecosystem outgrows the runtime, version skew is everywhere, "we
> deprecated `aws_eip` last year" is a permanent state.

### 3. No control-plane sprawl

Loops run on agentd, locally, against local state, with explicit
per-plan policy. **Never** controllers as a first-class abstraction,
admission webhooks, finalizers, leader election, CRD-as-data-model.

> Kubernetes itself, Puppet masters + PuppetDB + ENC, Chef Server,
> ArgoCD/Flux + their CRDs — the control plane that promised to remove
> complexity *is* the complexity.

**Mental test:** if a feature would benefit from a control loop, ask
*where* the loop runs. "On agentd, against local state, with explicit
policy" — good. "On a controller that watches the fleet" — stop.

### 4. No git-coupled audit

Audit is JSONL run-logs + plan-hashes in `~/.mooncake/runs/`. **Never**
git-as-the-API, push-vs-pull religious wars, ArgoCD/Flux-shaped
deployment models.

> GitOps' attribution property (author + reason + timestamp) is the
> surviving idea. Git as a queryable database isn't.

### 5. No image monoculture

Heterogeneity is the default. Mooncake stays *above* the OS — a fleet
mixes Ubuntu / Debian / Fedora / macOS / Windows and applies the same
plan with per-host overlay differences. **Never** PXE-based image
provisioning, golden-image enforcement, vendor-locked compute.

> Bright Cluster Manager / Rocks lost ground when GPU diversity
> exploded; xCAT and Warewulf survived by accepting heterogeneity.

### 6. No ACID claims

Be honest about SAGA semantics: each step is a local transaction, some
have compensators (`Reverse()`), some are explicitly irreversible.
**Never** claim atomicity for system mutation. Surface irreversibility
in the plan UI, before execution, with explicit operator
acknowledgement.

> "Partial apply on failure" lies in Terraform; "stop on first failure"
> lies in Ansible; "this run failed, try again next cycle" lies in
> Puppet. The honest model is the trustworthy model.

### 7. No pipeline DSL

Rollout is *flags on `fleet apply`*, not a separate artifact language.
`--canary <peer>`, `--wave-size N`, `--health-gate <name>`. **Never**
`pipeline.yml`, pipeline-shaped variable scoping, matrix expansion, or
secret handling as a separate-from-apply concept.

> Jenkinsfile, GitHub Actions YAML, CircleCI config — each started as
> "describe your rollout" and grew into an ad-hoc programming language
> with its own scoping rules. Every CI/CD platform regrets it.

---

## The full no-build catalog

The seven above are the most-repeated traps. The full list, with the
historical referent for each:

| Never build | Because (which system died from it) |
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

## Borrow vocabulary, not implementation

Two concrete patterns where Mooncake should adopt existing names
rather than invent its own:

- **Dependency edges**: systemd's `after` / `before` / `requires` /
  `wants` / `conflicts` / `on_failure`. Operators already know these
  names. Don't invent new edge-type vocabulary.
- **Invariants**: call them `assert:`, `healthcheck:`, `invariant:`, or
  `constraint:` — words operators already know. **Not** "promises"
  (CFEngine's social failure), not "contracts," not "convergence
  deltas." Pick the boring name.

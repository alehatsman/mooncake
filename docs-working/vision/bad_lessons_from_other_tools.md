# Bad lessons from other tools

The pathologies that killed or hollowed-out prior systems. Each line is
a specific receipt — what the system tried to do, why it ended up
worse than the alternative.

This is the longer-form rationale behind [`non_goals.md`](./non_goals.md).
When somebody proposes a feature that looks reasonable, the question is
not "does it sound clever?" but "which of these graves are we walking
into?"

## Three pathologies that kill config-management systems

### A. DSL religion

> The config language grows into a programming language, then a
> runtime, then a culture.

| System | What happened |
|---|---|
| **Puppet** | Manifest compilation became a language with classes, defines, parameterized types, Hiera lookup hierarchies. Debugging required understanding the compiler. |
| **CFEngine** | "Promises" as marketing terminology; the DSL accumulated body slots, edit_line bundles, with control flow that was both alien and underpowered. |
| **Ansible** | YAML + Jinja2 was the elevator pitch; the reality became Jinja-as-programming-language with `loop_control`, `with_subelements`, `combine(recursive=True)`, and "look at this AnsiblePilcher trick." |
| **Chef** | Ruby-as-DSL. Resources collected actions; metadata blocks ran arbitrary Ruby. Debugging required knowing Ruby's load order and what the recipe DSL macro-expanded to. |
| **HCL functions** | Terraform added `for_each`, dynamic blocks, `try()`, `flatten()`, `coalesce()` — at some point HCL became a language you have to learn, not a config format you skim. |
| **Kubernetes CRDs** | Each CRD ships its own DSL inside YAML. The user is now learning 30 mini-languages, none of them transferable. |

**The trap mechanism:** every DSL feature is added because *one user's
real problem* couldn't be expressed otherwise. The feature ships. The
next user finds a similar problem and asks for the next feature. Two
years in, the DSL is Turing-complete and the cognitive load is
prohibitive.

**Mooncake's stance:** YAML schema is closed. Expressiveness wanted?
Write Go, ship a handler. The handler is the extension point — not the
template language.

### B. Provider / plugin marketplace

> The core stays small; the periphery becomes 90% of the user's
> surface area, with no API discipline.

| System | What happened |
|---|---|
| **Terraform** | 3,000+ providers. The `aws` provider alone has 1,500+ resources. Provider authors version independently from core; breaking changes happen at random intervals. "Which provider version supports what" is a permanent puzzle. |
| **Kubernetes operators** | Operator Framework, OLM, OperatorHub. Every database, every queue, every cache ships an operator. Each operator brings its own CRDs, its own bugs, its own update cadence. Cluster-wide upgrade orchestration is a job role. |
| **Jenkins plugins** | The plugin ecosystem is the surface area. Plugin compatibility matrices are full-time work; "Jenkins administrator" became a specialty primarily about plugin management. |
| **Ansible Galaxy** | Quality varies wildly. Roles depend on roles depend on roles. Pinning is a survival skill. |

**The trap mechanism:** core authors look at the unanswered demand
("our action set doesn't cover X cloud provider") and decide the
right answer is *to let other people add it.* That sounds humble. What
it actually means: an unbounded surface area that the core team can't
maintain, with quality and security spread across N maintainers of
varying skill, none accountable to the core team's standards.

**Mooncake's stance:** the closed action set is a feature. If a need
is real and recurring, it becomes a built-in action under the same
quality bar as the rest. Out-of-tree integrations are *separate tools
that produce Mooncake YAML*, not plugins inside the runtime.

### C. Control-plane sprawl

> The system that promised to remove complexity grows a control plane
> more complex than the workloads it manages.

| System | What happened |
|---|---|
| **Kubernetes itself** | Started as "container orchestration." Now: etcd quorum, control plane HA, admission webhooks, finalizers, RBAC, network policy, CSI/CNI/CRI plugins, autoscalers, custom controllers. Operating a cluster is a full-time team. |
| **Puppet masters** | PuppetDB + ENC + console + r10k + Hiera; the Puppetfile is now its own ecosystem. The master being slow is the new "ops problem." |
| **Chef Server** | Same trajectory: bookshelf, knife configuration, supermarket sync, audit reports. |
| **ArgoCD / Flux** | Their own CRDs (`Application`, `ApplicationSet`, `Kustomization`, `GitRepository`). Operating ArgoCD is harder than operating the apps you deploy with it. |
| **Mesos + Marathon + Aurora** | Each one's control plane required its own ops manual; the original "datacenter OS" pitch died under the weight. |

**The trap mechanism:** every centralization decision looks like
simplification at the time. "We need a single source of truth for
inventory" → control plane. "We need policy in one place" → control
plane gets a policy engine. "We need approvals" → control plane gets
auth. Five years later, the control plane has more moving parts than
the systems it's controlling, and an outage of the control plane is
worse than an outage of any single workload.

**Mooncake's stance:** loops run on agentd, against local state, with
explicit per-plan policy. The controller (the human at the CLI) does
not hold fleet-wide state authoritatively. If a feature would benefit
from a fleet-wide control loop, the answer is usually "run that loop
on each agentd locally with consistent inputs," not "add a controller."

## Three more graves worth marking

### D. Image monoculture

| System | What happened |
|---|---|
| **Bright Cluster Manager / Rocks** | PXE-boot a golden image. Worked great when nodes were homogeneous. GPU diversity in HPC exploded; both products lost ground. |
| **Container-only deployment** | Pushed everything into images; runtime patches became image rebuilds; "just rebuild the world" stopped scaling once the world was 80,000 microservices. |
| **VMware templates** | Identity images mean operators are forever fighting "the image is slightly stale" without a structured fix loop. |

**Trap:** images optimize the same-shape case at the cost of the
diverse-shape case. Mooncake stays *above* the OS — heterogeneity is
the default, per-host overlays land naturally, the same plan applies
to Ubuntu / Debian / macOS / Windows with declarative differences.

### E. Pure-functional purity

| System | What happened |
|---|---|
| **Nix / NixOS** | The right answer to reproducibility, taxed by a learning cliff so steep that the user base remains specialty after 20 years. `nix flakes` was its own DSL evolution problem. |
| **Guix** | Same shape as Nix, with even less mass. |
| **Bazel / Bazel rules** | Hermetic builds are gold; the BUILD-file rule ecosystem is its own learning curve. Most teams ship Bazel adoption efforts that take quarters. |

**Trap:** purity demands rewriting the world in your model. Mooncake
borrows Nix's *attribution* property (every run is logged, every plan
has a hash) without demanding that the user abandon imperative thinking.

### F. Pipeline DSLs

| System | What happened |
|---|---|
| **Jenkinsfile** | "Describe your rollout in Groovy." Within a year, every team had a `library` of shared steps, scoping rules, and `parallel` blocks the docs barely covered. |
| **GitHub Actions YAML** | The expression syntax (`${{ … }}`) became a language inside YAML — matrices, `if:` conditionals, `outputs:`, reusable workflows. The docs page count exceeds the runtime that interprets them. |
| **CircleCI / GitLab CI** | Same trajectory. Pipeline-shape DSLs have a 100% historical record of growing into ad-hoc programming languages with their own scoping rules. |

**Trap:** rollout *is* a feature of the apply command, not a separate
language. Mooncake's stance: rollout is `--canary <peer>` /
`--wave-size N` / `--health-gate <name>` flags on `fleet apply`. Not
`pipeline.yml`. Not pipeline-shaped variable scoping. Not "matrix
expansion."

## The "AI for X" startups failing in real time

A more recent grave worth marking. The current generation of "LLM +
infrastructure" startups generally lands on:

- **LLM + shell + hope.** The agent has full shell access; the only
  safety is the system prompt and operator vigilance. Reversibility
  isn't structured because the underlying interface (shell) doesn't
  declare what's reversible.
- **No typed surface.** The agent gets command outputs as strings,
  parses them with regex, and breaks the moment an output format
  shifts. Compare to Mooncake's MCP server: structured JSON in,
  structured JSON out, every field schemaed.
- **Sandbox at the *environment* level**, not the *intent* level. (E2B,
  Daytona, Modal.) Useful and complementary — Mooncake fits inside
  their VMs — but they sandbox the *what the agent can reach*, not the
  *what the agent can do*. Both layers matter.

**Trap:** the AI hype skips over the boring engineering — typed
actions, dry-run, structured diff, rollback — exactly the things that
make agents *safe* rather than impressive. Mooncake's wager is that
boring beats clever.

## How to use this list

Before greenlighting any feature proposal, run it past these graves:

1. Does it grow the YAML DSL? → see (A).
2. Does it create a plugin / provider surface? → see (B).
3. Does it require a central controller or fleet-wide state? → see (C).
4. Does it bake in OS or distro assumptions? → see (D).
5. Does it ask the user to learn a purity model? → see (E).
6. Does it propose a separate pipeline / rollout language? → see (F).
7. Does it sound like "AI + X with no typed surface"? → see the
   AI-startup graves.

If any answer is "yes," the proposal needs a different *shape*, not a
polished version of the same shape.

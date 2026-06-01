# Mooncake — Vision & Brainstorm

> Status: brainstorming doc, not a roadmap. Use it to surface possibilities and
> tensions. Decisions belong in `docs-next/architecture-decisions/` once made.

---

## 1. The thesis

> **Mooncake is a typed, idempotent, audited execution layer between an actor
> (human, script, or AI) and a system.**

The same properties that make declarative config managers good (idempotency,
dry-run, typed actions, audit trails) are *exactly* the properties that make an
execution layer safe for AI agents. And the same control plane that gives an
enterprise visibility over a fleet is the same control surface a solo dev wants
over their personal machines — just at a different scale.

The product is not "Ansible for AI". The product is **a system-call ABI for
intent → physical change**, with three increasingly large rings of value built
on top:

1. **The kernel** — declarative actions, idempotency, planning, facts. (today)
2. **The runtime** — host daemon, fleet orchestration, audit, policy.
3. **The economy** — preset marketplace, agent SDK, signed plans, integrations.

---

## 2. The core insight (the unifying primitive)

Every actor — a sysadmin in a terminal, a CI pipeline, a Claude/Cursor agent —
ultimately mutates a host through some thin interface (shell, API calls, file
writes). Today that interface is *unconstrained*: anyone with the credential
can do anything.

Mooncake turns that interface into a **constrained, observable funnel**:

```
   ┌──────────┐   ┌──────────┐   ┌──────────┐
   │  Human   │   │  Script  │   │   AI     │
   └────┬─────┘   └────┬─────┘   └────┬─────┘
        │              │              │
        └──────────────┼──────────────┘
                       ▼
              ┌────────────────┐
              │   Mooncake     │   ← typed actions, validation,
              │   execution    │     dry-run, policy, audit,
              │   engine       │     idempotency, rollback
              └────────┬───────┘
                       ▼
                  ┌─────────┐
                  │ System  │
                  └─────────┘
```

If every mutation flows through this funnel, you get:

- **Auditability for free** (every change is a typed event)
- **Idempotency for free** (the engine guarantees it, not the actor)
- **Policy enforcement for free** (one place to gate)
- **Reversibility for free** (snapshot + replay)
- **Agent safety for free** (the agent literally cannot do anything else)

That last bullet is the wedge. Nobody else is selling this to AI developers.

---

## 3. Where Mooncake is today (honest snapshot, 2026-05-15)

What exists in `internal/`:

| Capability | Module | Maturity |
|---|---|---|
| Typed actions (40+: file/text/pkg/os/git/container/repo/wait families) | `actions/` | Production |
| **Extended handler ABI** (`Diff` / `Reverse` / `Cost` / `Permissions`) | `actions/handler_abi.go` | Production — all phases shipped across priority handlers |
| **`transaction:` blocks with LIFO auto-revert** (spec-30) | `executor/transaction.go` | Production — `examples/transactions/rollback-demo.yml` |
| **Typed secret refs** (`!secret env:KEY`) + 3 providers (env/file/stdin) | `security/secrets*.go` | Production — resolved values auto-added to Redactor denylist |
| **Reactive triggers** (`on_change:`) | `executor/`, `plan/` | Production — `examples/triggers/on-change-config-reload.yml` |
| Preset system (330+ built-in) | `presets/` | Production |
| Plan compilation, include resolution, structural Diff in JSON output | `plan/` | Production |
| Executor with idempotency, Permissions preflight, secret resolution | `executor/` | Production |
| System facts (cached) | `facts/` | Production |
| MCP server (LLM tool calls) | `mcp/` | Working |
| Agent loop (iterate-until-done) | `agent/` | Working |
| Run log, structured events | `runlog/`, `events/` | Working |
| Snapshot / diff | `snapshot/` | Working |
| Secret redaction | `security/redact.go` | Production |
| Effects system | `effects/` | Working |
| Artifact capture | `artifacts/` | Working |
| **Personal fleet** — `mooncake fleet apply/status/logs/bootstrap` across N peers (peer-to-peer, no hub) | `agentd/`, `fleet/` | Production — Phase A+B complete, Phase C partial (mDNS + `fleet init` interactive remain) |

**What this means**: ring 1 (kernel) is *complete*, including the spec-22
ABI + spec-23 §1+§3 + spec-30 transactions that ring 2 (the safe-agent
runtime) was supposed to need first. The agent-safety wedge that
`docs-working/analysis/next-priorities-2026-05.md` flagged as the strategic
pivot now has working primitives — `transaction:` blocks auto-revert,
`!secret` refs don't leak, `on_change:` triggers fire on real change.
The README's safety claims map 1:1 to runnable examples in
`examples/transactions/`, `examples/secrets/`, `examples/triggers/`.

Personal fleet shipped too — bootstrap a fresh box, apply across N
peers, multiplex logs, peer-tag filter — all working against a real
WSL + Windows two-peer testbed.

Expansion path from here is outward into ring 3 (economy): marketplace,
WASM plugins, IDE extensions, lighthouse-user case studies.

---

## 4. Three market wedges, one engine

The strategic trick: **the same engine** serves three audiences. Each wedge
funds the next.

### 4.1 Solo developer — "Dotfiles & dev box on autoagent"

The friendliest entry point. Already most of the way there.

- `mooncake init` scaffolds a dotfiles repo with sensible presets.
- One-command bootstrap of a new laptop (`curl … | mooncake apply`).
- Drift detection: "your dev box no longer matches your committed config."
- A small TUI showing what's installed, what's missing, what's drifted.
- "Borrow this": import another dev's mooncake config as a starting point.

**Hook**: nobody loves their dotfiles repo. Mooncake makes it boring and safe.

### 4.2 AI agent developer — "Docker for AI agents"

The most defensible wedge. Mooncake is the **sandboxed substrate** an agent
runs against.

- Mooncake as a Claude/Cursor/Codex sub-agent: "do anything you want, but you
  must propose a plan in YAML, and I'll execute it."
- Constrained mode: the agent has NO shell access at all — only the Mooncake
  ABI. Every mutation is typed, dry-runnable, reversible.
- The agent's "tools" are mooncake actions. Adding a new tool = adding a new
  action handler, available to every agent for free.
- Replay: rerun an agent's exact plan deterministically.
- Sandboxed test: execute the agent's plan in a Linux namespace / Lima VM /
  Docker container first, only commit if it passes assertions.

### 4.3 Platform team — "Fleet control plane with audit by default"

The monetizable wedge. Same engine, scaled out.

- Inventory of all hosts (facts, package versions, drift).
- Fleet plans: roll a preset across N hosts with canary / wave strategy.
- Audit log: signed, append-only, exportable (SOC2 / ISO friendly).
- RBAC: who can run what action on what host.
- Approval gates: "any action touching `/etc/security/*` requires a human".
- Dashboards: change rate per service, drift heatmap, runs-per-host.

**Pricing instinct**: free CLI forever; control plane priced per-host or per-run.

---

## 5. The product surface, layered

```
┌──────────────────────────────────────────────────────────────┐
│  L5: Marketplace + Agent SDK + integrations (GitHub, IDEs)   │
├──────────────────────────────────────────────────────────────┤
│  L4: Cloud Hub (SaaS or self-hosted) — fleet, audit, policy  │
├──────────────────────────────────────────────────────────────┤
│  L3: Host daemon (mooncake agentd) — mTLS, pull/push, queue  │
├──────────────────────────────────────────────────────────────┤
│  L2: CLI + MCP server + agent loop                           │
├──────────────────────────────────────────────────────────────┤
│  L1: Kernel — actions, planner, executor, facts, snapshot    │
└──────────────────────────────────────────────────────────────┘
```

Each layer is independently usable. A solo dev only ever sees L1+L2. An agent
developer adds L5's SDK. An enterprise consumes all five.

### Naming sketch (open)

- `mooncake` — the CLI (today)
- `mooncake agentd` — the host daemon
- `mooncake hub` — the control plane (also `station` / `mission-control`)
- `mooncake guard` — the agent sandbox runtime
- `mooncake registry` — the preset marketplace

---

## 6. Capability brainstorm (wide, unfiltered)

Group these by where they sit in the layered architecture.

### 6.1 Kernel extensions (L1)

- **More action surfaces** (in order of likely demand):
  - Cloud primitives: DNS records, certs (ACME), S3 objects, secrets in
    Vault/1Password/age, GitHub/GitLab repo state.
  - Kubernetes objects: `kubectl apply` as a typed action with diff.
  - Container actions: build / push / run / stop with idempotency.
  - Database actions: schema migrations as idempotent operations.
  - Network actions: firewall rules, port forwards, VPN configs.
  - Identity: user / group / SSH key management with drift detection.
- **Reverse plans** — given a snapshot diff, generate a rollback plan
  automatically. The killer demo for "safe agents".
- **Multi-host plans with a DAG**: "deploy DB before app" expressed in YAML.
- **Effect prediction** — static analysis (or LLM) that classifies each step
  by blast radius before execution.
- **Cost estimation** — "this plan will modify 412 files across 14 hosts,
  estimated 90s, risk score 7/10."
- **Policy DSL or OPA integration** — `deny: agent.touches("/etc/passwd")`.
- **Plan signing** — Sigstore-style signed plans; daemon refuses unsigned
  plans in production mode.
- **WASM plugin actions** — let people write custom actions in Rust / TS / Go
  without forking.

### 6.2 Runtime / Daemon (L3)

- mTLS to a hub, pull-mode by default, push-mode when needed.
- Local plan queue, runs asynchronously, reports back.
- Offline operation: cache last good plan, keep running if hub is down.
- Streaming facts (subscribe to "package version of X across fleet").
- Self-update with safety net (refuse to update if last 3 health checks failed).
- Embedded SQLite as local state store.
- Process supervision: take over from systemd for managed services.

### 6.3 Control plane (L4)

- **Inventory view** — every host, its facts, its current "compliance" with
  declared state.
- **Drift heatmap** — visual: green = matches plan, red = drift.
- **Fleet runs** — apply a preset to a tag selector (`role=db AND env=prod`).
- **Rollout strategies** — canary, wave, ring deployments. Halt-on-failure.
- **Approval workflows** — Slack/email DM for risky plans, two-person rule.
- **Audit explorer** — query "every change touching nginx config in the last
  90 days, by who, with diff."
- **Drift alerts** — page someone when a host diverges from declared state.
- **Change windows** — only allow non-emergency runs in approved windows.
- **Compliance packs** — preset bundles tied to CIS / PCI / SOC2 controls.
- **Postgres-backed multi-tenant** — for MSPs serving many small clients.

### 6.4 Agent SDK (L5)

This is the most novel layer; deserves its own section. See §7.

### 6.5 Ecosystem (L5)

- **Preset marketplace** — `mooncake install postgres@2.1.0`. Signed, versioned,
  optionally paid.
- **GitHub Action / GitLab CI step** — drop-in CI integration.
- **IDE extensions** — VSCode / Cursor / Zed plugin: "preview this AI change
  through Mooncake before applying."
- **`mooncake studio`** — TUI + web UI for plan visualization, drift, runs.
- **Terraform / Pulumi bridge** — read TF state, emit Mooncake actions.
- **Backstage plugin** — surface fleet state in internal developer portals.
- **Foundation-model fine-tuning** — open dataset of Mooncake YAML so models
  generate it natively, like they do with Dockerfiles.

---

## 7. Deep dive — the Safe Agent Runtime

This is the most differentiated wedge. Worth expanding.

### 7.1 The pitch

Today's AI agents either run with full shell access (terrifying) or get
hand-built bespoke sandboxes for each project (slow, leaky). Mooncake offers a
third option: **a typed, declarative sandbox where every system mutation is
mediated, auditable, and reversible.**

### 7.2 Modes of agent integration

| Mode | What the agent gets | Use case |
|---|---|---|
| **Tool mode** | Mooncake actions exposed as MCP tools | Lightweight: agent uses them when helpful (today) |
| **Funnel mode** | Agent can run shell, but only via `shell` action — every command logged | Migration path from raw access |
| **Sandbox mode** | Agent has *no* shell or file API. Only the Mooncake ABI. Everything typed. | Production-safe |
| **Replay mode** | Agent emits a plan; mooncake executes — agent never touches the system directly | Best for autonomous, unattended runs |

### 7.3 Concrete features the agent layer would need

- **Action surface for everything the agent might want to do**: read files,
  edit files, run tests, query APIs, ask questions. (Some exist; most don't yet.)
- **Cost/risk classifier** per step: agents see "this step is reversible /
  irreversible / requires approval" before running.
- **Step-level confirmation** mode: a human approves each step (good for high-
  stakes agents).
- **Plan diffing**: "agent's plan changed since last review — here's what
  changed." Powerful for code review of agent outputs.
- **Deterministic replay** for debugging agent behavior.
- **Per-action quotas** — "this agent may make at most 10 file edits".
- **Egress policy** — "this agent may only download from npm/pypi/github".

### 7.4 Why this is defensible

- Existing agent sandboxes (Daytona, E2B, Modal) sandbox the *environment*.
  Mooncake sandboxes the *intent*. They're complementary; Mooncake fits
  *inside* their VMs.
- LLM vendors will not build this — it's orthogonal to their model business.
- Idempotency + dry-run are very hard to bolt on later. Mooncake has them.

---

## 8. Deep dive — the Enterprise Control Plane

### 8.1 Why now

The "Ansible Tower / AWX" market is sleepy. Customers want:

- A modern UI (Ansible Tower's is ~2015 vintage).
- Audit that satisfies auditors out of the box.
- AI integration (their devs are using Cursor/Claude and IT has zero
  visibility).

Mooncake can enter from the AI angle: "let your engineers use Claude on prod,
safely." Then *expand* into general fleet management.

### 8.2 Minimum lovable enterprise feature set

1. **Inventory** with facts, last-seen, drift state.
2. **Run history** — every plan applied, by whom, with diff.
3. **Approval gates** — risky plans (configurable) need human sign-off.
4. **RBAC** — at minimum: read / propose / approve / execute roles per host
   tag.
5. **Audit export** — signed JSON-lines stream, S3-compatible sink.
6. **Slack/Teams integration** — approvals + run notifications.
7. **SSO** — SAML / OIDC.

### 8.3 What would surprise people

- **AI assistants as first-class actors**: their runs appear in the audit log
  alongside humans, with cost attribution.
- **Self-service drift remediation**: if a host drifts, the hub proposes a fix
  plan automatically; human just approves.
- **Time-travel inventory**: "show me what this host looked like last Tuesday."

---

## 9. Deep dive — the Solo Developer angle

Don't underestimate this. It's the funnel that gets devs to bring Mooncake into
work.

- `mooncake init dotfiles` — scaffolded repo + first run.
- `mooncake doctor` — interactive: "your nvim config is 14 days stale vs git;
  pull?".
- `mooncake share <preset>` — push a preset to the marketplace.
- **"Try this dotfile"** — preview a public preset in a Lima VM in 30s before
  applying.
- **Multi-machine sync** — same config across laptop / desktop / VPS, with
  per-host overrides.

The free-forever offering. No telemetry. Bring-your-own-AI for the AI features.

---

## 10. Strategic tensions and decisions to make

These are real forks. Worth deciding intentionally rather than drifting.

### 10.1 Open-core vs fully open

- **Fully open**: easier adoption, harder monetization.
- **Open-core**: kernel + CLI MIT, hub & enterprise features source-available
  or commercial. (HashiCorp / GitLab model.)
- **Recommendation hint**: lean open-core. The kernel must stay MIT to win
  agent-developer trust. Hub features can be commercial.

### 10.2 Agent-first vs admin-first framing

- **Agent-first**: ride the AI wave, gets attention, but pigeonholes you.
- **Admin-first**: bigger market but you're "another Ansible".
- **Hybrid**: lead with agent ("the safe runtime for AI infra"), upsell into
  admin features. The agent story is what gets the press; the admin features
  are what get the renewal.

### 10.3 SaaS vs self-hosted hub

- Self-host first is faster to ship and more palatable to enterprise.
- SaaS is where the recurring revenue and observability data live.
- **Recommendation hint**: both, eventually. Self-host first.

### 10.4 Action breadth vs depth

- **Breadth** (every cloud API, every k8s resource): tempting, but a treadmill.
- **Depth** (best-in-class file, template, shell, service, secrets, k8s
  basics): wins agent-developer hearts because their agents are 80% file edits.
- **Recommendation hint**: depth first, then breadth via WASM plugins so the
  community fills the long tail.

### 10.5 Daemon now or later?

- Daemon unlocks fleet and AI-runs-on-prod stories.
- But it's a major operational surface to maintain.
- **Recommendation hint**: prototype the daemon now, but ship it as
  "experimental" alongside the CLI for a long time. Don't bet the project on it.

### 10.6 Compete with Ansible directly?

- Don't. The Ansible market is shrinking and grumpy. Position as
  *complementary* ("Mooncake for AI-driven config, Ansible for your big yaml
  monorepo") until you've earned the right to displace it.

### 10.7 Policy language

- **OPA/Rego**: powerful, already trusted by enterprise, off-the-shelf.
- **Native DSL**: friendlier syntax, but yet-another-language.
- **Recommendation hint**: start with a tiny native DSL that compiles to
  OPA when complexity demands.

---

## 11. Possible phasing (6 → 18 months, sketch)

### Phase A — Solidify the kernel + win agent developers (0–6mo) — **largely done as of 2026-05-15**
- ✅ Frozen action ABI: spec-22 phases 1–5 shipped across the priority handler set
- ✅ Reverse-plans + transactional auto-revert: spec-30 PR A+B in production
- ✅ `!secret` typed refs + 3 providers (env/file/stdin) with redaction
- ✅ MCP server with `get_facts`/`run_plan`/`check_plan`/`get_snapshot`/`get_metrics`/`fact_query`
- ⏳ Mature MCP server — surfacing Diff/Permissions/transactions to agent tools is still draft
- ⏳ Tiny preset marketplace (signed, GitHub-hosted) — Stream 5; not started
- ⏳ Land 2–3 lighthouse agent-developer users; write case studies — **the next strategic move**

### Phase B — Daemon + lightweight hub (6–12mo) — **partially done; personal-fleet shipped**
- ✅ `mooncake agentd` (production) — TCP + Unix socket, bearer auth, SSE event hub, sandboxed file sync
- ✅ Personal fleet (peer-to-peer) — `mooncake fleet apply/status/logs/bootstrap` across N peers without a hub
- ⏳ Enterprise hub MVP: inventory, run history, audit export, approvals, Slack — zero specs yet; deferred until a paying user asks
- ⏳ WASM plugin SDK for custom actions — spec-31 not started; in-tree Go plugin model still works for the first year
- ⏳ First commercial agents — gated on Phase A lighthouse users + a hub need surfacing

### Phase C — Enterprise polish + ecosystem (12–18mo)
- RBAC, SSO, compliance packs.
- Cloud SaaS hub.
- IDE extensions, GitHub Actions, Backstage plugin.
- Marketplace with paid presets / revenue share for authors.
- Position publicly: "the standard runtime for AI system configuration."

---

## 12. Development streams

The three rings and three wedges above map to five parallel development
streams. Each stream is independently useful and has a distinct audience.
Stream 1 is the infrastructure layer that all others depend on.

```
Stream 1: Action Surface          ← kernel completeness; serves all wedges
Stream 2: Safe Agent Runtime      ← "Docker for AI agents" wedge (§4.2)
Stream 3: Fleet & Cluster Mgmt    ← platform team wedge (§4.3)
Stream 4: Developer Experience    ← solo developer wedge (§4.1); the funnel
Stream 5: Ecosystem               ← ring 3 (economy): marketplace, plugins, integrations
```

**Stream 1 — Action Surface** *(largely done)*: Typed mutation vocabulary
covers 40+ actions across file/text/pkg/os/git/container/repo/wait families.
Extended handler ABI (`Diff`, `Reverse`, `Cost`, `Permissions`) shipped
across the priority handler set. Action breadth is no longer the
bottleneck — the long tail is community / Tier-2 plugin territory.

**Stream 2 — Safe Agent Runtime** *(load-bearing primitives shipped)*: AI
agents call a typed ABI; every mutation is dry-runnable, reversible, audited.
**Shipped**: `transaction:` blocks with LIFO auto-revert (spec-30),
`!secret env:KEY` typed refs + redaction (spec-23 §3, three providers),
`on_change:` reactive triggers (spec-23 §1), Permissions preflight,
structural Diff in plan JSON. **Open**: `try/catch/finally` (spec-23 §2,
design overlap with transactions now resolved), policy DSL (`deny:`
patterns over Permissions/Diff), plan signing (Sigstore), per-action
quotas, egress policy, deterministic replay command. The marketing
claim from the README ("safe execution runtime for AI-driven system
configuration") is now backed by working primitives, not promises.

**Stream 3 — Fleet & Cluster Management** *(personal fleet shipped)*:
GitOps for software state at fleet scale. **Personal-fleet sub-stream
done**: `fleet apply/status/logs/facts/bootstrap` across N peers,
peer-to-peer, no hub, validated against a real WSL + Windows testbed.
**Enterprise sub-stream deferred** (no users asking for hub yet). See
`docs-working/epics/done/epic-personal-fleet.md` and
`docs-working/epics/epic-cluster-management.md`.

**Stream 4 — Developer Experience**: The funnel. `mooncake doctor`, drift
detection UX, multi-machine sync, TUI dashboard. Gets solo devs adopting
Mooncake, who then bring it to work via streams 2 and 3.

**Stream 5 — Ecosystem**: WASM plugins, preset marketplace, GitHub Actions
integration, IDE extensions. Converts Mooncake from a tool into a standard.

For current spec assignments, status, and work order see
[`docs-working/streams.md`](docs-working/streams.md).

---

## 13. Open questions worth brainstorming further

These are the unknowns I'd want to pull on next:

1. **What does the daemon's wire protocol look like?** (gRPC vs HTTP+SSE, push
   vs pull, queue semantics, offline behavior.)
2. **How do plans get versioned?** Content-hash? Semver? Both? How does the
   hub deal with two hosts running v1.2 and v1.3 of the same plan?
3. **What's the smallest viable policy language** that covers 80% of
   "agent shouldn't do X" rules without becoming Rego-with-extra-steps?
4. **What's the killer demo for non-AI buyers?** ("we replaced 4000 lines of
   Ansible with 800 lines of Mooncake" vs "drift detection paid for itself in
   month 1" — pick one).
5. **What does "agent sandbox mode" feel like from inside Claude/Cursor?**
   What's the UX when the agent hits a policy wall?
6. **How does Mooncake interact with Terraform / Pulumi state?** Read-only?
   Bidirectional? Ignore entirely?
7. **What's the monetization wedge if everything stays free?** Hosted hub?
   Marketplace cut? Support contracts? Enterprise compliance pack?
8. **Should presets be more like packages (semver, deps, registry) or more
   like recipes (copy-paste, fork-friendly)?** Probably both, but which is the
   default?
9. **What's the right name for the "agent sandbox runtime"?** It deserves a
   product identity separate from `mooncake` itself.
10. **What's the unfair advantage?** ANSWER (as of 2026-05-15): **tight
    coupling of plan + Reverse + audit, all typed.** An Ansible+OPA+AWX
    combo can audit (AWX) and gate (OPA) but cannot automatically revert
    a half-applied transaction byte-identically to pre-state, because no
    handler in that stack declares a `Reverse()` method. Mooncake's
    `transaction:` blocks do that as a built-in. The
    `examples/transactions/rollback-demo.yml` is the demo that makes the
    claim falsifiable. **Deterministic agent replay** is the next leg —
    a `mooncake replay <run-id>` command would close the audit + reproduce
    loop. Not built yet.

---

## 14. One-paragraph elevator pitch (working draft)

> Mooncake is the safe execution runtime for AI-driven system configuration.
> It's a single Go binary today, with idempotent typed actions, dry-run, and
> built-in audit — used by solo devs to manage dotfiles and by AI agent
> developers to keep their agents from breaking production. The next chapter
> is a host daemon and control plane that bring the same guarantees to whole
> fleets, so platform teams can finally let AI touch prod without losing sleep.

---

*This doc is meant to be edited. Cross out what's wrong, expand what's
underdeveloped, fork sections into their own proposals under
`docs-next/proposals/` when they're ready to move from "idea" to "plan."*

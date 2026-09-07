# Goals

## The thesis

> **Mooncake is a typed, idempotent, audited execution layer between an
> actor (human, script, or AI) and a system.**

The same properties that make declarative config managers good
(idempotency, dry-run, typed actions, audit trails) are *exactly* the
properties that make an execution layer safe for AI agents. The same
control surface a platform engineer wants over a fleet is what a solo
developer wants over their personal machines — just at a different
scale.

The product is not "Ansible for AI." It is a **system-call ABI for
intent → physical change**, with three increasingly large rings of
value built on top:

1. **The kernel** — declarative typed actions, idempotency, planning,
   facts. *(Shipped.)*
2. **The runtime** — host daemon, fleet orchestration, audit, policy.
   *(Personal fleet shipped; enterprise hub unvalidated, not planned.)*
3. **Module distribution** — a Git-native, Go-module-shaped index for
   discovering and pinning other people's config modules (see
   [`sharing_and_modules.md`](./sharing_and_modules.md)). *(In design.)*
   No central marketplace, no registry to run — the Git URL is the
   identity, same as `go get`.

## The core insight

Every actor — sysadmin in a terminal, CI pipeline, Claude/Cursor agent
— ultimately mutates a host through some thin interface (shell, API
calls, file writes). Today that interface is unconstrained: anyone
with the credential can do anything.

Mooncake turns it into a **constrained, observable funnel**:

```
┌──────────┐   ┌──────────┐   ┌──────────┐
│  Human   │   │  Script  │   │   AI     │
└────┬─────┘   └────┬─────┘   └────┬─────┘
     │              │              │
     └──────────────┼──────────────┘
                    ▼
           ┌────────────────┐
           │    Mooncake    │   ← typed actions, plan/dry-run, policy,
           │   execution    │     audit, idempotency, rollback
           │     engine     │
           └────────┬───────┘
                    ▼
               ┌─────────┐
               │ System  │
               └─────────┘
```

If every mutation flows through this funnel, you get auditability,
idempotency, policy enforcement, reversibility, and agent safety —
each for free, because the engine guarantees them rather than the
actor.

That combination is the wedge. It's a property of the engine, useful
to whatever drives it — a human, a script, or an agent — not a
business case for a distinct "AI agent developer" market on its own.

## What "done" looks like, per user

Mooncake serves two validated audiences with one engine. Each
scenario below is the success bar for that audience.

> **Demoted 2026-09-07:** a third audience used to live here — "AI
> agent developer / Docker for AI agents" — pitched on policy DSL,
> plan signing, quotas, egress policy, sandbox mode, deterministic
> replay. Cut because: zero real users ever asked for it, the gap
> list (all six items) hadn't moved since it was written, and the
> premise ("an LLM agent has no shell, no raw file API") is aging
> badly now that mainstream coding agents get real sandboxed shell +
> file access gated by harness-level approval, not a mediating typed
> kernel. The underlying primitives it cited (typed ABI, MCP server,
> transactions, plan/diff, snapshot, secret redaction) are real,
> shipped, and stay — see the layered surface and unfair-advantage
> sections below. What's cut is the standalone audience claim, not
> the capability. Revisit only if a real external user asks.

### 1. Solo developer — dotfiles + dev box on autoagent

Pick up a fresh laptop / VM / WSL / Mac. Run one command. End up with:
dotfiles + dev tools + packages + services + drift detection + audit
trail. Works on Linux, macOS, Windows (WSL or native).

**State:** ~95% there. `mooncake init` ✓, default config discovery ✓,
`mooncake plan --diff` ✓, `mooncake apply` ✓, `mooncake doctor` ✓,
`mooncake history` ✓, parameterized preset system ✓ (in-tree library
retired in favor of a Git-native module system — see
[`sharing_and_modules.md`](./sharing_and_modules.md)), snapshot/diff ✓,
structured errors ✓, run history ✓. Gap: module distribution + lockfile
+ `mooncake share` UX are in design; "import existing dotfiles" doesn't
exist.

### 2. Multi-device on local network — the personal fleet

From any box, `fleet apply config.yml` runs across every machine you
own. Interleaved logs. `fleet status` shows health. `fleet bootstrap
user@new-box` adds a new machine in 60 seconds. Per-host overlays land
naturally. No hub, no SaaS, peer-to-peer over LAN.

**State:** ~99% there. agentd + bearer auth + SSE + sandboxed sync ✓,
multiplexed `fleet apply` ✓, `fleet status` ✓, `fleet logs/facts/exec/
watch/ps/discover/init/upgrade/doctor` ✓, native SSH driver ✓,
per-host overlays + tag selectors ✓, mDNS ✓, `fleet apply <machine>`
✓, Windows agentd ✓. The "Friday-evening demo" success criteria from
the personal-fleet epic are all met.

> **Demoted 2026-09-07:** a "Platform team — fleet control plane with
> audit by default" audience used to live here (canary/wave rollout,
> RBAC, approval gates, dashboards, priced control plane). Same
> problem as the AI-agent-developer ring: zero external users, purely
> conditional ("deferred until a paying user asks"), no work has ever
> moved on it. Cut for the same reason — an unvalidated audience
> doesn't earn a numbered slot next to two that are proven by daily
> use. The personal-fleet wire protocol and agentd shape it would have
> built on are real and stay; the enterprise product on top of them
> is not planned. Revisit only if a real paying user asks.

## The product surface, layered

```
┌──────────────────────────────────────────────────────────────┐
│  L5: Module discovery index (Git-native, no registry)        │ ← in design
├──────────────────────────────────────────────────────────────┤
│  L4: Cloud Hub (SaaS or self-hosted) — fleet, audit, policy  │ ← unvalidated, not planned
├──────────────────────────────────────────────────────────────┤
│  L3: Host daemon (agentd) — TCP+SSE, bearer auth, sync       │ ← shipped (personal fleet)
├──────────────────────────────────────────────────────────────┤
│  L2: CLI + MCP server                                        │ ← shipped
├──────────────────────────────────────────────────────────────┤
│  L1: Kernel — actions, planner, executor, facts, snapshot    │ ← shipped
└──────────────────────────────────────────────────────────────┘
```

Each layer is independently usable. A solo dev only ever sees L1+L2.
The personal fleet uses L3. The MCP server + transactions/secrets in
L2 make the same surface callable by a script or an agent, not just
a human at the CLI — that's a capability of L2, not a separate
product tier. L4 stays on the diagram as an honest "not planned"
marker, not a roadmap commitment.

## The unfair-advantage statement

The combination that no other tool ships today:

```
plan + snapshot + reverse + deterministic replay,
all typed end-to-end
```

Three of four are in master and demoable. Deterministic replay is the
last open piece on that line. An Ansible+OPA+AWX combo can audit
(AWX) and gate (OPA) but cannot automatically revert a half-applied
transaction byte-identically to pre-state — because no handler in that
stack declares a `Reverse()` method. Mooncake's `transaction:` blocks
do that as a built-in.

## The strategic constraint

The validated surface is solo-dev provisioning + the personal fleet —
both driven by daily real use. Everything past that (enterprise hub,
marketplace, a distinct agent-developer product) is unvalidated: no
external user has asked for it yet. The next bottleneck is **staying
inside what's validated** — sharpening provisioning, modules, and
fleet management for the audience that actually exists — not
speccing further out on the strength of a good story.

## Why the agent-framework idea was cut

*(Folded 2026-09-07 from `docs-working/vision/agent_framework.md`,
retired the same day — its own detail, kept here so the reasoning
isn't lost.)*

The framework thesis was "mooncake as a substrate other agents build
on": external consumers (moongit, a planned `openclaw`) registering
their own typed Go handlers, a four-layer cognitive architecture
(L1 registry / L2 kernel / L3 grounding / L4 reasoning), and a fully
offline general-purpose agent on top. It had a lighthouse-consumer
test: moongit was to register a `moongit.issue` typed action pack into
mooncake's registry, so a moongit-driven agent mutated the issue
tracker through mooncake's typed, reversible, audited ABI instead of
shell (moongit #107). That attempt closed the other way:

> "Closing as redundant. moongit now exposes its full surface over
> MCP. Typed moongit.\* mooncake actions are the long way around — #12
> (mcp_tool) gives the same coverage generically once it lands."

moongit became an MCP **server** that mooncake (or anything) calls
*into*, rather than a consumer that imports mooncake and registers
actions *into* it — the opposite integration direction from the one
the framework proposed. Neither the `openclaw` reference agent nor the
L3/L4 grounding split it existed to serve was ever built. The whole
external-framework narrative was speculative from the start.

Two things that were built on the way to it were re-justified on their
own merits, independent of any framework story: **registry-as-
dependency** (#105, the action registry as an injectable `*Registry`
rather than a package global) stays — it's internal hygiene, cleaner
DI. The public `sdk/` facade (#106) does not — its only real-world
justification, `internal/agent`'s iterate-plan-apply loop, left this
repo the same day (#182), and the facade's one other consumer
(`examples/notify`) was a demo of that same loop. `sdk/` was deleted
with it; the typed `Handler` ABI remains how mooncake's own action set
grows, through the normal spec path, same as before.

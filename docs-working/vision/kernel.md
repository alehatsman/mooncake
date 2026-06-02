# Kernel — what Mooncake actually is

> This is the source-of-truth doc for what Mooncake is. The README,
> the goals doc, the spec preambles, and every external positioning
> statement defer to this page. When the README and this page
> disagree, the README is wrong.

## In one sentence

> **Mooncake is a typed mutation kernel.** Every action it can perform
> has four typed properties — what it changes, how to undo it, what
> it costs, what authority it requires. Frontends (CLI, daemon, MCP
> server, agent loop, fleet) are renderings of the same kernel, not
> separate products.

That sentence is the test. If a proposed feature can't be described
as either *adding to the kernel* or *adding a rendering of the
kernel*, it doesn't belong.

## The kernel

The kernel is the four typed properties carried by every handler
through the `Handler` ABI (`internal/actions/handler_abi.go`):

```
              ┌───────────────────────────────────┐
              │           THE KERNEL              │
              │                                   │
              │   typed operational intent        │
              │                                   │
              │   ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐
              │   │ DIFF│  │ REV │  │COST │  │PERM │
              │   └─────┘  └─────┘  └─────┘  └─────┘
              │      ↓        ↓        ↓        ↓
              │  inspect-  reverse-  risk    auditable
              │  ability   ibility           authority
              │                                   │
              └───────────────────────────────────┘
                              │
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
       audit trail       reversibility       inspect /
       (runlog,          (per-step           rehearsal
        events)          Reverse +           (plan, --diff,
                         transactions)        --dry-run)
```

| Property | ABI | What it means | Today's coverage |
|---|---|---|---|
| **Diff** | `Differ.Diff(step)` | Typed structured delta — `Resource`, `Operation`, `Before`, `After`. Not text. Machine-readable. | Shipped on priority handlers (file, text.*, copy, template, download, pkg, git.*, os.*). |
| **Reverse** | `Reverser.Reverse(result)` | Returns the typed `Step` that undoes this one. Operates on `result.ReverseData` captured pre-mutation in apply mode. | Shipped on 11/13 priority handlers. Two explicit refusals (`os.service`, `file.unarchive`) documented as follow-ups. |
| **Cost** | `Coster.Cost(step)` | `Risk` band (1–10), `Resources` touched, `Bytes`, `Reversible` flag. Surfaced at plan time. | Shipped on every priority handler. |
| **Permissions** | `Permitter.Permissions(step)` | `Sudo`, `Network`, `RequiredBinaries`, `FilesystemWrite` paths. Surfaced at plan time. | Shipped on every priority handler. |

All four are **opt-in** sub-interfaces; default-resolvers in the
registry produce safe defaults for handlers that don't implement
them. The required `Handler` interface (`Metadata` / `Validate` /
`Run`) is unchanged — the kernel grew without breaking the contract.

That property — *evolving the kernel without breaking the contract*
— is itself part of the moat. See §5.

---

## What this enables — derived, not claimed

Every advanced capability the project ships or plans falls out of
the four properties, not from net-new infrastructure:

| Capability | Derived from |
|---|---|
| **Plan mode** (`mooncake plan`, `inspect`) | Diff per step → per-step prediction without mutation |
| **`--diff` output** | Diff with `Lines` populated → unified-diff renderer |
| **Transactions** (spec-30) | Reverse on each completed step → LIFO rollback on failure |
| **`mooncake explain <resource>`** (not yet shipped) | Audit trail (runlog) indexed by `Resource` from each Diff |
| **`mooncake rewind --to <t>`** (gated) | Reverse + persistent `ReverseData` across runs |
| **Risk scorecard** (`plan` recap, partially shipped) | Cost.Risk aggregated across the plan |
| **`mooncake plan --format graph`** (not yet shipped) | Diff payloads + implicit edges (`TriggeredBy`, `Try/Catch`, `Transaction`, `Reverse`) — already a graph; nothing renders it yet |
| **MCP `apply_approved`** (drafted) | Cost + Permissions surface lets the agent propose, human approves, Mooncake decides |
| **Drift detection** (spec-58, drafted) | Diff between current snapshot and last-applied state |
| **Rehearsal** | Plan mode is the cheap version; sandbox-host version is the expensive version. Both ride on Diff. |
| **AI-safe execution** | Permissions + Cost give the agent a contract it cannot exceed without surfacing it |

None of these required inventing a new primitive. Each is a
**rendering** of one or more of the four typed properties. That's
what "kernel" means here.

---

## What this is NOT

Each of the labels below is tempting, often used in adjacent docs,
and **wrong**. Each would compromise the architecture if
internalized.

| Label | Why it's wrong |
|---|---|
| **NOT** "modern Ansible" | Ansible is `LLM + shell` minus the LLM. No typed Diff, no Reverse, no typed Cost. Mooncake differs in mechanism, not polish. The comparison undersells what's there. |
| **NOT** "AI provisioning tool" | The kernel is agent-agnostic. The MCP server is one frontend. The kernel works identically when called by a script, a human, or a CI pipeline. Leading with "AI" miscasts the substrate as a wrapper. |
| **NOT** "cluster orchestrator" | No CRDs, no admission webhooks, no operator reconciliation loops. The fleet substrate is *peer-to-peer with a daemon*, not a control plane. Non-goal #4. |
| **NOT** "local Kubernetes" | No declarative-state ACID, no eventual-consistency reconciler, no resource generation numbers. `transaction:` is SAGA-shaped (LIFO compensation), not 2PC. Non-goal #6. |
| **NOT** "generic automation framework" | The action set is closed. No plugin marketplace. No DSL evolution. Non-goals #1 and #2. The closed surface is what makes the kernel typed end-to-end. |

These NOT-positions are load-bearing. The kernel claim only holds
because we refuse the plugin-extensibility that would break it.

---

## The comparison table

The kernel claim, made concrete:

| Tool | Diff (typed) | Reverse (typed) | Cost (typed) | Permissions (typed) |
|---|---|---|---|---|
| **Mooncake** | ✓ | ✓ | ✓ | ✓ |
| Terraform | ✓ (HCL plan) | ✓ (destroy plan) | ✗ | ~ (provider-declared) |
| Pulumi | ✓ (SDK preview) | ✓ (destroy plan) | ✗ | ~ (provider-declared) |
| Puppet | ~ (catalog diff) | ✗ | ✗ | ✗ |
| Ansible | ~ (text-only `--diff`) | ✗ | ✗ | ~ (`become:` declared, not surfaced at plan) |
| Chef | ✗ | ✗ | ✗ | ✗ |
| Salt | ~ (state-diff partial) | ✗ | ✗ | ✗ |
| `LLM + shell + hope` | ✗ | ✗ | ✗ | ✗ |

Legend: ✓ = typed at plan time per resource · ~ = declared somewhere
but not surfaced typed at plan time · ✗ = absent

**Nobody else ships all four typed.** That's the moat. It is not the
YAML. It is not the Go binary. It is not the daemon. It is the four
columns being filled in for every node in the plan.

---

## The frontends — renderings, not products

Mooncake has five frontends today. Each is a rendering of the same
kernel. None of them is the product.

```
                      ┌─────────────────────┐
                      │      KERNEL         │
                      │                     │
                      │  Compile → Plan     │
                      │  Plan → Inspection  │
                      │  Plan → Result      │
                      │  Result → Reverse   │
                      │  Resource → History │
                      └──────────┬──────────┘
                                 │
              ┌──────┬───────────┼───────────┬──────┐
              ▼      ▼           ▼           ▼      ▼
            ┌────┐ ┌────┐    ┌──────┐    ┌─────┐ ┌──────┐
            │CLI │ │MCP │    │agentd│    │agent│ │fleet │
            │    │ │svr │    │HTTP+ │    │loop │ │ orch │
            │    │ │    │    │ SSE  │    │     │ │      │
            └────┘ └────┘    └──────┘    └─────┘ └──────┘
            human  LLM       remote     iterate  multi-
            ops    via MCP   trigger    until-   peer
                                        done
```

Today these five frontends each re-implement parts of the kernel's
orchestration because there is no exported entry point to call.
`fleetApplyAction` in `cmd/fleet.go` is the kernel's "apply to many
peers" entry point trapped inside a CLI handler. The MCP server
implements parts of it differently. The agent loop implements parts
of it differently again. **That duplication is the structural debt.**

The refactor plan ([`arch-report/2026-05-15-refactoring-plan.md`](../arch-report/2026-05-15-refactoring-plan.md))
exists to draw the boundary: every cmd extraction is "expose the
kernel," not "tidy up cmd." When the plan completes, all five
frontends call the same `Apply` / `FleetApply` / `Explain` / etc.

---

## Risks to the framing

Four threats. Each named so a future reviewer can recognize them
without re-deriving.

### R1. Documentation drift

`PROGRESS.md`, the README, individual spec preambles, and ad-hoc
analysis docs all describe what Mooncake "is." They drift apart
because no single document is canonical. **This document is now
canonical.** Downstream pages defer.

The accompanying engineering fix: spec status should be **derivable
from code** (e.g., "this handler implements `Reverser`" → "spec-22
phase 5 covers this handler") rather than tracked by hand in
PROGRESS.md.

### R2. Narrative fragmentation

The repo carries seeds of seven possible products:

```
provisioning tool
agent runtime
fleet manager
reconciliation engine
GitOps-like system
cluster-management substrate
AI execution layer
```

Internally this is fine — they all sit on the same kernel.
**Externally, pick one sentence.** This document's one sentence is
the one. If a new doc / talk / pitch invents a different sentence,
that doc is wrong; the sentence here is the constraint.

### R3. Graph / ontology overreach

ChangeGraph is **execution reasoning IR**, not universal semantic
truth.

Edges (`depends_on`, `reverses`, `conflicts_with`, `triggers`) are
grounded in operational facts the kernel actually computes. They are
not LLM-inferred. They are not "what dependencies should exist
according to the model." Refuse:

- Semantic philosophy graphs
- Infinite dependency inference
- LLM-generated operational truth
- "Knowledge graphs" of any kind

A graph that the kernel cannot compute deterministically does not
ship.

### R4. Plugin / provider ecosystem explosion

Terraform's 3000 providers is the grave. What makes the kernel typed
end-to-end is that **every action carries the four properties** — *not*
that the set is small. The threat to guard against is an **untyped**
extension boundary (downloaded `.so` / WASM / opaque RPC), which can't
answer Diff/Reverse/Cost/Permissions and so collapses to `LLM + shell +
hope` for those actions.

The sanctioned extension is therefore the typed `Handler` ABI itself,
in two shapes: **built-in** (recurring need, normal spec path) and
**consumer-registered, compile-time** (a Go consumer imports Mooncake
and registers its own typed handlers — the agent-framework path, see
[`agent_framework.md`](./agent_framework.md)). Both *spread* the typed
contract; neither dilutes it. A one-off stays in `shell:`. What stays
forbidden: a runtime-loaded, versioned, untyped plugin marketplace.
The line is *compile-time + typed*, not *no extension at all*.

---

## How to use this doc

Consult before:

- Writing a new spec preamble ("what Mooncake is" / "the problem")
- Editing the README's tagline or first paragraph
- Drafting an external pitch / blog post / README in another repo
- Naming a new top-level feature
- Reviewing a proposed feature that "doesn't quite fit"

The test, every time:

```
Is the proposed thing
  (a) adding to the kernel (a new typed property, a sharper Diff
      shape, a deeper Reverse capability)?
  (b) adding a rendering of the kernel (a frontend, a UX command,
      a graph emitter, an MCP tool)?
```

If neither — re-shape the proposal until it is. If still neither,
it doesn't belong.

## How this doc changes

This is not a fixed document. The kernel can grow:

- A fifth typed property would be a meaningful claim and would
  update §1 and the comparison table.
- A demonstrated case where one of the four properties is wrong
  (e.g., Cost doesn't actually surface what operators need) would
  refine §2 or §3.

What does *not* update this doc:

- Adding handlers. The kernel doesn't grow because there are now
  68 handlers instead of 65 — the kernel is the **shape**, not the
  count.
- Adding a frontend. A second daemon transport / a TUI / a Slack
  bot doesn't change what Mooncake is; it adds a rendering.
- Strategic positioning whims. If "AI infrastructure" is hot this
  quarter and "operational substrate" is hot next quarter, the
  one-sentence framing here stays put. The category we are in does
  not depend on the category that's currently fashionable.

---

## See also

- [`goals.md`](./goals.md) — what "done" looks like, layered ring
  model. Operational complement to this doc.
- [`non_goals.md`](./non_goals.md) — the seven explicit refusals.
  Each one protects a column of the comparison table above.
- [`agent_framework.md`](./agent_framework.md) — how the kernel becomes
  a framework for building agents (compile-time typed action extension)
  without ceasing to be the kernel. Refines R4 above.
- [`good_lessons_from_other_tools.md`](./good_lessons_from_other_tools.md)
  — what 30 years of provisioning tooling got right.
- [`bad_lessons_from_other_tools.md`](./bad_lessons_from_other_tools.md)
  — the pathologies that killed the prior systems whose names appear
  in the comparison table above.
- [`arch-report/2026-05-15-arch-report.md`](../arch-report/2026-05-15-arch-report.md)
  — the structural review that grounds the kernel claim in package
  metrics.
- [`arch-report/2026-05-15-refactoring-plan.md`](../arch-report/2026-05-15-refactoring-plan.md)
  — the mechanism to make the frontends *actually* call the same
  kernel.

# Agent framework — mooncake as a substrate for building agents

> This is the source-of-truth doc for the **agent-framework** direction:
> growing the mooncake agent loop into a framework that consumers
> (moongit, `openclaw`, third parties) build agents *on top of*, by
> registering their own typed actions. It is subordinate to
> [`kernel.md`](./kernel.md) — when this page and the kernel page
> disagree about what mooncake *is*, the kernel page wins. This page
> says how the kernel becomes a framework without ceasing to be the
> kernel.

## In one sentence

> **mooncake is the typed execution substrate an agent runs against;
> the agent is a frontend that compiles intent into typed plans the
> kernel executes.** A consumer builds an agent by importing mooncake
> as a Go library, registering its own typed actions, and pointing a
> reasoning backend at the loop.

## The tension this resolves

The non-goals forbid a "plugin / provider marketplace" and call the
closed action set a feature ([`non_goals.md`](./non_goals.md) #2,
[`kernel.md`](./kernel.md) R4). The framework ask — "let moongit
register a `moongit.issue` action the agent can emit" — looks like a
direct violation. It isn't, once you name what the moat actually is.

**The moat is not that the action set is small. The moat is that every
action in a plan is typed** — it carries Diff / Reverse / Cost /
Permissions ([`kernel.md`](./kernel.md) comparison table). Terraform's
grave is not "3000 providers"; it is "3000 providers with version skew
and no uniform typed contract." A custom action that *implements the
four-property Handler ABI* does not dilute the moat — it extends it.
Now even "create a moongit issue" is permission-gated, cost-scored, and
reversible (its `Reverse()` deletes the issue it created).

What *would* break the moat is an **untyped extension boundary** — a
downloaded `.so`, an opaque RPC to "do stuff," a WASM blob — because
the four properties cannot be enforced across it, and those actions
collapse back to `LLM + shell + hope`.

So the refinement (see the [`non_goals.md`](./non_goals.md) #2 amendment):

> "Closed" means **the typing contract is mandatory — there is no
> untyped middle layer.** It does *not* mean the count is fixed.

## The extension model — decided

**Go, compile-time, typed `Handler` ABI only.** A consumer imports
mooncake as a library, registers typed handlers into a registry, and
compiles their own agent binary:

```go
reg := mooncake.NewRegistry()
mooncake.RegisterBuiltins(reg)
reg.Register(moongit.IssueHandler{})   // Metadata/Validate/Run + Permissions/Cost (+ Reverse where honest)
agent.RunLoop(ctx, agent.Options{Registry: reg, Backend: claude, ...})
```

| Allowed | Forbidden |
|---|---|
| Custom actions implementing the typed `Handler` ABI, registered in-process at compile time | Runtime-loaded plugins (`.so` / WASM / subprocess marketplace) |
| Consumers compiling their own agent binary off the mooncake library | A versioned third-party provider ecosystem with skew |
| Promoting recurring `shell` patterns into typed built-in actions | An *untyped* custom action — a "do-anything" handler |

There is exactly one way to add an action: **implement the ABI, or use
the already-governed `shell` / `cmd` escape hatch.** No third option.
That single rule is the whole moat, preserved. (`shell`/`cmd` stay the
honest pressure valve — logged, policy-deniable, explicitly *not*
reversible per [`non_goals.md`](./non_goals.md) #6.)

The cost of this choice is that extenders must write Go and rebuild.
That is the price of zero version skew and a boundary the type system
enforces. A language-agnostic typed-RPC boundary is a *possible future*
(the protocol would still have to answer Diff/Reverse/Cost/Permissions
over the wire), but it is explicitly deferred — compile-time first.

## The layered cognitive architecture

The agent is four layers. The top two are **swappable models**; the
bottom two are **mooncake's deterministic core + its extension point**.

```
┌─────────────────────────────────────────────────────────────────┐
│ L4  REASONING / PLANNER     (DeepSeek R1, Claude — swappable)     │
│     goal → strategy, decomposition, "what next", "are we done"    │
├─────────────────────────────────────────────────────────────────┤
│ L3  GROUNDING / COMPILER    (small/local/fine-tuned, or templates)│
│     intent → typed mooncake-JSON, grounded in the registry schema │
├─────────────────────────────────────────────────────────────────┤
│ L2  EXECUTION KERNEL        (mooncake, deterministic)             │
│     typed plan → mutation: Diff/Reverse/Cost/Permissions, txn     │
├─────────────────────────────────────────────────────────────────┤
│ L1  ACTION REGISTRY         (built-ins + consumer typed handlers) │
│     file/http/shell/… + moongit.issue, openclaw.*, …              │
└─────────────────────────────────────────────────────────────────┘
```

The load-bearing idea:

> **The registry's schema is the API between cognition (L3/L4) and
> execution (L1/L2).**

Register a typed handler into L1 → L3's target vocabulary grows
automatically → L4 reasons in natural language and never needs to know
mooncake JSON. Cognition becomes a *rendering of the kernel* (the
[`kernel.md`](./kernel.md) framing); the deterministic typed core does
not move.

Why split L3 from L4:

- **L4 (expensive reasoning) decides *what*. L3 (cheap, local,
  fine-tunable, or deterministic templates for common intents) decides
  *how to say it in typed JSON*.** Big-model tokens go to judgment, not
  syntax.
- L3 is a *constrained* generation problem — emit valid JSON against a
  known vocabulary — which is far more tractable to run **offline** on a
  small model, and **verifiable**: the plan validates against the
  registry schema or it doesn't, and the loop already feeds structured
  validation/execution feedback back to the next turn.

Today the loop is effectively single-model (intent ≈ plan in one shot).
Formalizing the L3/L4 boundary is a follow-on, not a prerequisite.

## openclaw and the fully-offline general-purpose agent

`openclaw` is **not** a competitor to Claude Code / Cursor — it is a
*reference agent built on the framework*: L4 reasoning + L3 grounding +
mooncake L1/L2 + a general action pack. Its differentiator is the one
this project has had since day one: **the only general-purpose agent
whose every mutation is typed, reversible, and audited.**

A general agent is the hardest stress test of the typed-only rule —
"do anything" pressures toward an untyped escape hatch. The answer is
the discipline above: typed actions for the recurring 90%, `shell` for
the visible/gate-able long tail, and a standing practice of *promoting
recurring shell into typed handlers*. Custom actions must implement at
least `Permissions` + `Cost`, with honest reversibility.

"Fully offline" is a **backend choice, not a rearchitecture**: L3/L4
point at a local runtime (ollama / llama.cpp / vLLM) through the
existing `llm.Client` OpenAI-shape provider, and `dex` supplies local
code/knowledge intel as read-only context. No cloud in the loop.

## The unlock — registry-as-dependency

The framework move depends on one refactor, and the code shows exactly
where it bites. Today the registry is a **global** and the agent's
action vocabulary comes from a **static generated file**:

- the executor resolves handlers via `actions.Get(...)` → the package
  global (`internal/executor/scope.go`, `internal/actions/registry.go`'s
  `globalRegistry`);
- the planner's vocabulary comes from an embedded `schema.json`
  (`internal/agent/prompt_schema.go` `BuildSchemaChunk`), generated from
  `actions.List()` at build time (`internal/schemagen/generator.go`).

The framework needs:

1. **Inject `*Registry`** through the executor and the agent loop
   (`agent.Options{Registry: ...}`) instead of reaching for the global.
   The registry is already an instance type (`NewRegistry()`); the work
   is threading it, not inventing it.
2. **Live-registry-derived schema chunk** — `BuildSchemaChunk(reg)`
   reads the injected registry, not the embedded file, so a registered
   `moongit.issue` surfaces to the planner with no prompt edits.
3. Keep the global as the CLI's default-populated convenience
   (back-compat); make injection the framework path.

This is the same dependency-injection discipline as the ctx work
(#101–#103) and is squarely the [`kernel.md`](./kernel.md) "expose the
kernel" refactor — every frontend (CLI, MCP, agentd, agent loop, fleet)
eventually calling the same parameterized core. The agent framework is
the forcing function that makes it concrete.

## Repo & module boundary

The end state is a **separate `mooncake-agent` module/repo** that imports
`mooncake` (the kernel) as a versioned dependency. The arguments are
decisive: the dependency arrow runs one way (framework → kernel, never
back); the kernel's "single lean Go binary" story
([`goals.md`](./goals.md)) must not absorb LLM-SDK weight; models churn
monthly while the kernel should be boring and stable; and it makes the
[`kernel.md`](./kernel.md) "frontends are renderings, not products"
framing physical.

A forcing fact: everything the framework needs is under **`internal/`**
today (`internal/agent`, `internal/actions`, `internal/executor`), and
Go forbids importing `internal/` from another module. So no consumer can
import the agent as a library *at all* yet — same-repo or not.
Framework-ization is gated on promoting the kernel's public surface out
of `internal/`, **independent of any repo split.** The split is a
*consequence* of exporting the API, not an alternative to it.

**Decision: export-first, split when stable.** Split *last*, not first —
splitting during the ongoing kernel-API refactor would tax every change
with a two-repo, version-bump dance while the seam is still moving.

- **Phase 1 (in one Go module):** promote the kernel's public surface
  out of `internal/` (exported `actions` / `executor` entry point /
  `agent`), dependency-inject the registry, and **dogfood the API** —
  make the existing CLI / MCP / fleet frontends call the exported
  surface. This is the [`kernel.md`](./kernel.md) "expose the kernel"
  refactor; it's needed regardless of the framework.
- **Phase 2 (when the API has settled):** extract `mooncake-agent` into
  its own repo/module importing `mooncake` as a versioned dependency.
  By then the boundary is real and exercised, so the lift is a clean
  `git mv` + `go.mod`, not archaeology.

Avoid the multi-module-in-one-repo middle option — Go makes it fiddly.
One module with clean exported packages until the repo split.

## Sequencing

1. **Registry-as-dependency + public surface** (Phase 1 above): inject
   `*Registry` through executor + agent loop, make `BuildSchemaChunk`
   live-registry-derived, and move the kernel/agent API out of
   `internal/` into importable packages.
2. **Public Go SDK surface** — a stable `mooncake/agent` +
   `mooncake/actions` API so a consumer compiles their own agent binary.
   Land **spec-31 / issue #5** (`notify.*` tier-2 pack) as the first
   custom-action pack proving the shape; `moongit.*` as the lighthouse
   consumer.
3. **Extract `mooncake-agent`** (Phase 2 above) once the API is proven by
   the in-repo frontends + moongit.
4. **L3/L4 split** — introduce an explicit grounding step so a cheap or
   local model can own intent→JSON while a big model owns strategy.
5. **Offline profile** — local backends + dex-as-knowledge wired
   end-to-end; `openclaw` as the reference build.

Each step is independently useful and none requires betraying the
kernel — they require evolving one sentence in
[`non_goals.md`](./non_goals.md) and doing the DI refactor the arch
report already wants.

## What does *not* change

- The four typed properties and the comparison table
  ([`kernel.md`](./kernel.md)) — the framework *spreads* them to custom
  actions, it does not weaken them.
- The honest SAGA model ([`non_goals.md`](./non_goals.md) #6) — custom
  actions declare reversibility truthfully; `shell` stays irreversible.
- No control-plane sprawl, no DSL evolution, no git-coupled audit
  ([`non_goals.md`](./non_goals.md) #1, #3, #4) — a framework for
  building agents is not a license to grow any of those.

## See also

- [`kernel.md`](./kernel.md) — what mooncake *is*; this page defers to it.
- [`non_goals.md`](./non_goals.md) — #2 amended for the typed-extension line.
- [`goals.md`](./goals.md) — the "Docker for AI agents" wedge (§4.2) this
  direction operationalizes.
- [`sharing_and_modules.md`](./sharing_and_modules.md) — the *config*
  sharing story (Git-native modules); distinct from *action* extension,
  which is this page.

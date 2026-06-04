---
id: sdk
status: draft
owners: [aleh]
covers:
  - sdk/**
---

# SDK

## Intent

The `sdk` package (`package mooncake`) is the one public surface an external
consumer imports to build on the kernel — it never reaches into `internal/`.
It re-exports the typed-action ABI, the agent loop, the no-LLM execution
entries, the event/observability contract, and the custom-action authoring
helpers as a stable facade, so a downstream agent (openclaw) or a tool-call
driver (Claude Code over the MCP server, model id `mooncake-agent` over
`mooncake agent run`) compiles against versioned symbols rather than internal
churn. The expansion goal is a **coding-execution surface**: the same facade
exposes read/search/edit/exec primitives behind a swappable backend, so a
coding agent runs its everyday tool calls on the mooncake engine — typed,
gated, reversible, audited — with native-feeling latency.

## Behavior

- WHERE a consumer imports the facade, every symbol it needs is re-exported
  from `sdk` (action ABI, `Registry`, `Step`, `Result`, event types, `Policy`,
  agent options) — importing `internal/**` is never required (#120/#121).
- WHEN a consumer drives the agent loop, `RunLoop` / `Run` accept a
  `RunOptions` whose injected `Registry` extends the planner vocabulary and
  executor handlers and whose injected `LLMClient` bypasses provider
  resolution, so a custom-action, custom-backend run needs no global mutation
  (#105/#106).
- WHEN a consumer needs the planner's action vocabulary,
  `BuildSchemaChunkForRegistry` derives it from a live registry rather than the
  embedded `schema.json`.
- WHEN a consumer runs a typed plan with no LLM, `Apply(ctx, ApplyOptions)`
  executes it through the kernel funnel and returns the typed `ApplyResult`
  (per-step outcomes, audit event tail, summary); `OutputFormat` defaults to
  quiet and `LogLevel` to error so an embedded run is silent and `Subscribers`
  carry the signal (#122).
- WHEN a consumer previews a plan, `Plan(ctx, PlanOptions)` compiles and runs
  it in non-mutating mode, returning per-step `WouldChange` + structural Diff +
  Cost with no side effects; inline `Vars` overlay `VarsFiles` (#122).
- WHERE a run enforces a contract, `ApplyOptions.Policy` is the
  permissions-as-contract gate (#11) checked at preflight, and
  `ApplyOptions.Subscribers` receive every kernel event in order under the
  run's lifecycle (the caller must not close them).
- WHEN a consumer authors a custom typed action, `NewTestContext` plus the
  `With*` option helpers and `AssertHandlerConformance` exercise a handler's
  `Validate`/`Run` and its ABI conformance without standing up a real run
  (#123).
- WHEN a coding driver dispatches a single synthesized step, the facade MUST
  accept an in-memory plan — `ApplyConfig` / `ApplySteps` / `ApplyBytes` — so an
  edit or exec runs without writing a temp YAML, sharing the `Apply` policy,
  registry, and subscriber plumbing (planned, not built).
- WHEN a coding driver reads context, the facade MUST expose direct
  `Read` / `Grep` / `Glob` query helpers that return file content and matches
  WITHOUT compiling a plan or invoking the executor, so the read path stays at
  native latency — reads are queries, not convergent mutations (planned, not
  built).
- WHEN a coding driver mutates the workspace, `Edit` / `Write` / `Exec` MUST
  compile to a one-step `Apply` so the mutation carries typed Diff, Reverse
  capture, the `Policy` gate, and a run-log audit event, with `Plan` available
  as the pre-flight diff/cost preview (planned, not built).
- WHERE the execution backend is selected, a `CodingBackend` interface
  (`Read`/`Grep`/`Glob`/`Edit`/`Write`/`Exec`) MUST define the seam with a
  Mooncake-kernel impl, a Native pass-through impl (for A/B and migration), and
  a Remote impl over the daemon HTTP transport; the MCP server is this
  interface's wire form, so swapping backends needs no prompt change (planned,
  not built).
- WHEN a resident driver issues many calls against one checkout, an optional
  `Session` handle MAY bind a repo root + `Policy` + `Subscribers` once so
  per-call options need not be re-threaded (planned, not built).

## Non-goals

- The agent loop internals, provider resolution, and confirm/control lifecycle
  — `internal/agent/**`, owned by the agent-framework spec; the facade only
  re-exports them.
- The executor, action handlers, transaction rollback, and the typed ABI
  semantics — owned by the kernel specs; the SDK is a stable surface over them.
- The MCP server's JSON-RPC wire protocol and tool registry —
  `internal/mcp/**`, owned by the mcp-server spec; the coding-execution tools
  there are the wire form of this package's `CodingBackend`, not redefinitions.
- The agent daemon run store / HTTP lifecycle — `internal/agentd/**`.
- Runtime plugins / WASM / shared-object loading — extension is Go
  compile-time only (`reg.Register` + recompile), per the agent-framework
  non-goals.

## Checklist

- [x] Facade re-exports the action ABI, registry, event, and agent surfaces;
      no `internal/**` import required (#120/#121)
- [x] DI through the facade: injected `Registry` + `LLMClient` on `RunOptions`
      (#105/#106)
- [x] `BuildSchemaChunkForRegistry` — planner vocab from a live registry
- [x] `Apply` no-LLM typed execution → `ApplyResult` (#122)
- [x] `Plan` dry-run preview (would-change + diff + cost, no side effects) (#122)
- [x] `ApplyOptions.Policy` gate (#11) + ordered `Subscribers` channel
- [x] Authoring/testing: `NewTestContext`, `With*`, `AssertHandlerConformance`
      (#123)
- [ ] Inline execution input: `ApplyConfig` / `ApplySteps` / `ApplyBytes`
      (planned — follow-up issue)
- [ ] Read surface: `Read` / `Grep` / `Glob` direct query helpers, no executor
      (planned — follow-up issue)
- [ ] Single-step mutation helpers: `Edit` / `Write` / `Exec` over one-step
      `Apply` with `Plan` pre-flight (planned — follow-up issue)
- [ ] `CodingBackend` interface + Mooncake / Native / Remote impls; MCP server
      as its wire form (planned — follow-up issue)
- [ ] Optional `Session` handle (repo root + `Policy` + `Subscribers`)
      (planned — follow-up issue)

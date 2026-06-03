---
id: actions
status: draft
owners: [aleh]
covers:
  - "internal/actions/*.go"
  - "internal/register/*.go"
---

# Action Catalog & Handler ABI

## Intent

Actions are the unit of work mooncake performs: each maps one YAML verb (`shell`,
`file`, `git.clone`, `os.user`, …) to a typed, idempotent, optionally reversible
system mutation. This spec defines the *contract every action obeys* — the handler
interface, the execution environment it is handed, and how handlers are registered
and dispatched — independent of any single action's behavior.

## Behavior

- WHERE an action is implemented, it lives as a package under `internal/actions/<name>`
  exposing a `Handler` and registering itself in `init()` via `actions.Register`.
- WHEN a handler is registered, it MUST satisfy the minimal `Handler` interface:
  `Metadata() ActionMetadata`, `Validate(*config.Step) error`, and
  `Run(Context, *config.Step) (Result, error)` (`internal/actions/handler.go:144`).
- WHEN `Run` is invoked it consults `ctx.Mode()`: `ModeApply` performs real
  side effects; `ModePlan` only inspects target state and predicts change, returning
  a `Result` with `WouldChange`/`Reason`/`Checkable` and no mutation
  (`internal/actions/mode.go:14`). The single `Run` method replaced the legacy
  Execute/DryRun/Check trio.
- WHERE a handler must mutate the filesystem, it routes writes through
  `ctx.Effects()` (a `Performer`) so plan and apply share one predicate, rather than
  calling `os.*` directly (`internal/actions/interfaces.go:108`).
- WHERE a handler shells out with possible privilege escalation, it MUST call
  `ctx.Privileged().Run(...)` and MUST NOT read `step.AsUser` itself — the
  primitive decides the sudo wrap (`internal/actions/interfaces.go:116`).
- WHILE running, a handler MUST thread `ctx.Ctx()` into every external call so the
  apply observes SIGINT / fleet kill / MCP shutdown / per-step timeout cancellation
  (`internal/actions/interfaces.go:141`).
- WHERE a handler can predict, capture, undo, or describe its effect, it opts into
  the spec-22 sub-interfaces — `Differ`, `Reverser`, `Coster`, `Permitter` — each
  optional with a safe registry default when unimplemented
  (`internal/actions/handler_abi.go:1`).
- IF a handler implements `Reverser`, `Reverse(ctx, step, result)` returns an inverse
  `*config.Step` (or `(nil,nil)` for a noop, or `(nil,error)` to declare itself
  irreversible), consumed by transaction rollback (`internal/actions/handler_abi.go:209`).
- IF a handler implements `RawRunner`, the executor owns its retry loop and
  `changed_when`/`failed_when` overrides; `RunRaw` executes exactly one attempt and
  never applies overrides itself (`internal/actions/interfaces.go:278`).
- IF a handler also implements `Retryable`, the executor consults
  `IsRetryable(result, err, step)` per attempt instead of retrying every non-nil error
  (`internal/actions/interfaces.go:303`).
- WHEN a step is dispatched, the executor derives the verb via
  `step.DetermineActionType()` and looks the handler up in the registry; an injected
  registry is honored, else the process-wide `GlobalRegistry()`
  (`internal/actions/registry.go:159`, `internal/executor/executor.go:444`).
- WHERE the catalog is assembled, `internal/register/register.go` blank-imports every
  action package so each `init()` registers before `main()` runs; the catalog spans
  file/text-patch/git/os/observe/package/wait/container/read/repo/runtime/network/
  windows families.
- WHEN `Registry.List()` is called, the four `Implements*` capability bools are
  derived from live interface satisfaction (`IsDiffer`/`IsCoster`/`IsReverser`/
  `IsPermitter`), so the reported capabilities cannot drift from reality
  (`internal/actions/registry.go:85`).
- WHEN a framework consumer needs the built-ins plus its own handlers, it clones the
  global registry or calls `RegisterBuiltins(dst)` rather than mutating the global
  (`internal/actions/registry.go:140`).

## Non-goals

- The `executor.Result` envelope itself (`Operation` verbs, recap counters,
  ReverseData wire encoding) — owned by the execution-engine spec; this spec only
  references it.
- Step dispatch orchestration, retry backoff timing, idempotency/skip gating, and
  transaction sequencing — execution-engine concerns.
- Per-action behavior, YAML schema, and option semantics of individual verbs.
- Template rendering, expression evaluation, and facts — owned by
  `specs/templating-and-facts.md`.

## Checklist

- [x] Minimal `Handler` interface: Metadata / Validate / Run.
- [x] `Context` execution environment (Template/Evaluator/Logger/Variables/Effects/
      Privileged/Ctx/Mode).
- [x] `Mode` (ModeApply/ModePlan) collapsing legacy Execute/DryRun/Check.
- [x] Spec-22 opt-in ABI: Differ / Reverser / Coster / Permitter with registry defaults.
- [x] `RawRunner` + `Retryable` opt-in retry/override delegation.
- [x] Self-registration via `init()` + global/injected `Registry`, dispatch via
      `DetermineActionType`.
- [x] Capability bools derived from live interface satisfaction in `List()`.
- [x] Breadth of catalog wired in `internal/register`.
- [ ] `internal/actions/doc.go` is stale: it still documents the legacy
      Execute/DryRun migration flow and `*executor.ExecutionContext` signatures
      rather than the current Run/Context ABI — should be rewritten to match
      `handler.go`.

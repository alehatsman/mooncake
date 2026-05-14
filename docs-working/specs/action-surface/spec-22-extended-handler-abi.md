# Spec 22: Extended Handler ABI — Diff / Reverse / Cost / Permissions

**Status:** 🟡 In progress. Phases 1-3 shipped. All 11 priority handlers (file family, text family, pkg, os.service) declare `Permissions()` and the executor `dispatchRunner` preflights Sudo + RequiredBinaries. Phases 4-8 (Diff, Reverse, Cost, planner/MCP wiring, docs) still draft.
**Epic:** E9 Modern Action Surface — bucket E9.1
**Effort:** M (1–2 weeks)
**Value:** Foundational. Unblocks `transaction:` groups (spec 30), the
agent-safety pitch, policy gating, and structural diffs for richer UIs.

**Design principles:** `docs-working/action-design-principles.md`

---

## Problem

Today's Handler interface (`internal/actions/handler.go`) is the spec-16
collapsed shape:

```go
type Handler interface {
    Metadata() ActionMetadata
    Validate(step *config.Step) error
    Run(ctx Context, step *config.Step) (Result, error)
}
```

`Run` returns a `Result` whose semantics differ between `ctx.Mode() ==
ModePlan` and `ModeApply`. It's enough for *executing*, but not for the
three things spec-21's vision depends on:

1. **Structural diffs** for UIs and the agent SDK. Today plan output is a
   bag of strings ("would create directory", "would write 14 lines");
   there's no machine-readable delta a UI or LLM can branch on without
   parsing prose.
2. **Reverse / rollback.** No way to ask an action "given that you just
   applied X, what's the Step that undoes it?". Without this,
   `transaction:` (spec 30) is impossible, and the "agent did something
   dumb, undo it" story is marketing.
3. **Cost / risk classification.** Agents and policy engines need a
   pre-execution signal: "this would touch 412 files / requires sudo /
   talks to the network." Today there's no contract for surfacing that.
4. **Permission declaration.** What does this action need to run? Root?
   Network egress? A specific binary on PATH? The daemon and policy
   engine need this *before* execution, not via runtime failure.

---

## Goals

- **G1** Define four new optional Handler methods: `Diff`, `Reverse`,
  `Cost`, `Permissions`. Optional means each handler opts in; missing
  methods get safe defaults.
- **G2** Standard types for each: `Diff`, `CostEstimate`, `PermissionSet`,
  and the existing `Step` for `Reverse`.
- **G3** Wire `Diff` through `mooncake plan` output (JSON mode emits the
  structural delta; text mode keeps current rendering).
- **G4** Wire `Permissions` through executor's preflight: a step that
  declares `requires: [sudo]` fails fast with a clear error if running
  as non-root and `as_user` isn't set.
- **G5** Wire `Cost` into the run-recap and JSON plan output. Used later
  by the policy/agent gating layer; for now: informational.
- **G6** Document the migration path for existing handlers — opt-in,
  non-breaking.

**Out of scope (separate specs):**

- `transaction:` block semantics (spec 30 — depends on `Reverse`).
- Policy DSL that consumes `Permissions` (later epic).
- Agent SDK consumption of `Diff` (depends on this spec landing).

---

## Design

### New types (in `internal/actions/handler.go`)

```go
// Diff is a machine-readable structural delta of what this Step would change.
type Diff struct {
    // Resource identifies the thing being changed: path, service name,
    // pkg name, k8s object ref, …
    Resource ResourceRef

    // Operation is a coarse classifier:
    //   create | update | delete | noop
    Operation Operation

    // Before / After are typed payloads (action-specific). For file.write:
    // both are FileSnapshot; for pkg: both are PkgSnapshot; …
    // nil on the appropriate side for create/delete.
    Before any
    After  any

    // Lines (optional) is a unified-diff style line-level breakdown for
    // text/file actions. Empty for non-textual actions.
    Lines []DiffLine
}

// Reverse returns a Step that, when applied, undoes the side effect this
// Step would produce. nil + nil = no-op (already a noop, or fully
// reversible-by-omission). nil + error = action declares itself
// irreversible.
//
// Callers should treat nil-without-error as "no reverse needed" and nil-
// with-error as "rollback would require manual intervention" — different
// upstream UX.

// CostEstimate is a coarse, pre-execution signal of blast radius.
type CostEstimate struct {
    // Resources is the count of distinct things touched (files, packages,
    // service units, …). Lower bound — actions may touch more if dynamic.
    Resources int

    // Bytes is an order-of-magnitude estimate of bytes written / mutated.
    Bytes int64

    // Reversible reports whether Reverse() would return a non-nil Step.
    Reversible bool

    // Risk is a 1..10 informational classifier.
    //   1–3:  safe (read-only assertions, idempotent writes to scratch)
    //   4–6:  routine (config writes, package installs)
    //   7–9:  high impact (service restarts, kernel param changes)
    //   10:   destructive (deletes, drops, rm -rf)
    Risk int
}

// PermissionSet declares what an action needs to execute. Daemon / policy
// engine consume this preflight.
type PermissionSet struct {
    // Sudo: action requires elevated privileges.
    Sudo bool

    // Network: action makes outbound network calls.
    Network bool

    // RequiredBinaries: programs that must exist on PATH (e.g.
    // ["systemctl"], ["git"]).
    RequiredBinaries []string

    // FilesystemWrite: declared write paths (glob ok). "*" = anywhere.
    FilesystemWrite []string

    // Notes: human-readable extras for surfacing in UIs.
    Notes []string
}
```

### Interface extension

Keep `Handler` minimal; introduce **optional** sub-interfaces so existing
handlers don't break:

```go
type Differ interface {
    Diff(ctx Context, step *config.Step) (Diff, error)
}

type Reverser interface {
    Reverse(ctx Context, step *config.Step, result Result) (*config.Step, error)
}

type Coster interface {
    Cost(ctx Context, step *config.Step) (CostEstimate, error)
}

type Permitter interface {
    Permissions(step *config.Step) PermissionSet
}
```

Callers type-assert on each. Missing implementation = sensible default:

| Missing | Default |
|---|---|
| `Differ`  | derived from `Run(ctx, ModePlan)` Result → coarse Diff (Operation only, no Before/After) |
| `Reverser` | returns `nil, nil` → "no reverse available" |
| `Coster`  | `{Risk: 5, Reversible: <whether Reverser is implemented>}` |
| `Permitter` | empty `PermissionSet{}` |

### Implementation priorities (which handlers gain which methods)

| Handler | Differ | Reverser | Coster | Permitter |
|---|---|---|---|---|
| `file.write` | ✓ (line diff) | ✓ (snapshot-based) | ✓ | ✓ (Sudo if path∈/etc/, FilesystemWrite) |
| `file.template` | ✓ | ✓ | ✓ | ✓ |
| `file.copy` / `download` / `unarchive` | ✓ | ✓ | ✓ | ✓ |
| `text.replace`/`insert`/`delete_range`/`patch` | ✓ (line diff) | ✓ (snapshot) | ✓ | ✓ |
| `pkg` | ✓ (pkg list before/after) | ✓ (install→remove, remove→install) | ✓ (count of pkgs) | ✓ (Sudo, Network) |
| `os.service` | ✓ (state before/after) | ✓ (running→stopped etc.) | ✓ | ✓ (Sudo) |
| `shell` | ✗ (opaque) | ✗ (irreversible by default) | ✗ (default Risk=8) | ✗ (default) |
| `cmd` | ✗ | ✗ | ✗ | ✗ |
| `assert` | n/a (no side effects) | n/a | `{Risk: 1}` | empty |
| `use` (preset) | n/a (composed at plan time) | n/a | n/a | n/a |

For shell/cmd: explicit `reversible: false` declaration via the action's
metadata is enough; no need for a `Reverse` impl that always errors.

### Snapshot integration

`Reverse` for filesystem-touching actions piggybacks on the existing
`internal/snapshot/` subsystem: snapshot the resource pre-Apply, generate
the inverse Step from the snapshot data on Reverse call.

For non-filesystem actions (pkg, service): Reverse is computed from the
"opposite operation" — pkg install ↔ remove, service start ↔ stop.

### Where Diff surfaces

1. **`mooncake plan --format json`** — each plan step gains a `diff:`
   field with the structured shape.
2. **`mooncake plan` text mode** — unchanged for the common case (line
   diffs continue to render as text). Add `--diff structural` for the
   machine-readable form to stdout.
3. **MCP tool calls** — the `plan` tool returns Diff per step. Agent SDK
   consumers see `step.diff.operation`, `step.diff.lines`, etc.

### Where Permissions surfaces

1. **Executor preflight** — before running a step, check `Permissions()`:
   - `Sudo: true` + non-root + empty `AsUser`: fail with clear error.
   - `RequiredBinaries`: each must resolve via `exec.LookPath`; missing →
     fail with which binary and which action.
   - `Network: true`: informational today; gated by policy layer later.
2. **`mooncake plan`** — output includes a per-step `requires:` line
   summarizing the PermissionSet.

### Where Cost surfaces

1. **Run recap** — `RECAP changed=12 ok=61 risk=4.2 resources=314` —
   averaged risk + total resources across the plan.
2. **JSON plan output** — `cost:` per step.

---

## Key files

| File | Change |
|---|---|
| `internal/actions/handler.go` | New types (`Diff`, `CostEstimate`, `PermissionSet`, `ResourceRef`, `Operation`, `DiffLine`); new sub-interfaces (`Differ`, `Reverser`, `Coster`, `Permitter`). |
| `internal/actions/registry.go` | Helpers: `GetDiffer(name)`, `GetReverser(name)`, etc. that type-assert and return safe defaults. |
| `internal/actions/<each>/handler.go` | Per-handler implementation of the appropriate sub-interfaces (see priorities table). |
| `internal/executor/executor.go` | Preflight check via `Permissions()`; cost aggregation for recap. |
| `internal/plan/planner.go` | Wire `Diff` into `mooncake plan` output via the existing planner pipeline. |
| `cmd/plan.go` (or `cmd/mooncake.go`) | New `--diff structural` flag. |
| `internal/config/schema.json` | Regenerate — adds `requires:` to step shape (read-only field surfaced at plan time, not user-set). |
| `internal/mcp/tools.go` | MCP `plan` tool returns Diff per step. |

---

## Tasks (phased)

1. **Phase 1** ✅ — types and interfaces (`Diff`, `CostEstimate`,
   `PermissionSet`, `ResourceRef`, `Operation`, `DiffLine`; `Differ`,
   `Reverser`, `Coster`, `Permitter`). Landed in
   `internal/actions/handler_abi.go`. No behavior change.
2. **Phase 2** ✅ — registry helpers + safe defaults (`ResolveDiffer`,
   `ResolveReverser`, `ResolveCoster`, `ResolvePermitter`, plus the
   `Is*` capability checks). Landed in `internal/actions/registry_abi.go`
   + 8 unit tests in `handler_abi_test.go` proving every default and the
   "native implementation wins" contract.
3. **Phase 3** ✅ — `Permissions()` per-handler + executor preflight.
   - ✅ Executor preflight wired into `dispatchRunner` →
     `internal/executor/preflight.go`. Fails fast on Sudo+non-root
     +no-AsUser; checks RequiredBinaries via `exec.LookPath`;
     Network is informational only.
   - ✅ Shared `actions.PathNeedsSudo` + `actions.SystemPathPrefixes`
     in `internal/actions/handler_abi.go` so every file-family
     handler shares one canonical list of system roots.
   - ✅ File family: `file.write`, `file.template`, `file.copy`,
     `file.download` (with Network=true), `file.unarchive`. Each
     declares Sudo for system paths + FilesystemWrite=[Dest].
   - ✅ Text family: `text.replace`, `text.insert`,
     `text.delete_range`, `text.patch`. Each declares Sudo for
     system paths + FilesystemWrite=[Path]. text.patch's PatchFile
     is correctly excluded from the write set (it's a read-only
     input on the controller's FS).
   - ✅ Categorical handlers: `pkg` always declares Sudo+Network
     (every supported manager mutates system state and reaches
     remote repos; FilesystemWrite empty because installs go to
     system-managed paths). `os.service` always declares Sudo
     (every backend — systemd, launchd, Windows SCM — needs root).
   - ✅ 11/11 priority handlers done. Phase complete.
4. **Phase 4** — implement `Diff()` on the file/text/pkg/service
   handlers. Wire into `mooncake plan --format json` and `--diff
   structural`. Snapshot tests for diff output stability.
5. **Phase 5** — implement `Reverse()` on the same handlers. Snapshot-
   integration tests: apply then reverse should restore prior state.
6. **Phase 6** — implement `Cost()` on the same handlers. Surface in
   recap + JSON.
7. **Phase 7** — MCP server exposes Diff/Cost/Permissions in plan tool
   output. Update agent prompt to mention these are available.
8. **Phase 8** — docs (`docs-next/guide/config/actions.md` updated with
   each action's PermissionSet shape; `docs-next/api/handler-abi.md` new
   page describing the optional sub-interfaces).

---

## Acceptance criteria

- `go build ./...`, `go vet ./...`, `make lint`, `make test-race` all
  clean.
- `mooncake plan --format json examples/hello-world/config.yml` emits
  per-step `diff: { operation: "create"|"update"|..., before: ..., after: ... }`.
- `mooncake plan` on a step using `as_user: root` without sudo perms
  surfaces a clear preflight error mentioning the missing permission.
- Snapshot test: applying a `file.write` then `reverse` returns the file
  to its pre-apply content (byte-identical).
- `Cost` for a 100-package `pkg.install` reports `Resources >= 100`.
- New `docs-next/api/handler-abi.md` page documents the four
  sub-interfaces and how to implement them.

---

## Open questions

1. **Diff shape for actions that touch external state** (DNS records,
   k8s objects). Spec deliberately scopes Diff to filesystem + local
   process state; cloud-aware Diff is a later spec when those actions
   land (Tier-2).
2. **Idempotency of `Reverse`.** Should reversing twice be safe? Probably
   yes (apply Reverse to a system that's already pre-state should be a
   no-op). Codify in tests.
3. **`CostEstimate.Risk` calibration** — purely subjective today. Later,
   we could fit a model from runlogs. For v1: keep it as documented bands.
4. **Should `Permissions.FilesystemWrite` paths be glob or literal?**
   Probably glob (`/etc/**`). Decide before exposing to a policy DSL.
5. **Backwards-compat policy for the new interfaces.** Are they "v2.1
   experimental" until first plugin author writes against them? Probably
   yes — gives us one release to iterate.

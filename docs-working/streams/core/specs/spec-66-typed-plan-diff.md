# Spec 66: Typed plan diffs (`plan --diff` for every action category)

**Status:** In progress — waves 1–5 shipped (`cc6042d0`→`9c94e315`); waves 6–8 pending
**Stream:** core
**Promotes:** [`streams/core/proposals/proposal-04-typed-plan-diff.md`](../proposals/proposal-04-typed-plan-diff.md)
**Effort:** M (~5–7 days, rolled out incrementally — `core/diff` plumbing,
then one PR per action category)
**Prerequisites:** none — kernel-side `actions.Diff` ABI already shipped
in spec-22 phase 4; 26 handlers already produce typed `Diff` payloads
that are computed but not rendered.

---

## Problem

`mooncake plan --diff` produces an excellent diff for **file content
changes** today (`cmd/mooncake.go:648` calls `extractUnifiedDiff(ins.Detail)`),
and an opaque "would create" / "would update" placeholder for
**every other action category** — even though 42 sites in the tree
already implement the `actions.Differ` interface and produce typed
`Diff` payloads.

The plan output for a typical mixed plan today:

```
↑ write /etc/foo.conf
    --- /etc/foo.conf
    +++ /etc/foo.conf (proposed)
    @@ -1,1 +1,1 @@
    -host = localhost
    +host = db.production.com

↑ install nginx           would install
↑ create user alice       would create
↑ open firewall port 8080 would update
↑ enable nginx service    would update
```

For an LLM agent reviewing a plan before approval — exactly the
`vision/kernel.md` "agent proposes, Mooncake decides, human approves"
story — "would update firewall" is useless. The kernel already
computes *what* would change; the planner just doesn't surface it.

The gap is **the renderer**, not the kernel.

## Goals

1. **Plan output is typed end-to-end.** Every step that has a
   `Diff` payload renders it in a way that lets the operator (or
   agent) audit before mutating. Today this works for `file.write`
   / `text.*` / `copy` only.
2. **The diff shape ships in JSON / YAML output** so MCP and the
   future agent SDK consume the same typed structure.
3. **Rollout is incremental** — one action category per PR.
   Handlers that haven't been wired yet keep today's placeholder.
4. **The kernel surface stays small.** No new ABI method on
   `actions.Handler`. The existing `Differ.Diff` return is what
   the renderer consumes; we just give it more presentation
   capability.

## Non-goals

- **Probing external state at plan time** (e.g., looking up the
  latest available package version from the package manager).
  Plan-mode handlers stay within their existing
  `actions.ModePlan` boundaries. If a Diff payload doesn't
  contain "available upstream version," the renderer renders what
  the payload has.
- **New action categories.** This spec rolls out the renderer for
  existing typed actions; it doesn't add `pkg.upgrade-to-latest`
  or anything new.
- **A unified diff format for non-text actions.** A package install
  diff isn't a `+`/`-` line-diff — it's a typed before/after.
  Render shapes are per-kind.
- **MCP tool wiring.** The agent stream's `agent/proposal-04-diff-plan-tool.md`
  builds on this spec's output; not bundled here.

## Reuse map — what's already in the tree

| Capability | Where | Status |
|---|---|---|
| `actions.Differ` interface | `internal/actions/handler_abi.go` | ✓ Spec-22 phase 4 |
| Typed `Diff` struct (Resource / Operation / Before / After / Lines) | `internal/actions/handler_abi.go` | ✓ Shipped |
| Per-handler `diff.go` producing typed Diff | `internal/actions/{file,copy,package,pkg_repo,git_config,os_mount,os_user,os_group,...}/diff.go` | ✓ 26 files |
| Plan-mode dispatch that fills `result.Detail` | `internal/executor/inspect.go` | ✓ Shipped |
| Current renderer (file-only) | `cmd/mooncake.go:725 formatPlanText` + `cmd/mooncake.go:729 extractUnifiedDiff` | needs replacement |
| JSON / YAML plan output | `cmd/mooncake.go:690 formatPlanJSON`, `:696 formatPlanYAML` | needs widening |

The proposal already lays out the target render shapes for each
kind (`package`, `user`, `group`, `firewall`, `service`, `cron`,
`mount`, `git`, `repo`, `transaction-children`).

## Design

### 1. New `internal/diff` package

Two responsibilities, kept narrow:

```go
package diff

// Renderer takes a per-step diff payload (whatever the handler's
// Differ.Diff method returned) and renders it for a given output
// format. Format-specific behavior lives entirely in this package;
// handlers stay format-agnostic.
type Renderer interface {
    Kind() string                                  // "file", "package", "user", ...
    Render(w io.Writer, format Format) error
}

type Format string

const (
    FormatText Format = "text"
    FormatJSON Format = "json"
    FormatYAML Format = "yaml"
)
```

Renderer implementations live as one file per kind:

```
internal/diff/
├── doc.go
├── renderer.go          (interface + registry)
├── render_file.go       (lifts the current unified-diff path)
├── render_package.go
├── render_user.go
├── render_group.go
├── render_firewall.go
├── render_service.go
├── render_cron.go
├── render_mount.go
├── render_git.go
├── render_repo.go
└── render_transaction.go  (compound — recurses into children)
```

Each `render_<kind>.go` is ~50–100 LOC. Total new package: ~700 LOC.

### 2. Per-kind typed payloads (shared with handlers)

The proposal sketched typed structs (`FileDiff`, `PackageDiff`, etc.).
These live alongside the `actions.Diff` type so handlers populate
them and the renderer consumes them:

```go
// in internal/actions/diff_payloads.go (new file)

type PackageDiff struct {
    Manager  string
    Package  string
    Before   *PackageState  // nil = absent
    After    *PackageState
}

type PackageState struct {
    Version   string
    Installed bool
}

type UserDiff struct {
    User    string
    Before  *UserState
    After   *UserState
}

// ... one per kind
```

The existing `actions.Diff{ Before, After any }` fields stay — the
typed payload goes in `Before` / `After` as the action-defined shape.
The renderer type-asserts back to the typed struct.

### 3. Wire the renderer into `formatPlanText`

`cmd/mooncake.go:725 formatPlanText` currently does:

```go
if udiff := extractUnifiedDiff(ins.Detail); udiff != "" {
    // render unified diff
}
```

Replace with:

```go
if r := diff.Lookup(ins.Detail); r != nil {
    r.Render(os.Stdout, diff.FormatText)
}
```

`diff.Lookup` consults the renderer registry based on the `Diff.Kind`
field on the payload (or a type switch on `Detail`).

### 4. JSON / YAML widening

`formatPlanJSON` / `formatPlanYAML` already serialise the full plan
including `StepInspection.Detail`. The typed payloads in `Detail`
will marshal naturally. No new code in cmd; just make sure typed
payloads have proper JSON tags.

## Implementation order

Each step is a separate PR, reviewable independently. Wave the
work the same way the refactor plan did.

| Wave | PR | What | Effort |
|---|---|---|---|
| ✅ 1 | `cc6042d0` | `internal/diff` package skeleton + `render_file.go` (lifts current unified-diff path) + registry + JSON marshal tags on existing `actions.Diff`. cmd/mooncake.go switches to the renderer. Zero behavior change for file diffs. | S |
| ✅ 2 | `910cd770` | `render_package.go` + `PackageDiff` payload in actions/. Wire 1 handler (`package`). | S |
| ✅ 3 | `e4ba7a55` | `render_user.go` + `render_group.go` for `os.user` / `os.group`. | S |
| ✅ 4 | `70459e5e` | `render_firewall.go` + `render_service.go` (`os.firewall` + `os.systemd`). | M |
| ✅ 5 | `9c94e315` | `render_cron.go` + `render_mount.go` (`os.cron` + `os.mount`). | S |
| 6 | spec-66-6 | `render_git.go` + `render_repo.go` (`git.checkout` + `git.config` + `pkg.repo`). | S |
| 7 | spec-66-7 | `render_transaction.go` — compound diffs recursing into children. Same renderer for `try:` compounds. | M |
| 8 | spec-66-8 | Handler audit: any remaining `Differ` implementer not yet wired produces a typed payload + has a renderer kind. | M |

Total: ~5–7 days end-to-end. Each PR independently mergeable.

## DONE criteria

After all 8 sub-PRs land:

- `mooncake plan -c <any-plan> --diff` shows a typed diff for every
  step that has a `Differ` implementation. No "would update"
  placeholders for typed actions.
- `mooncake plan -c <any-plan> --diff --format json` emits a JSON
  array where each entry has a `diff: { kind, ... }` typed payload.
- `internal/diff` is at instability ≤ 0.5 with at least 8 renderer
  files.
- No new public API on `actions.Handler` — the existing `Differ.Diff`
  is the only contract.
- The existing `extractUnifiedDiff` helper in `cmd/mooncake.go` is
  removed (its body now lives in `internal/diff/render_file.go`).

## Open questions

1. **Where does the typed payload struct live — `internal/actions/` or `internal/diff/`?**
   The handlers populate the struct; the renderer consumes it. Two
   options:
   - **A.** Payloads in `internal/actions/diff_payloads.go`. Renderer
     imports `actions`. Tightest coupling between payload definition
     and the ABI it's part of.
   - **B.** Payloads in `internal/diff/payloads.go`. Handlers
     import `internal/diff` just for the typed structs.
   - **C.** Payloads in *both* (a tiny `internal/actions/diff_kinds.go`
     for shared types like `FromTo[T]`, with kind-specific structs
     in `internal/diff/`).

   **My recommendation:** A. Handlers should not have to depend on
   the renderer package. The payload is part of the kernel surface.
   Decide in PR 1.

2. **JSON shape stability.** Once a `PackageDiff` ships in
   `--format json`, agent consumers will lock to that shape. Worth
   one PR-1 review pass to make sure field names / nullability are
   right before the rollout starts.

3. **Render width / terminal sizing.** Today's file diff respects
   the terminal width (sort of). Typed diffs may need a similar
   convention. Probably defer to a separate UX-polish PR after the
   structural work lands.

## Pairs with

- **[`agent/proposal-04-diff-plan-tool.md`](../../agent/proposals/proposal-04-diff-plan-tool.md)** — the MCP-tool surface
  for `plan --diff --format json`. Decoupled from this spec but
  motivated by the same payloads.
- **spec-22 phase 4** — the `Differ` ABI this spec consumes.
- **`vision/kernel.md`** — typed Diff is one of the four ABI
  properties. This spec is "rendering of an existing kernel surface,"
  not new infrastructure.

## Receipts

- 26 handlers ship `diff.go` files producing typed payloads that
  are computed in plan mode and **silently discarded**:
  `internal/actions/{file,copy,package,pkg_repo,git_config,
  os_mount,os_user,os_group,os_firewall,os_systemd,os_cron,...}/diff.go`
- 42 `actions.Differ` implementation sites across the tree.
- `cmd/mooncake.go:648` is the only place that renders any of
  this output today, and it only handles the unified-diff string
  for file content.

The work below the renderer is largely done. This spec is a
rendering layer. The kernel's claim — typed Diff on every action —
becomes user-visible after this lands.

## Why this lives in core (not dx)

The renderer is the dx-facing layer; the *payloads* it consumes
are the kernel's. Putting the spec in core keeps the implementation
review anchored on the handler-side typed shapes; cmd just wires
the renderer in.

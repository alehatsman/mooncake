# Spec 16: Plan/Apply Model (Model X)

**Epic:** E2 Reliability + E4 Check Mode rework — supersedes/absorbs [Spec 15](spec-15-check-mode.md)
**Effort:** XL (~2 weeks across all handlers; phased)
**Value:** High — collapses three overlapping CLI concepts (plan / --dry-run / --check) into two (`plan` / `apply`), and three handler methods (Execute / DryRun / Check) into one (`Run` + mode)

---

## Problem

Today mooncake exposes three overlapping non-mutating concepts to users:

- **`mooncake plan`** — static YAML expansion. Doesn't touch the target system. Useful for debugging config structure.
- **`mooncake run --dry-run`** — runs handler `DryRun` per step. Doesn't inspect target state. Logs *intent* only. Can lie (recently shipped: directory mode default mismatch and missing idempotency awareness).
- **`mooncake run --check`** — runs handler `Check` per step. Inspects target state. Reports drift. The smart one — shipped by Spec 15.

Three concepts answering similar questions, at different layers, with inconsistent fidelity. Plus three parallel handler methods (`Execute`, `DryRun`, `Check`) per action that must stay in sync.

Symptoms we know about:
- `--dry-run` lied about directory modes (`0644` vs the actual `0755`) because `DryRun` and `createDirectory` defaulted differently. Fixed in interim Phase 1.
- `--dry-run` says "would create" for already-existing paths because it doesn't inspect state.
- `mooncake plan` doesn't answer the question users most often want — "what would this change?"
- `Metadata.ImplementsCheck` is decorative; ~10 handlers declare it, only `file` and `package` actually implement `Check`.
- `--check`'s "exit 1 on drift" is documented but not implemented.
- `runFromPlan` only threads `--dry-run` through; `--check`, tags, sudo, vars, artifacts dir are dropped.

The root cause: there isn't a single source of truth for "what would happen if I ran this." It's spread across `plan`, `--dry-run`, and `--check` with different semantics each.

---

## Goal

Two top-level operations. One non-mutating answer to the question "what would change?"

```
mooncake plan         — produce a state-aware plan. Inspects target system. Saveable.
mooncake apply        — execute (auto-plans first, or loads via --from-plan).
```

Drop `--dry-run` and `--check` flags. Drop `mooncake run`'s current meaning (renamed `apply`) — or keep `run` as an alias for `apply` for one release.

The plan artifact becomes the single source of truth: it carries the step list, per-step `WouldChange` / `Reason` / `Checkable`, optional content diffs, and the variables used to produce it. `apply` consumes a plan (just-built or loaded) and executes.

```
$ mooncake plan -c main.yml
↑ Install neovim          would install
✓ Set shell to zsh        already /usr/bin/zsh
↑ Link .zshrc             would change (target differs)
- Install OpenJDK         skip [when: apt_available]
? Run migrations          not checkable (shell)

PLAN SUMMARY  would-change=2  ok=1  skipped=1  not-checkable=1

$ mooncake plan -c main.yml --output plan.json
$ mooncake apply --from-plan plan.json
```

Exit codes:
- `plan` returns 0 on success, 1 on error, 2 if `would-change > 0` (`--detailed-exitcode`, Terraform-style; default 0 for ergonomics).
- `apply` returns 0 on success, non-zero on failure.

---

## Design

### Two layers

**Compile layer** (`internal/plan/planner.go`): YAML → flat `[]config.Step`. Same as today. Evaluates includes, loops, vars, tag filters, plan-time templating, system facts. No target inspection.

**Inspect layer** (new): walks the compiled steps and asks each handler "would this change?" via the unified `Run(ctx, step)` in `ModePlan`. Annotates each step with `WouldChange`, `Reason`, `Checkable`. Produces the *full* Plan artifact.

A plan today carries `Plan.Steps []config.Step`. After this spec it also carries `Plan.StepInspections []StepInspection` (or per-step inline fields) holding the inspection result for each step.

```go
type Plan struct {
    Version, GeneratedAt, RootFile string
    Steps        []config.Step
    Inspections  []StepInspection  // parallel to Steps, indexed by ID
    InitialVars  map[string]any
    Tags         []string
    GeneratedOn  HostFacts          // OS/distro/arch at plan time
}

type StepInspection struct {
    StepID      string
    WouldChange bool
    Checkable   bool
    Reason      string
    Detail      any   // optional: diff, current vs desired, etc.
}
```

### Mode enum + unified handler

```go
type Mode int
const (
    ModeExecute Mode = iota
    ModePlan
)

type Handler interface {
    Metadata() Metadata
    Validate(step *config.Step) error
    Run(ctx Context, step *config.Step) (Result, error)
}
```

`Run` in `ModePlan` is today's `Check`, returning a richer `Result`. `Run` in `ModeExecute` is today's `Execute`. Both share up-front state inspection and predicates.

`Result` absorbs `CheckResult`:

```go
type Result struct {
    Changed     bool
    WouldChange bool    // set in ModePlan
    Reason      string  // human description
    Checkable   bool    // false for shell-style steps
    StartTime, EndTime time.Time
    Duration    time.Duration
    Detail      any
}
```

`Execute`, `DryRun`, `Check` go away as separate methods. `Checker` interface and `CheckResult` go away.

### Effect helpers

Side-effect primitives become mode-aware so a single call site decides what to do:

```go
func (ec *ExecutionContext) Mkdir(path string, mode os.FileMode) Effect {
    info, err := os.Stat(path)
    alreadyOk := err == nil && info.IsDir() && info.Mode().Perm() == mode

    switch ec.Mode {
    case ModeExecute:
        if alreadyOk { return Effect{AlreadyOk: true} }
        err := os.MkdirAll(path, mode)
        return Effect{Performed: err == nil, Err: err}
    case ModePlan:
        return Effect{WouldChange: !alreadyOk, AlreadyOk: alreadyOk}
    }
}
```

Primitives to wrap: `Mkdir`, `WriteFile`, `Symlink`, `Remove`, `Chmod`, `Chown`, `RunCommand`. ~10 covers every existing handler. Handlers stop calling `os.*` directly.

### CLI

- `mooncake plan` — compiles + inspects + prints + optionally saves. The default output mode shows the per-step plan and recap. `--output plan.json` saves a complete plan including inspections.
- `mooncake apply` — primary execute verb.
  - Without args: compile + inspect + execute (auto-plan).
  - `--from-plan path`: load saved plan, execute it (no re-inspect). Forwards all relevant context (sudo, vars, tags, artifacts).
  - `--confirm`: print plan, prompt before applying (opt-in; useful for production runs).
- `mooncake run` — alias for `apply` for one release. Logs a deprecation warning.
- `--dry-run`, `--check` flags — removed (or kept as deprecated aliases for `plan` for one release). Print a deprecation pointer.
- `mooncake compile` — optional new debug subcommand that's the old `plan` (static expansion, no state inspection). Probably not worth shipping unless someone asks; can be `plan --no-inspect`.

### Stale-plan policy

Saved plans embed host facts and target-state assumptions that decay. Policy:

- A plan records `Plan.GeneratedOn` (facts subset) and `Plan.GeneratedAt`.
- `apply --from-plan` compares current facts against `Plan.GeneratedOn`. On mismatch (OS, arch, distro), refuses with a clear error unless `--allow-stale`.
- Optional `--max-plan-age 1h` flag; if exceeded, refuse without explicit override.
- Saved plans do NOT skip the per-step idempotency check inside `Execute` (handlers already do `if matches { skip }`). So an apply of a slightly stale plan against a target that's already been partially modified will still be safe — the worst case is a few no-op steps, not double-apply.

This keeps Terraform-style apply useful for CI without inviting "applied 3-week-old plan, broke production" footguns.

### Plan-time facts: keep or move?

Today `BuildPlan` calls `facts.Collect()`. After this spec, both compile *and* inspect happen inside `mooncake plan`, so facts collection stays in the planner. The interesting question is: what happens during `apply --from-plan`?

- `apply` does NOT re-collect facts from the saved plan. It uses what's in `Plan.InitialVars`.
- Stale-plan policy (above) is what guards against fact drift.

### Output formatting

Plan output replaces today's `[DRY-RUN] Would …` and check-mode `▷ ↑ ✓` lines with a single, unified plan format inspired by Spec 15's symbols:

```
↑ <name>          <reason>          (would change)
✓ <name>          <reason>          (already in desired state)
- <name>          <reason>          (skipped — tag/when)
? <name>          <reason>          (not checkable)
× <name>          <reason>          (error during inspection)

PLAN SUMMARY  would-change=N  ok=N  skipped=N  not-checkable=N  errors=N
```

`apply` output is the existing run-time event log (started/completed/changed). Adds a top-of-run plan summary so users see the plan before changes happen (unless `--quiet`).

---

## Migration

### Phase 1 — Foundation (small, low-risk)
1. Add `Mode` enum on `ExecutionContext`. Keep `DryRun bool` / `CheckMode bool` mapped to it for one release.
2. Extend `Result` with `WouldChange`, `Reason`, `Checkable`. `CheckResult` keeps existing definition for now.
3. Tests: new fields default to zero values; nothing else changes.

**Acceptance:** all existing tests pass; no observable behavior change.

### Phase 2 — Unified `Run` for `file` handler
1. Add `Runner` interface (`Run(ctx, step) (Result, error)`) as **optional** alongside existing methods. Dispatcher prefers `Run` if implemented; falls back to today's `Execute`/`DryRun`/`Check`.
2. Implement effect helpers used by `file`: `Mkdir`, `WriteFile`, `Symlink`, `Chmod`, `Chown`, `Remove`. They consult `ec.Mode`.
3. Implement `file.Run`. Lift `file.Check` logic into `Run`'s `ModePlan` branch. Lift `Execute` mutations into effect-helper calls. Delete `file.Execute`, `file.DryRun`, `file.Check`.
4. Regression test: drives `Run` through both modes against the same fixture; asserts plan output matches execute behavior. Applied to the directory-mode bug, it would have failed before the interim fix.

**Acceptance:** file handler tests pass; the failing regression test from interim Phase 1 still passes; `--dry-run` and `--check` continue to work (now both engage `ModePlan`).

### Phase 3 — Plan inspection wiring
1. Extend `Plan` struct with `Inspections []StepInspection` and `GeneratedOn HostFacts`.
2. After `BuildPlan` finishes compiling, run an inspection pass: for each step, dispatch to handler's `Run` in `ModePlan` and collect the result. Skip if handler hasn't been migrated yet (record `Checkable: false, Reason: "handler not yet migrated"`).
3. `mooncake plan` prints the new plan format (`↑ ✓ - ? ×` per step + `PLAN SUMMARY`).
4. `plan --output plan.json` saves the full plan including inspections.
5. `plan --no-inspect` skips Phase 3's inspection pass (regression escape hatch + the old "compile only" use case).

**Acceptance:** `mooncake plan` shows accurate per-step inspection for `file` steps; other actions show `not checkable` until migrated; saved plan round-trips through `--from-plan`.

### Phase 4 — `apply` command
1. Rename `run` to `apply`. Keep `run` as a deprecated alias for one release.
2. `apply` (no args): build plan (with inspection), then execute.
3. `apply --from-plan path`: load saved plan, validate `GeneratedOn` against current facts, execute. Forward sudo, vars, tags, artifacts dir, etc. (closes the `runFromPlan` gap).
4. `apply --confirm`: print plan summary, prompt, apply.
5. Implement stale-plan checks (`--max-plan-age`, `--allow-stale`).
6. `--dry-run` / `--check` flags: removed (or print deprecation warning pointing to `mooncake plan`).

**Acceptance:** `mooncake apply` works end-to-end for a small playbook; `--from-plan` works with sudo/tags; stale-plan refusal works.

### Phase 5 — Migrate remaining handlers
One handler per PR. For each: lift `Check` (if it exists) into `Run`'s `ModePlan` branch, fold `Execute` mutations into effect helpers, delete the three old methods.

Order (cheapest first):
1. `package` (`Check` already exists, surface small)
2. `link` (small)
3. `service`
4. `copy`
5. `template`
6. `download`
7. `unarchive`
8. `command`
9. `shell` (special: `Run` in `ModePlan` always returns `{Checkable: false, Reason: "shell steps not checkable"}`)
10. Remaining actions (`include_vars`, `vars`, `print`, `assert`, etc. — mostly already runtime-only or compile-only; assess case by case)

Each migration deletes the three old methods. After all handlers are migrated, the `Handler` interface drops `Execute`/`DryRun`/the optional `Checker`.

**Acceptance:** all handlers implement `Run`; `Execute`/`DryRun`/`Checker` removed from public interface; full test suite green.

### Phase 6 — Cleanup
1. Drop `IsDryRun()` / `IsCheckMode()` accessors.
2. Drop `CheckResult` type.
3. Drop `ec.DryRun` / `ec.CheckMode` bool fields.
4. Drop `Metadata.ImplementsCheck` / `Metadata.SupportsDryRun` (now meaningless).
5. Remove `mooncake run` alias (or keep — minor maintenance).
6. Remove `--dry-run` / `--check` deprecation shims.

Each phase is independently shippable. Phase 1 ships first as a no-op foundation. Phase 2 ships the unified `file` handler. Phase 3 ships the new `plan` UX (the user-visible win). Phase 4 ships the new `apply` UX. Phase 5 spreads across multiple PRs over weeks.

---

## Resolved Design Decisions

- **Effect helpers location:** new `internal/effects` package exposing a
  `Performer` interface, with `ExecutionContext` (or a thin wrapper)
  implementing it. Keeps the surface testable in isolation and avoids
  bloating `ExecutionContext`.
- **`Effect` return shape:** carries the action kind it represents
  (`Action string` — `"mkdir"`, `"write_file"`, etc.) plus `Performed`,
  `WouldChange`, `AlreadyOk`, `Err`, and a free-form `Detail any` for
  future diff payloads.
- **Become/sudo integration:** effect helpers accept become options
  (`Opts{Become, BecomeUser}`) and dispatch to sudo internally. Handlers
  read `step.Become` and pass it through; they do not shell out
  themselves.
- **Dispatcher migration strategy:** shim path. Add a `Runner` interface
  alongside the existing `Handler`/`Checker`; dispatcher prefers `Run`
  when implemented and falls back to `Execute`/`DryRun`/`Check` otherwise.
  After all handlers migrate (Phase 5), the legacy path and shim are
  removed in Phase 6.
- **Stale-plan policy:** facts subset + YAML hash + optional age limit
  (option F). Plan stores `{generated_at, generated_on: {os_family, arch,
  distro_family}, input_files_hash}`. Apply refuses on facts or hash
  mismatch. `--allow-stale` overrides both. `--max-plan-age` is opt-in
  for CI; no default.

## Open Questions

1. **Keep `mooncake run` as alias?** Probably yes for one release, then
   drop. Or keep indefinitely — costs nothing.
2. **Auto-prompt on `apply`?** Terraform prompts unless `-auto-approve`.
   Mooncake's audience is probably scripts/CI; default no-prompt, opt in
   with `--confirm`. Worth confirming.
3. **Diff output.** `--diff` for content diffs on file/template/copy.
   Worth a follow-up spec after Phase 4 lands; the
   `StepInspection.Detail` field is the hook.
4. **JSONL agent output.** `EventStepChecked` becomes `EventStepInspected`
   or similar. Decide naming once Phase 3 lands.
5. **`--detailed-exitcode` on `plan`.** Terraform-style: exit 2 if drift,
   1 on error, 0 if no change. Default off; opt-in for CI. Add in Phase 3.

---

## Acceptance Criteria (overall)

1. `mooncake plan` and `mooncake apply` exist and replace `mooncake run` + `--dry-run` + `--check`.
2. `mooncake plan` inspects target state and reports per-step `WouldChange` with accurate symbols and summary.
3. `mooncake apply --from-plan` works end-to-end with full context (sudo, tags, vars, artifacts).
4. Stale-plan policy prevents applying plans built on a different host or older than `--max-plan-age`.
5. Handler interface has one method (`Run`); `Execute`/`DryRun`/`Checker` removed.
6. Effect helpers route mutations through `ec.Mode`.
7. Regression test from interim Phase 1 (`TestHandler_DryRun_ReportsCorrectDefaultMode`) — and its successors — pass.
8. Test suite green throughout the migration; each phase independently shippable.

---

## Out of Scope

- Changing the YAML surface or step schema.
- Adding new actions.
- Parallel/dependency-aware execution.
- Plan-time content diffs (`--diff` flag is a follow-up spec).
- Remote state / locking. Mooncake runs locally; not Terraform-scale.

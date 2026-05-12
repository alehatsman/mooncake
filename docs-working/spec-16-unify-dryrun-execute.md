# Spec 16: Plan/Apply Model (Model X)

**Status:** Shipped — 14 commits between `5309066` and `3683b98` on `master`
**Epic:** E2 Reliability + E4 Check Mode rework — supersedes/absorbs [Spec 15](spec-15-check-mode.md)
**Effort delivered:** ~2 days of focused work end-to-end
**Value:** High — collapses three overlapping CLI concepts (plan / --dry-run / --check) into two (`plan` / `apply`), and three handler methods (Execute / DryRun / Check) into one (`Run` + mode)

---

## Problem (as-was)

Mooncake exposed three overlapping non-mutating concepts:

- **`mooncake plan`** — static YAML expansion. Didn't touch the target system. Useful for debugging config structure.
- **`mooncake run --dry-run`** — ran handler `DryRun` per step. Didn't inspect target state. Logged *intent* only. Could lie (the bug that started this work: `--dry-run` for `state: directory` printed `mode: 0644` while real Execute used `0755`).
- **`mooncake run --check`** — ran handler `Check` per step. Inspected target state. Reported drift. The smart one — Spec 15.

Three concepts answering similar questions, at different layers, with inconsistent fidelity. Plus three parallel handler methods (`Execute`, `DryRun`, `Check`) per action that had to stay in sync. They drifted.

---

## Outcome (as-shipped)

Two top-level operations, one non-mutating answer to "what would change?"

```
mooncake plan         — state-aware preview. Inspects target system. Saveable.
mooncake apply        — execute (auto-plans first, or loads via --from-plan).
mooncake run          — deprecated alias for `apply`.
--dry-run / --check   — deprecated flags; print a one-line note pointing at `mooncake plan`.
```

Per-step output uses a consistent symbol set:

| | |
| - | - |
| `↑` | would change |
| `✓` | already in desired state |
| `-` | skipped (tags / when) |
| `?` | not checkable (handler can't predict — e.g. shell) |

End-to-end demo:

```
$ mooncake plan -c ./playbook.yml -v ./vars.yml
Plan: ./playbook.yml
Generated: 2026-05-12 22:20:47 on linux/amd64/arch

↑ Ensure demo directory exists       would create directory
↑ Drop a config file                  would create file (40 bytes)
↑ Render a templated config           would create file (55 bytes)
✓ Print a friendly message            would print: Hello from Spec 16!
↑ Touch a marker                      would create file

PLAN SUMMARY  would-change=4  ok=1  skipped=0  not-checkable=0
```

After `mooncake apply --from-plan plan.json`:

```
$ mooncake plan -c ./playbook.yml -v ./vars.yml
✓ Ensure demo directory exists       directory exists with desired mode
✓ Drop a config file                  file content and mode already match
✓ Render a templated config           file content and mode already match
✓ Print a friendly message            would print: Hello from Spec 16!
↑ Touch a marker                      would update mtime
PLAN SUMMARY  would-change=1  ok=4  skipped=0  not-checkable=0
```

Stale-plan policy:

```
$ echo "" >> playbook.yml
$ mooncake apply --from-plan plan.json
refusing to apply stale plan: plan input files have changed since the plan was built
Use --allow-stale to override.

$ mooncake apply --from-plan plan.json --max-plan-age 5s
refusing to apply stale plan: plan is 59s old; --max-plan-age is 5s
```

---

## Architecture (as-shipped)

### Handler interface

```go
// internal/actions/handler.go
type Handler interface {
    Metadata() ActionMetadata
    Validate(step *config.Step) error
    Run(ctx Context, step *config.Step) (Result, error)
}
```

One method. Execute, DryRun, and the optional Checker interface are gone from the contract. Concrete handlers may retain Execute methods internally for test backward compat — the dispatcher never calls them.

### Mode

```go
// internal/actions/mode.go
type Mode int
const (
    ModeExecute Mode = iota
    ModePlan
)
```

`ctx.Mode()` is the source of truth. The legacy `ExecutionContext.DryRun` and `ExecutionContext.CheckMode` bools remain as deprecated accessors (`IsDryRun()`, `IsCheckMode()`) — both return `Mode() == ModePlan`.

### Result extensions

```go
// internal/executor/result.go
type Result struct {
    Changed     bool
    WouldChange bool    // set in ModePlan
    Reason      string  // human description: "would create", "already matches", ...
    Checkable   bool    // false when the action can't predict outcomes (shell)
    // ... existing fields ...
}
```

The legacy `actions.CheckResult` type has been removed; its fields are folded into `Result`.

### Effects package

```go
// internal/actions/performer.go — interface
type Performer interface {
    Mode() Mode
    Mkdir(path string, mode os.FileMode, opts PerformerOpts) Effect
    WriteFile(path string, content []byte, mode os.FileMode, opts PerformerOpts) Effect
    Symlink(target, path string, opts PerformerOpts) Effect
    Hardlink(target, path string, opts PerformerOpts) Effect
    Touch(path string, mode os.FileMode, opts PerformerOpts) Effect
    Remove(path string, recursive bool, opts PerformerOpts) Effect
    Chmod(path string, mode os.FileMode, opts PerformerOpts) Effect
    Chown(path, owner, group string, opts PerformerOpts) Effect
}

// internal/effects/default.go — implementation
func NewPerformer(modeFn ModeFunc, sudoPass string) actions.Performer { … }
```

Each primitive inspects current state once, then branches on `Mode()` to either perform the side effect or return a prediction. Sudo is centralized in `runSudo` (one place that knows how to escalate).

Handlers obtain the Performer via `ctx.Effects()`. Filesystem mutations route through it; `os.*` direct calls are gone from migrated handlers.

### Dispatch

```go
// internal/executor/executor.go
func DispatchStepAction(step config.Step, ec *ExecutionContext) error {
    // ... validate ...
    return dispatchRunner(step, ec, handler)  // single path
}
```

The previous `dispatchCheck` function and the Execute/DryRun/Check fallback branches are gone. In `ExecuteStep`, plan-mode dispatch via Runner bypasses lifecycle events (started/completed) and emits only `EventStepChecked` — matching the legacy CheckMode UX so subscribers continue to work.

### Plan struct

```go
// internal/plan/plan.go
type Plan struct {
    Version        string
    GeneratedAt    time.Time
    GeneratedOn    HostFacts            // os_family / arch / distro_family
    RootFile       string
    InputFiles     []string             // every YAML file touched during planning
    InputFilesHash string               // SHA256 over (sorted paths + contents)
    Steps          []config.Step
    Inspections    []StepInspection     // per-step prediction
    InitialVars    map[string]interface{}
    Tags           []string
}

type StepInspection struct {
    StepID, ActionType string
    WouldChange, Checkable, Skipped bool
    Reason string
}
```

### Stale-plan policy (Spec 16 Option F)

`plan.ValidateForApply(p, opts)` runs three independent checks at apply time:

1. Host facts subset match (`os_family` / `arch` / `distro_family`).
2. Input-files hash match — re-hash the recorded `InputFiles`; refuse on mismatch or missing.
3. Optional `--max-plan-age` (no default).

Each rejection returns a `*StaleError` with a `Reason` constant (`StaleReasonHostMismatch`, `StaleReasonHashMismatch`, `StaleReasonFileMissing`, `StaleReasonAgeExceeded`). `--allow-stale` demotes all rejections to nil error.

---

## What shipped (commit-by-commit)

| Commit | Phase | Summary |
| ------ | ----- | ------- |
| `87b407d` | 1+2 | Foundation (Mode, Result extensions) + Runner interface + effects package + file handler migration |
| `788f387` | (fix) | Hermetic `TestDiscoverAllPresets_EmptyDirectories` |
| `a00d422` | 3 | Plan inspection pass + new `mooncake plan` UX (symbols + summary + save) |
| `b5f0859` | 4 | `mooncake apply` command + stale-plan policy + `--from-plan` threading fix |
| `93d7425` | 5/1 | Migrate package handler to Runner |
| `c1e232a` | 5/2 | Migrate template handler |
| `fb0911d` | 5/3 | Migrate copy handler |
| `e9a226f` | 5/4 | Six trivial migrations (shell, command, assert, print, vars, include_vars) |
| `5de49fc` | 5/5 | Nine wrappers (service, download, unarchive, repo_*, artifact_*, preset) |
| `181ecc0` | 5/6 | Final five (file_replace family + wait) |
| `5839bed` | 6 | Drop deprecated handler surface (Execute/DryRun/Checker/CheckResult/dispatchCheck) |
| `6dff1e2` | 7-10 | Quality state inspection upgrades for 9 handlers |
| `ffe48ca` | (UX) | Preview text for shell/command/wait/print |
| `3683b98` | (tests) | Production-quality fixes + 12 new `run_test.go` files |

---

## Handler inventory (post-migration)

| Handler | Plan-mode quality | Notes |
| ------- | ----------------- | ----- |
| `file` | full state inspection via Performer | drift-proof |
| `package` | queries pkg manager for installed status | works on apt/dnf/pacman/zypper/apk/brew/etc. |
| `template` | renders template, byte-compares to dest via Performer | drift-proof |
| `copy` | byte-compares src to dest via Performer | drift-proof; mtime no longer preserved |
| `file_replace` / `file_insert` / `file_delete_range` / `file_patch_apply` | in-memory transform + diff vs original | drift-proof |
| `service` | queries `systemctl is-active`/`is-enabled`; compares unit/dropin content | Linux-only inspection; non-Linux reports not-checkable honestly |
| `download` | checksum / size compare on existing dest | force=true bypass works |
| `repo_apply_patchset` | applies in-memory, compares per file | drift-proof |
| `unarchive` | `creates:` marker check | deep archive content compare not implemented (see Limitations) |
| `assert` / `artifact_validate` | delegates Execute (read-only) | failures surface as plan failures |
| `repo_search` / `repo_tree` | delegates Execute (read-only) | |
| `print` | renders message, surfaces first-line preview | no mutation |
| `shell` / `command` | not-checkable, surfaces rendered command in Reason | inherently unpredictable |
| `wait` | inspects current state for `file_exists` / `file_absent`; surfaces target for http/port/git_clean/command | |
| `artifact_capture` | always WouldChange (not idempotent) | inner-step count surfaced |
| `vars` / `include_vars` / `preset` | usually expanded at plan time by the planner | trivial wrappers for the rare cases that reach the executor |

---

## CLI

| Command | Behavior |
| ------- | -------- |
| `mooncake plan -c file.yml [-v vars.yml] [-o plan.json] [--no-inspect] [--show-origins]` | Compile YAML + state-inspect each step. Print symbol+reason per step + PLAN SUMMARY. Optional save. `--no-inspect` reverts to old static-only behavior. |
| `mooncake apply -c file.yml [-v vars.yml] [...]` | Build plan + execute. Same flag surface as the old `run` command. |
| `mooncake apply --from-plan plan.json [--allow-stale] [--max-plan-age 1h]` | Load saved plan; validate stale-plan policy; execute. |
| `mooncake run …` | Deprecated alias for `apply`. Prints note. |
| `--dry-run`, `--check` (on `apply`/`run`) | Deprecated. Print notes pointing at `mooncake plan`. |

---

## Test coverage

`run_test.go` mode-parity tests in 15 handlers:

```
internal/actions/{file,package,template,copy}/run_test.go
internal/actions/{file_replace,file_insert,file_delete_range,file_patch_apply}/run_test.go
internal/actions/{shell,command,wait,print}/run_test.go
internal/actions/{download,service,unarchive}/run_test.go
internal/actions/{artifact_capture,repo_apply_patchset}/run_test.go
```

Each drives `Run` through both modes against the same fixture and asserts the plan prediction matches what execute would do — the structural answer to the original directory-mode bug.

Plus integration coverage:
- `internal/effects/default_test.go` — Performer primitives with mode-parity regression
- `internal/executor/inspect_test.go` — `InspectPlan` happy path + tolerant when in plan mode
- `internal/plan/validate_test.go` — all four stale-plan reject reasons + hash determinism
- `internal/executor/context_test.go` — `Mode()` derives correctly from legacy bools

Full suite green. No regressions from pre-Spec-16 baseline (one pre-existing failure was fixed along the way as `788f387`).

---

## Limitations & follow-ups

These are honest about what we ship and what we don't:

1. **`unarchive` deep inspection.** Without a `creates:` marker, plan always reports would-extract. Proper inspection would mean opening the archive and walking it against the destination tree (tar / tar.gz / zip / etc.). Format-specific, not done. Workaround: set `creates:` to a known post-extraction path — the standard idempotency pattern.

2. **`service` unit file mode.** The plan-mode inspection compares unit/dropin file *content* but not file *mode*. Execute applies the mode anyway; plan may miss mode-only drift.

3. **`copy` no longer preserves mtime.** The byte-compare idempotency check via `Performer.WriteFile` is strictly more correct than the legacy size+mtime heuristic, but the destination file's mtime is now the apply timestamp instead of the source's. Nothing else in the codebase depended on this.

4. **`shell` / `command` are not-checkable by design.** The plan output surfaces the *rendered command text* via `WouldChange + Reason`, which is the best we can do without running the command. Users who want predictable idempotency on shell steps should use `creates:` / `unless:` (already honored by the executor before dispatch).

5. **External Handler authors.** The `Handler` interface change (Execute → Run) is a breaking change for any handler implementations outside this repo. Mooncake is pre-1.0; flagged in the relevant commit message.

6. **Diff output.** `mooncake plan --diff` for showing line-level diffs of would-be changes is a follow-up. The `StepInspection.Detail` field (and `effects.ContentDiff` for write_file effects) are the hooks.

---

## Original design rationale (preserved)

The drift between Execute and the non-mutating preview paths was a structural defect — the type system didn't enforce agreement, and tests didn't catch it because they tested each method in isolation. Spec 16's fix: one entry point per handler, one `Performer` interface that routes mutations by `Mode()`. Drift becomes impossible by construction.

For the user-facing CLI shape, Terraform's `plan` / `apply` model — single non-mutating preview, single execute verb — proved more usable than the three-axis (plan + --dry-run + --check) maze it replaced. Ansible converges on the same idea with `--check`.

---

## Resolved decisions (formerly Open Questions)

- **Effect helpers location:** `internal/effects` package; types in `internal/actions/performer.go`. ✓
- **`Effect` shape:** carries `Action` constant, `Path`, `Reason`, `Performed`, `WouldChange`, `AlreadyOk`, `Err`, `Detail`. ✓
- **Become/sudo:** Performer reads `PerformerOpts{Become}`; helpers escalate internally via `runSudo`. ✓
- **Shim vs hard cut:** shim during migration (Phase 5 added handlers one PR at a time); hard cut in Phase 6 after all migrated. ✓
- **Stale-plan policy:** facts subset + YAML hash + optional age (Option F). ✓
- **`mooncake run` alias:** kept for one release with deprecation note. ✓
- **`--dry-run` / `--check` flags:** kept for one release with deprecation notes. ✓
- **Plan UX format:** `↑ ✓ - ?` symbols + `PLAN SUMMARY` recap. ✓

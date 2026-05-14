# Spec 40: Default Config Discovery, `--dry-run`, and First-Run Hints

**Status:** Draft
**Epic:** Stream 4 — Developer Experience
**Effort:** S (1 day)
**Value:** High — removes the most frequent paper cut (`-c <path>` on every
invocation), makes the safety story discoverable, and replaces the cryptic
`"config file path is empty"` error with one that points at `mooncake init`.

**Background:** `docs-working/analysis/dx-audit-2026-05.md` (§2.3, §2.4,
§2.5, §2.9; recommendations R2, R4, R5, R10).
**Pairs with:** `spec-39-mooncake-init.md`. Together they form the
cohesive first-run experience: `init` scaffolds, the rest Just Works.

---

## Problem

Three closely related friction points, all rooted in the same assumption
(the user already knows what they're doing):

1. **No convention.** `apply`, `plan`, `validate`, and `runs apply` all
   require `-c <path>`. `terraform`, `docker compose`, `make`, and every
   peer tool auto-discover a project file. Mooncake refuses.

2. **`plan` is the dry-run but isn't sold as one.** `mooncake plan`
   already produces a `↑ ✓ - ?` table with optional `--diff`
   (`cmd/mooncake.go:496-507`). The infrastructure for non-mutating
   inspection is solid (`internal/actions/mode.go` defines `ModePlan`,
   spec-16 unified the paths). But the README's quickstart references a
   non-existent `--dry-run` flag, and `apply --help` never points the user
   at `plan`. The safety story is real and invisible.

3. **The "missing config" error is hostile.** With no `-c`, the executor
   bails with `&SetupError{Component:"config", Issue:"config file path
   is empty"}` (`internal/executor/executor.go:678-679`). No mention of
   `mooncake init`, no mention of `--config`, no example.

4. **No next-step hint after a first successful run.** A user who got
   through their first `apply` has no idea what to do next. Re-run?
   Edit? Share?

---

## Goals

- **G1** `mooncake apply`, `mooncake plan`, and `mooncake validate` with
  no `--config` flag auto-discover a project file in the current
  directory.
- **G2** Adopt one canonical filename (`mooncake.yml`) and one multi-file
  convention (`mooncake/main.yml`) — accept both, document the choice.
- **G3** Add `mooncake apply --dry-run` as a first-class flag that
  delegates to the existing plan path.
- **G4** Update help text and the README so the safety story is visible.
- **G5** When no config can be found, print a friendly error that names
  the searched paths and suggests `mooncake init`.
- **G6** After the first successful `apply` (per host, gated on a
  one-shot file), print a single line of next-step hints. Never repeat.

**Non-goals:**

- Searching parent directories (`git`-style upward walk). Stay scoped to
  cwd; surprises from finding a config in `..` outweigh convenience.
- Auto-loading `*.vars.yml` siblings. The `-v` flag remains explicit;
  invisible variable injection is a footgun.
- Workspace / monorepo features. One project per directory for v1.

---

## User experience

### Auto-discovery (the happy path)

```
$ ls
mooncake.yml  mooncake.vars.yml  .gitignore  .mooncake/

$ mooncake plan
Plan: mooncake.yml
Generated: 2026-05-14 12:34:01 on linux/arm64/arch
…
PLAN SUMMARY  would-change=2  ok=5  skipped=0  not-checkable=0

$ mooncake apply
▶ Ensure ~/.config exists
✓ Ensure ~/.config exists
…
RECAP  changed=2  ok=5  skipped=0  failed=0  duration=0.3s

★ First run — nice. Try `mooncake plan` before `apply` to preview changes.
  Browse 330+ built-in workflows with `mooncake presets list`.
```

### Dry-run via `apply`

```
$ mooncake apply --dry-run
Plan: mooncake.yml
↑ Ensure ~/.config exists                            (would create)
✓ Print system summary                               (not checkable)
…
PLAN SUMMARY  would-change=1  ok=0  skipped=0  not-checkable=1
```

Output is byte-identical to `mooncake plan`. The flag is sugar.

### Missing config

```
$ cd /tmp/empty && mooncake apply
no mooncake config found in /tmp/empty

  searched:
    ./mooncake.yml
    ./mooncake/main.yml

  scaffold one with:    mooncake init
  or point explicitly:  mooncake apply -c <path>
```

Exit code 2 (validation/config error — matches `exitCodeValidationError`
in `cmd/mooncake.go:46`).

### Multi-file project

```
$ ls mooncake/
main.yml  vars.yml  packages.yml  …

$ mooncake apply
Plan: mooncake/main.yml
…
```

### Explicit flag still wins

`-c mooncake.yml` continues to work and always takes precedence over
auto-discovery. Auto-discovery is *only* engaged when `--config` is
absent.

---

## Design

### Auto-discovery rules

Search order in the current working directory:

1. `./mooncake.yml`
2. `./mooncake/main.yml`

First match wins. If neither exists, return a sentinel error
(`config.ErrNoConfigFound`) carrying the searched paths.

No parent-directory walk. No `*.yaml` second-pass. No hidden filenames.
Predictability beats cleverness.

### `--dry-run` flag

Add a boolean flag to `applyFlags()` (`cmd/mooncake.go:68-120`):

```go
&cli.BoolFlag{
    Name:  "dry-run",
    Usage: "Preview changes without executing (delegates to `mooncake plan`)",
},
```

In `run(c *cli.Context)`:

```go
if c.Bool("dry-run") {
    if c.String("from-plan") != "" {
        return fmt.Errorf("--dry-run is incompatible with --from-plan; the plan was already produced")
    }
    return planCommand(c)
}
```

`planCommand` already accepts `--config`, `--vars`, `--tags`, `--format`,
`--diff`, `--show-origins`, `--no-inspect` — all flags shared with apply.
`--output` is plan-only and not relevant under `--dry-run`. The
delegation is mechanical.

### Resolving `--config`

Introduce a single helper used by every command that takes `--config`:

```go
// cmd/config_resolve.go (new)
package main

import (
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/urfave/cli/v2"
)

// resolveConfigPath returns the explicit --config value if set, otherwise
// walks the auto-discovery search order. Returns config.ErrNoConfigFound
// (with the search paths attached) when nothing matches.
func resolveConfigPath(c *cli.Context) (string, error) {
    if explicit := c.String("config"); explicit != "" {
        return explicit, nil
    }
    return config.DiscoverConfig(".")
}
```

Call sites:

- `run` (`cmd/mooncake.go:122`) — before building `StartConfig`.
- `planCommand` (`cmd/mooncake.go:396`) — replace the direct
  `c.String("config")` read.
- `validateCommand` (`cmd/mooncake.go:694`) — same.
- `runsApplyCommand` (`cmd/runs.go`) — same.

The `Required: true` on `plan` / `validate` `--config` flags is removed
(`cmd/mooncake.go:778`, `:977`); the helper handles "missing" with a
better error.

### `internal/config/discover.go` (new)

```go
package config

import (
    "errors"
    "os"
    "path/filepath"
)

// SearchPaths is the ordered list of relative paths checked for a project
// config when --config is not provided. Exported so callers can show the
// list in error messages.
var SearchPaths = []string{
    "mooncake.yml",
    "mooncake/main.yml",
}

// ErrNoConfigFound is returned by DiscoverConfig when no candidate exists.
// It carries the absolute search directory so callers can render a useful
// error.
type ErrNoConfigFound struct {
    Dir string
}

func (e *ErrNoConfigFound) Error() string {
    return "no mooncake config found in " + e.Dir
}

// DiscoverConfig returns the first existing candidate from SearchPaths,
// rooted at dir. Returns *ErrNoConfigFound if nothing matches.
func DiscoverConfig(dir string) (string, error) {
    abs, err := filepath.Abs(dir)
    if err != nil {
        return "", err
    }
    for _, rel := range SearchPaths {
        candidate := filepath.Join(abs, rel)
        info, err := os.Stat(candidate)
        if err == nil && !info.IsDir() {
            return candidate, nil
        }
    }
    return "", &ErrNoConfigFound{Dir: abs}
}
```

Tested with: file present, dir-not-file (rejected), neither present
(returns sentinel), permission denied (propagates), symlink (followed).

### Friendly error rendering

In each call site, when the helper returns `*ErrNoConfigFound`, print the
multi-line guidance shown in the UX section above and exit with
`exitCodeValidationError` (2). Centralise the message in
`internal/config/discover.go`:

```go
// HintNoConfigFound returns the user-facing remediation message for
// *ErrNoConfigFound. cmdName is the subcommand the user invoked, used in
// the "point explicitly" suggestion.
func HintNoConfigFound(e *ErrNoConfigFound, cmdName string) string { … }
```

CLI handlers call:

```go
if errors.As(err, &nfe) {
    fmt.Fprintln(os.Stderr, config.HintNoConfigFound(nfe, "apply"))
    os.Exit(exitCodeValidationError)
}
```

The existing `&SetupError{Component:"config", Issue:"config file path is
empty"}` branch in `internal/executor/executor.go:678-679` becomes
unreachable from the CLI (since the path is resolved before `Start`),
but is left in place as a defensive belt-and-braces — it still serves
direct callers of `executor.Start()`.

### First-run hint

After `RECAP` prints, check for `~/.mooncake/.first-run-completed`. If
absent and the run succeeded (`failed=0`), print:

```
★ First run — nice. Try `mooncake plan` before `apply` to preview changes.
  Browse 330+ built-in workflows with `mooncake presets list`.
```

Then `touch ~/.mooncake/.first-run-completed`. Best-effort: a write
failure does not affect the run's exit code.

Implementation: a new subscriber `logger.NewFirstRunHintSubscriber()` on
`EventRunCompleted`. Lives next to `RunLogSubscriber`
(`internal/logger/runlog_subscriber.go`) and follows the same shape.
Wire it into the publisher in `run()` after the existing subscribers
(`cmd/mooncake.go:192-218`).

Suppress when:
- `--output-format=json|agent|quiet` (the hint is human prose).
- `MOONCAKE_NO_HINTS=1` env var is set (CI / scripted users).

### Help text touch-ups

In `cmd/mooncake.go`:

- `apply` Usage: `"Apply a playbook or saved plan. Use --dry-run to
  preview without changes."` (currently `cmd/mooncake.go:767`).
- `plan` Usage: `"Generate and display execution plan (dry-run)."`
  (currently `cmd/mooncake.go:773`).
- `--config` Usage on all three: `"Path to configuration file (default:
  ./mooncake.yml or ./mooncake/main.yml)"`.

### README + hello-world fixes

Out of scope for the code changes but listed here so the spec is
self-contained:

- Replace `mooncake run --config X --dry-run` (`README.md:53`) with
  `mooncake apply --dry-run`.
- Replace `mooncake run --config X` (`README.md:56`, `:124`) with
  `mooncake apply`.
- Fix `examples/01-hello-world/` reference (`README.md:124`) to
  `examples/hello-world/`.
- Remove the broken "Continue to 02-variables-and-facts/" link in
  `examples/hello-world/README.md:68`.
- Update the comparison-table cell ("Dry-run: Native") to
  `"Dry-run: mooncake apply --dry-run"`.

---

## Acceptance criteria

1. With `mooncake.yml` in cwd, `mooncake apply` (no flags) succeeds and
   produces the same output as `mooncake apply -c mooncake.yml`.
2. With `mooncake/main.yml` in cwd and no `mooncake.yml`, `mooncake
   apply` succeeds and uses that file.
3. With both present, `mooncake.yml` wins (matches `SearchPaths` order).
4. In an empty directory, `mooncake apply` exits 2 and prints the
   "no mooncake config found" message including both candidate paths
   and a literal `mooncake init` suggestion.
5. `mooncake apply --dry-run -c examples/hello-world/config.yml`
   produces byte-identical output to `mooncake plan -c
   examples/hello-world/config.yml`.
6. `mooncake apply --dry-run --from-plan saved.json` exits 1 with a
   clear error about incompatibility.
7. `mooncake plan` and `mooncake validate` also pick up the
   auto-discovered config (no `--config` required).
8. Explicit `-c` always overrides auto-discovery, including when the
   explicit path is `./mooncake.yml` itself.
9. After the first successful `apply` on a host,
   `~/.mooncake/.first-run-completed` is created; subsequent applies do
   not print the hint.
10. `MOONCAKE_NO_HINTS=1 mooncake apply` never prints the hint, even on
    first run, and never creates the marker file.
11. `mooncake apply --output-format=json` never prints the hint
    (would corrupt downstream JSON parsers).
12. `mooncake apply --help` mentions `--dry-run` and the default
    config search paths.
13. With a directory named `mooncake.yml` (pathological), discovery
    skips it (not a regular file) and continues to `mooncake/main.yml`.
14. Unit tests cover `DiscoverConfig`: file present, dir-not-file,
    neither present, both present, symlink-to-file.

---

## Open questions

1. **Should `--dry-run` also accept short `-n`?** Common convention
   (`make -n`, `rsync -n`). Lean yes. Confirm there's no flag
   collision in `applyFlags()`.
2. **What if `mooncake.yaml` (with the `.yaml` extension) exists?**
   Spec is silent and won't auto-discover it. Two camps:
   - Strict: one filename, predictability wins.
   - Friendly: also accept `mooncake.yaml`.
   Lean strict; if community pushes back, add a one-line entry to
   `SearchPaths`.
3. **Should the first-run hint surface a *project-specific* next step
   (e.g. point at presets used in the playbook)?** Tempting but
   couples the hint to the plan. Keep it generic for v1.
4. **Should `mooncake validate` auto-discover too?** Yes per G1, but
   note that validate has been "explicit by design" elsewhere. Confirm
   no scripted callers rely on the current "required: true" behavior.
5. **Should the missing-config error mention `--list-templates` (from
   spec-39)?** Probably overload — `mooncake init` alone is enough.
   Templates are a `mooncake init` subtlety.

---

## Dependencies / sequencing

- **None upstream.** Lands standalone.
- **Pairs with spec-39** (`mooncake init`). Ship together — the error
  message references `mooncake init`, and `init` produces files that
  this spec discovers. They are the same user-visible win.
- **Unblocks** spec-dx-3 (`mooncake doctor`) — doctor wants to report
  "no project config in cwd" using the same discovery logic.
- **Affects** `cmd/runs.go` `runsApplyCommand` for parity; can be done
  in the same PR or a fast follow.

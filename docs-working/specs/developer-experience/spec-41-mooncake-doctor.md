# Spec 41: `mooncake doctor` — Health Check

**Status:** Draft
**Epic:** Stream 4 — Developer Experience
**Effort:** M (3–5 days)
**Value:** Medium-high — the standard "my tool isn't working, what now"
escape hatch. Surfaces install / state / tool / project / service
problems in one place, in under a second. Reduces the support load
(GitHub issues, Discord questions) for everything that isn't a bug in
the engine itself.

**Background:** `docs-working/analysis/dx-audit-2026-05.md` (§2.7, §2.8,
recommendation R8). Vision pointer: `VISION.md:326` ("`mooncake doctor`
— interactive…").
**Depends on:** `spec-40-config-discovery-and-dry-run.md` (uses the
same `config.DiscoverConfig` for the project-config check).

---

## Problem

When something is off, a user has no single command that tells them
*what* is off. Examples of questions a doctor command should answer at
a glance:

- Is `mooncake` on PATH? Which binary? Which version?
- Are the preset search paths (`./presets/`, `~/.mooncake/presets/`,
  `/usr/local/share/mooncake/presets/`, `/usr/share/mooncake/presets/`)
  present? How many presets in each?
- Is `~/.mooncake/` writable? Big enough? Does `runs.jsonl` look
  healthy?
- Is `fzf` on PATH? The `mooncake presets` interactive selector silently
  degrades without it.
- Is there an agentd socket? Is anything listening?
- Does the cwd have a `mooncake.yml` (or `mooncake/main.yml`)? Does it
  validate?
- Are there any stale `mooncake.lock` files?
- What did the last run do?

Today the answers are scattered across `mooncake facts`, `mooncake last`,
`mooncake presets list`, `mooncake validate`, plus ad-hoc shell. Most
new users never find them.

`brew doctor`, `rustup show`, and `nvim :checkhealth` set the
expectation: one command, scannable output, severity-tagged, with
remediation hints.

---

## Goals

- **G1** Ship `mooncake doctor` that runs a fixed battery of checks and
  prints a single scannable report.
- **G2** Three severity levels: **OK** (`✓`), **warning** (`⚠`),
  **error** (`✗`). Plus informational notes (`ℹ`) for context that
  isn't a check.
- **G3** Every warning and error carries a `fix:` line with concrete
  remediation.
- **G4** `--format json` for tooling and CI.
- **G5** Exit code: `0` for OK and warnings only; `1` for any errors.
  `--strict` promotes warnings to errors for CI gates.
- **G6** Doctor must finish in under one second on a warm cache; never
  block on slow network calls or daemon RPC.

**Non-goals:**

- **No `--fix` flag in v1.** Doctor reports; it does not mutate. A
  follow-on spec can add `--fix` for the subset of remediations that
  are safe and reversible.
- **No extension API.** Checks are hard-coded for v1. The list is
  curated; community-added checks belong in a later spec once we know
  the shape.
- **No long-running monitoring.** Doctor is a snapshot, not a daemon.
- **No telemetry.** Whatever doctor reports stays on the host.

---

## User experience

### Default text output

```
mooncake doctor — health check

Install
  ✓ mooncake v1.2.3 (/usr/local/bin/mooncake)
  ✓ Go runtime: go1.22.0 linux/arm64

System
  ✓ os=linux  arch=arm64  distribution=arch (rolling)
  ✓ package_manager=pacman  (apt_available=false)
  ℹ Use `mooncake facts` for the full list

State
  ✓ ~/.mooncake/ exists and writable
  ✓ runs.jsonl: 47 entries, last write 2h ago
  ✓ disk: 14 GiB free in $HOME

Preset search paths
  ✓ ./presets/                          (12 presets)
  ✓ ~/.mooncake/presets/                (3 presets)
  - /usr/local/share/mooncake/presets/  (not found)
  ✓ /usr/share/mooncake/presets/        (330 presets)

Tools
  ✓ git 2.45.1
  ⚠ fzf not on PATH
       used by:  `mooncake presets` interactive selector
       fix:      install fzf — https://github.com/junegunn/fzf

Project (./)
  ✓ mooncake.yml found
  ✓ validates clean
  ℹ 7 steps, 2 includes, presets used: zsh, neovim
  - no stale mooncake.lock detected

Optional services
  - agentd socket: not running (/run/user/1000/mooncake/agentd.sock)
  - MCP server:    not running

Summary: 14 ok, 1 warning, 0 errors
```

### Error path

```
…
State
  ✗ ~/.mooncake/ is not writable: permission denied
       fix: chmod u+w ~/.mooncake  (or rm -rf ~/.mooncake to reset)
…
Summary: 11 ok, 1 warning, 1 error
exit 1
```

### JSON output

```
$ mooncake doctor --format json
{
  "ok": 14,
  "warnings": 1,
  "errors": 0,
  "checks": [
    {
      "section": "install",
      "name": "binary",
      "status": "ok",
      "message": "mooncake v1.2.3",
      "detail": "/usr/local/bin/mooncake"
    },
    {
      "section": "tools",
      "name": "fzf",
      "status": "warning",
      "message": "fzf not on PATH",
      "fix": "install fzf — https://github.com/junegunn/fzf",
      "used_by": ["mooncake presets interactive selector"]
    }
  ]
}
```

JSON is the contract for CI consumers; text format is allowed to
churn.

### Section filter

```
$ mooncake doctor --section project
Project (./)
  ✓ mooncake.yml found
  ✓ validates clean
  ℹ 7 steps, 2 includes, presets used: zsh, neovim
Summary: 3 ok, 0 warnings, 0 errors
```

### Flags

| Flag | Purpose |
|---|---|
| `--format <fmt>` | `text` (default) or `json`. |
| `--strict` | Exit 1 on warnings (default exits 1 only on errors). |
| `--section <name>` | Run only one section: `install`, `system`, `state`, `presets`, `tools`, `project`, `services`. Repeatable. |
| `--skip-project` | Skip the cwd project-config check. Useful when running outside any project. |
| `--no-color` | Disable colour even on TTY. (Standard `NO_COLOR` env var honoured too.) |

---

## Check catalogue (v1)

Each row below is one `Check` implementation. Severity in brackets is
the *worst case* — most checks fire `OK` in normal operation.

| Section | Check | Worst case | Fix hint |
|---|---|---|---|
| install | `binary` — version + resolved path | error if can't resolve | – (debug shell PATH) |
| install | `go-runtime` — Go runtime version | info | – |
| system | `facts` — facts collection succeeds, prints summary | error if collection fails | report a bug |
| system | `metrics` — metrics collection succeeds (non-fatal sanity check) | warning | report a bug |
| state | `home-dir` — `~/.mooncake/` exists, is a dir, is writable | error | chmod / re-create |
| state | `runs-log` — `~/.mooncake/runs.jsonl` is writable, last write age | warning if absent | – (created on first run) |
| state | `disk-space` — at least 100 MiB free in `$HOME` | warning under threshold | free up space |
| presets | `search-paths` — for each path: exists? readable? preset count | warning if 0 presets found anywhere | install mooncake-presets package, or run `mooncake presets update` |
| tools | `git` — `git --version` resolvable | warning | install git |
| tools | `fzf` — `fzf --version` resolvable | warning | install fzf |
| tools | `sudo` — `sudo` resolvable on Linux/macOS | warning | install sudo (rare) |
| project | `config` — `config.DiscoverConfig(".")` finds a config (skipped under `--skip-project`) | info ("no project config in cwd") | `mooncake init` |
| project | `validate` — found config validates cleanly | error | run `mooncake validate -c <path>` for details |
| project | `summary` — step count, include count, presets used | info only | – |
| project | `lockfile` — no stale `mooncake.lock` (older than 24h with no PID) | warning | inspect and remove |
| services | `agentd` — socket exists; if exists, accepts connection | info (not running is OK) | `mooncake agentd` if you want it |
| services | `mcp` — `mooncake mcp` reachable? Probably just "not running" check | info | – |

The two `services` checks are intentionally info-only — these are
opt-in features and "not running" is the correct default state for
most users.

---

## Design

### Package layout

```
cmd/doctor.go                      # CLI wiring
internal/doctor/
├── doctor.go                       # Run(opts) entry; aggregates and renders
├── check.go                        # Check interface, Result/Status types
├── render_text.go                  # text formatter
├── render_json.go                  # JSON formatter
└── checks/
    ├── install.go                  # binary, go-runtime
    ├── system.go                   # facts, metrics
    ├── state.go                    # home-dir, runs-log, disk-space
    ├── presets.go                  # search-paths
    ├── tools.go                    # git, fzf, sudo
    ├── project.go                  # config, validate, summary, lockfile
    └── services.go                 # agentd, mcp
```

### Core types

```go
// internal/doctor/check.go
package doctor

type Status int

const (
    StatusOK Status = iota
    StatusInfo
    StatusWarning
    StatusError
)

type Result struct {
    Section string   // "install", "system", "state", "presets", "tools",
                     // "project", "services"
    Name    string   // short ID; stable for JSON consumers
    Status  Status
    Message string   // headline line
    Detail  string   // optional multi-line context
    Fix     string   // remediation; empty for OK/Info
    UsedBy  []string // for tool checks: which mooncake features need this
}

type Check interface {
    Section() string
    Name() string
    Run(ctx Context) Result
}

// Context carries shared state so checks don't each independently
// shell out or stat the same files.
type Context struct {
    HomeDir      string                  // resolved ~/.mooncake/
    Cwd          string
    Facts        *facts.Facts            // collected once, shared
    SkipProject  bool
    SearchPaths  []string                // preset search paths from loader
}
```

### Runner

```go
func Run(opts Options) (Report, error) {
    ctx := buildContext(opts) // collect facts once, resolve $HOME, etc.

    checks := registeredChecks(opts.Sections)
    results := make([]Result, 0, len(checks))

    // Sequential. Total wall time budget < 1s on warm cache; no goroutines
    // needed at v1 scale (~17 checks). Add parallelism only if profiling
    // shows it's worth the complexity.
    for _, c := range checks {
        results = append(results, c.Run(ctx))
    }

    return Report{
        Cwd:     ctx.Cwd,
        Results: results,
    }, nil
}
```

`registeredChecks` returns a hard-coded slice. No registry pattern — we
control the list.

### Timeouts

Each check that touches the filesystem or network gets a 200ms budget
via `context.WithTimeout`. Doctor honours its <1s total promise even if
something is wedged:

```go
func (c *agentdCheck) Run(ctx Context) Result {
    cctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()
    if _, err := net.Dial("unix", c.socketPath); err != nil {
        // not running → info, not error
        return Result{Section: "services", Name: "agentd", Status: StatusInfo,
                      Message: "agentd socket: not running",
                      Detail:  c.socketPath}
    }
    // …
}
```

### Rendering

Text renderer groups results by section in catalogue order, prints the
section header, then one line per result with `✓ / ℹ / ⚠ / ✗` glyph
(colourised on TTY when colour enabled). Multi-line `Detail` and `Fix`
indent under the headline.

JSON renderer dumps the `Report` struct directly. Field names stable;
documented as the contract.

### Exit code

```go
switch {
case report.HasErrors():                  return 1
case opts.Strict && report.HasWarnings(): return 1
default:                                  return 0
}
```

### Reusing existing internals

| Need | Reuse from |
|---|---|
| Facts | `internal/facts.Collect()` (cached `sync.Once`) |
| Project-config discovery | `internal/config.DiscoverConfig` (spec-40) |
| Project-config validation | `internal/config.ReadConfigWithValidation` (already used by `validateCommand`) |
| Preset search paths | export from `internal/presets/loader.go` |
| Agentd socket default | `internal/agentd/config.go:24,36` |
| Run history | `internal/runlog.Last()` |
| Disk-space probe | `golang.org/x/sys/unix.Statfs` on Linux/macOS; stub on Windows |

No new external dependencies.

### CLI wiring

```go
// cmd/doctor.go
func doctorCommand() *cli.Command {
    return &cli.Command{
        Name:  "doctor",
        Usage: "Check mooncake's installation, state, tools, and project for issues",
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
            &cli.BoolFlag{Name: "strict", Usage: "Exit 1 on warnings"},
            &cli.StringSliceFlag{Name: "section", Usage: "Run only this section (repeatable)"},
            &cli.BoolFlag{Name: "skip-project", Usage: "Skip the cwd project-config checks"},
            &cli.BoolFlag{Name: "no-color"},
        },
        Action: doctorAction,
    }
}
```

Register in `createApp()` Commands list alongside `last`, `facts`, etc.

---

## Acceptance criteria

1. `mooncake doctor` exits 0 in a clean install with no project and no
   missing tools.
2. `mooncake doctor` exits 1 when at least one check reports
   `StatusError`.
3. `mooncake doctor --strict` exits 1 when any check reports
   `StatusWarning` even with no errors.
4. `mooncake doctor --format json` produces well-formed JSON; the
   `checks` array is deterministically ordered (catalogue order).
5. Every result with status `warning` or `error` includes a non-empty
   `fix` string.
6. `mooncake doctor --section install` runs only the `install` section
   and the summary line reflects only those check counts.
7. `mooncake doctor --skip-project` does not stat or parse any
   project-local file.
8. Total wall time for `mooncake doctor` on a warm cache (facts already
   collected, no daemon to dial) is under 1 second on a developer
   laptop. Add an integration test that asserts wall time < 1.5s.
9. If `~/.mooncake/` does not exist, doctor reports `state/home-dir`
   as `info` (not error) — it'll be created on first run; the `fix`
   line says `run `mooncake apply` (auto-created on first run) or
   `mooncake init` to scaffold a project`.
10. If `~/.mooncake/` exists but isn't writable, doctor reports
    `state/home-dir` as `error` with a concrete `chmod` suggestion.
11. The `services/agentd` check does *not* hang when the socket file
    exists but nothing is listening (200ms dial timeout enforced).
12. With no preset search paths populated, doctor reports
    `presets/search-paths` as `warning`, not `error` — mooncake can
    run without any presets.
13. With a malformed `mooncake.yml`, the `project/validate` check
    reports `error` and `fix` points at `mooncake validate -c <path>`
    for full diagnostics. (Doctor itself prints only the first
    diagnostic.)
14. `NO_COLOR=1 mooncake doctor` produces output free of ANSI escape
    sequences.
15. `MOONCAKE_NO_HINTS=1` from spec-40 does not affect doctor output —
    doctor is opt-in by invocation, so its tips are not noise.

---

## Open questions

1. **Should doctor short-circuit slow checks under a global
   `--quick` flag?** Probably overkill given the <1s budget and per-
   check timeout. Defer until users report a slow check.
2. **`mooncake doctor` vs `mooncake check` vs `mooncake status`?**
   Convention is `doctor` (brew, rustup, nvim/:checkhealth all use it
   or a synonym). Stick with `doctor`.
3. **Should the cwd-project section run by default when invoked
   outside any project?** Spec says yes — it'll emit `info` saying
   "no project config in cwd; run `mooncake init`". The
   `--skip-project` flag is for scripted use that doesn't want that
   line.
4. **Should we emit a one-line "all clear" tip when 0 warnings / 0
   errors?** Lean no — silence is fine; the summary line already
   shows `0 warnings, 0 errors`.
5. **What about Windows?** Stubs land alongside other modules. `git`
   /`fzf` checks work the same; `disk-space` uses a Windows-specific
   syscall (`GetDiskFreeSpaceExW`); `sudo` check is skipped on
   Windows. `agentd` socket check uses named pipes once agentd has
   Windows support (out of scope for this spec).

---

## Dependencies / sequencing

- **Depends on spec-40** for `config.DiscoverConfig` (the
  `project/config` check). If spec-40 ships first as planned, this
  spec adds zero new infrastructure beyond the doctor package itself.
- **Independent of spec-39.** Doctor doesn't need `mooncake init` to
  exist — but its `fix` hints reference `mooncake init`, which means
  shipping all three together creates a coherent story:
  - new user: `mooncake init` → scaffold
  - confused user: `mooncake doctor` → diagnosis
  - daily use: `mooncake apply` → action
- **Unblocks** a future `mooncake doctor --fix` for safe, reversible
  remediations (create `~/.mooncake/`, run `mooncake presets update`,
  …). Out of scope here.
- **Future extension** by community-contributed checks via a plugin
  hook should reuse the `Check` interface above; design it private but
  shaped to graduate to `pkg/doctor` later. No churn at that point.

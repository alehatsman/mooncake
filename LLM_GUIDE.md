# Mooncake — LLM Navigation Guide

> **Purpose**: Compressed navigation map with pointers to canonical docs + unique LLM insights
>
> **Critical**: Never commit or push. User handles all git operations.

## Project Identity

**Mooncake** = Declarative config management tool (Go). "Docker for AI agents" - safe execution runtime with idempotency guarantees.

- **Audience**: AI agent developers, platform engineers
- **Status**: Pre-release / sideproject / learning vehicle. **Not production**.
- **Platforms**: Linux (full), macOS (full), Windows (stubs)

### Reshape freely — no backwards compatibility

This project is a learning / demo / pre-release sideproject. **There are no
shipped users to protect.** When in doubt:

- Break APIs, rename fields, change defaults, restructure modules.
- Do NOT add deprecation shims, compatibility wrappers, `// removed` comments,
  or feature flags to preserve old behavior.
- Do NOT carry forward dead code "in case it's needed later."
- Delete instead of marking unused. Replace instead of layering.

The cost of a backwards-compat shim is permanent; the cost of changing a call
site once is trivial. Always pay the trivial cost.

## Documentation Map

**CRITICAL**: We follow Single Source of Truth (SSOT) architecture. See `DOCUMENTATION.md` for details.

### Where to Find Things

| Topic | Canonical Source | Quick Summary |
|-------|-----------------|---------------|
| **Action reference** | `docs/guide/config/actions.md` | Complete action documentation with examples |
| **Property reference** | `docs/guide/config/reference.md` | All properties, types, defaults |
| **Preset standards** | `docs/presets/definitive-style-guide.md` (1390 lines) | Gold standard for preset authoring |
| **Development setup** | `docs/development/contributing.md` | Dev environment, testing, workflow |
| **Release process** | `docs/development/releasing.md` | Complete release guide with GoReleaser |
| **Build commands** | `Taskfile.yml` | All task targets (build, test, lint, release) |
| **Adding actions** | `docs/development/adding-actions.md` | How to add new actions |
| **Schema validation** | `internal/config/schema.json` | JSON Schema (source of truth) |
| **Examples** | `examples/*.yml` | Working examples for all features |
| **Non-goals** | `docs-working/non-goals.md` | Seven things Mooncake will not become — check every feature against this list |

### Quick Commands

```bash
# Build
task build              # → out/mooncake
task install            # → /usr/local/bin/mooncake (sudo)

# Test
task test-race          # CRITICAL: run before commit
task ci                 # Full CI suite (lint + test + scan)

# Release
git tag -a v1.0.0 -m "Release v1.0.0" && git push origin v1.0.0

# Docs
task                    # Show all task targets
```

**See `Taskfile.yml` for complete list of targets.**

## Architecture (Core Understanding)

### System Overview

```
mooncake/
├── cmd/mooncake.go                 # CLI entry
├── internal/
│   ├── actions/                    # Handler-based actions (NEW)
│   │   ├── handler.go              # Interface: Metadata, Validate, Execute, DryRun
│   │   ├── registry.go             # Auto-registration
│   │   └── <name>/handler.go       # Self-contained (100-500 lines each)
│   ├── presets/                    # Preset system
│   │   ├── loader.go               # Search paths + load
│   │   ├── validator.go            # Parameter validation
│   │   └── expander.go             # Expansion into steps
│   ├── plan/                       # Plan compilation
│   │   └── planner.go              # Include expansion, path resolution ← KEY
│   ├── executor/                   # Execution engine
│   │   ├── executor.go             # Main dispatch
│   │   └── errors.go               # Typed errors (RenderError, CommandError, etc.)
│   ├── config/
│   │   ├── config.go               # Config structs
│   │   └── schema.json             # JSON Schema (source of truth)
│   └── facts/                      # System facts (cached)
├── presets/                        # 330+ built-in presets
└── docs/                           # Canonical documentation
```

### 5 Core Systems

**1. Actions** (`internal/actions/`)
- Self-contained handlers, no dispatcher updates needed
- 4 methods: `Metadata()`, `Validate()`, `Execute()`, `DryRun()`
- Action vocabulary lives in `internal/config/config.go` (search for the
  `action:"..."` struct tags on `Step`) — that's the single source of
  truth; the JSON schema (`internal/config/schema.json`) and the
  validator's allowed-action list are generated from it.
- Top-level shape, by domain:
  - foundational: `shell`, `cmd`, `assert`, `use`, `import`, `vars`, `vars.load`, `log`, `wait.command`, `wait.file`, `wait.http`, `wait.port`, `tool`
  - file & content: `file.write`, `file.template`, `file.copy`, `file.download`, `file.unarchive`
  - structured text editing: `text.replace`, `text.insert`, `text.delete_range`, `text.line`, `text.patch`, `text.patch.ini`, `text.patch.json`, `text.patch.yaml`
  - packages: `pkg`, `pkg.hold`, `pkg.list`, `pkg.repo`, `pkg.upgrade`
  - OS resources: `os.service`, `os.user`, `os.group`, `os.cron`, `os.mount`, `os.sysctl`, `os.systemd`, `os.ssh_key`, `os.firewall`
  - repo & artifact: `repo.search`, `repo.tree`, `repo.patch`, `artifact.capture`, `artifact.validate`
  - git: `git.clone`, `git.checkout`, `git.config`
  - read & observe: `read.json`, `read.yaml`, `observe.cpu`, `observe.memory`, `observe.disk`, `observe.gpu`, `observe.port`, `observe.process`, `observe.service`, `observe.http`
  - container: `container`, `container.image`
  - Windows: `windows.firewall_rule`, `windows.scheduled_task`
- Registry: Thread-safe auto-registration

**2. Presets** (`internal/presets/`)
- Flat only (NO nesting - presets cannot call presets)
- Search paths: `./presets/` → `~/.mooncake/presets/` → `/usr/local/share/mooncake/presets/` → `/usr/share/mooncake/presets/`
- Parameter namespace: `parameters.name` (NOT just `name`)
- BaseDir stored for relative path resolution

**3. Planner** (`internal/plan/`)
- Plan-time: Loop expansion, include resolution, variable loading, tag filtering
- Context: `ExpansionContext { Variables, CurrentDir, Tags }`
- **Critical**: `CurrentDir` updates with each include (see below)

**4. Executor** (`internal/executor/`)
- Pipeline: Plan → Pre-checks → Var merge → Handler dispatch → Result
- Idempotency: `creates`, `unless`, `changed_when`, built-in state checks
- Handler priority: Registry (new) → Legacy methods

**5. Facts** (`internal/facts/`)
- Cached per-process (`sync.Once`) — static, describe what the machine *is*
- Available: `os`, `arch`, `apt_available`, `brew_available`, `cpu_cores`, `memory_total_mb`, etc.
- Use in templates: `{{ os }}`, `{{ cpu_cores }}`

**6. Metrics** (`internal/metrics/`)
- Live system utilization — describe what the machine is *doing right now*
- Per-metric TTL caching (2s for CPU/GPU/net, 5s for load/mem) — daemon-friendly
- Available: `cpu_usage_pct`, `load_avg_1m`, `memory_used_pct`, `gpus_metrics`, `net_rx_bps`, etc.
- Use in templates: `{{ cpu_usage_pct }}`, `when: load_avg_1m < 4`
- Surfaces: `mooncake metrics` CLI, MCP `get_metrics` tool
- See `docs-next/guide/config/metrics.md`

## Critical: Path Resolution (Common Confusion)

**THE KEY INSIGHT**: Relative paths resolve from **including file's directory**, and `CurrentDir` updates with each include.

### How It Works (Code Level)

```go
// internal/plan/planner.go:23-29
func resolvePath(path, baseDir string) (string, error) {
    if !filepath.IsAbs(path) {
        absPath = filepath.Join(baseDir, path)  // Relative → join with baseDir
    }
    return filepath.Abs(absPath)
}
```

### Include Expansion Flow

```
BuildPlan("/path/to/config.yml")
  ctx.CurrentDir = "/path/to"

  → include: "presets/kubectl/preset.yml"
    → resolvePath("presets/kubectl/preset.yml", "/path/to")
    → Result: "/path/to/presets/kubectl/preset.yml"
    → NewCtx.CurrentDir = "/path/to/presets/kubectl"  ← UPDATES

      → include: "tasks/install.yml"  (from within preset.yml)
        → resolvePath("tasks/install.yml", "/path/to/presets/kubectl")
        → Result: "/path/to/presets/kubectl/tasks/install.yml"
        → NewCtx.CurrentDir = "/path/to/presets/kubectl/tasks"  ← UPDATES AGAIN

          → include: "verify.yml"  (from within install.yml)
            → resolvePath("verify.yml", "/path/to/presets/kubectl/tasks")
            → Result: "/path/to/presets/kubectl/tasks/verify.yml"
```

### Preset Flow

```
LoadPreset("kubectl")  (internal/presets/loader.go:47)
  → Searches: ./presets/kubectl/preset.yml
  → Sets: preset.BaseDir = "./presets/kubectl"

ExpandPreset(invocation)  (internal/presets/expander.go:13)
  → ExpandStepsWithContext(preset.Steps, params, preset.BaseDir)
    → Planner uses preset.BaseDir as CurrentDir
    → Include paths resolve relative to preset.BaseDir
```

### Practical Example

**Preset structure**:
```
presets/kubectl/
├── preset.yml
├── tasks/
│   ├── install.yml
│   └── verify.yml
└── templates/
    └── config.j2
```

**From preset.yml**:
```yaml
steps:
  - include: tasks/install.yml      # → presets/kubectl/tasks/install.yml
```

**From tasks/install.yml**:
```yaml
steps:
  - template:
      src: ../templates/config.j2   # → presets/kubectl/templates/config.j2
  - include: verify.yml              # → presets/kubectl/tasks/verify.yml
```

**Why this works**: Each include updates `CurrentDir` to the included file's directory.

## Error Handling

**Typed Errors** (`internal/executor/errors.go`):
- `RenderError` - template failures
- `CommandError` - command execution failures
- `FileOperationError` - file operations
- `StepValidationError` - config validation
- `AssertionError` - assertion failures

**Usage**: `errors.Is()`, `errors.As()` for inspection

## Development Rules

### Code Style (CRITICAL)

❌ **Avoid**:
- Over-engineering / premature abstractions
- "Improvements" beyond request
- Extra error handling for impossible scenarios
- Backwards-compatibility hacks
- Unused code (delete completely)

✅ **Do**:
- Minimal, focused solutions
- Three similar lines > premature helper
- Comments only where logic isn't self-evident
- Security-first (command injection, XSS, SQL injection = critical)

### Git Workflow

**NEVER**:
- Run `git commit` or `git push`
- Create commits (even if user asks)
- Amend commits
- Force push

**DO**:
- Make changes, stage files
- Suggest single-line messages: `<verb> <brief description>`
- Example: "add kubectl preset", "fix path resolution"

### Testing

- **Before commit**: `task test-race` or `task ci`
- **Test artifacts**: ALL to `./testing-output/`
- **Idempotency**: Run twice, second should report no changes

## Platform Patterns

### Use Facts, NOT OS Checks

```yaml
# ✅ Good - specific capability
- shell: apt-get install -y tool
  when: apt_available

# ❌ Bad - broad OS check
- shell: apt-get install -y tool
  when: os == "linux"  # Not all Linux has apt!
```

### Installation Hierarchy
1. Package manager (preferred)
2. Official installation script
3. Binary download + checksum
4. Source compilation (last resort)

## Quick Reference

| Task | Command/Location |
|------|-----------------|
| Add action | Create `internal/actions/<name>/handler.go` → Implement interface → Register in `internal/register/register.go` |
| Add preset | Create `presets/<name>.yml` or `presets/<name>/preset.yml` → Follow `docs/presets/definitive-style-guide.md` |
| Build | `task build` or `go build -o mooncake cmd/mooncake.go` |
| Test | `task test-race` (critical before commit) |
| Release | Tag version, push tag → GitHub Actions auto-builds (see `docs/development/releasing.md`) |
| Facts | Cached, available as `{{ os }}`, `{{ cpu_cores }}`, etc. |
| Templates | Jinja2-like: `{{ variable }}`, `{% if condition %}`, `{% for item in list %}` |

## Key Files to Know

- `internal/plan/planner.go:23-29` - Path resolution logic (resolvePath)
- `internal/plan/planner.go:265-339` - Include expansion (expandInclude)
- `internal/presets/loader.go:47-117` - Preset loading (LoadPreset)
- `internal/presets/expander.go:13-50` - Preset expansion (ExpandPreset)
- `internal/executor/errors.go` - All typed errors
- `internal/config/schema.json` - JSON Schema (source of truth for validation)
- `Makefile` - All build/test/release targets
- `docs/presets/definitive-style-guide.md` - Preset standards (1390 lines)

## Common Pitfalls

1. **Path confusion**: Remember `CurrentDir` updates with each include
2. **Preset nesting**: Presets cannot call other presets (flat only)
3. **Parameter namespace**: Use `parameters.name`, not just `name`
4. **OS checks**: Use `apt_available` not `os == "linux"`
5. **Duplication**: Link to canonical docs, don't duplicate (see `DOCUMENTATION.md`)

## Notes

- Event system: Non-blocking, 100-event buffer, type-safe
- Dry-run: Same plan, no side effects, shows diffs
- Service: systemd (Linux), launchd (macOS), Windows (stubs)
- 330+ presets: 16 enhanced (production-ready), 314+ minimal

---

**Remember**: This is a navigation guide. For detailed docs, see `docs/` directory. For duplication policy, see `DOCUMENTATION.md`.

---

## Architecture soft caps

Three review-time prompts (not CI gates) that keep the architecture
self-policing as the project grows. When a PR crosses one of these
thresholds, the reviewer asks the question; nothing auto-blocks.

Grounded in the 2026-05-15 arch report
(`docs-working/arch-report/2026-05-15-arch-report.md`).

### 1. Handler LOC > 1,500 → split

Reason: handlers that grow past ~1,500 LOC are almost always
cross-platform multiplexers (file, service, package, os_mount) that
accreted per-OS branches into one file. Past the cap, split into
per-OS sub-packages (`internal/actions/<name>/{linux,darwin,windows}`)
or into sibling action types.

### 2. `internal/config.Step` universal-field count > 40 → flag

Reason: every universal field on `Step` is a concept every step type
must ignore or honor. The closed action set is the kernel's moat
(see `docs-working/vision/kernel.md`); the cost of that is a
monotonically-growing `config.go`. Today's count is ≈25. Past 40,
the field has become a tag everyone has to ignore, and the question
"why does *every* step need this?" stops having a good answer.

### 3. `gocyclo` > 35 in any non-test function → refactor on next touch

Reason: gocyclo > 35 means six or more independent decision branches
in one function — almost always a CLI handler doing business logic
that belongs in an `internal/` package. The cap doesn't force a
refactor; it surfaces the smell on the next PR that touches the
function.

### Today's known violations (tracked, not blocking)

- `internal/actions/file` — 2,044 LOC
- `internal/actions/tool` — 1,676 LOC
- `internal/actions/service` — 1,466 LOC (just under)
- `copy.Execute` — gocyclo 41
- `os_systemd.computePlan` — gocyclo 34 (just under)
- `fleetApplyAction` — gocyclo 49 (will be fixed by R2.1a in the
  refactor plan)

These are documented, not hidden. New violations should be
explicitly defended in the PR description that lands them.

See `docs-working/arch-report/2026-05-15-refactoring-plan.md` §2
(R0.4) for the rationale; this is the project's first formal soft-cap
policy.

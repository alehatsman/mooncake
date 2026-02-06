# Mooncake — LLM Navigation Guide

> **Purpose**: Compressed navigation map with pointers to canonical docs + unique LLM insights
>
> **Critical**: Never commit or push. User handles all git operations.

## Project Identity

**Mooncake** = Declarative config management tool (Go). "Docker for AI agents" - safe execution runtime with idempotency guarantees.

- **Audience**: AI agent developers, platform engineers
- **Status**: Production-ready, 13 actions migrated ✅
- **Platforms**: Linux (full), macOS (full), Windows (stubs)

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
| **Build commands** | `Makefile` (lines 17-170) | All make targets (build, test, lint, release) |
| **Adding actions** | `docs/development/adding-actions.md` | How to add new actions |
| **Schema validation** | `internal/config/schema.json` | JSON Schema (source of truth) |
| **Examples** | `examples/*.yml` | Working examples for all features |

### Quick Commands

```bash
# Build
make build              # → out/mooncake
make install            # → /usr/local/bin/mooncake (sudo)

# Test
make test-race          # CRITICAL: run before commit
make ci                 # Full CI suite (lint + test + scan)

# Release
git tag -a v1.0.0 -m "Release v1.0.0" && git push origin v1.0.0

# Docs
make help               # Show all Makefile targets
```

**See `Makefile` for complete list of targets.**

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
- 13 actions: print, vars, shell, command, include_vars, file, template, copy, download, unarchive, assert, preset, service
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
- Cached per-process (`sync.Once`)
- Available: `os`, `arch`, `apt_available`, `brew_available`, `cpu_cores`, `memory_total_mb`, etc.
- Use in templates: `{{ os }}`, `{{ cpu_cores }}`

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

- **Before commit**: `make test-race` or `make ci`
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
| Build | `make build` or `go build -o mooncake cmd/mooncake.go` |
| Test | `make test-race` (critical before commit) |
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

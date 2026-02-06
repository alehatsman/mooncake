# Development Guide for AI Agents

**Target Audience:** AI agents, LLMs, and automated code contributors

**Status:** Mooncake is production-ready. This guide helps you contribute code, add actions, create presets, and extend functionality.

---

## Quick Reference

| Resource | Purpose |
|----------|---------|
| **[CLAUDE.md](https://github.com/alehatsman/mooncake/blob/master/CLAUDE.md)** | AI-specific project instructions (rules, constraints) |
| **[LLM_GUIDE.md](https://github.com/alehatsman/mooncake/blob/master/LLM_GUIDE.md)** | Complete codebase navigation guide |
| **[ADR 001](../architecture-decisions/001-handler-based-action-architecture.md)** | Handler-based action architecture |
| **[ADR 002](../architecture-decisions/002-preset-expansion-system.md)** | Preset expansion system |
| **Tests** | `go test ./...` (300+ tests, must pass) |

---

## For AI Agents: Quick Start

### Codebase Essentials

```
Language: Go 1.21+
Dependencies: Minimal (yaml parser, expr evaluator, testify for tests)
Architecture: Handler-based actions + plan-execute model
Tests: 300+ tests with race detector
CI: All tests must pass, zero linter warnings
```

### Core Constraints

1. **No external dependencies** beyond Go stdlib (exceptions: yaml parsing, expr evaluation)
2. **All actions must be idempotent** (safe to run multiple times)
3. **Cross-platform** (Linux, macOS, Windows)
4. **Dry-run mode required** for all state-changing actions
5. **Zero breaking changes** to existing configs

### Repository Structure

```
mooncake/
├── cmd/mooncake/           # CLI entry point
├── internal/
│   ├── actions/            # Action handlers (handler.go, registry.go)
│   │   ├── shell/          # Shell action handler
│   │   ├── file/           # File action handler
│   │   ├── template/       # Template action handler
│   │   └── ...             # 13 total actions
│   ├── config/             # Config structs + schema validation
│   ├── executor/           # Execution engine
│   ├── plan/               # Plan compiler (parse → plan → execute)
│   ├── facts/              # System facts collection
│   ├── presets/            # Preset loader + expander
│   └── events/             # Event system for observability
├── presets/                # 388+ preset definitions
│   └── <name>/
│       ├── preset.yml      # Preset definition
│       ├── README.md       # Documentation
│       └── tasks/          # Task files (install, uninstall, etc.)
└── docs-next/              # Documentation (MkDocs)
```

---

## Adding New Actions

### 1. Action Interface (Machine-Readable Spec)

All actions implement this interface:

```go
// internal/actions/handler.go
type Handler interface {
    // Metadata returns action metadata (name, description, category)
    Metadata() ActionMetadata
    
    // Validate checks configuration before execution
    Validate(config interface{}) error
    
    // Execute runs the action and returns results
    Execute(ctx Context, config interface{}) (Result, error)
    
    // DryRun previews what would happen (no side effects)
    DryRun(ctx Context, config interface{}) error
}

type ActionMetadata struct {
    Name        string
    Description string
    Category    string  // command, file, system, data, network, output
}

type Context interface {
    Variables() map[string]interface{}  // Template variables
    Facts() map[string]interface{}      // System facts
    DryRun() bool                        // Is this a dry-run?
    Events() EventEmitter                // Emit events
    // For ExecutionContext: SudoPass, PathUtil, etc.
}

type Result interface {
    SetChanged(bool)
    SetFailed(bool)
    SetSkipped(bool)
    SetOutput(stdout, stderr string)
    SetExitCode(int)
    ToMap() map[string]interface{}
}
```

### 2. Implementation Steps

**Step 1: Create handler file**
```bash
internal/actions/<action_name>/handler.go
```

**Step 2: Implement Handler interface**
```go
package actionname

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/events"
)

type Handler struct{}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:        "action_name",
        Description: "What this action does",
        Category:    actions.CategoryCommand,  // or File, System, etc.
    }
}

func (h *Handler) Validate(config interface{}) error {
    // Type assert and validate configuration
    cfg, ok := config.(*Config)
    if !ok {
        return fmt.Errorf("invalid config type")
    }
    
    // Validate required fields
    if cfg.RequiredField == "" {
        return fmt.Errorf("required_field is required")
    }
    
    return nil
}

func (h *Handler) Execute(ctx actions.Context, config interface{}) (actions.Result, error) {
    cfg := config.(*Config)
    result := actions.NewResult()
    
    // Get execution context for additional functionality
    execCtx, ok := ctx.(actions.ExecutionContext)
    if !ok {
        return result, fmt.Errorf("invalid context type")
    }
    
    // Emit event
    ctx.Events().Emit(events.Event{
        Type: events.EventStepStarted,
        Data: events.StepStartedData{
            Action: h.Metadata().Name,
        },
    })
    
    // Execute action logic
    // ... your implementation ...
    
    result.SetChanged(true)
    result.SetOutput("output", "")
    
    return result, nil
}

func (h *Handler) DryRun(ctx actions.Context, config interface{}) error {
    // Preview what would happen (no side effects)
    // Read files, compare content, but don't modify anything
    return nil
}
```

**Step 3: Register handler**
```go
// internal/register/register.go
import "github.com/alehatsman/mooncake/internal/actions/actionname"

func init() {
    registry.Register("action_name", &actionname.Handler{})
}
```

**Step 4: Add config struct**
```go
// internal/config/config.go
type ActionNameConfig struct {
    RequiredField string            `yaml:"required_field"`
    OptionalField string            `yaml:"optional_field,omitempty"`
    State         string            `yaml:"state,omitempty"`  // present, absent, etc.
}

type Step struct {
    // ... existing fields ...
    ActionName *ActionNameConfig `yaml:"action_name,omitempty"`
}
```

**Step 5: Update JSON schema**
```go
// internal/config/schema.json
// Add to "properties" of Step:
"action_name": {
  "type": "object",
  "properties": {
    "required_field": {"type": "string"},
    "optional_field": {"type": "string"},
    "state": {"type": "string", "enum": ["present", "absent"]}
  },
  "required": ["required_field"],
  "additionalProperties": false
}

// Add to all oneOf exclusion blocks:
{"not": {"required": ["action_name"]}}

// Add new oneOf block:
{
  "required": ["action_name"],
  "not": {
    "anyOf": [
      {"required": ["shell"]},
      {"required": ["file"]},
      // ... other actions ...
    ]
  }
}
```

**Step 6: Write tests**
```go
// internal/actions/actionname/handler_test.go
func TestHandler_Execute(t *testing.T) {
    handler := &Handler{}
    
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name: "valid config",
            config: &Config{RequiredField: "value"},
            wantErr: false,
        },
        // ... more test cases ...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := &mockContext{}
            result, err := handler.Execute(ctx, tt.config)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 3. Key Patterns

**Idempotency:**
```go
// Check if already in desired state
if alreadyExists {
    result.SetChanged(false)
    return result, nil
}

// Make the change
err := performAction()
result.SetChanged(true)
```

**Dry-run:**
```go
func (h *Handler) DryRun(ctx actions.Context, config interface{}) error {
    // Read current state
    currentState := readState()
    desiredState := config.DesiredState
    
    // Compare and log what would change
    if currentState != desiredState {
        ctx.Events().Emit(events.Event{
            Type: events.EventDryRunLog,
            Data: fmt.Sprintf("Would change from %v to %v", currentState, desiredState),
        })
    }
    
    return nil
}
```

**Error handling:**
```go
// Use custom error types from internal/executor/errors.go
if err := validatePath(cfg.Path); err != nil {
    return result, errors.NewFileOperationError(
        fmt.Sprintf("invalid path: %s", cfg.Path),
        err,
    )
}
```

---

## Creating Presets

Presets are reusable workflows. See [Preset Authoring Guide](../guide/preset-authoring.md) for complete details.

### Minimal Preset Structure

```
presets/<name>/
├── preset.yml          # Preset definition (required)
├── README.md           # Documentation (required)
└── tasks/
    ├── install.yml     # Installation task (required)
    └── uninstall.yml   # Uninstallation task (required)
```

### preset.yml Example

```yaml
name: example-tool
version: "1.0.0"
description: "Install and configure example-tool"

parameters:
  state:
    type: string
    required: true
    enum: [present, absent]
    description: "present to install, absent to uninstall"
  
  version:
    type: string
    required: false
    default: "latest"
    description: "Tool version to install"

steps:
  - include: tasks/install.yml
    when: parameters.state == "present"
  
  - include: tasks/uninstall.yml
    when: parameters.state == "absent"
```

### Preset Best Practices

1. **Detect platform** using facts: `{{os}}`, `{{arch}}`, `{{distribution}}`
2. **Provide defaults** for all optional parameters
3. **Use idempotent operations** (creates, checksums, state checks)
4. **Add assertions** to verify prerequisites
5. **Document thoroughly** in README.md with examples

### Preset Style Guide

See [Definitive Preset Style Guide](../presets/style-guide.md) for:

- Naming conventions
- File structure standards
- Platform handling patterns
- Documentation requirements
- Validation rules

---

## Architecture Overview

### Three-Phase Execution Model

```
1. PARSE       → Config structs + validation
2. PLAN        → Deterministic execution plan (IR)
3. EXECUTE     → Run plan steps sequentially
```

**Benefits:**

- Deterministic (same config → same plan → same result)
- Inspectable (view plan before execution)
- Reproducible (save plan, execute later)

**See:** [ADR 000: Planner Execution Model](../architecture-decisions/000-planner-execution-model.md)

### Handler-Based Actions

Actions are modular handlers registered at runtime:

```
Action Request → Registry.Get(name) → Handler.Execute(ctx, config) → Result
```

**Benefits:**

- Add actions with 1 file (~200-500 lines)
- No dispatcher updates needed
- No dry-run logger updates needed
- Zero breaking changes

**See:** [ADR 001: Handler-Based Architecture](../architecture-decisions/001-handler-based-action-architecture.md)

### Preset Expansion

Presets expand into steps at plan-time:

```
Preset Invocation → Loader.Load() → Validator.Validate() → Expander.Expand() → Steps
```

**Benefits:**

- Flat presets (no nesting)
- Parameter validation
- Type safety
- Full observability

**See:** [ADR 002: Preset Expansion](../architecture-decisions/002-preset-expansion-system.md)

---

## Current Status & Roadmap

### ✅ Production Ready (v0.3.0)

**13 Actions Implemented:**

- shell, command - Execute commands
- file, copy, download, unarchive - File operations
- template - Template rendering
- vars, include_vars - Variable management
- assert - State verification
- preset - Preset expansion
- service - Service management (systemd, launchd)
- print - Output display

**Core Features:**

- ✅ Deterministic plan compiler (parse → plan → execute)
- ✅ Idempotency guarantees (creates, unless, state checks)
- ✅ Dry-run mode (preview without changes)
- ✅ Expression engine (when, changed_when, failed_when)
- ✅ Loop expansion (with_items, with_filetree)
- ✅ System facts (150+ auto-detected facts)
- ✅ Preset system (388+ presets)
- ✅ Service management (systemd, launchd)
- ✅ Cross-platform (Linux, macOS, Windows stubs)
- ✅ Sudo support (interactive, file, env var)

### 🚧 In Progress

**Package Action (High Priority):**

- Auto-detect package manager (apt, dnf, yum, brew, choco)
- Install/remove/upgrade packages
- Cross-platform support
- Idempotent operations

**Windows Support:**

- Complete service action (Windows services)
- Windows-specific facts
- Path handling improvements
- PowerShell integration

**Git Action:**

- Clone repositories
- Pull updates
- Checkout branches/tags
- Sparse checkouts

### 📋 Planned (v0.4.0+)

**User Management:**

- Create/modify/delete users
- Group management
- SSH key management

**Cron/Scheduled Tasks:**

- cron (Linux/macOS)
- launchd periodic jobs (macOS)
- Task Scheduler (Windows)

**Archive Management:**

- tar/zip creation (compression)
- In-place updates
- Selective extraction

**Network Actions:**

- HTTP requests with retries
- WebSocket connections
- DNS queries

**Database Actions:**

- Execute SQL queries
- Database creation/migration
- Backup/restore

### 🔬 Research

**Rollback Support:**

- Automatic backup creation
- Rollback on failure
- Checkpoint/restore

**Parallel Execution:**

- DAG-based dependency resolution
- Concurrent step execution
- Resource locking

**Remote Execution:**

- SSH transport
- Agent-based deployment
- Inventory management

---

## Testing Requirements

### Test Coverage

All code must have tests:

- **Unit tests** for handlers (>80% coverage)
- **Integration tests** for complex workflows
- **Platform tests** for OS-specific code

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# With race detector (CI requirement)
go test ./... -race

# Specific package
go test ./internal/actions/shell

# Verbose output
go test -v ./internal/executor
```

### Test Patterns

**Table-driven tests:**
```go
tests := []struct {
    name    string
    input   Config
    want    Result
    wantErr bool
}{
    {
        name:  "valid config",
        input: Config{Path: "/tmp/test"},
        want:  Result{Changed: true},
        wantErr: false,
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := handler.Execute(ctx, tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
        }
        if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("got %v, want %v", got, tt.want)
        }
    })
}
```

---

## Code Style & Standards

### Go Style

Follow standard Go conventions:

- `gofmt` formatting (enforced by CI)
- Exported functions have doc comments
- Error messages lowercase, no trailing punctuation
- Use `internal/` for non-exported packages

### Linting

All code must pass:
```bash
golangci-lint run
```

Zero warnings allowed. Common issues:

- Unused variables/imports
- Error checking (errcheck)
- Cyclomatic complexity (gocyclo)
- Security issues (gosec)

### Commit Messages

```
<type>: <short description>

<optional detailed explanation>
```

**Types:** feat, fix, docs, refactor, test, chore

**Examples:**
```
feat: add package action with apt/dnf support
fix: resolve race condition in event emitter
docs: update preset authoring guide with examples
```

---

## AI-Specific Guidance

### What AIs Should Focus On

**High-Value Contributions:**

1. **New actions** - Implement missing actions (package, user, cron)
2. **Preset creation** - Add presets for popular tools (300+ presets needed)
3. **Cross-platform support** - Windows implementations
4. **Test coverage** - Increase coverage to 90%+
5. **Documentation** - Examples, guides, API docs

**Avoid:**

- Breaking changes to existing APIs
- Adding dependencies without discussion
- Complex abstractions (keep it simple)
- Over-engineering (solve actual problems)

### Reading the Codebase

**Start here:**

1. `/CLAUDE.md` - AI project instructions
2. `/LLM_GUIDE.md` - Codebase navigation
3. `internal/actions/handler.go` - Core interfaces
4. `internal/actions/shell/handler.go` - Reference implementation
5. `internal/plan/planner.go` - Plan compilation
6. `internal/executor/executor.go` - Execution engine

**Key concepts:**

- Actions are handlers (not hardcoded dispatchers)
- Plans are IR (not direct execution)
- Presets are expanded (not executed directly)
- Results are structured (not free-form)

### Common Patterns

**Get execution context:**
```go
execCtx, ok := ctx.(actions.ExecutionContext)
if !ok {
    return result, fmt.Errorf("invalid context")
}
```

**Render templates:**
```go
rendered, err := execCtx.Evaluator().RenderTemplate(template, vars)
```

**Evaluate expressions:**
```go
result, err := execCtx.Evaluator().Evaluate(expression, vars)
```

**Emit events:**
```go
ctx.Events().Emit(events.Event{
    Type: events.EventStepCompleted,
    Data: events.StepCompletedData{
        StepID:  "step-0001",
        Changed: true,
    },
})
```

---

## Contributing Process

### For AI Agents

1. **Read constraints** in CLAUDE.md (critical)
2. **Check existing code** - don't duplicate
3. **Follow patterns** - match existing style
4. **Write tests** - coverage required
5. **Update docs** - if adding features
6. **Run validation:**
   ```bash
   go test ./...
   go test ./... -race
   golangci-lint run
   ```

### Pull Request Checklist

- [ ] All tests pass (`go test ./...`)
- [ ] Race detector clean (`go test ./... -race`)
- [ ] Linter clean (`golangci-lint run`)
- [ ] Coverage >80% for new code
- [ ] Documentation updated (if applicable)
- [ ] No breaking changes
- [ ] Commit messages follow format

---

## Resources

### Documentation

- **[Actions Guide](../guide/config/actions.md)** - Complete action reference
- **[Variables Guide](../guide/config/variables.md)** - Variables and facts
- **[Control Flow](../guide/config/control-flow.md)** - Conditionals and loops
- **[Preset Authoring](../guide/preset-authoring.md)** - Create presets
- **[Examples](../examples/index.md)** - Runnable examples

### Architecture

- **[ADR 000](../architecture-decisions/000-planner-execution-model.md)** - Planner execution
- **[ADR 001](../architecture-decisions/001-handler-based-action-architecture.md)** - Handler architecture
- **[ADR 002](../architecture-decisions/002-preset-expansion-system.md)** - Preset system

### External

- **[Go Documentation](https://go.dev/doc/)** - Go language reference
- **[Expr Language](https://expr-lang.org/)** - Expression syntax
- **[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)** - Docs framework

---

## Questions?

- **GitHub Issues:** https://github.com/alehatsman/mooncake/issues
- **Discussions:** https://github.com/alehatsman/mooncake/discussions

**For AI agents:** Read CLAUDE.md first, then LLM_GUIDE.md, then start coding. Follow the patterns you see in existing actions.

# Adding New Actions to Mooncake

This guide explains how to add new actions to Mooncake using the handler-based architecture.

## Architecture Overview

Mooncake uses a **handler-based architecture** where each action is a self-contained package implementing a standard interface. This replaces the old approach of spreading action logic across 7+ files.

### Key Components

```
internal/actions/
├── handler.go           # Handler interface definition
├── registry.go          # Thread-safe action registry
├── interfaces.go        # Context and Result interfaces
└── <action_name>/
    └── handler.go       # Self-contained action implementation

internal/register/
└── register.go          # Imports all actions to trigger registration

internal/executor/
└── executor.go          # Dispatches to handlers via registry
```

### The Handler Interface

Every action must implement this 4-method interface:

```go
type Handler interface {
    // Metadata returns action information
    Metadata() ActionMetadata

    // Validate checks if the step configuration is valid
    Validate(step *config.Step) error

    // Execute performs the action and returns a result
    Execute(ctx Context, step *config.Step) (Result, error)

    // DryRun logs what would happen without making changes
    DryRun(ctx Context, step *config.Step) error
}
```

## Step-by-Step Guide

### 1. Create the Action Package

Create a new directory for your action:

```bash
mkdir -p internal/actions/myaction
```

### 2. Implement the Handler

Create `internal/actions/myaction/handler.go`:

```go
// Package myaction implements the myaction action handler.
// Brief description of what this action does.
package myaction

import (
    "fmt"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the myaction action handler.
type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:           "myaction",
        Description:    "Brief description of what this action does",
        Category:       actions.CategorySystem, // or CategoryCommand, CategoryFile, etc.
        SupportsDryRun: true,
    }
}

// Validate validates the action configuration.
func (h *Handler) Validate(step *config.Step) error {
    if step.MyAction == nil {
        return fmt.Errorf("myaction requires configuration")
    }

    // Validate required fields
    if step.MyAction.SomeRequiredField == "" {
        return fmt.Errorf("myaction.some_required_field is required")
    }

    return nil
}

// Execute executes the action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    // Cast context to ExecutionContext for full access
    ec, ok := ctx.(*executor.ExecutionContext)
    if !ok {
        return nil, fmt.Errorf("invalid context type")
    }

    myAction := step.MyAction

    // Render template variables in user input
    renderedValue, err := ec.Template.Render(myAction.SomeField, ec.Variables)
    if err != nil {
        return nil, &executor.RenderError{Field: "myaction.some_field", Cause: err}
    }

    // Perform the action
    // ... your logic here ...

    // Create and return result
    result := executor.NewResult()
    result.Changed = true // or false if idempotent and no change made
    result.Stdout = "Output message"

    return result, nil
}

// DryRun logs what the action would do.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
    ec, ok := ctx.(*executor.ExecutionContext)
    if !ok {
        return fmt.Errorf("invalid context type")
    }

    myAction := step.MyAction

    ec.Logger.Infof("  [DRY-RUN] Would execute myaction with value: %s", myAction.SomeField)

    return nil
}
```

### 3. Add Configuration Struct

Add your action's configuration to `internal/config/config.go`:

```go
// In the Step struct, add your action field:
type Step struct {
    // ... existing fields ...

    MyAction *MyActionConfig `yaml:"myaction" json:"myaction,omitempty"`

    // ... other fields ...
}

// Define your action's configuration:
type MyActionConfig struct {
    SomeRequiredField string `yaml:"some_required_field" json:"some_required_field"`
    OptionalField     string `yaml:"optional_field" json:"optional_field,omitempty"`
}
```

Update the `DetermineActionType()` method:

```go
func (s *Step) DetermineActionType() string {
    // ... existing checks ...

    if s.MyAction != nil {
        return "myaction"
    }

    // ... rest of checks ...
}
```

Update the `countActions()` method:

```go
func (s *Step) countActions() int {
    count := 0
    // ... existing counts ...
    if s.MyAction != nil { count++ }
    // ... rest of counts ...
    return count
}
```

### 4. Register the Handler

Add your action to `internal/register/register.go`:

```go
import (
    // ... existing imports ...
    _ "github.com/alehatsman/mooncake/internal/actions/myaction"
)
```

### 5. Update JSON Schema (Optional but Recommended)

Add your action to `internal/config/schema.json` for validation and IDE support.

### 6. Test Your Action

Create a test YAML file:

```yaml
- name: Test my action
  myaction:
    some_required_field: "test value"
    optional_field: "{{ some_var }}"
  register: result

- name: Show result
  print: "Changed: {{ result.changed }}, Output: {{ result.stdout }}"
```

Run it:

```bash
# Dry-run first
go run cmd/mooncake.go run --config test.yml --dry-run

# Then actual execution
go run cmd/mooncake.go run --config test.yml
```

## Common Patterns

### Rendering Variables

Always render user input that might contain template variables:

```go
rendered, err := ec.Template.Render(input, ec.Variables)
if err != nil {
    return nil, &executor.RenderError{Field: "myaction.field", Cause: err}
}
```

### Error Types

Use typed errors from the executor package:

- `executor.RenderError` - Template rendering failures
- `executor.StepValidationError` - Invalid configuration
- `executor.FileOperationError` - File operations
- `executor.CommandError` - Command execution
- `executor.SetupError` - Infrastructure/setup issues

Example:

```go
return nil, &executor.FileOperationError{
    Operation: "read",
    Path:      path,
    Cause:     err,
}
```

### Idempotency

Check current state before making changes:

```go
// Check if change is needed
currentState, err := checkCurrentState()
if err != nil {
    return nil, err
}

if currentState == desiredState {
    result.Changed = false
    return result, nil
}

// Make the change
if err := applyChange(); err != nil {
    return nil, err
}

result.Changed = true
```

### Sudo/Privilege Escalation

If your action needs elevated privileges, check the `step.Become` field:

```go
if step.Become {
    // Use sudo for operations
    cmd := exec.Command("sudo", "-S", "some-command")
    // ... handle sudo execution ...
}
```

Access sudo password via `ec.SudoPass` if needed.

### Working with Files

Use the PathUtil for path operations:

```go
// Expand ~ and render variables
expandedPath, err := ec.PathUtil.ExpandPath(path, ec.CurrentDir, ec.Variables)
if err != nil {
    return nil, err
}

// Make paths absolute relative to config directory
if !filepath.IsAbs(expandedPath) {
    expandedPath = filepath.Join(ec.CurrentDir, expandedPath)
}
```

### Emitting Events

Emit events for important operations:

```go
ec.EmitEvent(events.EventFileCopied, events.FileCopyData{
    Src:  srcPath,
    Dest: destPath,
})
```

### Result Registration

Results are automatically registered if `step.Register` is set. Just create and return the result:

```go
result := executor.NewResult()
result.Changed = true
result.Stdout = "Command output"
result.Stderr = "Error output"
result.Rc = 0

return result, nil
```

## Categories

Choose the appropriate category in your Metadata:

- `CategoryCommand` - Command execution (shell, command)
- `CategoryFile` - File operations (file, copy, template)
- `CategorySystem` - System management (service, assert)
- `CategoryData` - Data operations (vars, include_vars)
- `CategoryNetwork` - Network operations (download)
- `CategoryOutput` - Output operations (print)

## Examples

### Simple Action (Print)

See `internal/actions/print/handler.go` - ~98 lines, straightforward implementation.

### Medium Complexity (Template)

See `internal/actions/template/handler.go` - ~320 lines, file operations and rendering.

### Complex Action (File)

See `internal/actions/file/handler.go` - ~795 lines, multiple states and operations.

### Very Complex (Service)

See `internal/actions/service/handler.go` - ~1090 lines, platform-specific logic.

## Benefits of This Architecture

1. **Self-contained** - All logic for an action in one file
2. **No dispatcher updates** - Registry handles routing automatically
3. **Type safety** - Compiler enforces Handler interface
4. **Easy testing** - Can test handlers in isolation
5. **Clear contracts** - Interface documents requirements
6. **Less boilerplate** - 1 file vs 7 files per action

## Migration from Old System

If migrating an existing action:

1. Copy logic from `internal/executor/<action>_step.go`
2. Wrap in Handler interface methods
3. Update package references (add `executor.` prefix where needed)
4. Register in `internal/register/register.go`
5. Test thoroughly
6. Keep old implementation until verified

## Checklist

- [ ] Created handler package in `internal/actions/<name>/`
- [ ] Implemented all 4 Handler methods
- [ ] Added `init()` with `actions.Register()`
- [ ] Added config struct to `internal/config/config.go`
- [ ] Updated `DetermineActionType()` in config
- [ ] Updated `countActions()` in config
- [ ] Registered in `internal/register/register.go`
- [ ] Added to JSON schema (optional)
- [ ] Created test YAML file
- [ ] Tested in dry-run mode
- [ ] Tested actual execution
- [ ] Verified all existing tests still pass
- [ ] Added documentation (optional)

## Questions?

See existing handlers in `internal/actions/` for reference implementations.

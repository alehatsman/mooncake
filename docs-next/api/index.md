# API Reference

Complete Go package documentation for Mooncake.

## Core Packages

### [Actions](actions.md)
Action handler registry and interfaces. All actions (shell, file, template, etc.) are registered here.

**Key Interfaces:**
- `Handler` - Base interface for all actions
- `Context` - Execution context passed to handlers
- `Result` - Action execution results

### [Config](config.md)
Configuration structures and validation. Defines the YAML schema for plans and steps.

**Key Types:**
- `Plan` - Top-level configuration
- `Step` - Individual execution steps
- Action structs (Shell, File, Template, etc.)

### [Executor](executor.md)
Execution engine that runs plans and steps. Handles dry-run mode, variable expansion, and result tracking.

**Key Types:**
- `Executor` - Main execution engine
- `ExecutionContext` - Runtime context
- Custom error types (RenderError, CommandError, etc.)

### [Events](events.md)
Event system for execution lifecycle. All events emitted during runs are defined here.

**Key Types:**
- `Event` - Base event structure
- `EventType` - Event type constants
- Event data types (StepStartedData, FileOperationData, etc.)

## System Packages

### [Facts](facts.md)
System information collection. Auto-detects OS, hardware, network, and software facts.

**Key Types:**
- `Facts` - Complete system information
- Platform-specific collectors (Linux, macOS, Windows)

### [Presets](presets.md)
Preset system for reusable workflows. Loads, validates, and expands preset definitions.

**Key Functions:**
- `LoadPreset()` - Load preset from file
- `ValidateParameters()` - Validate preset parameters
- `ExpandSteps()` - Expand preset into steps

### [Logger](logger.md)
Logging infrastructure with TUI and text output modes.

**Key Types:**
- `Logger` - Base logger interface
- `TUILogger` - Terminal UI logger
- `TextLogger` - Plain text logger

## Command Line

### [Commands](cmd.md)
CLI command implementations (run, plan, facts, etc.)

**Commands:**
- `run` - Execute a plan
- `plan` - Generate execution plan
- `facts` - Display system facts

---

## Package Organization

```
mooncake/
├── cmd/               # CLI commands
└── internal/
    ├── actions/       # Action handlers
    ├── config/        # Configuration
    ├── executor/      # Execution engine
    ├── events/        # Event system
    ├── facts/         # System facts
    ├── presets/       # Preset system
    ├── logger/        # Logging
    ├── template/      # Template engine
    ├── expression/    # Expression evaluator
    ├── pathutil/      # Path utilities
    └── utils/         # Shared utilities
```

## Usage Examples

### Implementing a Custom Action

```go
package myaction

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
)

type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:           "myaction",
        Description:    "My custom action",
        Category:       actions.CategorySystem,
        SupportsDryRun: true,
        Version:        "1.0.0",
    }
}

func (h *Handler) Validate(step *config.Step) error {
    // Validate configuration
    return nil
}

func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
    // Implement action logic
    return nil, nil
}

func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
    // Show what would be done
    return nil
}
```

### Using the Executor Programmatically

```go
package main

import (
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
    "github.com/alehatsman/mooncake/internal/logger"
)

func main() {
    // Load plan
    plan, _ := config.LoadPlan("config.yml")

    // Create executor
    log := logger.NewTextLogger()
    exec := executor.NewExecutor(log)

    // Execute
    result, _ := exec.Execute(plan, executor.ExecuteOptions{
        DryRun: false,
    })

    // Check results
    if !result.Success {
        log.Error("Execution failed")
    }
}
```

## External References

- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/alehatsman/mooncake) - Official Go package docs
- [GitHub Repository](https://github.com/alehatsman/mooncake) - Source code
- [User Guide](../guide/core-concepts.md) - Getting started guide

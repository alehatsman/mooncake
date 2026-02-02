# Action System Refactoring Proposal

## Executive Summary

**Problem**: Adding a new action to mooncake currently requires changes in 12+ files and ~1,000 lines of code. The architecture doesn't scale well, with O(N²) schema validation complexity and high maintenance burden.

**Goal**: Reduce new action implementation from ~1,000 lines across 12 files to ~100 lines in 2-3 files while improving maintainability and consistency.

**Approaches**: Two viable paths:
1. **Go Abstractions** (OOP-like patterns) - ~2-3 weeks implementation
2. **Code Generation** (specification-driven) - ~4-5 weeks implementation

This document provides:
- Current pain points analysis
- Two detailed implementation approaches
- Code examples for both approaches
- Migration strategy
- Implementation estimates

---

## Current State Analysis

### What Happens When Adding a New Action Today

Example: Adding a "notify" action (send messages to Slack, email, etc.)

**Required Changes:**
1. **internal/config/config.go** (~20 lines)
   - Define `NotifyAction` struct
   - Add `Notify *NotifyAction` field to `Step`
   - Update `countActions()` (+1 if statement)
   - Update `DetermineActionType()` (+1 if statement)
   - Update `Clone()` (+1 field copy)

2. **internal/config/schema.json** (~150 lines)
   - Add `notify` property reference
   - Add notify to 14 oneOf exclusion blocks (28 lines)
   - Add notify definition section (115 lines)

3. **internal/executor/notify_step.go** (~200-500 lines, new file)
   - Implement `HandleNotify()` function
   - Validation logic
   - Dry-run handling
   - Event emission
   - Result tracking
   - Error handling

4. **internal/executor/executor.go** (~3 lines)
   - Add case to `dispatchStepAction()` switch

5. **internal/executor/dryrun.go** (~10 lines)
   - Add `LogNotifyOperation()` method

6. **internal/events/event.go** (~15 lines)
   - Add event type constants
   - Add event data structs

7. **internal/config/error_messages.go** (~5 lines)
   - Add notify to error messages

**Total: 12+ files, ~1,000+ lines, 4-8 hours of work**

### Key Pain Points

1. **Schema Explosion**: O(N²) growth - each new action adds N lines to existing actions
2. **Boilerplate Repetition**: countActions/DetermineActionType/Clone need manual updates
3. **Inconsistent Patterns**: Different handlers use different result/error/event patterns
4. **Tight Coupling**: Switch dispatch creates central bottleneck
5. **Manual Wiring**: Easy to forget steps, leading to bugs

---

## Approach 1: Go Abstractions (OOP-like Patterns)

**Philosophy**: Use Go interfaces and registration patterns to eliminate boilerplate while keeping code readable and type-safe.

### Core Design

#### 1.1 Action Handler Interface

```go
// internal/actions/handler.go
package actions

import (
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
)

// ActionMetadata describes an action type
type ActionMetadata struct {
    Name        string              // "shell", "file", "notify"
    Description string              // Human-readable description
    Category    ActionCategory      // Command, File, System, Network
    HasDryRun   bool                // Supports dry-run mode
    EmitsEvents []string            // Event types this action emits
}

// ActionCategory groups related actions
type ActionCategory string

const (
    CategoryCommand ActionCategory = "command"
    CategoryFile    ActionCategory = "file"
    CategorySystem  ActionCategory = "system"
    CategoryNetwork ActionCategory = "network"
)

// Handler defines the interface for all action handlers
type Handler interface {
    // Metadata returns action metadata
    Metadata() ActionMetadata

    // Validate checks if the action configuration is valid
    Validate(step *config.Step) error

    // Execute runs the action and returns a result
    Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error)

    // DryRun logs what would happen without executing
    DryRun(ctx *executor.ExecutionContext, step *config.Step) error
}

// Registry manages all registered action handlers
type Registry struct {
    handlers map[string]Handler
}

// Global registry instance
var globalRegistry = NewRegistry()

func NewRegistry() *Registry {
    return &Registry{
        handlers: make(map[string]Handler),
    }
}

func (r *Registry) Register(handler Handler) {
    meta := handler.Metadata()
    r.handlers[meta.Name] = handler
}

func (r *Registry) Get(actionType string) (Handler, bool) {
    h, ok := r.handlers[actionType]
    return h, ok
}

func (r *Registry) List() []ActionMetadata {
    result := make([]ActionMetadata, 0, len(r.handlers))
    for _, h := range r.handlers {
        result = append(result, h.Metadata())
    }
    return result
}

// Register registers a handler globally
func Register(handler Handler) {
    globalRegistry.Register(handler)
}

// Get retrieves a handler by action type
func Get(actionType string) (Handler, bool) {
    return globalRegistry.Get(actionType)
}
```

#### 1.2 Config Step Refactoring

```go
// internal/config/config.go

// ActionConfig is a union type holding exactly one action configuration
type ActionConfig struct {
    Type   string      `json:"type"`           // "shell", "file", etc.
    Config interface{} `json:"config"`         // Actual action config
}

// Step represents a single configuration step
type Step struct {
    // Identification
    Name string `yaml:"name" json:"name,omitempty"`

    // Conditionals
    When string `yaml:"when" json:"when,omitempty"`

    // Action - NEW: unified action field
    Action *ActionConfig `yaml:"action,omitempty" json:"action,omitempty"`

    // DEPRECATED: Keep for backward compatibility during migration
    Template    *Template          `yaml:"template,omitempty" json:"template,omitempty"`
    File        *File              `yaml:"file,omitempty" json:"file,omitempty"`
    Shell       *ShellAction       `yaml:"shell,omitempty" json:"shell,omitempty"`
    // ... other legacy fields

    // Rest of fields unchanged
    Become      bool   `yaml:"become" json:"become,omitempty"`
    Tags        []string `yaml:"tags" json:"tags,omitempty"`
    Register    string   `yaml:"register" json:"register,omitempty"`
    // ...
}

// Helper method to migrate legacy format
func (s *Step) MigrateToActionConfig() {
    if s.Action != nil {
        return // Already using new format
    }

    // Auto-detect which action is set and migrate
    if s.Shell != nil {
        s.Action = &ActionConfig{Type: "shell", Config: s.Shell}
    } else if s.File != nil {
        s.Action = &ActionConfig{Type: "file", Config: s.File}
    }
    // ... for all action types
}

// DetermineActionType returns action type (works with both formats)
func (s *Step) DetermineActionType() string {
    if s.Action != nil {
        return s.Action.Type
    }

    // Legacy detection
    if s.Shell != nil { return "shell" }
    if s.Command != nil { return "command" }
    if s.File != nil { return "file" }
    // ... rest

    return "unknown"
}

// countActions is now trivial
func (s *Step) countActions() int {
    count := 0
    if s.Action != nil { count++ }

    // Count legacy fields
    if s.Template != nil { count++ }
    if s.File != nil { count++ }
    // ... rest

    return count
}

// Clone is now simplified
func (s *Step) Clone() *Step {
    return &Step{
        Name:   s.Name,
        When:   s.When,
        Action: s.Action,
        // Copy other universal fields
        Become:   s.Become,
        Tags:     append([]string{}, s.Tags...),
        Register: s.Register,
        // ...
    }
}
```

#### 1.3 Example Handler Implementation

```go
// internal/actions/shell/handler.go
package shell

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/executor"
    "github.com/alehatsman/mooncake/internal/events"
)

type ShellHandler struct{}

func init() {
    actions.Register(&ShellHandler{})
}

func (h *ShellHandler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:        "shell",
        Description: "Execute shell commands",
        Category:    actions.CategoryCommand,
        HasDryRun:   true,
        EmitsEvents: []string{
            string(events.EventStepStdout),
            string(events.EventStepStderr),
        },
    }
}

func (h *ShellHandler) Validate(step *config.Step) error {
    shell := step.Shell
    if shell == nil {
        return fmt.Errorf("shell action is nil")
    }
    if shell.Cmd == "" {
        return fmt.Errorf("shell command is empty")
    }
    return nil
}

func (h *ShellHandler) Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error) {
    shell := step.Shell

    // Existing shell execution logic here
    result := &executor.Result{
        Changed: true,
        Stdout:  output,
        Stderr:  stderr,
        Rc:      exitCode,
    }

    // Emit events
    ctx.EmitEvent(events.EventStepStdout, events.StepOutputData{
        StepID: ctx.CurrentStepID,
        Stream: "stdout",
        Line:   output,
    })

    return result, nil
}

func (h *ShellHandler) DryRun(ctx *executor.ExecutionContext, step *config.Step) error {
    shell := step.Shell
    ctx.Logger.Infof("  [DRY-RUN] Would execute: %s", shell.Cmd)
    if step.Become {
        ctx.Logger.Infof("  [DRY-RUN] With sudo privileges")
    }
    return nil
}
```

#### 1.4 Unified Dispatcher

```go
// internal/executor/executor.go

func dispatchStepAction(step config.Step, ec *ExecutionContext) error {
    // Migrate legacy format if needed
    step.MigrateToActionConfig()

    // Get action type
    actionType := step.DetermineActionType()
    if actionType == "unknown" {
        return fmt.Errorf("unknown action type")
    }

    // Get handler from registry
    handler, ok := actions.Get(actionType)
    if !ok {
        return fmt.Errorf("no handler registered for action: %s", actionType)
    }

    // Validate
    if err := handler.Validate(&step); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Handle dry-run
    if ec.DryRun {
        return handler.DryRun(ec, &step)
    }

    // Execute
    result, err := handler.Execute(ec, &step)
    if err != nil {
        return err
    }

    // Store result for registration
    ec.CurrentResult = result

    // Register if needed
    if step.Register != "" {
        result.RegisterTo(ec.Variables, step.Register)
    }

    return nil
}
```

#### 1.5 Schema Generation

```go
// internal/config/schemagen/generator.go
package schemagen

import (
    "encoding/json"
    "reflect"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
)

// GenerateSchema generates JSON schema from registered actions
func GenerateSchema() (map[string]interface{}, error) {
    schema := map[string]interface{}{
        "$schema": "http://json-schema.org/draft-07/schema#",
        "type":    "array",
        "items": map[string]interface{}{
            "$ref": "#/definitions/step",
        },
        "definitions": map[string]interface{}{
            "step": generateStepSchema(),
        },
    }

    // Add action definitions
    definitions := schema["definitions"].(map[string]interface{})
    for _, meta := range actions.List() {
        definitions[meta.Name] = generateActionSchema(meta)
    }

    return schema, nil
}

func generateStepSchema() map[string]interface{} {
    properties := map[string]interface{}{
        "name":     map[string]interface{}{"type": "string"},
        "when":     map[string]interface{}{"type": "string"},
        "become":   map[string]interface{}{"type": "boolean"},
        "register": map[string]interface{}{"type": "string"},
        "tags": map[string]interface{}{
            "type":  "array",
            "items": map[string]interface{}{"type": "string"},
        },
    }

    // Add all action properties
    for _, meta := range actions.List() {
        properties[meta.Name] = map[string]interface{}{
            "$ref": "#/definitions/" + meta.Name,
        }
    }

    // Generate oneOf blocks (mutual exclusivity)
    oneOf := generateMutualExclusivity()

    return map[string]interface{}{
        "type":       "object",
        "properties": properties,
        "oneOf":      oneOf,
    }
}

func generateMutualExclusivity() []map[string]interface{} {
    actionNames := []string{}
    for _, meta := range actions.List() {
        actionNames = append(actionNames, meta.Name)
    }

    oneOf := []map[string]interface{}{}

    for i, name := range actionNames {
        // Build exclusion list (all actions except this one)
        exclusions := []map[string]interface{}{}
        for j, otherName := range actionNames {
            if i != j {
                exclusions = append(exclusions, map[string]interface{}{
                    "required": []string{otherName},
                })
            }
        }

        oneOf = append(oneOf, map[string]interface{}{
            "required": []string{name},
            "not": map[string]interface{}{
                "anyOf": exclusions,
            },
        })
    }

    return oneOf
}

func generateActionSchema(meta actions.ActionMetadata) map[string]interface{} {
    // Use reflection to generate schema from struct tags
    // This would introspect the action config struct
    // and generate properties based on yaml/json tags

    // Simplified example:
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            // Auto-generated from struct
        },
    }
}
```

### Approach 1 Benefits

✅ **Pros:**
- Minimal breaking changes (backward compatible)
- Standard Go patterns (interfaces, registration)
- Type-safe at compile time
- Easy to understand for Go developers
- Can migrate incrementally
- ~60% reduction in boilerplate
- Estimated: 2-3 weeks implementation

❌ **Cons:**
- Still requires manual handler implementation
- Reflection-based schema generation adds complexity
- Runtime registration can be implicit (init functions)

---

## Approach 2: Code Generation (Specification-Driven)

**Philosophy**: Define actions in a simple spec file, generate all the boilerplate automatically.

### Core Design

#### 2.1 Action Specification Format

```yaml
# internal/actions/specs/notify.action.yaml
action:
  name: notify
  description: Send notifications to various channels
  category: system

  # Configuration structure
  config:
    channel:
      type: string
      required: true
      description: Notification channel (slack, email, webhook)
      enum: [slack, email, webhook]

    message:
      type: string
      required: true
      description: Message to send (supports templates)

    webhook_url:
      type: string
      required_if: channel == "webhook"
      description: Webhook URL

    email_to:
      type: string
      required_if: channel == "email"
      description: Recipient email address

  # Handler configuration
  handler:
    supports_dry_run: true
    supports_sudo: false
    emits_events:
      - notify.message_sent

    # Generated event data
    event_data:
      - name: channel
        type: string
      - name: message
        type: string
      - name: status
        type: string

  # Error messages
  errors:
    - code: channel_invalid
      message: "invalid notification channel: {channel}"
    - code: webhook_failed
      message: "webhook request failed: {error}"
```

#### 2.2 Code Generator

```go
// cmd/mooncakegen/main.go
package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "path/filepath"

    "github.com/alehatsman/mooncake/internal/codegen"
)

func main() {
    specDir := flag.String("specs", "internal/actions/specs", "Directory with action specs")
    outDir := flag.String("out", "internal/actions/generated", "Output directory")
    flag.Parse()

    // Load all action specs
    specs, err := codegen.LoadSpecs(*specDir)
    if err != nil {
        log.Fatal(err)
    }

    // Generate code for each spec
    generator := codegen.NewGenerator()
    for _, spec := range specs {
        files, err := generator.Generate(spec)
        if err != nil {
            log.Fatalf("Failed to generate %s: %v", spec.Name, err)
        }

        // Write generated files
        for filename, content := range files {
            outPath := filepath.Join(*outDir, spec.Name, filename)
            if err := ioutil.WriteFile(outPath, []byte(content), 0644); err != nil {
                log.Fatal(err)
            }
        }

        fmt.Printf("Generated action: %s\n", spec.Name)
    }

    // Generate schema
    schema := generator.GenerateSchema(specs)
    if err := ioutil.WriteFile("internal/config/schema.json", []byte(schema), 0644); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Generated schema.json")
}
```

#### 2.3 Generated Code Example

From `notify.action.yaml`, generate:

**A. Config struct** (`internal/actions/generated/notify/config.go`):
```go
// Code generated by mooncakegen. DO NOT EDIT.
package notify

type NotifyConfig struct {
    Channel    string `yaml:"channel" json:"channel"`
    Message    string `yaml:"message" json:"message"`
    WebhookURL string `yaml:"webhook_url,omitempty" json:"webhook_url,omitempty"`
    EmailTo    string `yaml:"email_to,omitempty" json:"email_to,omitempty"`
}

func (c *NotifyConfig) Validate() error {
    if c.Channel == "" {
        return fmt.Errorf("channel is required")
    }

    validChannels := []string{"slack", "email", "webhook"}
    if !contains(validChannels, c.Channel) {
        return fmt.Errorf("invalid notification channel: %s", c.Channel)
    }

    if c.Message == "" {
        return fmt.Errorf("message is required")
    }

    // Conditional validation
    if c.Channel == "webhook" && c.WebhookURL == "" {
        return fmt.Errorf("webhook_url is required when channel is webhook")
    }

    if c.Channel == "email" && c.EmailTo == "" {
        return fmt.Errorf("email_to is required when channel is email")
    }

    return nil
}
```

**B. Handler skeleton** (`internal/actions/generated/notify/handler.go`):
```go
// Code generated by mooncakegen. DO NOT EDIT.
package notify

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/executor"
    "github.com/alehatsman/mooncake/internal/events"
)

type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:        "notify",
        Description: "Send notifications to various channels",
        Category:    actions.CategorySystem,
        HasDryRun:   true,
        EmitsEvents: []string{"notify.message_sent"},
    }
}

func (h *Handler) Validate(step *config.Step) error {
    cfg := step.Notify
    if cfg == nil {
        return fmt.Errorf("notify config is nil")
    }
    return cfg.Validate()
}

func (h *Handler) Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error) {
    cfg := step.Notify

    // TODO: Implement actual notification logic
    // This is where developer adds custom code

    result := &executor.Result{
        Changed: true,
        Data: map[string]interface{}{
            "channel": cfg.Channel,
            "message": cfg.Message,
            "status":  "sent",
        },
    }

    // Auto-generated event emission
    ctx.EmitEvent(events.EventNotifyMessageSent, events.NotifyData{
        Channel: cfg.Channel,
        Message: cfg.Message,
        Status:  "sent",
    })

    return result, nil
}

func (h *Handler) DryRun(ctx *executor.ExecutionContext, step *config.Step) error {
    cfg := step.Notify
    ctx.Logger.Infof("  [DRY-RUN] Would send notification to %s: %s", cfg.Channel, cfg.Message)
    return nil
}
```

**C. Event types** (`internal/events/generated_notify.go`):
```go
// Code generated by mooncakegen. DO NOT EDIT.
package events

const EventNotifyMessageSent EventType = "notify.message_sent"

type NotifyData struct {
    Channel string `json:"channel"`
    Message string `json:"message"`
    Status  string `json:"status"`
}
```

**D. JSON Schema fragment** (merged into `schema.json`):
```json
{
  "notify": {
    "type": "object",
    "properties": {
      "channel": {
        "type": "string",
        "enum": ["slack", "email", "webhook"],
        "description": "Notification channel"
      },
      "message": {
        "type": "string",
        "description": "Message to send (supports templates)"
      },
      "webhook_url": {
        "type": "string",
        "description": "Webhook URL"
      },
      "email_to": {
        "type": "string",
        "description": "Recipient email address"
      }
    },
    "required": ["channel", "message"]
  }
}
```

#### 2.4 Developer Workflow

**Adding a new action:**

1. **Create spec file**: `internal/actions/specs/notify.action.yaml`
2. **Run generator**: `make generate-actions`
3. **Implement handler**: Edit `internal/actions/generated/notify/handler.go`
4. **Test**: Write tests in `internal/actions/generated/notify/handler_test.go`
5. **Done**: All wiring is automatic

**Generator command:**
```makefile
# Makefile
.PHONY: generate-actions
generate-actions:
	go run cmd/mooncakegen/main.go \
		-specs internal/actions/specs \
		-out internal/actions/generated
	go fmt ./internal/actions/generated/...
```

### Approach 2 Benefits

✅ **Pros:**
- **Maximum boilerplate reduction** (~90%)
- **Single source of truth** (spec file)
- **Consistent patterns** across all actions
- **Easy for junior developers** (just fill in handler logic)
- **Schema is always in sync** with code
- **Documentation can be generated** from specs
- **Validation is auto-generated**
- **Breaking changes are caught** at generation time

❌ **Cons:**
- **Higher initial investment** (~4-5 weeks)
- **Generator complexity** (needs maintenance)
- **Generated code can be hard to debug**
- **Requires build step** (can be automated)
- **Learning curve for spec format**

---

## Comparison Matrix

| Aspect | Current | Approach 1 (Abstractions) | Approach 2 (Codegen) |
|--------|---------|---------------------------|----------------------|
| **Lines to add action** | ~1,000 | ~400 | ~150 (100 spec + 50 impl) |
| **Files to modify** | 12+ | 3-4 | 1 (spec) |
| **Schema maintenance** | Manual (N²) | Generated (N) | Generated (N) |
| **Type safety** | Compile-time | Compile-time | Compile-time |
| **Breaking changes** | Runtime errors | Compile errors | Generate-time errors |
| **Consistency** | Manual enforcement | Interface enforcement | Spec enforcement |
| **Developer complexity** | High | Medium | Low (after generator built) |
| **Implementation time** | - | 2-3 weeks | 4-5 weeks |
| **Maintenance burden** | High | Medium | Low |
| **Debugging difficulty** | Easy | Easy | Medium (generated code) |

---

## Migration Strategy

### Phase 1: Foundation (Week 1-2)
**For Approach 1:**
- Create `internal/actions` package
- Define Handler interface and Registry
- Implement registration system
- Update executor dispatcher

**For Approach 2:**
- Design spec format
- Implement spec parser
- Create code generator framework
- Set up build tooling

### Phase 2: Pilot Actions (Week 2-3)
- Migrate 2-3 simple actions (print, vars) using new system
- Test backward compatibility
- Validate approach with real actions
- Gather feedback

### Phase 3: Bulk Migration (Week 3-4)
- Migrate remaining actions
- Update documentation
- Generate updated schema
- Run full test suite

### Phase 4: Cleanup (Week 4-5 for Approach 2)
- Remove deprecated legacy code
- Update examples and docs
- Final testing and validation

---

## Recommendations

### For Small Teams (1-2 developers)
**Recommend: Approach 1 (Go Abstractions)**
- Faster implementation
- Lower risk
- Standard Go patterns
- Easier to debug
- Still achieves 60% boilerplate reduction

### For Growing Teams (3+ developers, hiring planned)
**Recommend: Approach 2 (Code Generation)**
- Best long-term investment
- Maximum productivity gain
- Junior developers can add actions easily
- Scales to 50+ actions without issues
- Worth the upfront cost

### Hybrid Approach
Start with Approach 1, build generator later:
1. Week 1-3: Implement registry and interface pattern
2. Week 4-6: Use new system, gather learnings
3. Month 2-3: Build generator based on proven patterns
4. Benefit from both: faster start, better end state

---

## Implementation Checklist

### Approach 1 Tasks
- [ ] Create action handler interface
- [ ] Implement registry system
- [ ] Create base action struct
- [ ] Update Step struct with union type
- [ ] Implement backward compatibility layer
- [ ] Refactor executor dispatcher
- [ ] Create schema generation from registry
- [ ] Migrate 3 pilot actions
- [ ] Write migration guide
- [ ] Update documentation
- [ ] Migrate remaining actions
- [ ] Remove deprecated code

### Approach 2 Tasks
- [ ] Design action spec format
- [ ] Implement spec parser
- [ ] Create code generator:
  - [ ] Config struct generator
  - [ ] Handler skeleton generator
  - [ ] Event types generator
  - [ ] Schema generator
  - [ ] Validation generator
- [ ] Set up Makefile integration
- [ ] Create 2 example specs
- [ ] Generate and test pilot actions
- [ ] Write spec authoring guide
- [ ] Migrate all actions to specs
- [ ] Update build process
- [ ] Documentation and examples

---

## Cost Estimates

### Development Time
- **Approach 1**: 80-120 hours (2-3 weeks)
- **Approach 2**: 160-200 hours (4-5 weeks)

### Maintenance Savings (per year)
- **Current**: ~40 hours/year for 5 new actions + schema updates
- **Approach 1**: ~15 hours/year
- **Approach 2**: ~8 hours/year

### ROI Breakeven
- **Approach 1**: ~6 months
- **Approach 2**: ~12 months

---

## Next Steps

1. **Decision**: Choose approach based on team size and timeline
2. **Spike**: 1-week prototype to validate chosen approach
3. **Review**: Assess prototype, adjust design if needed
4. **Implementation**: Follow migration strategy
5. **Documentation**: Update developer guides
6. **Training**: Onboard team on new patterns

---

## Questions for Discussion

1. What's your expected growth in number of actions over next year?
2. How many developers will be adding new actions?
3. What's your risk tolerance for breaking changes?
4. Do you prefer faster delivery or better long-term architecture?
5. Any existing code generation tools in your toolchain?

---

**Document Version**: 1.0
**Date**: 2026-02-05
**Author**: Architecture Analysis Agent

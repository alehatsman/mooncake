# Approach 1 (Abstractions): Adding a "notify" Action

This shows what's required with the Go abstractions approach.

## Required Changes

### 1. internal/actions/notify/handler.go (~200 lines, NEW FILE)

```go
package notify

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/events"
    "github.com/alehatsman/mooncake/internal/executor"
)

// Config defines the notify action configuration
type Config struct {
    Channel string `yaml:"channel" json:"channel"`
    Message string `yaml:"message" json:"message"`
    URL     string `yaml:"url,omitempty" json:"url,omitempty"`
}

// Handler implements the notify action
type Handler struct{}

// Register this action on import
func init() {
    actions.Register(&Handler{})
}

// Metadata returns action metadata
func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:        "notify",
        Description: "Send notifications to various channels",
        Category:    actions.CategorySystem,
        HasDryRun:   true,
        EmitsEvents: []string{string(events.EventNotifySent)},
        ConfigType:  &Config{},  // For reflection-based validation
    }
}

// Validate checks if the configuration is valid
func (h *Handler) Validate(step *config.Step) error {
    cfg, err := h.getConfig(step)
    if err != nil {
        return err
    }

    validChannels := []string{"slack", "email", "webhook"}
    if !contains(validChannels, cfg.Channel) {
        return fmt.Errorf("channel must be one of: %v", validChannels)
    }

    if cfg.Channel == "webhook" && cfg.URL == "" {
        return fmt.Errorf("url is required for webhook channel")
    }

    if cfg.Message == "" {
        return fmt.Errorf("message is required")
    }

    return nil
}

// Execute runs the notification action
func (h *Handler) Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error) {
    cfg, err := h.getConfig(step)
    if err != nil {
        return nil, err
    }

    // Render message template
    renderedMsg, err := ctx.Template.Render(cfg.Message, ctx.Variables)
    if err != nil {
        return nil, fmt.Errorf("failed to render message: %w", err)
    }

    // Send notification
    var status string
    var sendErr error

    switch cfg.Channel {
    case "slack":
        status, sendErr = h.sendSlack(cfg, renderedMsg)
    case "email":
        status, sendErr = h.sendEmail(cfg, renderedMsg)
    case "webhook":
        status, sendErr = h.sendWebhook(cfg, renderedMsg)
    }

    if sendErr != nil {
        return nil, fmt.Errorf("notification failed: %w", sendErr)
    }

    // Emit event
    ctx.EmitEvent(events.EventNotifySent, events.NotifyData{
        Channel: cfg.Channel,
        Message: renderedMsg,
        Status:  status,
    })

    // Return result
    return &executor.Result{
        Changed: true,
        Data: map[string]interface{}{
            "channel": cfg.Channel,
            "message": renderedMsg,
            "status":  status,
        },
    }, nil
}

// DryRun logs what would happen
func (h *Handler) DryRun(ctx *executor.ExecutionContext, step *config.Step) error {
    cfg, err := h.getConfig(step)
    if err != nil {
        return err
    }

    renderedMsg, err := ctx.Template.Render(cfg.Message, ctx.Variables)
    if err != nil {
        renderedMsg = cfg.Message + " (template render failed)"
    }

    ctx.Logger.Infof("  [DRY-RUN] Would send notification:")
    ctx.Logger.Infof("    Channel: %s", cfg.Channel)
    ctx.Logger.Infof("    Message: %s", renderedMsg)
    if cfg.URL != "" {
        ctx.Logger.Infof("    URL: %s", cfg.URL)
    }

    return nil
}

// getConfig extracts config from step
func (h *Handler) getConfig(step *config.Step) (*Config, error) {
    if step.Notify == nil {
        return nil, fmt.Errorf("notify config is nil")
    }
    return step.Notify, nil
}

// Implementation methods
func (h *Handler) sendSlack(cfg *Config, message string) (string, error) {
    payload := map[string]interface{}{"text": message}
    body, _ := json.Marshal(payload)

    resp, err := http.Post(cfg.URL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("slack returned status %d", resp.StatusCode)
    }

    return "sent", nil
}

func (h *Handler) sendEmail(cfg *Config, message string) (string, error) {
    // Email implementation
    return "sent", nil
}

func (h *Handler) sendWebhook(cfg *Config, message string) (string, error) {
    payload := map[string]interface{}{"message": message}
    body, _ := json.Marshal(payload)

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Post(cfg.URL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }

    return "sent", nil
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

### 2. internal/config/config.go (~3 lines)

```go
// Just add the config struct reference
type Step struct {
    // ... existing fields ...

    Notify *notify.Config `yaml:"notify,omitempty" json:"notify,omitempty"`  // ADD THIS

    // ... rest unchanged ...
}

// countActions, DetermineActionType, Clone are AUTO-GENERATED or use reflection
// No manual updates needed!
```

### 3. internal/events/event.go (~8 lines)

```go
const (
    EventNotifySent EventType = "notify.sent"
)

type NotifyData struct {
    Channel string `json:"channel"`
    Message string `json:"message"`
    Status  string `json:"status"`
}
```

### 4. Auto-Generated Files

**internal/config/schema.json** - Generated by `make generate-schema`

**internal/executor/executor.go** - Dispatcher uses registry (no changes!)

**internal/executor/dryrun.go** - Dry-run handled by handler (no changes!)

---

## Total Impact

**Files Modified**: 3 (handler + config + events)
**Lines Added**: ~215
**Time Required**: 2-3 hours
**Maintenance Burden**: Minimal - just implement the 4 interface methods

**Improvements over Current:**
- ✅ No schema.json editing (auto-generated)
- ✅ No dispatcher updates (registry-based)
- ✅ No helper method updates (reflection-based)
- ✅ No dry-run logger methods (handled by handler)
- ✅ Standardized structure (interface enforcement)
- ✅ Type-safe registration at compile time

**Remaining Manual Work:**
- Define config struct
- Implement 4 interface methods (Metadata, Validate, Execute, DryRun)
- Add event types

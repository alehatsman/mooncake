# Approach 2 (Code Generation): Adding a "notify" Action

This shows what's required with the code generation approach.

## Required Changes

### 1. internal/actions/specs/notify.action.yaml (~60 lines, NEW FILE)

```yaml
# Action specification for the notify action
action:
  name: notify
  description: Send notifications to various channels (Slack, email, webhooks)
  category: system
  version: "1.0.0"

  # Configuration schema
  config:
    channel:
      type: string
      required: true
      description: Notification channel to use
      enum:
        - slack
        - email
        - webhook
      default: webhook

    message:
      type: string
      required: true
      description: Message to send (supports template variables)

    url:
      type: string
      required_if: channel == "webhook" || channel == "slack"
      description: Webhook or Slack webhook URL
      validation: url

    email_to:
      type: string
      required_if: channel == "email"
      description: Recipient email address
      validation: email

    timeout:
      type: duration
      description: Request timeout for webhooks
      default: "30s"

  # Handler configuration
  handler:
    supports_dry_run: true
    supports_become: false
    supports_check_mode: true

    # Events emitted by this action
    events:
      - name: notify.sent
        description: Emitted when notification is successfully sent
        data:
          - name: channel
            type: string
            description: The channel used
          - name: message
            type: string
            description: The rendered message
          - name: status
            type: string
            description: Delivery status

  # Error definitions
  errors:
    - code: invalid_channel
      message: "invalid notification channel: {channel}"

    - code: webhook_failed
      message: "webhook request failed: {error}"

    - code: email_failed
      message: "failed to send email: {error}"

  # Examples for documentation
  examples:
    - name: Send Slack notification
      yaml: |
        - name: Notify team
          notify:
            channel: slack
            url: "{{ slack_webhook_url }}"
            message: "Deployment complete for {{ app_name }}"

    - name: Send webhook notification
      yaml: |
        - name: Trigger webhook
          notify:
            channel: webhook
            url: "https://example.com/hook"
            message: "Event occurred"
```

### 2. internal/actions/notify/custom.go (~100 lines, NEW FILE)

This is the ONLY code you write - the implementation logic:

```go
package notify

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/alehatsman/mooncake/internal/executor"
)

// ExecuteNotification is the custom implementation logic
// This is called by the generated Execute() method
func (h *Handler) ExecuteNotification(ctx *executor.ExecutionContext, cfg *Config) (map[string]interface{}, error) {
    // Render message template
    renderedMsg, err := ctx.Template.Render(cfg.Message, ctx.Variables)
    if err != nil {
        return nil, fmt.Errorf("failed to render message: %w", err)
    }

    // Send notification based on channel
    var status string
    var sendErr error

    switch cfg.Channel {
    case "slack":
        status, sendErr = sendSlackNotification(cfg, renderedMsg)
    case "email":
        status, sendErr = sendEmailNotification(cfg, renderedMsg)
    case "webhook":
        status, sendErr = sendWebhookNotification(cfg, renderedMsg)
    default:
        return nil, fmt.Errorf("unsupported channel: %s", cfg.Channel)
    }

    if sendErr != nil {
        return nil, sendErr
    }

    // Return result data (will be auto-wrapped by generated code)
    return map[string]interface{}{
        "channel": cfg.Channel,
        "message": renderedMsg,
        "status":  status,
    }, nil
}

// Helper functions
func sendSlackNotification(cfg *Config, message string) (string, error) {
    payload := map[string]interface{}{
        "text": message,
    }

    body, _ := json.Marshal(payload)
    resp, err := http.Post(cfg.URL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", fmt.Errorf("slack request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("slack returned status %d", resp.StatusCode)
    }

    return "sent", nil
}

func sendEmailNotification(cfg *Config, message string) (string, error) {
    // Email implementation
    return "sent", nil
}

func sendWebhookNotification(cfg *Config, message string) (string, error) {
    payload := map[string]interface{}{
        "message": message,
    }

    body, _ := json.Marshal(payload)
    client := &http.Client{Timeout: cfg.Timeout}

    resp, err := client.Post(cfg.URL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", fmt.Errorf("webhook request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }

    return "sent", nil
}
```

### 3. Run Generator

```bash
make generate-actions
```

This command generates ALL the following files automatically:

---

## Auto-Generated Files (You Never Touch These)

### internal/actions/notify/config.go (Generated)

```go
// Code generated by mooncakegen. DO NOT EDIT.
package notify

import "time"

type Config struct {
    Channel  string        `yaml:"channel" json:"channel"`
    Message  string        `yaml:"message" json:"message"`
    URL      string        `yaml:"url,omitempty" json:"url,omitempty"`
    EmailTo  string        `yaml:"email_to,omitempty" json:"email_to,omitempty"`
    Timeout  time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// Validate checks configuration validity
func (c *Config) Validate() error {
    validChannels := []string{"slack", "email", "webhook"}
    if !contains(validChannels, c.Channel) {
        return newInvalidChannelError(c.Channel)
    }

    if c.Message == "" {
        return fmt.Errorf("message is required")
    }

    // Conditional validation
    if (c.Channel == "webhook" || c.Channel == "slack") && c.URL == "" {
        return fmt.Errorf("url is required for %s channel", c.Channel)
    }

    if c.Channel == "email" && c.EmailTo == "" {
        return fmt.Errorf("email_to is required for email channel")
    }

    return nil
}

func (c *Config) SetDefaults() {
    if c.Channel == "" {
        c.Channel = "webhook"
    }
    if c.Timeout == 0 {
        c.Timeout = 30 * time.Second
    }
}
```

### internal/actions/notify/handler.go (Generated)

```go
// Code generated by mooncakegen. DO NOT EDIT.
package notify

import (
    "github.com/alehatsman/mooncake/internal/actions"
    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/events"
    "github.com/alehatsman/mooncake/internal/executor"
)

type Handler struct{}

func init() {
    actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
    return actions.ActionMetadata{
        Name:        "notify",
        Description: "Send notifications to various channels (Slack, email, webhooks)",
        Category:    actions.CategorySystem,
        HasDryRun:   true,
        EmitsEvents: []string{"notify.sent"},
        Version:     "1.0.0",
    }
}

func (h *Handler) Validate(step *config.Step) error {
    cfg := step.Notify
    if cfg == nil {
        return fmt.Errorf("notify config is nil")
    }
    cfg.SetDefaults()
    return cfg.Validate()
}

func (h *Handler) Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error) {
    cfg := step.Notify
    cfg.SetDefaults()

    // Call custom implementation (defined in custom.go)
    data, err := h.ExecuteNotification(ctx, cfg)
    if err != nil {
        return nil, err
    }

    // Auto-emit event
    ctx.EmitEvent(events.EventNotifySent, events.NotifyData{
        Channel: cfg.Channel,
        Message: data["message"].(string),
        Status:  data["status"].(string),
    })

    // Return result
    return &executor.Result{
        Changed: true,
        Data:    data,
    }, nil
}

func (h *Handler) DryRun(ctx *executor.ExecutionContext, step *config.Step) error {
    cfg := step.Notify
    cfg.SetDefaults()

    // Attempt to render message
    renderedMsg, err := ctx.Template.Render(cfg.Message, ctx.Variables)
    if err != nil {
        renderedMsg = cfg.Message + " (template render failed)"
    }

    // Auto-generated dry-run logging
    ctx.Logger.Infof("  [DRY-RUN] Would send notification:")
    ctx.Logger.Infof("    Channel: %s", cfg.Channel)
    ctx.Logger.Infof("    Message: %s", renderedMsg)
    if cfg.URL != "" {
        ctx.Logger.Infof("    URL: %s", cfg.URL)
    }
    if cfg.EmailTo != "" {
        ctx.Logger.Infof("    To: %s", cfg.EmailTo)
    }

    return nil
}
```

### internal/actions/notify/errors.go (Generated)

```go
// Code generated by mooncakegen. DO NOT EDIT.
package notify

import "fmt"

type InvalidChannelError struct {
    Channel string
}

func (e *InvalidChannelError) Error() string {
    return fmt.Sprintf("invalid notification channel: %s", e.Channel)
}

func newInvalidChannelError(channel string) error {
    return &InvalidChannelError{Channel: channel}
}

type WebhookFailedError struct {
    Err error
}

func (e *WebhookFailedError) Error() string {
    return fmt.Sprintf("webhook request failed: %v", e.Err)
}

func (e *WebhookFailedError) Unwrap() error {
    return e.Err
}

func newWebhookFailedError(err error) error {
    return &WebhookFailedError{Err: err}
}

// ... similar for other errors
```

### internal/events/notify_events.go (Generated)

```go
// Code generated by mooncakegen. DO NOT EDIT.
package events

const (
    EventNotifySent EventType = "notify.sent"
)

type NotifyData struct {
    Channel string `json:"channel"` // The channel used
    Message string `json:"message"` // The rendered message
    Status  string `json:"status"`  // Delivery status
}
```

### internal/config/schema_notify.json (Generated, merged into schema.json)

```json
{
  "notify": {
    "type": "object",
    "properties": {
      "channel": {
        "type": "string",
        "enum": ["slack", "email", "webhook"],
        "default": "webhook",
        "description": "Notification channel to use"
      },
      "message": {
        "type": "string",
        "description": "Message to send (supports template variables)"
      },
      "url": {
        "type": "string",
        "format": "uri",
        "description": "Webhook or Slack webhook URL"
      },
      "email_to": {
        "type": "string",
        "format": "email",
        "description": "Recipient email address"
      },
      "timeout": {
        "type": "string",
        "pattern": "^[0-9]+(ns|us|µs|ms|s|m|h)$",
        "default": "30s",
        "description": "Request timeout for webhooks"
      }
    },
    "required": ["channel", "message"]
  }
}
```

### docs/actions/notify.md (Generated documentation)

```markdown
# notify

Send notifications to various channels (Slack, email, webhooks)

## Configuration

### channel (string, required)

Notification channel to use

**Valid values:** `slack`, `email`, `webhook`
**Default:** `webhook`

### message (string, required)

Message to send (supports template variables)

### url (string)

Webhook or Slack webhook URL

**Required when:** channel is "webhook" or "slack"

### email_to (string)

Recipient email address

**Required when:** channel is "email"

### timeout (duration)

Request timeout for webhooks

**Default:** `30s`

## Examples

### Send Slack notification

```yaml
- name: Notify team
  notify:
    channel: slack
    url: "{{ slack_webhook_url }}"
    message: "Deployment complete for {{ app_name }}"
```

### Send webhook notification

```yaml
- name: Trigger webhook
  notify:
    channel: webhook
    url: "https://example.com/hook"
    message: "Event occurred"
```

## Events

- `notify.sent` - Emitted when notification is successfully sent

## Error Codes

- `invalid_channel` - invalid notification channel
- `webhook_failed` - webhook request failed
- `email_failed` - failed to send email
```

---

## Total Impact

**Files You Create**: 2 (spec + custom logic)
**Lines You Write**: ~160 (60 spec + 100 implementation)
**Time Required**: 1-2 hours
**Maintenance Burden**: Almost zero - generator handles everything

**Auto-Generated (Never Touch):**
- ✅ Config struct with validation
- ✅ Handler with all 4 interface methods
- ✅ Error types
- ✅ Event definitions
- ✅ JSON schema
- ✅ Documentation
- ✅ Default values handling
- ✅ Dry-run logging
- ✅ Event emission

**Your Work:**
- Write spec file (declares structure)
- Implement business logic (1 function)
- Done!

**Comparison:**
- Current approach: ~600 lines across 7 files
- Approach 1: ~215 lines across 3 files
- Approach 2: ~160 lines across 2 files (rest generated)

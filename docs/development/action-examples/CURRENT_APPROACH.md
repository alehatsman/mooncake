# Current Approach: Adding a "notify" Action

This shows what's required TODAY to add a simple notification action.

## Required Changes

### 1. internal/config/config.go (~25 lines)

```go
// Add new action struct
type NotifyAction struct {
    Channel string `yaml:"channel" json:"channel"`             // slack, email, webhook
    Message string `yaml:"message" json:"message"`             // Message to send
    URL     string `yaml:"url,omitempty" json:"url,omitempty"` // Webhook URL
}

// Add field to Step struct (line ~312)
type Step struct {
    // ... existing fields ...

    Notify *NotifyAction `yaml:"notify" json:"notify,omitempty"` // ADD THIS

    // ... rest of fields ...
}

// Update countActions() (line ~395)
func (s *Step) countActions() int {
    count := 0
    // ... existing conditions ...
    if s.Notify != nil {  // ADD THIS
        count++
    }
    return count
}

// Update DetermineActionType() (line ~450)
func (s *Step) DetermineActionType() string {
    // ... existing conditions ...
    if s.Notify != nil {  // ADD THIS
        return "notify"
    }
    return "unknown"
}

// Update Clone() (line ~515)
func (s *Step) Clone() *Step {
    return &Step{
        // ... existing fields ...
        Notify: s.Notify,  // ADD THIS
        // ... rest of fields ...
    }
}
```

### 2. internal/config/schema.json (~150 lines)

```json
{
  "definitions": {
    "step": {
      "properties": {
        // ADD THIS (line ~85)
        "notify": {
          "$ref": "#/definitions/notify",
          "description": "Send notifications to various channels"
        }
      },
      "oneOf": [
        // ADD THIS ENTIRE BLOCK (13 lines)
        {
          "required": ["notify"],
          "not": {
            "anyOf": [
              {"required": ["shell"]},
              {"required": ["command"]},
              {"required": ["template"]},
              {"required": ["file"]},
              {"required": ["copy"]},
              {"required": ["unarchive"]},
              {"required": ["download"]},
              {"required": ["service"]},
              {"required": ["assert"]},
              {"required": ["preset"]},
              {"required": ["print"]},
              {"required": ["include"]},
              {"required": ["include_vars"]},
              {"required": ["vars"]}
            ]
          }
        },
        // ALSO ADD {"required": ["notify"]} to ALL OTHER 14 oneOf blocks
        {
          "required": ["shell"],
          "not": {
            "anyOf": [
              {"required": ["notify"]},  // ADD TO EACH BLOCK
              {"required": ["command"]},
              // ... etc for all 13 other actions
            ]
          }
        }
        // ... repeat for command, template, file, copy, etc. (14 more blocks)
      ]
    },

    // ADD NEW DEFINITION SECTION (~115 lines)
    "notify": {
      "type": "object",
      "properties": {
        "channel": {
          "type": "string",
          "enum": ["slack", "email", "webhook"],
          "description": "Notification channel to use"
        },
        "message": {
          "type": "string",
          "description": "Message to send (supports template variables)"
        },
        "url": {
          "type": "string",
          "format": "uri",
          "description": "Webhook URL (required for webhook channel)"
        }
      },
      "required": ["channel", "message"],
      "allOf": [
        {
          "if": {
            "properties": {
              "channel": {"const": "webhook"}
            }
          },
          "then": {
            "required": ["url"]
          }
        }
      ]
    }
  }
}
```

### 3. internal/executor/notify_step.go (~300 lines, NEW FILE)

```go
package executor

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/alehatsman/mooncake/internal/config"
    "github.com/alehatsman/mooncake/internal/events"
)

// HandleNotify handles notification actions
func HandleNotify(step config.Step, ec *ExecutionContext) error {
    notify := step.Notify
    if notify == nil {
        return fmt.Errorf("notify action is nil")
    }

    // Validate channel
    validChannels := []string{"slack", "email", "webhook"}
    if !contains(validChannels, notify.Channel) {
        return &StepValidationError{
            Field: "channel",
            Value: notify.Channel,
            Issue: fmt.Sprintf("must be one of: %v", validChannels),
        }
    }

    // Validate webhook URL if needed
    if notify.Channel == "webhook" && notify.URL == "" {
        return &StepValidationError{
            Field: "url",
            Issue: "url is required for webhook channel",
        }
    }

    // Render message template
    renderedMsg, err := ec.Template.Render(notify.Message, ec.Variables)
    if err != nil {
        return &RenderError{Field: "message", Cause: err}
    }

    // Handle dry-run mode
    if ec.DryRun {
        ec.HandleDryRun(func(dryRun *dryRunLogger) {
            dryRun.LogNotifyOperation(notify.Channel, renderedMsg)
        })
        return registerNotifyResult(step, ec, false, "dry-run", "")
    }

    // Execute notification based on channel
    var status string
    var errMsg string

    switch notify.Channel {
    case "slack":
        status, errMsg = sendSlackNotification(notify, renderedMsg)
    case "email":
        status, errMsg = sendEmailNotification(notify, renderedMsg)
    case "webhook":
        status, errMsg = sendWebhookNotification(notify, renderedMsg)
    default:
        return fmt.Errorf("unsupported channel: %s", notify.Channel)
    }

    if errMsg != "" {
        return &CommandError{
            Command: fmt.Sprintf("notify to %s", notify.Channel),
            Cause:   fmt.Errorf(errMsg),
        }
    }

    // Emit event
    ec.EmitEvent(events.EventNotifySent, events.NotifyData{
        Channel: notify.Channel,
        Message: renderedMsg,
        Status:  status,
    })

    return registerNotifyResult(step, ec, true, status, "")
}

func sendSlackNotification(notify *config.NotifyAction, message string) (string, string) {
    // Slack API integration
    payload := map[string]interface{}{
        "text": message,
    }

    body, _ := json.Marshal(payload)
    resp, err := http.Post(notify.URL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", fmt.Sprintf("slack request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Sprintf("slack returned status %d", resp.StatusCode)
    }

    return "sent", ""
}

func sendEmailNotification(notify *config.NotifyAction, message string) (string, string) {
    // Email sending logic (SMTP)
    // Simplified for example
    return "sent", ""
}

func sendWebhookNotification(notify *config.NotifyAction, message string) (string, string) {
    payload := map[string]interface{}{
        "message": message,
    }

    body, _ := json.Marshal(payload)
    client := &http.Client{Timeout: 30 * time.Second}

    resp, err := client.Post(notify.URL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", fmt.Sprintf("webhook request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Sprintf("webhook returned status %d", resp.StatusCode)
    }

    return "sent", ""
}

func registerNotifyResult(step config.Step, ec *ExecutionContext, changed bool, status, errorMsg string) error {
    result := &Result{
        Changed: changed,
        Data: map[string]interface{}{
            "channel": step.Notify.Channel,
            "status":  status,
        },
    }

    if errorMsg != "" {
        result.Failed = true
        result.Data["error"] = errorMsg
    }

    ec.CurrentResult = result

    if step.Register != "" {
        result.RegisterTo(ec.Variables, step.Register)
    }

    return nil
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

### 4. internal/executor/executor.go (~5 lines)

```go
func dispatchStepAction(step config.Step, ec *ExecutionContext) error {
    switch {
    // ... existing cases ...

    case step.Notify != nil:  // ADD THIS
        return HandleNotify(step, ec)

    default:
        return nil
    }
}
```

### 5. internal/executor/dryrun.go (~8 lines)

```go
// LogNotifyOperation logs a notification operation in dry-run mode
func (d *dryRunLogger) LogNotifyOperation(channel, message string) {
    d.logger.Infof("  [DRY-RUN] Would send notification")
    d.logger.Infof("    Channel: %s", channel)
    d.logger.Infof("    Message: %s", message)
}
```

### 6. internal/events/event.go (~15 lines)

```go
// Event types for notifications
const (
    EventNotifySent EventType = "notify.sent"
)

// NotifyData contains data for notify.sent events
type NotifyData struct {
    Channel string `json:"channel"` // slack, email, webhook
    Message string `json:"message"` // Rendered message
    Status  string `json:"status"`  // sent, failed
}
```

### 7. internal/config/error_messages.go (~5 lines)

```go
func generateMultipleActionsError(step Step) error {
    actions := []string{}
    // ... existing checks ...
    if step.Notify != nil {
        actions = append(actions, "notify")
    }
    // ... rest of function ...
}
```

---

## Total Impact

**Files Modified**: 7
**Lines Added**: ~500-600
**Time Required**: 4-6 hours
**Maintenance Burden**: Every future action repeats this

**Pain Points:**
- Must remember to update 7 different files
- Easy to forget schema.json oneOf blocks (14 places to update!)
- countActions/DetermineActionType/Clone are manual
- Copy-paste errors are common
- Schema validation grows O(N²)

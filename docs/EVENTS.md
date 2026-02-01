# Event System Architecture

## Overview

Mooncake's event system provides a clean, extensible architecture for observability, logging, and artifact generation. The system decouples execution from presentation, allowing multiple consumers to process execution events independently.

## Architecture

```
┌──────────┐         ┌───────────┐         ┌─────────────┐
│ Executor │ Events  │ Publisher │ Events  │ Subscribers │
│          ├────────>│           ├────────>│             │
└──────────┘         │ (async)   │         │  - Console  │
                     │           │         │  - TUI      │
                     └───────────┘         │  - Artifacts│
                                          └─────────────┘
```

###Components

#### 1. Event Publisher
- **Location**: `internal/events/publisher.go`
- **Purpose**: Async event distribution to multiple subscribers
- **Implementation**: Buffered channels (100 events per subscriber)
- **Behavior**: Non-blocking send (drops events if subscriber slow)

#### 2. Event Types
- **Location**: `internal/events/event.go`
- **15+ event types** covering full execution lifecycle
- **Type-safe**: Strongly-typed payload structs

#### 3. Subscribers
Implement the `Subscriber` interface:
```go
type Subscriber interface {
    OnEvent(event Event)
    Close()
}
```

**Built-in Subscribers**:
- **Console**: Text or JSON output (`internal/logger/console_subscriber.go`)
- **TUI**: Animated terminal UI (`internal/logger/tui_subscriber.go`)
- **Artifacts**: Persistent run logs (`internal/artifacts/writer.go`)

## Event Types

### Run Lifecycle

#### `run.started`
Emitted when execution begins.
```json
{
  "type": "run.started",
  "timestamp": "2026-02-04T14:14:19Z",
  "data": {
    "root_file": "/path/to/config.yml",
    "tags": ["setup", "deploy"],
    "dry_run": false,
    "total_steps": 10
  }
}
```

#### `plan.loaded`
Emitted after plan compilation.
```json
{
  "type": "plan.loaded",
  "timestamp": "2026-02-04T14:14:19Z",
  "data": {
    "root_file": "/path/to/config.yml",
    "total_steps": 10,
    "tags": ["setup", "deploy"]
  }
}
```

#### `run.completed`
Emitted when execution finishes.
```json
{
  "type": "run.completed",
  "timestamp": "2026-02-04T14:15:30Z",
  "data": {
    "total_steps": 10,
    "success_steps": 9,
    "failed_steps": 1,
    "skipped_steps": 0,
    "changed_steps": 7,
    "duration_ms": 71000,
    "success": false,
    "error_message": "Step 'Deploy app' failed"
  }
}
```

### Step Lifecycle

#### `step.started`
Emitted before step execution.
```json
{
  "type": "step.started",
  "timestamp": "2026-02-04T14:14:20Z",
  "data": {
    "step_id": "step-0001",
    "name": "Install nginx",
    "level": 0,
    "global_step": 1,
    "action": "shell",
    "tags": ["setup"],
    "when": ""
  }
}
```

**Fields**:
- `step_id`: Unique identifier (e.g., "step-0001")
- `name`: Step name from config
- `level`: Nesting level (0 = root, 1+ = nested)
- `global_step`: Sequential step number across entire run
- `action`: Type of step ("shell", "file", "template", "vars", etc.)

#### `step.completed`
Emitted after successful step execution.
```json
{
  "type": "step.completed",
  "timestamp": "2026-02-04T14:14:21Z",
  "data": {
    "step_id": "step-0001",
    "name": "Install nginx",
    "level": 0,
    "duration_ms": 1250,
    "changed": true,
    "result": {}
  }
}
```

#### `step.failed`
Emitted when step fails.
```json
{
  "type": "step.failed",
  "timestamp": "2026-02-04T14:14:22Z",
  "data": {
    "step_id": "step-0002",
    "name": "Deploy app",
    "level": 0,
    "error_message": "connection refused",
    "duration_ms": 500
  }
}
```

#### `step.skipped`
Emitted when step is skipped.
```json
{
  "type": "step.skipped",
  "timestamp": "2026-02-04T14:14:22Z",
  "data": {
    "step_id": "step-0003",
    "name": "Windows setup",
    "level": 0,
    "reason": "when:ansible_os_family == 'Windows'"
  }
}
```

### Output Streaming

#### `step.stdout` / `step.stderr`
Emitted for each line of shell command output.
```json
{
  "type": "step.stdout",
  "timestamp": "2026-02-04T14:14:21Z",
  "data": {
    "step_id": "step-0001",
    "stream": "stdout",
    "line": "nginx installed successfully",
    "line_number": 1
  }
}
```

### File Operations

#### `file.created`
Emitted when file is created.
```json
{
  "type": "file.created",
  "timestamp": "2026-02-04T14:14:23Z",
  "data": {
    "path": "/etc/nginx/nginx.conf",
    "mode": "0644",
    "size_bytes": 1024,
    "changed": true
  }
}
```

#### `file.updated`
Emitted when file is modified.

#### `directory.created`
Emitted when directory is created.

#### `template.rendered`
Emitted when template is rendered to file.
```json
{
  "type": "template.rendered",
  "timestamp": "2026-02-04T14:14:24Z",
  "data": {
    "template_path": "./nginx.conf.j2",
    "dest_path": "/etc/nginx/nginx.conf",
    "size_bytes": 1024,
    "changed": true
  }
}
```

### Variables

#### `variables.set`
Emitted when inline vars are set.
```json
{
  "type": "variables.set",
  "timestamp": "2026-02-04T14:14:25Z",
  "data": {
    "count": 3,
    "keys": ["env", "region", "version"]
  }
}
```

#### `variables.loaded`
Emitted when vars loaded from file.
```json
{
  "type": "variables.loaded",
  "timestamp": "2026-02-04T14:14:26Z",
  "data": {
    "file_path": "/path/to/vars.yml",
    "count": 10,
    "keys": ["db_host", "db_port", ...]
  }
}
```

## Usage

### For Users

**JSON Event Stream**:
```bash
mooncake run --config deploy.yml --raw --output-format json
```

**Process with jq**:
```bash
mooncake run --config deploy.yml --raw --output-format json | \
  jq 'select(.type == "step.completed") | .data.name'
```

### For Developers

**Creating a Custom Subscriber**:

```go
package mypackage

import "github.com/alehatsman/mooncake/internal/events"

type MySubscriber struct {
    // Your fields
}

func (s *MySubscriber) OnEvent(event events.Event) {
    switch event.Type {
    case events.EventStepStarted:
        data, ok := event.Data.(events.StepStartedData)
        if !ok {
            return
        }
        // Process step started

    case events.EventStepCompleted:
        data, ok := event.Data.(events.StepCompletedData)
        if !ok {
            return
        }
        // Process step completed

    // Handle other event types...
    }
}

func (s *MySubscriber) Close() {
    // Cleanup
}
```

**Registering Subscriber**:

```go
publisher := events.NewPublisher()
defer publisher.Close()

mySubscriber := &MySubscriber{}
publisher.Subscribe(mySubscriber)

// Execute with event publisher
executor.ExecutePlanWithEvents(plan, password, dryRun, logger, publisher)
```

**Adding New Event Type**:

1. Add event type constant in `event.go`:
```go
const EventMyNewEvent EventType = "my.new.event"
```

2. Define data struct:
```go
type MyNewEventData struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}
```

3. Emit in executor:
```go
ec.EmitEvent(events.EventMyNewEvent, events.MyNewEventData{
    Field1: "value",
    Field2: 42,
})
```

4. Handle in subscribers as needed.

## Performance

- **Event emission**: < 1μs per event (buffered channels)
- **Memory**: ~100 events * subscribers * ~500 bytes = ~50KB per subscriber
- **Non-blocking**: Executor never waits for subscribers
- **Throughput**: ~1M events/sec (limited by channel operations)

## Best Practices

1. **Always check type assertions**:
   ```go
   if data, ok := event.Data.(EventDataType); ok {
       // Use data
   }
   ```

2. **Handle events quickly**: Slow subscribers may drop events
3. **Use buffering**: If processing is slow, buffer events internally
4. **Thread safety**: Subscribers receive events from goroutine
5. **Error handling**: Log errors, don't panic

## Examples

See:
- `examples/json-output-example.md` - JSON output usage
- `examples/events-test.yml` - Test configuration
- `internal/logger/console_subscriber.go` - Reference implementation
- `internal/artifacts/writer.go` - Artifact writer example

## Related Documentation

- [Artifacts](ARTIFACTS.md) - Persistent run artifacts
- [Observability](OBSERVABILITY.md) - Monitoring and metrics
- Architecture overview in IMPLEMENTATION_SUMMARY.md

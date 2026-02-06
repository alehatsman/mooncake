# JSON Output Example

## Overview

Mooncake now supports structured JSON event output via the event system. This enables integration with external tools, monitoring systems, and custom processing pipelines.

## Usage

```bash
# Run with JSON event output
mooncake run --config myconfig.yml --raw --output-format json

# Process events with jq
mooncake run --config myconfig.yml --raw --output-format json | jq '.'

# Filter specific event types
mooncake run --config myconfig.yml --raw --output-format json | \
  jq 'select(.type == "step.completed")'

# Extract step durations
mooncake run --config myconfig.yml --raw --output-format json | \
  jq 'select(.type == "step.completed") | {name: .data.name, duration_ms: .data.duration_ms}'

# Monitor execution in real-time
mooncake run --config myconfig.yml --raw --output-format json | \
  jq --unbuffered -c 'select(.type | startswith("step."))'
```

## Event Types

### Run Lifecycle
- `run.started` - Execution begins
- `plan.loaded` - Plan has been built
- `run.completed` - Execution finished

### Step Lifecycle
- `step.started` - Step begins execution
- `step.completed` - Step completed successfully
- `step.failed` - Step failed with error
- `step.skipped` - Step was skipped

### Output Streaming
- `step.stdout` - Standard output line from shell step
- `step.stderr` - Standard error line from shell step

### File Operations
- `file.created` - File was created
- `file.updated` - File was updated
- `directory.created` - Directory was created
- `template.rendered` - Template was rendered

### Variables
- `variables.set` - Variables were set inline
- `variables.loaded` - Variables were loaded from file

## Event Schema

### run.started
```json
{
  "type": "run.started",
  "timestamp": "2026-02-04T14:14:19.699336+01:00",
  "data": {
    "root_file": "/path/to/config.yml",
    "tags": ["tag1", "tag2"],
    "dry_run": false,
    "total_steps": 10
  }
}
```

### step.started
```json
{
  "type": "step.started",
  "timestamp": "2026-02-04T14:14:19.699372+01:00",
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

### step.completed
```json
{
  "type": "step.completed",
  "timestamp": "2026-02-04T14:14:19.705515+01:00",
  "data": {
    "step_id": "step-0001",
    "name": "Install nginx",
    "level": 0,
    "duration_ms": 1250,
    "changed": true
  }
}
```

### step.stdout
```json
{
  "type": "step.stdout",
  "timestamp": "2026-02-04T14:14:19.705324+01:00",
  "data": {
    "step_id": "step-0001",
    "stream": "stdout",
    "line": "nginx installed successfully",
    "line_number": 1
  }
}
```

### run.completed
```json
{
  "type": "run.completed",
  "timestamp": "2026-02-04T14:14:29.180581+01:00",
  "data": {
    "total_steps": 10,
    "success_steps": 9,
    "failed_steps": 1,
    "skipped_steps": 0,
    "changed_steps": 7,
    "duration_ms": 15432,
    "success": false,
    "error_message": "Step 'Deploy app' failed: connection refused"
  }
}
```

## Use Cases

### 1. CI/CD Integration
```bash
# Parse execution results in CI/CD pipeline
mooncake run --config deploy.yml --raw --output-format json > execution.jsonl

# Check if execution succeeded
if jq -e '.type == "run.completed" and .data.success == true' execution.jsonl > /dev/null; then
  echo "Deployment successful"
  exit 0
else
  echo "Deployment failed"
  exit 1
fi
```

### 2. Performance Monitoring
```bash
# Extract step performance metrics
mooncake run --config myconfig.yml --raw --output-format json | \
  jq 'select(.type == "step.completed") |
      {step: .data.name, duration: .data.duration_ms}' | \
  jq -s 'sort_by(.duration) | reverse'
```

### 3. Real-Time Dashboard
```bash
# Stream events to monitoring dashboard
mooncake run --config myconfig.yml --raw --output-format json | \
  while read -r event; do
    # Send to Elasticsearch, Prometheus, etc.
    curl -X POST http://dashboard/api/events -d "$event"
  done
```

### 4. Log Aggregation
```bash
# Forward events to log aggregation system
mooncake run --config myconfig.yml --raw --output-format json | \
  jq -c '.' | \
  filebeat -e -c filebeat.yml
```

### 5. Custom Processing
```bash
# Filter and transform events with custom script
mooncake run --config myconfig.yml --raw --output-format json | \
  python process_events.py
```

## Notes

- JSON output requires `--raw` flag (disables TUI)
- Each event is a single-line JSON object (JSONL format)
- Events are emitted in real-time as execution progresses
- All timestamps are in ISO 8601 format
- Step IDs are unique within a run (e.g., "step-0001", "step-0002")
- Output lines include line numbers for multi-line output

## Example Processing Script

```python
#!/usr/bin/env python3
import sys
import json

for line in sys.stdin:
    event = json.loads(line)

    if event['type'] == 'step.completed':
        data = event['data']
        print(f"✓ {data['name']} ({data['duration_ms']}ms)")

    elif event['type'] == 'step.failed':
        data = event['data']
        print(f"✗ {data['name']}: {data['error_message']}")

    elif event['type'] == 'run.completed':
        data = event['data']
        if data['success']:
            print(f"\nSuccess! {data['success_steps']}/{data['total_steps']} steps completed")
        else:
            print(f"\nFailed: {data['error_message']}")
```

Save as `process_events.py` and use:
```bash
mooncake run --config myconfig.yml --raw --output-format json | python process_events.py
```

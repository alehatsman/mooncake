# Run Artifacts

## Overview

Mooncake can persist execution artifacts to disk for auditing, debugging, and compliance. Artifacts are stored in `.mooncake/runs/` with a unique run ID per execution.

## Directory Structure

```
.mooncake/
└── runs/
    └── 20260204-141419-a3f2c8/
        ├── plan.json          # Executed plan
        ├── facts.json         # System facts
        ├── summary.json       # Run summary
        ├── results.json       # Step results
        ├── events.jsonl       # Full event stream (JSONL)
        ├── diff.json          # Changed files
        ├── stdout.log         # Full stdout (optional)
        └── stderr.log         # Full stderr (optional)
```

## Run ID Format

Run IDs follow the format: `YYYYMMDD-HHMMSS-<hash>`

Example: `20260204-141419-a3f2c8`

- Timestamp: When run started
- Hash: Short hash of root file + hostname (for uniqueness)

## Artifact Files

### plan.json
The executed plan with all steps expanded.

```json
{
  "root_file": "/path/to/config.yml",
  "initial_vars": {...},
  "steps": [...]
}
```

### facts.json
System facts gathered at runtime.

```json
{
  "hostname": "server1",
  "os_family": "Darwin",
  "architecture": "arm64",
  ...
}
```

### summary.json
High-level run summary.

```json
{
  "run_id": "20260204-141419-a3f2c8",
  "root_file": "/path/to/config.yml",
  "start_time": "2026-02-04T14:14:19Z",
  "end_time": "2026-02-04T14:15:30Z",
  "duration_ms": 71000,
  "total_steps": 10,
  "success_steps": 9,
  "failed_steps": 1,
  "skipped_steps": 0,
  "changed_steps": 7,
  "success": false,
  "error_message": "Step 'Deploy app' failed"
}
```

### results.json
Per-step results.

```json
{
  "run_id": "20260204-141419-a3f2c8",
  "total_steps": 10,
  "success_steps": 9,
  "steps": [
    {
      "step_id": "step-0001",
      "name": "Install nginx",
      "action": "shell",
      "level": 0,
      "duration_ms": 1250,
      "changed": true,
      "status": "success"
    },
    {
      "step_id": "step-0002",
      "name": "Deploy app",
      "status": "failed",
      "error_message": "connection refused",
      "duration_ms": 500
    }
  ]
}
```

### events.jsonl
Full event stream in [JSONL format](http://jsonlines.org/).

```jsonl
{"type":"run.started","timestamp":"...","data":{...}}
{"type":"step.started","timestamp":"...","data":{...}}
{"type":"step.stdout","timestamp":"...","data":{...}}
{"type":"step.completed","timestamp":"...","data":{...}}
{"type":"run.completed","timestamp":"...","data":{...}}
```

One event per line, can be processed with:
```bash
cat events.jsonl | jq '.type'
```

### diff.json
List of files created or modified.

```json
{
  "changed_files": [
    {
      "path": "/etc/nginx/nginx.conf",
      "operation": "created",
      "size_bytes": 1024
    },
    {
      "path": "/etc/app/config.yml",
      "operation": "template",
      "size_bytes": 512
    }
  ],
  "total": 2
}
```

### stdout.log / stderr.log (Optional)
Full command output, organized by step.

```
[step-0001] nginx installed successfully
[step-0001] Starting nginx...
[step-0002] Deploying application...
[step-0002] ERROR: connection refused
```

## Usage

### Enable Artifacts

**Basic** (events only):
```bash
mooncake run --config deploy.yml --artifacts-dir .mooncake
```

**With Full Output**:
```bash
mooncake run --config deploy.yml \
  --artifacts-dir .mooncake \
  --capture-full-output
```

### Configuration Options

```bash
--artifacts-dir DIR        # Base directory for artifacts
--capture-full-output      # Capture stdout/stderr to files
--max-output-bytes NUM     # Max bytes per step in results.json (default: 1MB)
--max-output-lines NUM     # Max lines per step in results.json (default: 1000)
```

## Use Cases

### 1. Audit Trail
Keep historical record of all executions:
```bash
mooncake run --config deploy.yml --artifacts-dir /var/log/mooncake
```

### 2. Debugging Failures
Analyze failed runs:
```bash
# Find failed run
ls -lt .mooncake/runs/ | head -n 1

# View summary
jq '.' .mooncake/runs/20260204-141419-a3f2c8/summary.json

# View failed step
jq '.steps[] | select(.status == "failed")' \
  .mooncake/runs/20260204-141419-a3f2c8/results.json

# View full output
cat .mooncake/runs/20260204-141419-a3f2c8/stderr.log
```

### 3. Compliance
Export artifacts for compliance auditing:
```bash
# Package artifacts
tar czf deployment-$(date +%Y%m%d).tar.gz .mooncake/runs/

# Upload to compliance system
aws s3 cp deployment-20260204.tar.gz s3://compliance-bucket/
```

### 4. Performance Analysis
Analyze step performance:
```bash
jq '.steps[] | {name, duration_ms}' \
  .mooncake/runs/*/results.json | \
  jq -s 'sort_by(.duration_ms) | reverse | .[0:10]'
```

### 5. Diff Tracking
Track what changed:
```bash
# View all changed files
jq '.changed_files[].path' .mooncake/runs/*/diff.json | sort -u

# Count changes per run
jq '.total' .mooncake/runs/*/diff.json
```

## Cleanup

Artifacts can grow over time. Clean up old runs:

```bash
# Keep last 30 days
find .mooncake/runs/ -type d -mtime +30 -exec rm -rf {} +

# Keep last 100 runs
ls -1dt .mooncake/runs/* | tail -n +101 | xargs rm -rf

# Clean by size (keep smallest 1GB)
du -s .mooncake/runs/* | sort -n | \
  awk '{sum+=$1; if(sum>1048576) print $2}' | \
  xargs rm -rf
```

## Integration

### CI/CD Pipeline

```yaml
# GitLab CI example
deploy:
  script:
    - mooncake run --config deploy.yml --artifacts-dir artifacts/
  artifacts:
    paths:
      - artifacts/runs/
    when: always
    expire_in: 30 days
```

### Log Aggregation

```bash
# Forward to Elasticsearch
for run in .mooncake/runs/*/events.jsonl; do
  cat "$run" | while read event; do
    curl -X POST http://elasticsearch:9200/mooncake/_doc \
      -H 'Content-Type: application/json' -d "$event"
  done
done
```

### Monitoring

```bash
# Alert on failures
if jq -e '.success == false' .mooncake/runs/*/summary.json; then
  send_alert "Mooncake deployment failed"
fi
```

## Related Documentation

- [Events](EVENTS.md) - Event system architecture
- [Observability](OBSERVABILITY.md) - Monitoring and metrics

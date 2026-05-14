# Temporal - Durable Workflow Orchestration

Microservice orchestration platform that guarantees workflow completion. Write business logic as code, Temporal handles retries, failures, and state management. Build reliable distributed systems without complexity.

## Quick Start
```yaml
- preset: temporal
```

Access web UI: `http://localhost:8080`
Default namespace: `default`

## Features
- **Durable execution**: Workflows survive process crashes and restarts
- **Automatic retries**: Configurable retry policies for activities
- **Long-running workflows**: Run for days, weeks, or months
- **Event sourcing**: Complete workflow history stored
- **Versioning**: Deploy new versions without breaking running workflows
- **Multi-language SDKs**: Go, Python, TypeScript, Java, .NET, PHP
- **Visibility**: Search and filter workflows with custom attributes
- **Signals & queries**: Interact with running workflows
- **Timers & schedules**: Cron jobs and delayed execution
- **Saga pattern**: Compensation logic for distributed transactions

## Basic Usage
```bash
# Start Temporal Server (development)
temporal server start-dev

# CLI operations
temporal workflow list
temporal workflow describe -w <workflow-id>
temporal workflow show -w <workflow-id>
temporal workflow terminate -w <workflow-id>
temporal workflow cancel -w <workflow-id>

# Namespace management
temporal namespace list
temporal namespace create --namespace production
temporal namespace describe --namespace production

# Task queues
temporal task-queue list
temporal task-queue describe --task-queue myqueue

# Search workflows
temporal workflow list --query 'WorkflowType="MyWorkflow"'
temporal workflow list --query 'StartTime > "2024-01-01T00:00:00Z"'
```

## Architecture

### Components
```
┌─────────────────┐
│  Temporal CLI   │ ← Command-line management
└────────┬────────┘
         │
┌────────▼────────┐
│ Temporal Server │ ← Core orchestration engine
│  - Frontend     │
│  - History      │
│  - Matching     │
│  - Worker       │
└────────┬────────┘
         │
┌────────▼────────┐
│   Database      │ ← Cassandra, PostgreSQL, MySQL
│  (Persistence)  │
└─────────────────┘

┌─────────────────┐
│  Worker Process │ ← Your application code
│  - Workflows    │
│  - Activities   │
└─────────────────┘
```

### Key Concepts
- **Workflow**: Durable function that orchestrates activities
- **Activity**: Task that can fail and retry (API calls, DB queries)
- **Worker**: Process that executes workflows and activities
- **Task Queue**: Named queue that workers poll
- **Namespace**: Logical isolation boundary
- **Signal**: External message to running workflow
- **Query**: Read workflow state without side effects

## Advanced Configuration

### Development server
```yaml
- name: Install Temporal Server
  preset: temporal

- name: Start dev server
  shell: temporal server start-dev --db-filename temporal.db --ui-port 8080
  async: true
```

### Production deployment with PostgreSQL
```yaml
- name: Install PostgreSQL
  preset: postgres
  with:
    databases:
      - temporal
      - temporal_visibility
    users:
      - name: temporal
        password: "{{ temporal_db_password }}"

- name: Install Temporal Server
  preset: temporal
  become: true

- name: Configure Temporal
  template:
    src: temporal-config.yml.j2
    dest: /etc/temporal/config.yml
  become: true

- name: Start Temporal services
  shell: |
    temporal-server start \
      --config /etc/temporal/config.yml \
      --env production
  become: true
  async: true
```

### Worker deployment
```yaml
- name: Install Temporal CLI
  preset: temporal

- name: Deploy worker application
  copy:
    src: worker-binary
    dest: /usr/local/bin/myapp-worker
    mode: '0755'
  become: true

- name: Create worker service
  service:
    name: myapp-worker
    state: started
    unit:
      content: |
        [Unit]
        Description=Temporal Worker
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/myapp-worker
        Restart=always
        Environment="TEMPORAL_ADDRESS=temporal.example.com:7233"
        Environment="TEMPORAL_NAMESPACE=production"

        [Install]
        WantedBy=multi-user.target
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Temporal |

## Platform Support
- ✅ Linux (all distributions) - via binary
- ✅ macOS (Homebrew)
- ✅ Docker (official images)
- ❌ Windows (use WSL2 or Docker)

## Configuration

### File locations
- **Config**: `/etc/temporal/config.yml`
- **Data**: `/var/lib/temporal/`
- **Logs**: `/var/log/temporal/`

### config.yml example
```yaml
log:
  stdout: true
  level: info

persistence:
  defaultStore: default
  visibilityStore: visibility
  numHistoryShards: 4
  datastores:
    default:
      sql:
        pluginName: "postgres"
        databaseName: "temporal"
        connectAddr: "localhost:5432"
        connectProtocol: "tcp"
        user: "temporal"
        password: "secret"
    visibility:
      sql:
        pluginName: "postgres"
        databaseName: "temporal_visibility"
        connectAddr: "localhost:5432"
        connectProtocol: "tcp"
        user: "temporal"
        password: "secret"

global:
  membership:
    maxJoinDuration: 30s
    broadcastAddress: "0.0.0.0"

services:
  frontend:
    rpc:
      grpcPort: 7233
      membershipPort: 6933
      bindOnIP: "0.0.0.0"

  matching:
    rpc:
      grpcPort: 7235
      membershipPort: 6935
      bindOnIP: "0.0.0.0"

  history:
    rpc:
      grpcPort: 7234
      membershipPort: 6934
      bindOnIP: "0.0.0.0"

  worker:
    rpc:
      grpcPort: 7239
      membershipPort: 6939
      bindOnIP: "0.0.0.0"

clusterMetadata:
  enableGlobalNamespace: false
  failoverVersionIncrement: 10
  masterClusterName: "active"
  currentClusterName: "active"
  clusterInformation:
    active:
      enabled: true
      initialFailoverVersion: 1
      rpcAddress: "localhost:7233"
```

## SDK Examples

### Go Worker
```go
package main

import (
    "log"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    c, err := client.Dial(client.Options{
        HostPort: "localhost:7233",
    })
    if err != nil {
        log.Fatalln("Unable to create Temporal client", err)
    }
    defer c.Close()

    w := worker.New(c, "my-task-queue", worker.Options{})

    // Register workflows and activities
    w.RegisterWorkflow(MyWorkflow)
    w.RegisterActivity(MyActivity)

    // Start worker
    err = w.Run(worker.InterruptCh())
    if err != nil {
        log.Fatalln("Unable to start Worker", err)
    }
}

// Workflow definition
func MyWorkflow(ctx workflow.Context, input string) (string, error) {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: time.Minute,
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    var result string
    err := workflow.ExecuteActivity(ctx, MyActivity, input).Get(ctx, &result)
    return result, err
}

// Activity definition
func MyActivity(ctx context.Context, input string) (string, error) {
    // Do work (API call, DB query, etc.)
    return "result", nil
}
```

### Python Worker
```python
import asyncio
from temporalio.client import Client
from temporalio.worker import Worker
from temporalio import workflow, activity

@workflow.defn
class MyWorkflow:
    @workflow.run
    async def run(self, name: str) -> str:
        return await workflow.execute_activity(
            my_activity,
            name,
            start_to_close_timeout=timedelta(minutes=1),
        )

@activity.defn
async def my_activity(name: str) -> str:
    return f"Hello, {name}!"

async def main():
    client = await Client.connect("localhost:7233")

    worker = Worker(
        client,
        task_queue="my-task-queue",
        workflows=[MyWorkflow],
        activities=[my_activity],
    )

    await worker.run()

if __name__ == "__main__":
    asyncio.run(main())
```

### TypeScript Worker
```typescript
import { NativeConnection, Worker } from '@temporalio/worker';
import * as activities from './activities';

async function run() {
  const connection = await NativeConnection.connect({
    address: 'localhost:7233',
  });

  const worker = await Worker.create({
    connection,
    namespace: 'default',
    taskQueue: 'my-task-queue',
    workflowsPath: require.resolve('./workflows'),
    activities,
  });

  await worker.run();
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
```

## Common Patterns

### Saga pattern (compensating transactions)
```go
func TransferMoneyWorkflow(ctx workflow.Context, req TransferRequest) error {
    // Debit account A
    err := workflow.ExecuteActivity(ctx, DebitActivity, req.FromAccount, req.Amount).Get(ctx, nil)
    if err != nil {
        return err
    }

    // Credit account B
    err = workflow.ExecuteActivity(ctx, CreditActivity, req.ToAccount, req.Amount).Get(ctx, nil)
    if err != nil {
        // Compensation: refund account A
        workflow.ExecuteActivity(ctx, CreditActivity, req.FromAccount, req.Amount).Get(ctx, nil)
        return err
    }

    return nil
}
```

### Long-running workflow with heartbeat
```go
func ProcessLargeFile(ctx context.Context, filePath string) error {
    activity.RecordHeartbeat(ctx, "starting")

    for i := 0; i < 1000000; i++ {
        // Process chunk
        processChunk(i)

        // Heartbeat every 100 chunks
        if i % 100 == 0 {
            activity.RecordHeartbeat(ctx, i)
        }
    }

    return nil
}
```

### Cron workflow
```go
// Start workflow with cron schedule
workflowOptions := client.StartWorkflowOptions{
    ID:           "my-cron-workflow",
    TaskQueue:    "my-task-queue",
    CronSchedule: "0 */6 * * *", // Every 6 hours
}
```

### Signal and query
```go
func MyWorkflow(ctx workflow.Context) error {
    var status string = "running"

    // Handle signal
    signalChan := workflow.GetSignalChannel(ctx, "update-status")
    workflow.Go(ctx, func(ctx workflow.Context) {
        for {
            var newStatus string
            signalChan.Receive(ctx, &newStatus)
            status = newStatus
        }
    })

    // Handle query
    err := workflow.SetQueryHandler(ctx, "get-status", func() (string, error) {
        return status, nil
    })

    // Workflow logic...
    workflow.Sleep(ctx, time.Hour)

    return nil
}

// Send signal
client.SignalWorkflow(ctx, workflowID, runID, "update-status", "paused")

// Send query
response, err := client.QueryWorkflow(ctx, workflowID, runID, "get-status")
```

## Use Cases

### Microservice Orchestration
```yaml
- name: Install Temporal Server
  preset: temporal
  become: true

- name: Deploy order processing worker
  copy:
    src: order-worker
    dest: /usr/local/bin/order-worker
    mode: '0755'
  become: true

- name: Start worker
  service:
    name: order-worker
    state: started
  become: true
```

### Background Job Processing
```yaml
- name: Install Temporal CLI
  preset: temporal

- name: Deploy email worker
  shell: |
    ./email-worker \
      --temporal-address temporal.prod:7233 \
      --task-queue email-queue \
      --namespace production
  async: true
```

### Long-Running Workflows
```yaml
- name: Start data migration workflow
  shell: |
    temporal workflow start \
      --task-queue migration-queue \
      --type DataMigrationWorkflow \
      --workflow-id migration-2024-02-08 \
      --input '{"source": "old-db", "dest": "new-db"}'
```

## CLI Commands

### Workflow management
```bash
# Start workflow
temporal workflow start \
  --task-queue my-queue \
  --type MyWorkflow \
  --workflow-id my-unique-id \
  --input '{"key": "value"}'

# Describe workflow
temporal workflow describe -w my-unique-id

# Show workflow execution history
temporal workflow show -w my-unique-id

# Signal workflow
temporal workflow signal \
  --workflow-id my-unique-id \
  --name update-status \
  --input '"paused"'

# Query workflow
temporal workflow query \
  --workflow-id my-unique-id \
  --type get-status

# Terminate workflow
temporal workflow terminate -w my-unique-id --reason "manual termination"

# Cancel workflow (allows cleanup)
temporal workflow cancel -w my-unique-id --reason "user requested"

# Reset workflow to specific point
temporal workflow reset \
  --workflow-id my-unique-id \
  --event-id 10
```

### Namespace operations
```bash
# Create namespace
temporal namespace create \
  --namespace production \
  --retention 30

# Update namespace
temporal namespace update \
  --namespace production \
  --retention 60

# List namespaces
temporal namespace list

# Describe namespace
temporal namespace describe --namespace production
```

### Activity operations
```bash
# Complete activity (from external system)
temporal activity complete \
  --workflow-id my-unique-id \
  --run-id <run-id> \
  --activity-id <activity-id> \
  --result '{"status": "success"}'

# Fail activity
temporal activity fail \
  --workflow-id my-unique-id \
  --run-id <run-id> \
  --activity-id <activity-id> \
  --reason "external system failure"
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Temporal
  preset: temporal
```

### Development stack
```yaml
- name: Install Temporal
  preset: temporal

- name: Start dev server
  shell: temporal server start-dev --ui-port 8080
  async: true

- name: Wait for server
  shell: |
    for i in {1..30}; do
      temporal namespace list && break
      sleep 1
    done
```

### Production deployment
```yaml
- name: Setup database
  preset: postgres
  with:
    databases:
      - temporal
      - temporal_visibility

- name: Install Temporal Server
  preset: temporal
  become: true

- name: Configure Temporal
  template:
    src: temporal-config.yml.j2
    dest: /etc/temporal/config.yml
  become: true

- name: Start Temporal
  service:
    name: temporal-server
    state: started
  become: true
```

## Agent Use
- **Workflow orchestration**: Coordinate microservices reliably
- **Background jobs**: Process tasks with retries and durability
- **Saga pattern**: Implement distributed transactions with compensation
- **ETL pipelines**: Long-running data processing workflows
- **Scheduled tasks**: Cron-like jobs with history and visibility
- **Human-in-the-loop**: Workflows that wait for external approval
- **Stateful processes**: Order processing, booking systems, onboarding flows

## Troubleshooting

### Connection refused
```bash
# Check if server is running
ps aux | grep temporal
netstat -an | grep 7233

# Check connectivity
telnet localhost 7233
curl http://localhost:8080  # Web UI
```

### Workflow stuck
```bash
# Check workflow history
temporal workflow show -w <workflow-id>

# Check task queue
temporal task-queue describe --task-queue my-queue

# Verify worker is running
ps aux | grep worker
```

### Activity timeout
```go
// Increase timeout in workflow code
ao := workflow.ActivityOptions{
    StartToCloseTimeout: 5 * time.Minute, // Increase from default
    HeartbeatTimeout:    30 * time.Second,
}
```

### Database connection errors
```bash
# Test database connection
psql -h localhost -U temporal -d temporal

# Check Temporal logs
journalctl -u temporal-server -f
tail -f /var/log/temporal/temporal.log

# Verify config
cat /etc/temporal/config.yml | grep -A 10 persistence
```

### Worker not picking up tasks
```bash
# Verify task queue name matches
temporal task-queue describe --task-queue my-queue

# Check worker logs
journalctl -u myapp-worker -f

# Verify namespace
temporal namespace describe --namespace production
```

## Monitoring

### Metrics (Prometheus)
```yaml
# Temporal exposes metrics on :9090/metrics
- name: Scrape Temporal metrics
  shell: curl http://localhost:9090/metrics
```

### Common metrics
- `temporal_workflow_completed` - Completed workflows
- `temporal_workflow_failed` - Failed workflows
- `temporal_activity_execution_latency` - Activity latency
- `temporal_task_queue_length` - Task queue depth

## Uninstall
```yaml
- preset: temporal
  with:
    state: absent
```

**Note**: Database data is preserved. Drop Temporal databases manually if needed.

## Resources
- Official: https://temporal.io/
- Documentation: https://docs.temporal.io/
- SDKs: https://docs.temporal.io/dev-guide
- Samples: https://github.com/temporalio/samples-go
- Community: https://community.temporal.io/
- Learn: https://learn.temporal.io/
- GitHub: https://github.com/temporalio/temporal
- Search: "temporal workflow", "temporal tutorial", "temporal go sdk"

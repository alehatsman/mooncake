# Prefect - Workflow Orchestration

Modern workflow orchestration framework for building resilient data pipelines in Python with minimal friction.

## Quick Start

```yaml
- preset: prefect
```

## Features

- **Python-First**: Write workflows in pure Python with decorators
- **Dynamic Workflows**: Create workflows programmatically, no DSLs
- **Automatic Tracking**: Built-in state management and monitoring
- **Failure Handling**: Automatic retries, caching, and error recovery
- **Event-Driven**: Trigger workflows from events, schedules, or APIs
- **Cloud Native**: Run anywhere Python runs, cloud or on-premise

## Basic Usage

```bash
# Check version
prefect version

# Start local server
prefect server start

# View UI (default: http://localhost:4200)
open http://localhost:4200

# Deploy a flow
prefect deploy

# List flows
prefect flow ls

# Run a flow
prefect flow-run create --flow-name my-flow
```

## Advanced Configuration

```yaml
# Basic installation
- preset: prefect
  with:
    state: present

# Uninstall
- preset: prefect
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (pip, apt, dnf)
- ✅ macOS (pip, Homebrew)
- ✅ Windows (pip)

## Configuration

- **Config file**: `~/.prefect/config.toml`
- **Database**: SQLite (default), PostgreSQL (production)
- **UI Port**: 4200 (default)
- **API Port**: 4201 (default)

## Real-World Examples

### Data Pipeline

```python
from prefect import flow, task
from datetime import timedelta

@task(retries=3, retry_delay_seconds=60)
def extract_data(source):
    # Extract data from source
    return data

@task(cache_expiration=timedelta(hours=1))
def transform_data(data):
    # Transform data
    return transformed

@task
def load_data(data, destination):
    # Load data to destination
    pass

@flow(name="etl-pipeline")
def etl_pipeline():
    data = extract_data("s3://bucket/data")
    transformed = transform_data(data)
    load_data(transformed, "postgresql://db")

if __name__ == "__main__":
    etl_pipeline()
```

### ML Model Training

```python
from prefect import flow, task
import mlflow

@task
def load_training_data():
    return X_train, y_train

@task
def train_model(X, y, params):
    with mlflow.start_run():
        model = train(X, y, **params)
        mlflow.log_params(params)
        mlflow.log_metrics({"accuracy": score})
    return model

@task
def evaluate_model(model, X_test, y_test):
    return model.score(X_test, y_test)

@flow
def ml_training_pipeline(params):
    X_train, y_train = load_training_data()
    model = train_model(X_train, y_train, params)
    score = evaluate_model(model, X_test, y_test)
    return model, score
```

### Scheduled Data Sync

```yaml
# Install Prefect
- name: Install Prefect
  preset: prefect

# Deploy workflow
- name: Deploy sync workflow
  shell: |
    cat > sync_flow.py << 'EOF'
    from prefect import flow, task
    from prefect.deployments import Deployment
    from prefect.server.schemas.schedules import CronSchedule

    @task
    def sync_databases():
        # Sync logic here
        pass

    @flow
    def daily_sync():
        sync_databases()

    deployment = Deployment.build_from_flow(
        flow=daily_sync,
        name="daily-database-sync",
        schedule=CronSchedule(cron="0 2 * * *"),  # 2 AM daily
    )
    deployment.apply()
    EOF

    python sync_flow.py
```

## Common Operations

```bash
# Server management
prefect server start                    # Start local server
prefect server database reset           # Reset database

# Work pools
prefect work-pool create my-pool        # Create work pool
prefect work-pool ls                    # List work pools

# Workers
prefect worker start --pool my-pool     # Start worker

# Flows
prefect flow ls                         # List flows
prefect flow inspect my-flow            # Inspect flow
prefect flow-run ls --flow-name my-flow # List runs

# Deployments
prefect deployment build flow.py:my_flow  # Build deployment
prefect deployment apply                  # Apply deployment
prefect deployment run my-flow/prod       # Run deployment
```

## Agent Use

- Orchestrate data pipelines with automatic retries and failure handling
- Schedule recurring workflows (ETL, reporting, backups)
- Coordinate microservices and distributed tasks
- Build event-driven architectures
- Monitor and track workflow execution
- Implement complex dependencies between tasks
- Cache expensive computations
- Parallel and distributed execution

## Troubleshooting

### Server won't start

Check if ports are in use:
```bash
lsof -i :4200
lsof -i :4201
```

Reset database:
```bash
prefect server database reset --yes
```

### Flow not appearing in UI

Ensure the flow is deployed:
```bash
prefect deployment ls
prefect deployment apply
```

### Task retries not working

Check task decorator:
```python
@task(retries=3, retry_delay_seconds=60)
def my_task():
    pass
```

## Uninstall

```yaml
- preset: prefect
  with:
    state: absent
```

## Resources

- Official docs: https://docs.prefect.io/
- GitHub: https://github.com/PrefectHQ/prefect
- PyPI: https://pypi.org/project/prefect/
- Search: "prefect workflow orchestration", "prefect python tutorial", "prefect vs airflow"

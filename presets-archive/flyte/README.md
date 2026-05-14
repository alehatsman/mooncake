# Flyte - Workflow Orchestration Platform

Scalable and reproducible workflows for data and ML pipelines. Build, deploy, and monitor workflows with strong typing and versioning.

## Quick Start
```yaml
- preset: flyte
```

## Features
- **Reproducible workflows**: Version control for workflows and executions
- **Type safety**: Strong typing with Flytekit SDK
- **Multi-cloud**: Deploy on Kubernetes, AWS, GCP, Azure
- **Resource management**: Dynamic allocation of CPU, GPU, memory
- **Data lineage**: Automatic tracking of inputs, outputs, and artifacts
- **Multi-language**: Python, Java, and containerized tasks

## Basic Usage
```bash
# Initialize Flyte project
pyflyte init my-workflow

# Run workflow locally
pyflyte run workflow.py my_workflow --input1 value1

# Register workflow to Flyte cluster
pyflyte register workflows --project myproject --domain development

# Execute on cluster
flytectl get executions --project myproject --domain development

# Check execution status
flytectl get execution --project myproject --domain development <execution-id>

# View workflow history
flytectl get workflows --project myproject --domain development
```

## Advanced Configuration
```yaml
- preset: flyte
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Flyte CLI |

## Platform Support
- ✅ Linux (pip, binary download)
- ✅ macOS (Homebrew, pip)
- ✅ Windows (pip, binary download)

## Configuration
- **Config file**: `~/.flyte/config.yaml`
- **Flyte console**: `http://localhost:30081` (local deployment)
- **Storage**: S3, GCS, or MinIO for artifacts
- **Kubernetes**: Required for cluster deployment

## Real-World Examples

### Simple Python Workflow
```python
from flytekit import task, workflow

@task
def square(n: int) -> int:
    return n * n

@task
def sum_values(values: list[int]) -> int:
    return sum(values)

@workflow
def my_workflow(numbers: list[int]) -> int:
    squared = [square(n=n) for n in numbers]
    return sum_values(values=squared)

# Run locally
if __name__ == "__main__":
    result = my_workflow(numbers=[1, 2, 3, 4, 5])
    print(f"Result: {result}")  # 55
```

### ML Training Pipeline
```python
from flytekit import task, workflow, Resources
from flytekit.types.file import FlyteFile
import pandas as pd
from sklearn.ensemble import RandomForestClassifier

@task(requests=Resources(cpu="2", mem="4Gi"))
def load_data(data_path: str) -> pd.DataFrame:
    return pd.read_csv(data_path)

@task(requests=Resources(cpu="4", mem="8Gi"))
def train_model(data: pd.DataFrame, n_estimators: int = 100) -> FlyteFile:
    X = data.drop('target', axis=1)
    y = data['target']

    model = RandomForestClassifier(n_estimators=n_estimators)
    model.fit(X, y)

    # Save model
    import joblib
    model_path = "model.pkl"
    joblib.dump(model, model_path)
    return FlyteFile(path=model_path)

@workflow
def training_pipeline(data_path: str, n_estimators: int = 100) -> FlyteFile:
    data = load_data(data_path=data_path)
    return train_model(data=data, n_estimators=n_estimators)
```

### Distributed Data Processing
```python
from flytekit import task, workflow, dynamic
from typing import List

@task
def process_partition(data: List[int]) -> int:
    return sum(data)

@dynamic
def map_reduce(data: List[int], partition_size: int = 100) -> int:
    partitions = [
        data[i:i + partition_size]
        for i in range(0, len(data), partition_size)
    ]

    # Map phase
    partial_sums = [process_partition(data=p) for p in partitions]

    # Reduce phase
    return sum(partial_sums)

@workflow
def distributed_sum(data: List[int]) -> int:
    return map_reduce(data=data, partition_size=1000)
```

### GPU Task with Caching
```python
from flytekit import task, workflow, Resources
from flytekit.extras.accelerators import GPUAccelerator

@task(
    requests=Resources(gpu="1", mem="16Gi"),
    accelerator=GPUAccelerator("nvidia-tesla-t4"),
    cache=True,
    cache_version="1.0"
)
def train_on_gpu(epochs: int, batch_size: int) -> float:
    # Training code using GPU
    import torch
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    # ... training logic ...
    return final_accuracy

@workflow
def gpu_training_workflow(epochs: int = 10) -> float:
    return train_on_gpu(epochs=epochs, batch_size=32)
```

## Agent Use
- Orchestrate ML training pipelines with resource management
- Build reproducible data processing workflows
- Automate ETL jobs with dependency tracking
- Schedule recurring data pipelines
- Version control experiments and model training
- Deploy scalable batch processing jobs

## Troubleshooting

### Workflow registration fails
```bash
# Check Flyte connection
flytectl config view

# Verify project and domain exist
flytectl get projects
flytectl get domains --project myproject

# Re-register with verbose output
pyflyte register --verbose workflows --project myproject --domain development

# Check package requirements
pip install flytekit flytekitplugins-spark
```

### Task execution errors
```bash
# View execution logs
flytectl get execution --project myproject --domain development <exec-id>

# Check task logs
kubectl logs -n flyte <pod-name>

# Inspect task resources
flytectl get task-resource-attributes --project myproject --domain development

# Validate workflow locally first
pyflyte run --remote workflow.py my_workflow
```

### Resource quota exceeded
```bash
# Check resource usage
kubectl top nodes
kubectl top pods -n flyte

# Adjust task resource requests
@task(requests=Resources(cpu="1", mem="2Gi"))
def my_task():
    pass

# Configure default resources
# Edit FlyteAdmin config or task resource attributes
```

### Authentication issues
```bash
# Update config with credentials
flytectl config init

# Set endpoint
export FLYTECTL_CONFIG=~/.flyte/config.yaml

# Test connection
flytectl get projects

# Use service account
export FLYTE_CREDENTIALS_CLIENT_ID=<client-id>
export FLYTE_CREDENTIALS_CLIENT_SECRET=<secret>
```

## Uninstall
```yaml
- preset: flyte
  with:
    state: absent
```

## Resources
- Official docs: https://docs.flyte.org/
- GitHub: https://github.com/flyteorg/flyte
- Examples: https://github.com/flyteorg/flytesnacks
- Flytekit SDK: https://docs.flyte.org/projects/flytekit/
- Search: "flyte workflow orchestration", "flyte ml pipelines", "flyte kubernetes"

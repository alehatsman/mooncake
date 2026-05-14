# MLflow - Machine Learning Experiment Tracking and Model Registry

MLflow is an open-source platform that manages the full machine learning lifecycle, including experiment tracking, model versioning, and deployment.

## Quick Start

```yaml
- preset: mlflow
```

## Features

- **Experiment Tracking**: Log parameters, metrics, and artifacts for reproducible ML experiments
- **Model Registry**: Version, stage, and manage ML models across environments
- **Flexible Backends**: SQLite, PostgreSQL, MySQL for metadata storage
- **Artifact Storage**: Support for local filesystem, S3, GCS, and Azure storage
- **UI Dashboard**: Web interface to compare experiments and manage models
- **Python & R Support**: Native libraries for popular ML frameworks
- **Production Ready**: Deploy models as REST endpoints, Docker containers, or Kubernetes

## Basic Usage

```bash
# Start MLflow tracking server on default port 5000
mlflow server

# Start with custom backend store
mlflow server --backend-store-uri postgresql://user:pass@localhost/mlflow

# Log experiment from Python
python -c "
import mlflow
mlflow.start_run()
mlflow.log_param('learning_rate', 0.01)
mlflow.log_metric('accuracy', 0.95)
mlflow.end_run()
"

# View tracking server
curl http://localhost:5000/api/2.0/experiments

# List registered models
mlflow models list

# Load and run a model
mlflow models serve -m "models:/my-model/production" --port 8000
```

## Advanced Configuration

```yaml
# Basic MLflow installation
- preset: mlflow

# With service management and custom tracking server
- preset: mlflow
  with:
    service: true
    tracking_uri: http://0.0.0.0:5000
    backend_store_uri: postgresql://mlflow_user:password@db.example.com:5432/mlflow
    default_artifact_root: s3://my-bucket/mlflow-artifacts
  become: true

# Uninstall MLflow
- preset: mlflow
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove MLflow |
| tracking_uri | string | http://127.0.0.1:5000 | Tracking server URI |
| backend_store_uri | string | - | Backend store URI (sqlite:///mlflow.db, postgresql://user:pass@host/db) |
| default_artifact_root | string | - | Default artifact storage location (s3://bucket, /local/path) |
| service | bool | false | Enable MLflow as system service |

## Platform Support

- ✅ Linux (apt, dnf, pacman - Python via pip)
- ✅ macOS (Homebrew Python or pip)
- ❌ Windows (not yet supported - use WSL)

## Configuration

- **Python Package**: Installed via pip (requires Python 3.8+)
- **Tracking Server**: Default runs on `http://localhost:5000`
- **Default Backend**: SQLite at `./mlruns/`
- **Default Artifacts**: Stored in `./mlruns/` directory
- **Config Directory**: `~/.mlflow/` (Linux/macOS)
- **Service File**: `/etc/systemd/system/mlflow.service` (Linux with service=true)

## Real-World Examples

### ML Experiment Comparison

```bash
# Log multiple experiments
for model in linear_regression random_forest gradient_boosting; do
  mlflow run . -P model_type=$model --run-name "experiment-$model"
done

# Compare via UI
curl http://localhost:5000/api/2.0/experiments | jq '.[].name'
```

### Production Model Deployment

```yaml
# Deploy MLflow with persistent backend
- preset: mlflow
  with:
    service: true
    backend_store_uri: postgresql://mlflow:password@postgres.local:5432/mlflow
    default_artifact_root: s3://prod-ml-artifacts/
  become: true

- name: Verify MLflow service
  assert:
    http:
      url: http://localhost:5000/api/2.0/experiments
      status: 200
```

### Model Registry with Version Control

```bash
# Register model
mlflow models create -n my-model -d "Customer churn prediction"

# Promote to staging
mlflow models transition-request create --name my-model --version 1 --stage Staging

# Promote to production after validation
mlflow models transition-request create --name my-model --version 1 --stage Production

# Monitor production models
mlflow models list | grep Production
```

## Agent Use

- Programmatically log ML experiments in CI/CD pipelines
- Track hyperparameter tuning across multiple models
- Version and promote models through environments
- Query model registry for deployment targets
- Automate model serving based on registry state
- Generate model lineage and impact analysis reports

## Troubleshooting

### Tracking server won't start

Check if port 5000 is already in use:

```bash
lsof -i :5000  # Show process using port 5000
mlflow server --host 0.0.0.0 --port 8080  # Use different port
```

### Database connection errors

Verify backend store connection:

```bash
# Test PostgreSQL connection
psql -h localhost -U mlflow_user -d mlflow -c "SELECT 1"

# Use SQLite if PostgreSQL unavailable
mlflow server --backend-store-uri sqlite:///mlflow.db
```

### Artifact storage issues

Check permissions and connectivity:

```bash
# Local artifacts
ls -la ./mlruns/  # Verify directory exists

# S3 artifacts - verify credentials
aws s3 ls s3://your-bucket/

# Check MLflow logs
tail -f ~/.mlflow/mlflow.log  # If service logging enabled
```

### Models not appearing in registry

```bash
# Verify database contains models
mlflow models list

# Check backend store URI configuration
mlflow server --help | grep backend
```

## Uninstall

```yaml
- preset: mlflow
  with:
    state: absent
```

**Note**: Uninstallation removes MLflow but preserves tracking data in the backend store and artifact locations.

## Resources

- Official docs: https://mlflow.org/docs/latest/
- GitHub: https://github.com/mlflow/mlflow
- Python API: https://mlflow.org/docs/latest/python_api/
- Model Registry: https://mlflow.org/docs/latest/model-registry/
- Search: "mlflow experiment tracking", "mlflow model deployment", "mlflow best practices"

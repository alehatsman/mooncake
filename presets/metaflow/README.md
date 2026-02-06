# Metaflow - Data Science Workflow Framework

Metaflow is a human-friendly Python framework for building and managing real-world data science workflows. It enables data scientists to run computationally intensive tasks with orchestration, versioning, and cloud integration capabilities.

## Quick Start

```yaml
- preset: metaflow
```

## Features

- **Workflow orchestration**: DAG-based task scheduling and execution tracking
- **Version control**: Automatic tracking of runs with reproducible results
- **Cloud integration**: Deploy to AWS Step Functions, Kubernetes, or local execution
- **Production ready**: Error handling, retry logic, and monitoring built-in
- **Python-native**: Write workflows in pure Python, no DSL required
- **Distributed execution**: Scale computationally intensive tasks across multiple nodes

## Basic Usage

```bash
# Check Metaflow version
metaflow --version

# Show help and available commands
metaflow --help

# Initialize a new Metaflow project
python flow.py show

# Execute a workflow (assuming flow.py exists)
python flow.py run

# View workflow runs and results
python flow.py show-runs

# Execute with parameters
python flow.py run --param-key value
```

## Advanced Configuration

```yaml
# Install with custom Python version management
- preset: metaflow
  with:
    state: present

# Support for additional parameters (when available)
- preset: metaflow
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) Metaflow |

## Platform Support

- ✅ Linux (pip, package managers)
- ✅ macOS (Homebrew, pip)
- ⚠️ Windows (pip installation, but primary support is Linux/macOS)

## Configuration

Metaflow stores configuration and runtime data in:

- **Config**: `~/.metaflowconfig/` (Linux), `~/Library/Application Support/metaflow/` (macOS)
- **Data directory**: `~/.metaflow/` (Linux), `~/Library/Metaflow/` (macOS)
- **Logs**: Workflow execution logs stored with each run
- **Environment**: Set `METAFLOW_HOME` to customize storage location

Common configuration variables:
- `METAFLOW_HOME`: Root directory for Metaflow data
- `METAFLOW_DATASTORE`: Backend storage (local, s3, etc.)
- `METAFLOW_SERVICE_URL`: Remote Metaflow service endpoint (optional)

## Real-World Examples

### Basic Data Science Workflow

```python
# flow.py - Simple ML pipeline
from metaflow import FlowSpec, step

class DataPipeline(FlowSpec):

    @step
    def start(self):
        """Load and prepare data"""
        self.data = load_dataset()
        self.next(self.train)

    @step
    def train(self):
        """Train model"""
        self.model = train_model(self.data)
        self.next(self.evaluate)

    @step
    def evaluate(self):
        """Evaluate model performance"""
        self.metrics = evaluate_model(self.model, self.data)
        self.next(self.end)

    @step
    def end(self):
        """Complete workflow"""
        print(f"Model accuracy: {self.metrics['accuracy']}")

if __name__ == '__main__':
    DataPipeline()
```

Execute with:
```bash
python flow.py run
python flow.py show-runs
```

### Multi-Parallel Processing

```python
# Workflow with parallel steps for distributed computation
from metaflow import FlowSpec, step

class ParallelProcessing(FlowSpec):

    @step
    def start(self):
        """Split work across multiple parallel tasks"""
        self.batches = [batch1, batch2, batch3]
        self.next(self.process, foreach='self.batches')

    @step
    def process(self):
        """Process each batch independently"""
        self.results = process_batch(self.input)
        self.next(self.join)

    @step
    def join(self, inputs):
        """Aggregate results from parallel steps"""
        self.combined = aggregate_results([inp.results for inp in inputs])
        self.next(self.end)

    @step
    def end(self):
        """Finalize"""
        persist_results(self.combined)

if __name__ == '__main__':
    ParallelProcessing()
```

### Production Deployment with Parameters

```python
# flow.py - Production-ready workflow with configuration
from metaflow import FlowSpec, step, Parameter
from datetime import datetime

class ProductionWorkflow(FlowSpec):
    model_version = Parameter('model_version', default='latest')
    environment = Parameter('environment', default='staging')
    enable_notifications = Parameter('enable_notifications', default=False)

    @step
    def start(self):
        """Initialize workflow"""
        self.run_timestamp = datetime.now().isoformat()
        self.config = load_config(self.environment)
        print(f"Running {self.model_version} on {self.environment}")
        self.next(self.validate)

    @step
    def validate(self):
        """Validate inputs and configuration"""
        assert self.config is not None, "Configuration failed to load"
        self.next(self.execute)

    @step
    def execute(self):
        """Main workflow execution"""
        self.results = run_model(self.model_version, self.config)
        self.next(self.notify)

    @step
    def notify(self):
        """Send notifications if enabled"""
        if self.enable_notifications:
            send_notification(self.results)
        self.next(self.end)

    @step
    def end(self):
        """Complete and log results"""
        log_run_completion(self.results, self.run_timestamp)

if __name__ == '__main__':
    ProductionWorkflow()
```

Execute with parameters:
```bash
python flow.py run --model-version v2.1 --environment production --enable-notifications true
```

## Agent Use

AI agents can leverage Metaflow for:

- **Orchestrating multi-step data pipelines**: Automate complex data processing workflows with dependencies and error handling
- **Tracking experimental results**: Manage versions of training runs and compare metrics across iterations
- **Distributed data processing**: Split large datasets across parallel steps and aggregate results automatically
- **Production model deployment**: Run batch inference jobs with scheduling and monitoring
- **Parameter sweeps**: Execute multiple workflow instances with different configurations for hyperparameter tuning
- **Integration with cloud services**: Connect to AWS S3, Lambda, Step Functions for scalable execution

## Troubleshooting

### Installation issues

**Problem**: `pip install metaflow` fails with permission errors
```bash
# Try user-level installation
pip install --user metaflow

# Or use system package manager (macOS)
brew install metaflow
```

**Problem**: `metaflow` command not found after installation
```bash
# Ensure Python site-packages is in PATH
python -m metaflow --version

# Or add to PATH
export PATH="$HOME/.local/bin:$PATH"
```

### Workflow execution issues

**Problem**: Import errors when running workflows
```bash
# Verify installation
python -c "import metaflow; print(metaflow.__version__)"

# Check Python version compatibility
python --version  # Requires Python 3.7+
```

**Problem**: Workflow steps fail with data access errors
```bash
# Verify Metaflow home directory has correct permissions
ls -la ~/.metaflow/

# Check datastore configuration
python -c "from metaflow.util import get_metadata_service; print(get_metadata_service())"
```

### Configuration

**Check current configuration**:
```bash
python -c "from metaflow.environment import DefaultEnvironment; env = DefaultEnvironment(); print(env)"
```

**View workflow run history**:
```bash
# List all runs of a workflow
python flow.py show-runs | head -20

# View specific run details
python flow.py show-run <run-id>
```

## Uninstall

```yaml
- preset: metaflow
  with:
    state: absent
```

After uninstallation, optionally clean up stored data:
```bash
# Remove workflow history and cached data
rm -rf ~/.metaflow/

# Remove configuration
rm -rf ~/.metaflowconfig/
```

## Resources

- **Official documentation**: https://docs.metaflow.org/
- **GitHub repository**: https://github.com/Netflix/metaflow
- **Getting started guide**: https://docs.metaflow.org/getting-started/introduction
- **AWS integration**: https://docs.metaflow.org/metaflow-on-aws/introduction
- **Search**: "metaflow tutorial", "metaflow data science workflows", "metaflow python framework"
- **Community**: Metaflow issues and discussions on GitHub

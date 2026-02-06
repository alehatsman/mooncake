# dagster - Data Orchestration Platform

Dagster is a modern data orchestration platform for building, testing, and observing data pipelines with a focus on developer productivity and reliability.

## Quick Start
```yaml
- preset: dagster
```

## Features
- **Asset-centric**: Model data pipelines as versioned data assets
- **Type system**: Strong typing for data validation
- **Testing**: Unit test data pipelines locally
- **Observability**: Built-in lineage, logs, and monitoring
- **Scheduling**: Cron-based and sensor-driven execution
- **Multi-environment**: Dev, staging, prod separation

## Basic Usage
```bash
# Check version
dagster --version

# Initialize new project
dagster project scaffold --name my-dagster-project

# Start Dagster UI (dagit)
dagster dev

# Run pipeline
dagster job execute -m my_project -j my_job

# Launch scheduled run
dagster schedule execute my_schedule

# View asset materializations
dagster asset materialize my_asset

# Run tests
pytest
```

## Advanced Configuration
```yaml
- preset: dagster
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove dagster |

## Platform Support
- ✅ Linux (pip, binary)
- ✅ macOS (pip, binary)
- ❌ Windows (not yet supported by preset, but pip works)

## Configuration
- **Config file**: `dagster.yaml` (workspace configuration)
- **Web UI**: http://localhost:3000 (default for `dagster dev`)
- **Storage**: SQLite (dev), PostgreSQL (prod recommended)
- **Logs**: Stored in `$DAGSTER_HOME` or configured location

## Real-World Examples

### Simple Data Pipeline
```python
# my_pipeline.py
from dagster import asset, Definitions

@asset
def raw_data():
    """Download raw data from source."""
    import pandas as pd
    return pd.read_csv("https://example.com/data.csv")

@asset
def cleaned_data(raw_data):
    """Clean and transform raw data."""
    # Remove nulls, normalize columns
    return raw_data.dropna()

@asset
def analytics(cleaned_data):
    """Generate analytics from cleaned data."""
    return {
        "total_rows": len(cleaned_data),
        "avg_value": cleaned_data["value"].mean()
    }

# Define repository
defs = Definitions(
    assets=[raw_data, cleaned_data, analytics]
)
```

### ETL with Resources
```python
from dagster import asset, resource, Definitions
import psycopg2

@resource
def database_connection(context):
    """PostgreSQL connection resource."""
    conn = psycopg2.connect(
        host=context.resource_config["host"],
        database=context.resource_config["database"],
        user=context.resource_config["user"],
        password=context.resource_config["password"]
    )
    try:
        yield conn
    finally:
        conn.close()

@asset(required_resource_keys={"database"})
def users_table(context):
    """Extract users from database."""
    conn = context.resources.database
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM users")
    return cursor.fetchall()

@asset
def user_analytics(users_table):
    """Compute user statistics."""
    import pandas as pd
    df = pd.DataFrame(users_table, columns=["id", "name", "email", "created_at"])
    return {
        "total_users": len(df),
        "new_users_today": len(df[df["created_at"].dt.date == pd.Timestamp.today().date()])
    }

defs = Definitions(
    assets=[users_table, user_analytics],
    resources={
        "database": database_connection.configured({
            "host": "localhost",
            "database": "myapp",
            "user": "user",
            "password": "pass"
        })
    }
)
```

### Scheduled Job
```python
from dagster import asset, schedule, define_asset_job, Definitions
import datetime

@asset
def daily_report():
    """Generate daily analytics report."""
    return {"date": datetime.date.today(), "metrics": {}}

# Define job from assets
daily_job = define_asset_job("daily_job", selection="daily_report")

@schedule(
    job=daily_job,
    cron_schedule="0 9 * * *",  # 9 AM daily
)
def daily_schedule():
    """Run daily report at 9 AM."""
    return {}

defs = Definitions(
    assets=[daily_report],
    jobs=[daily_job],
    schedules=[daily_schedule]
)
```

### Sensors (Event-Driven)
```python
from dagster import sensor, RunRequest, asset, Definitions
import os

@asset
def process_file(context, file_path: str):
    """Process uploaded file."""
    context.log.info(f"Processing file: {file_path}")
    # Process logic here
    return {"file": file_path, "status": "processed"}

@sensor(job_name="process_file_job")
def file_sensor(context):
    """Watch directory for new files."""
    directory = "/data/uploads"
    processed_files = context.cursor or []

    for filename in os.listdir(directory):
        if filename not in processed_files:
            yield RunRequest(
                run_key=filename,
                run_config={"ops": {"process_file": {"config": {"file_path": filename}}}}
            )
            processed_files.append(filename)

    context.update_cursor(",".join(processed_files))

defs = Definitions(
    assets=[process_file],
    sensors=[file_sensor]
)
```

### Testing Pipelines
```python
# test_my_pipeline.py
from dagster import materialize
from my_pipeline import raw_data, cleaned_data, analytics

def test_pipeline():
    """Test full pipeline execution."""
    result = materialize([raw_data, cleaned_data, analytics])
    assert result.success

def test_cleaned_data():
    """Test data cleaning logic."""
    # Mock raw data
    mock_raw = pd.DataFrame({"value": [1, None, 3, 4]})

    # Test transformation
    result = cleaned_data(mock_raw)
    assert len(result) == 3  # Nulls removed
    assert result["value"].isna().sum() == 0
```

### Multi-Environment Configuration
```yaml
# dagster.yaml
run_storage:
  module: dagster_postgres.run_storage
  class: PostgresRunStorage
  config:
    postgres_url:
      env: DAGSTER_POSTGRES_URL

event_log_storage:
  module: dagster_postgres.event_log
  class: PostgresEventLogStorage
  config:
    postgres_url:
      env: DAGSTER_POSTGRES_URL

schedule_storage:
  module: dagster_postgres.schedule_storage
  class: PostgresScheduleStorage
  config:
    postgres_url:
      env: DAGSTER_POSTGRES_URL
```

### Partitioned Assets
```python
from dagster import asset, DailyPartitionsDefinition
import datetime

daily_partition = DailyPartitionsDefinition(start_date="2024-01-01")

@asset(partitions_def=daily_partition)
def daily_sales(context):
    """Sales data partitioned by day."""
    partition_date = context.partition_key
    # Fetch sales for specific date
    return fetch_sales_for_date(partition_date)

@asset(partitions_def=daily_partition)
def daily_revenue(daily_sales):
    """Revenue calculated from daily sales."""
    return sum(sale["amount"] for sale in daily_sales)
```

## Dagster UI (dagit)
```bash
# Start UI on custom port
dagster dev -h 0.0.0.0 -p 3001

# Start with specific workspace
dagster dev -w workspace.yaml

# Start in production mode
dagster-daemon run &
dagit -h 0.0.0.0 -p 3000
```

## Integration Examples
```python
# dbt integration
from dagster_dbt import dbt_cli_resource, dbt_run_op

@op(required_resource_keys={"dbt"})
def run_dbt_models(context):
    context.resources.dbt.run()

# Spark integration
from dagster_pyspark import pyspark_resource

@asset(required_resource_keys={"pyspark"})
def spark_transform(context):
    spark = context.resources.pyspark.spark_session
    df = spark.read.csv("s3://bucket/data.csv")
    return df.select("id", "value").toPandas()

# Airflow migration
from dagster_airflow import make_dagster_definitions_from_airflow_dags_path

defs = make_dagster_definitions_from_airflow_dags_path("/path/to/airflow/dags")
```

## Agent Use
- Orchestrate data pipelines and ETL workflows
- Schedule and monitor batch jobs
- Build data lakes and warehouses
- ML pipeline orchestration
- Event-driven data processing
- Asset lineage and data quality tracking

## Troubleshooting

### Asset materialization fails
Check logs and dependencies:
```bash
# View detailed logs
dagster asset materialize my_asset --log-level DEBUG

# Check asset dependencies
dagster asset list --show-deps
```

### Dagit won't start
Verify storage configuration:
```bash
# Check database connection
export DAGSTER_POSTGRES_URL="postgresql://user:pass@localhost/dagster"

# Reset SQLite database (dev only)
rm -rf $DAGSTER_HOME/storage

# Start with fresh workspace
dagster dev
```

### Schedule not triggering
Check daemon status:
```bash
# Ensure daemon is running
dagster-daemon run

# Check schedule status
dagster schedule list

# Enable schedule
dagster schedule start my_schedule
```

## Uninstall
```yaml
- preset: dagster
  with:
    state: absent
```

## Resources
- Official docs: https://docs.dagster.io/
- Getting started: https://docs.dagster.io/getting-started
- Examples: https://github.com/dagster-io/dagster/tree/master/examples
- Search: "dagster tutorial", "dagster orchestration examples"

# Apache Airflow - Workflow Orchestration

Platform for programmatically authoring, scheduling, and monitoring workflows as Directed Acyclic Graphs (DAGs). Orchestrate data pipelines and batch processing jobs.

## Quick Start
```yaml
- preset: airflow
```

## Features
- **DAG-based workflows**: Define pipelines as code in Python
- **Rich operators**: 100+ built-in operators for common tasks
- **Dynamic pipelines**: Generate DAGs programmatically
- **Scheduling**: Cron-based and data-driven scheduling
- **Monitoring**: Web UI with task status and logs
- **Extensible**: Custom operators, sensors, and hooks
- **Scalability**: Celery, Kubernetes, or Dask executors

## Basic Usage
```bash
# Initialize database
airflow db init

# Create admin user
airflow users create \
  --username admin \
  --password admin \
  --firstname Admin \
  --lastname User \
  --role Admin \
  --email admin@example.com

# Start web server
airflow webserver --port 8080

# Start scheduler
airflow scheduler

# Access UI: http://localhost:8080
```

## Advanced Configuration
```yaml
- preset: airflow
  with:
    state: present
  become: true
```

## Configuration
- **Config file**: `~/airflow/airflow.cfg` or `$AIRFLOW_HOME/airflow.cfg`
- **DAGs directory**: `~/airflow/dags/`
- **Logs**: `~/airflow/logs/`
- **Web UI**: http://localhost:8080
- **Database**: SQLite (dev) or PostgreSQL/MySQL (prod)
- **Default port**: 8080

## Real-World Examples

### Simple ETL DAG
```python
from airflow import DAG
from airflow.operators.bash import BashOperator
from airflow.operators.python import PythonOperator
from datetime import datetime, timedelta

default_args = {
    'owner': 'data-team',
    'depends_on_past': False,
    'email_on_failure': True,
    'email_on_retry': False,
    'retries': 2,
    'retry_delay': timedelta(minutes=5),
}

dag = DAG(
    'etl_pipeline',
    default_args=default_args,
    description='Daily ETL pipeline',
    schedule_interval='0 2 * * *',  # 2 AM daily
    start_date=datetime(2024, 1, 1),
    catchup=False,
)

extract = BashOperator(
    task_id='extract_data',
    bash_command='python /scripts/extract.py',
    dag=dag,
)

transform = PythonOperator(
    task_id='transform_data',
    python_callable=transform_function,
    dag=dag,
)

load = BashOperator(
    task_id='load_to_warehouse',
    bash_command='python /scripts/load.py',
    dag=dag,
)

extract >> transform >> load
```

### Deploy with Mooncake
```yaml
- name: Install Airflow
  preset: airflow

- name: Initialize Airflow
  shell: |
    export AIRFLOW_HOME=~/airflow
    airflow db init
    airflow users create --username admin --password admin \
      --firstname Admin --lastname User --role Admin \
      --email admin@example.com

- name: Deploy DAGs
  copy:
    src: dags/
    dest: ~/airflow/dags/
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pip)
- ✅ macOS (Homebrew, pip)
- ❌ Windows (WSL recommended)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated data pipeline deployment
- ETL job scheduling
- Batch processing orchestration
- ML pipeline coordination
- Report generation workflows
- Data quality monitoring

## Troubleshooting

### Scheduler not picking up DAGs
Check DAG folder and parsing errors.
```bash
# Verify DAG folder
ls ~/airflow/dags/

# Test DAG parsing
airflow dags list

# Check for errors
airflow dags list-import-errors
```

### Database initialization fails
Reset and reinitialize database.
```bash
# Reset database
airflow db reset

# Reinitialize
airflow db init
```

### Tasks stuck in "running" state
Clear task state and rerun.
```bash
# Clear task instance
airflow tasks clear dag_id task_id

# Or via UI: Browse > Task Instances > Clear
```

## Uninstall
```yaml
- preset: airflow
  with:
    state: absent
```

## Resources
- Official docs: https://airflow.apache.org/docs/
- GitHub: https://github.com/apache/airflow
- Astronomer docs: https://docs.astronomer.io/
- Search: "airflow dag examples", "airflow best practices"

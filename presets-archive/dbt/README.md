# dbt - Data Build Tool

Transform data in your warehouse using SQL SELECT statements. Modern data transformation workflow enabling analytics engineers to transform data using software engineering practices.

## Quick Start
```yaml
- preset: dbt
```

## Features
- **SQL-based transformations**: Transform data using SELECT statements
- **Version control**: Manage transformations in Git
- **Testing**: Built-in data quality tests
- **Documentation**: Auto-generated data catalog
- **Modularity**: Reusable SQL with Jinja templating
- **DAG orchestration**: Automatic dependency resolution

## Basic Usage
```bash
# Initialize new project
dbt init my_project

# Run models
dbt run

# Test data
dbt test

# Generate documentation
dbt docs generate
dbt docs serve

# Build everything
dbt build

# Run specific model
dbt run --select my_model

# Run models with tag
dbt run --select tag:daily
```

## Project Structure
```
my_dbt_project/
├── dbt_project.yml          # Project config
├── profiles.yml             # Database connections
├── models/                  # SQL transformations
│   ├── staging/            # Raw data cleaning
│   ├── marts/              # Business logic
│   └── schema.yml          # Tests and docs
├── macros/                 # Reusable SQL functions
├── tests/                  # Custom data tests
├── seeds/                  # CSV files to load
└── snapshots/              # Type-2 slowly changing dimensions
```

## Configuration

### profiles.yml
**Location**: `~/.dbt/profiles.yml`

```yaml
my_project:
  target: dev
  outputs:
    dev:
      type: postgres
      host: localhost
      port: 5432
      user: dbt_user
      password: "{{ env_var('DBT_PASSWORD') }}"
      dbname: analytics
      schema: dbt_dev
      threads: 4

    prod:
      type: postgres
      host: prod-db.example.com
      port: 5432
      user: dbt_prod
      password: "{{ env_var('DBT_PROD_PASSWORD') }}"
      dbname: analytics
      schema: dbt_prod
      threads: 8
```

### dbt_project.yml
```yaml
name: 'my_analytics'
version: '1.0.0'
config-version: 2

profile: 'my_project'

model-paths: ["models"]
analysis-paths: ["analyses"]
test-paths: ["tests"]
seed-paths: ["seeds"]
macro-paths: ["macros"]
snapshot-paths: ["snapshots"]

target-path: "target"
clean-targets:
  - "target"
  - "dbt_packages"

models:
  my_analytics:
    staging:
      +materialized: view
    marts:
      +materialized: table
```

## Model Development

### Basic Model
```sql
-- models/staging/stg_customers.sql
select
    customer_id,
    customer_name,
    email,
    created_at
from {{ source('raw', 'customers') }}
where deleted_at is null
```

### With Transformations
```sql
-- models/marts/fct_orders.sql
with orders as (
    select * from {{ ref('stg_orders') }}
),

customers as (
    select * from {{ ref('stg_customers') }}
)

select
    orders.order_id,
    orders.order_date,
    customers.customer_name,
    orders.order_amount,
    orders.order_status
from orders
left join customers
    on orders.customer_id = customers.customer_id
```

### Using Macros
```sql
-- models/marts/customer_summary.sql
select
    customer_id,
    {{ cents_to_dollars('total_amount') }} as total_dollars,
    count(*) as order_count
from {{ ref('fct_orders') }}
group by 1
```

## Testing

### schema.yml
```yaml
version: 2

models:
  - name: stg_customers
    description: Staging table for customers
    columns:
      - name: customer_id
        description: Primary key
        tests:
          - unique
          - not_null
      - name: email
        tests:
          - unique
          - not_null

  - name: fct_orders
    description: Order facts
    tests:
      - dbt_utils.expression_is_true:
          expression: "order_amount >= 0"
    columns:
      - name: order_id
        tests:
          - unique
          - not_null
      - name: customer_id
        tests:
          - relationships:
              to: ref('stg_customers')
              field: customer_id
```

### Custom Tests
```sql
-- tests/assert_positive_revenue.sql
select
    order_id,
    order_amount
from {{ ref('fct_orders') }}
where order_amount < 0
```

## Real-World Examples

### CI/CD Pipeline
```yaml
- name: Install dbt
  preset: dbt
  become: true

- name: Configure dbt profile
  template:
    dest: ~/.dbt/profiles.yml
    content: |
      my_project:
        target: {{ target_env }}
        outputs:
          prod:
            type: postgres
            host: {{ db_host }}
            user: {{ db_user }}
            password: {{ db_password }}
            dbname: analytics
            schema: dbt_prod
            threads: 8

- name: Install dbt dependencies
  shell: dbt deps
  cwd: /app/dbt_project

- name: Run dbt models
  shell: dbt run --target prod
  cwd: /app/dbt_project
  register: dbt_run

- name: Run dbt tests
  shell: dbt test --target prod
  cwd: /app/dbt_project

- name: Generate documentation
  shell: |
    dbt docs generate --target prod
    aws s3 cp target/catalog.json s3://docs/dbt/
  cwd: /app/dbt_project
```

### Development Workflow
```yaml
- name: Setup dbt development
  preset: dbt

- name: Clone dbt project
  shell: git clone https://github.com/company/dbt-analytics.git
  cwd: /home/dev

- name: Install packages
  shell: dbt deps
  cwd: /home/dev/dbt-analytics

- name: Run subset of models
  shell: dbt run --select staging.*
  cwd: /home/dev/dbt-analytics
```

### Daily Refresh
```bash
#!/bin/bash
# Daily data refresh script

cd /app/dbt_project

# Pull latest changes
git pull origin main

# Install/update dependencies
dbt deps

# Run models
dbt run --target prod

# Run tests
if ! dbt test --target prod; then
  echo "Tests failed!" | mail -s "dbt Alert" team@company.com
  exit 1
fi

# Update documentation
dbt docs generate --target prod

echo "dbt run completed successfully"
```

## Advanced Features

### Incremental Models
```sql
-- models/fct_events.sql
{{
  config(
    materialized='incremental',
    unique_key='event_id'
  )
}}

select
    event_id,
    event_timestamp,
    user_id,
    event_type
from {{ source('raw', 'events') }}

{% if is_incremental() %}
    where event_timestamp > (select max(event_timestamp) from {{ this }})
{% endif %}
```

### Snapshots
```sql
-- snapshots/customers_snapshot.sql
{% snapshot customers_snapshot %}
{{
    config(
      target_schema='snapshots',
      unique_key='customer_id',
      strategy='timestamp',
      updated_at='updated_at'
    )
}}

select * from {{ source('raw', 'customers') }}

{% endsnapshot %}
```

### Macros
```sql
-- macros/cents_to_dollars.sql
{% macro cents_to_dollars(column_name) %}
    ({{ column_name }} / 100.0)::numeric(10,2)
{% endmacro %}
```

### Sources
```yaml
# models/staging/sources.yml
version: 2

sources:
  - name: raw
    database: raw_data
    schema: public
    tables:
      - name: customers
        loaded_at_field: _loaded_at
        freshness:
          warn_after: {count: 12, period: hour}
          error_after: {count: 24, period: hour}
      - name: orders
        loaded_at_field: _loaded_at
```

## Commands

```bash
# Run models
dbt run                          # All models
dbt run --select my_model        # Specific model
dbt run --select staging.*       # All in folder
dbt run --select tag:daily       # By tag
dbt run --select +my_model       # Model and upstream
dbt run --select my_model+       # Model and downstream
dbt run --exclude staging.*      # Exclude folder

# Testing
dbt test                         # All tests
dbt test --select my_model       # Tests for model
dbt test --store-failures        # Store failures in database

# Documentation
dbt docs generate                # Generate docs
dbt docs serve                   # Serve on localhost:8080

# Compilation
dbt compile                      # Compile without running
dbt compile --select my_model    # Compile specific model

# Dependencies
dbt deps                         # Install packages
dbt clean                        # Clean artifacts

# Debugging
dbt debug                        # Test connections
dbt show --select my_model       # Preview results
dbt show --inline "select 1"     # Run inline query

# Snapshots
dbt snapshot                     # Run all snapshots

# Seeds
dbt seed                         # Load CSV files
```

## Database Adapters

### PostgreSQL
```yaml
outputs:
  prod:
    type: postgres
    host: postgres.example.com
    port: 5432
    user: dbt_user
    password: "{{ env_var('DBT_PASSWORD') }}"
    dbname: analytics
    schema: dbt_prod
    threads: 4
```

### Snowflake
```yaml
outputs:
  prod:
    type: snowflake
    account: xy12345.us-east-1
    user: dbt_user
    password: "{{ env_var('DBT_PASSWORD') }}"
    role: transformer
    database: ANALYTICS
    warehouse: TRANSFORMING
    schema: DBT_PROD
    threads: 8
```

### BigQuery
```yaml
outputs:
  prod:
    type: bigquery
    method: service-account
    project: my-project
    dataset: dbt_prod
    threads: 4
    keyfile: /path/to/keyfile.json
    location: US
```

### Redshift
```yaml
outputs:
  prod:
    type: redshift
    host: redshift-cluster.amazonaws.com
    port: 5439
    user: dbt_user
    password: "{{ env_var('DBT_PASSWORD') }}"
    dbname: analytics
    schema: dbt_prod
    threads: 4
```

## Package Management

### packages.yml
```yaml
packages:
  - package: dbt-labs/dbt_utils
    version: 1.1.1

  - package: calogica/dbt_expectations
    version: 0.10.1

  - git: "https://github.com/company/internal-macros.git"
    revision: main
```

## Troubleshooting

### Connection Issues
```bash
# Test database connection
dbt debug

# Check profiles.yml
cat ~/.dbt/profiles.yml

# Verify environment variables
echo $DBT_PASSWORD
```

### Model Failures
```bash
# Run with verbose logging
dbt run --select my_model --log-level debug

# Compile to see SQL
dbt compile --select my_model
cat target/compiled/my_project/models/my_model.sql

# Preview results
dbt show --select my_model
```

### Test Failures
```bash
# Run tests with failures stored
dbt test --store-failures

# View failed tests
select * from analytics.dbt_test_failures

# Debug specific test
dbt test --select my_model --store-failures
```

### Performance Issues
```bash
# Check execution time
dbt run --select my_model --log-level info

# Use more threads
dbt run --threads 8

# Check query plan
dbt compile --select my_model
# Then run EXPLAIN on compiled SQL
```

## Best Practices

### Project Organization
```
models/
├── staging/           # Clean raw data
│   ├── crm/          # Source system
│   └── ecommerce/
├── intermediate/      # Business logic
│   └── finance/
└── marts/            # Final models
    ├── marketing/
    └── product/
```

### Naming Conventions
- `stg_` - Staging models
- `int_` - Intermediate models
- `fct_` - Fact tables
- `dim_` - Dimension tables
- `rpt_` - Reports

### Performance
- Use incremental models for large tables
- Materialize as table for frequently queried models
- Use views for simple transformations
- Leverage database-specific features (clustering, partitioning)
- Monitor query performance with logs

## Platform Support
- ✅ Linux (pip, apt)
- ✅ macOS (pip, Homebrew)
- ✅ Windows (pip)
- ✅ Docker (official images)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated data transformation pipelines
- CI/CD for analytics code
- Scheduled data refreshes
- Data quality monitoring
- Documentation generation
- Testing data integrity
- Version control for transformations
- Collaboration on data models


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install dbt
  preset: dbt

- name: Use dbt in automation
  shell: |
    # Custom configuration here
    echo "dbt configured"
```
## Uninstall
```yaml
- preset: dbt
  with:
    state: absent
```

## Resources
- Official docs: https://docs.getdbt.com/
- Getting started: https://docs.getdbt.com/docs/introduction
- Best practices: https://docs.getdbt.com/guides/best-practices
- Community: https://www.getdbt.com/community/
- GitHub: https://github.com/dbt-labs/dbt-core
- Search: "dbt tutorial", "dbt best practices", "dbt sql transformations"

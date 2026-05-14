# Singer - Open Source ETL Framework

Open standard for data extraction and loading. Connect data sources (taps) to destinations (targets) using simple JSON streams.

## Quick Start
```yaml
- preset: singer
```

## Features
- **Open standard**: JSON-based messaging protocol
- **Modular**: Mix and match taps and targets
- **Extensible**: 200+ community taps and targets
- **Language agnostic**: Write taps/targets in any language
- **Schema evolution**: Automatic schema detection
- **Incremental sync**: Bookmark-based state management
- **Stream processing**: Handle large datasets efficiently

## Basic Usage
```bash
# Check version
pip show singer-python

# Run tap to target
tap-postgres --config postgres-config.json | target-jsonl

# With state for incremental sync
tap-postgres --config config.json --state state.json | \
  target-jsonl >> output.jsonl

# Discover schema
tap-postgres --config config.json --discover > catalog.json
```

## Architecture

### Components
```
┌─────────┐      JSON       ┌─────────┐      JSON       ┌─────────┐
│   TAP   │ ─────────────> │ STREAM  │ ─────────────> │ TARGET  │
│(Extract)│   SCHEMA/       │(Transform)│   RECORD/      │ (Load)  │
└─────────┘   RECORD        └─────────┘   STATE         └─────────┘
```

### Message Types
1. **SCHEMA**: Describes data structure
2. **RECORD**: Contains actual data
3. **STATE**: Bookmark for incremental sync

## Installation

### Singer Python SDK
```bash
# Install SDK
pip install singer-python

# For development
pip install singer-python[dev]
```

### Popular Taps (Sources)
```bash
# Databases
pip install tap-postgres
pip install tap-mysql
pip install tap-mongodb

# SaaS
pip install tap-salesforce
pip install tap-github
pip install tap-google-analytics

# Files
pip install tap-csv
pip install tap-s3-csv
```

### Popular Targets (Destinations)
```bash
# Databases
pip install target-postgres
pip install target-snowflake
pip install target-bigquery

# Data warehouses
pip install target-redshift
pip install target-s3

# Files
pip install target-jsonl
pip install target-csv
```

## Configuration

### Tap Config (tap-postgres example)
```json
{
  "host": "localhost",
  "port": 5432,
  "database": "mydb",
  "user": "myuser",
  "password": "mypass"
}
```

### Target Config (target-postgres example)
```json
{
  "host": "warehouse.example.com",
  "port": 5432,
  "database": "analytics",
  "user": "loader",
  "password": "secret",
  "schema": "public"
}
```

### Catalog (Schema Selection)
```json
{
  "streams": [
    {
      "tap_stream_id": "public-users",
      "stream": "users",
      "schema": {
        "type": "object",
        "properties": {
          "id": {"type": "integer"},
          "name": {"type": "string"},
          "created_at": {"type": "string", "format": "date-time"}
        }
      },
      "metadata": [
        {
          "breadcrumb": [],
          "metadata": {
            "selected": true,
            "replication-method": "INCREMENTAL",
            "replication-key": "updated_at"
          }
        }
      ]
    }
  ]
}
```

## Basic ETL Pipeline

### Step 1: Discover Schema
```bash
# Generate catalog
tap-postgres --config tap-config.json --discover > catalog.json

# Edit catalog to select streams
vim catalog.json
# Set "selected": true for streams to sync
```

### Step 2: Initial Sync
```bash
# Run sync
tap-postgres \
  --config tap-config.json \
  --catalog catalog.json | \
  target-postgres \
  --config target-config.json > state.json
```

### Step 3: Incremental Sync
```bash
# Use state for bookmarking
tap-postgres \
  --config tap-config.json \
  --catalog catalog.json \
  --state state.json | \
  target-postgres \
  --config target-config.json > state-new.json

# Update state
mv state-new.json state.json
```

## Advanced Usage

### With Transformations (Meltano Singer SDK)
```bash
# Install transform
pip install singer-transform

# Transform and load
tap-postgres --config config.json --catalog catalog.json | \
  singer-transform --transform schema.json | \
  target-postgres --config target-config.json
```

### Logging and Monitoring
```bash
# Enable debug logging
export LOG_LEVEL=debug

# With metrics
tap-postgres --config config.json | \
  target-postgres --config target-config.json 2>&1 | \
  tee pipeline.log
```

### Error Handling
```bash
# Capture errors
set -e
tap-postgres --config config.json --catalog catalog.json | \
  target-postgres --config target-config.json || {
    echo "Pipeline failed"
    exit 1
  }
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install Singer
  run: |
    pip install tap-postgres target-bigquery
    pip install singer-python

- name: Run ETL
  env:
    TAP_CONFIG: ${{ secrets.POSTGRES_CONFIG }}
    TARGET_CONFIG: ${{ secrets.BIGQUERY_CONFIG }}
  run: |
    echo "$TAP_CONFIG" > tap-config.json
    echo "$TARGET_CONFIG" > target-config.json

    tap-postgres --config tap-config.json --catalog catalog.json | \
      target-bigquery --config target-config.json > state.json

- name: Save state
  run: |
    aws s3 cp state.json s3://my-bucket/singer-state/state.json
```

### Scheduled Pipeline (cron)
```bash
#!/bin/bash
# singer-sync.sh

set -e

# Load state from S3
aws s3 cp s3://my-bucket/state.json state.json || echo '{}' > state.json

# Run sync
tap-postgres \
  --config /etc/singer/tap-config.json \
  --catalog /etc/singer/catalog.json \
  --state state.json | \
  target-postgres \
  --config /etc/singer/target-config.json > state-new.json

# Save state
mv state-new.json state.json
aws s3 cp state.json s3://my-bucket/state.json

echo "Sync completed at $(date)"
```

### Cron job
```cron
# Run every hour
0 * * * * /opt/singer/singer-sync.sh >> /var/log/singer.log 2>&1
```

## Real-World Examples

### Postgres to BigQuery
```yaml
- name: Install Singer tools
  shell: pip3 install tap-postgres target-bigquery

- name: Configure tap
  template:
    content: |
      {
        "host": "{{ source_db_host }}",
        "port": 5432,
        "database": "{{ source_db_name }}",
        "user": "{{ source_db_user }}",
        "password": "{{ source_db_pass }}"
      }
    dest: /etc/singer/tap-config.json
    mode: "0600"

- name: Discover schema
  shell: |
    tap-postgres --config /etc/singer/tap-config.json \
      --discover > /etc/singer/catalog.json

- name: Run sync
  shell: |
    tap-postgres \
      --config /etc/singer/tap-config.json \
      --catalog /etc/singer/catalog.json | \
    target-bigquery \
      --config /etc/singer/target-config.json
```

### API to Data Warehouse
```bash
# GitHub issues to Snowflake
pip install tap-github target-snowflake

# Configure
cat > github-config.json <<EOF
{
  "access_token": "ghp_xxx",
  "repository": "owner/repo"
}
EOF

# Sync
tap-github --config github-config.json --catalog catalog.json | \
  target-snowflake --config snowflake-config.json
```

## Writing Custom Taps

### Simple Tap Example (Python)
```python
#!/usr/bin/env python3
import singer
from datetime import datetime

# Define schema
schema = {
    'properties': {
        'id': {'type': 'integer'},
        'name': {'type': 'string'},
        'created_at': {'type': 'string', 'format': 'date-time'}
    }
}

# Write schema
singer.write_schema('users', schema, ['id'])

# Write records
records = [
    {'id': 1, 'name': 'Alice', 'created_at': datetime.now().isoformat()},
    {'id': 2, 'name': 'Bob', 'created_at': datetime.now().isoformat()}
]

for record in records:
    singer.write_record('users', record)

# Write state
singer.write_state({'last_id': 2})
```

### Run Custom Tap
```bash
chmod +x my-tap.py
./my-tap.py | target-jsonl
```

## Common Patterns

### Full Table Replication
```json
{
  "metadata": [{
    "breadcrumb": [],
    "metadata": {
      "selected": true,
      "replication-method": "FULL_TABLE"
    }
  }]
}
```

### Incremental Replication
```json
{
  "metadata": [{
    "breadcrumb": [],
    "metadata": {
      "selected": true,
      "replication-method": "INCREMENTAL",
      "replication-key": "updated_at"
    }
  }]
}
```

### Log-Based Replication
```json
{
  "metadata": [{
    "breadcrumb": [],
    "metadata": {
      "selected": true,
      "replication-method": "LOG_BASED"
    }
  }]
}
```

## Troubleshooting

### Connection Issues
```bash
# Test tap connection
tap-postgres --config config.json --discover

# Test target connection
echo '{"type": "SCHEMA", "stream": "test", "schema": {"type": "object"}}' | \
  target-postgres --config config.json
```

### Schema Mismatch
```bash
# Regenerate catalog
tap-postgres --config config.json --discover > catalog.json

# Validate catalog
cat catalog.json | jq '.streams[].schema'
```

### State Issues
```bash
# Reset state
echo '{}' > state.json

# Inspect state
cat state.json | jq .
```

## Best Practices
- Use incremental sync when possible
- Store state in durable storage (S3, database)
- Version control catalog files
- Monitor pipeline failures
- Use schema evolution features
- Test pipelines in staging first
- Document custom taps/targets
- Use official Singer SDK when available

## Platform Support
- ✅ Linux (Python 3.7+)
- ✅ macOS (Python 3.7+)
- ✅ Windows (Python 3.7+)
- ✅ Docker containers

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated data pipeline orchestration
- Schema discovery and documentation
- Data warehouse loading automation
- SaaS to database synchronization
- Log aggregation and ETL
- Multi-source data consolidation

## Advanced Configuration
```yaml
- preset: singer
  with:
    state: present
```

## Uninstall
```yaml
- preset: singer
  with:
    state: absent
```

## Resources
- Website: https://www.singer.io/
- Specification: https://github.com/singer-io/getting-started
- Hub: https://www.singer.io/#taps (200+ taps/targets)
- SDK: https://github.com/singer-io/singer-python
- Meltano: https://meltano.com/ (Singer orchestration)
- Search: "singer tap", "singer target", "singer etl"

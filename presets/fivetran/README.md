# Fivetran CLI - Data Pipeline Management

Command-line interface for managing Fivetran data connectors and pipelines. Automate ETL operations and monitor data synchronization.

## Quick Start
```yaml
- preset: fivetran
```

## Features
- **Connector management**: Create and configure data sources programmatically
- **Pipeline automation**: Automate ETL workflows and sync schedules
- **API access**: Full programmatic control of Fivetran resources
- **Monitoring**: Track sync status, data freshness, and pipeline health
- **Multi-cloud support**: Works with Snowflake, BigQuery, Redshift, Databricks

## Basic Usage
```bash
# Authenticate with API key
export FIVETRAN_API_KEY="your-api-key"
export FIVETRAN_API_SECRET="your-api-secret"

# List connectors
fivetran connector list

# Create database connector
fivetran connector create \
  --service postgres \
  --schema public \
  --destination my-warehouse

# Trigger manual sync
fivetran connector sync connector-id

# Check sync status
fivetran connector status connector-id

# View connector details
fivetran connector get connector-id
```

## Advanced Configuration
```yaml
- preset: fivetran
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Fivetran CLI |

## Platform Support
- ✅ Linux (pip, binary download)
- ✅ macOS (Homebrew, pip)
- ✅ Windows (pip, binary download)

## Configuration
- **API credentials**: Set `FIVETRAN_API_KEY` and `FIVETRAN_API_SECRET`
- **Config file**: `~/.fivetran/config`
- **Region**: Configure API endpoint for EU/US regions

## Real-World Examples

### Automated Pipeline Setup
```bash
# Create PostgreSQL to Snowflake pipeline
fivetran connector create \
  --service postgres \
  --host db.example.com \
  --port 5432 \
  --database production \
  --user fivetran_user \
  --schema public \
  --destination snowflake_warehouse

# Set sync frequency
fivetran connector update connector-id \
  --sync-frequency 360  # Every 6 hours
```

### CI/CD Integration
```yaml
# Automate schema changes
- name: Install Fivetran CLI
  preset: fivetran

- name: Update connector schema
  shell: |
    fivetran connector sync-schema connector-id
    fivetran connector sync connector-id --wait
```

## Agent Use
- Automate data pipeline provisioning in infrastructure-as-code
- Monitor ETL job status and trigger syncs programmatically
- Manage connectors across development/staging/production
- Integrate data pipelines with workflow orchestration
- Automate schema change management and migrations

## Troubleshooting

### Authentication failed
```bash
# Verify API credentials
echo $FIVETRAN_API_KEY
echo $FIVETRAN_API_SECRET

# Test connection
fivetran auth test
```

### Connector sync fails
```bash
# Check connector logs
fivetran connector logs connector-id

# Verify source database connectivity
fivetran connector test connector-id

# Force re-sync
fivetran connector resync connector-id
```

## Uninstall
```yaml
- preset: fivetran
  with:
    state: absent
```

## Resources
- Official docs: https://fivetran.com/docs/
- API docs: https://fivetran.com/docs/rest-api
- Connectors: https://fivetran.com/docs/databases
- Search: "fivetran cli", "fivetran api automation", "fivetran terraform"

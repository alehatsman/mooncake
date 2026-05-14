# Airbyte - Data Integration Platform

Open-source data integration platform for building ETL/ELT pipelines. Move data from APIs, databases, and files to warehouses, lakes, and databases.

## Quick Start
```yaml
- preset: airbyte
```

## Features
- **300+ connectors**: Pre-built sources and destinations
- **Custom connectors**: Build connectors with CDK (Connector Development Kit)
- **Change Data Capture**: Real-time data sync with CDC
- **Normalization**: Transform raw data to analytics-ready schemas
- **dbt integration**: Native dbt transformations
- **Open-source**: Self-hosted with full control
- **Scheduling**: Cron-based sync scheduling

## Basic Usage
```bash
# Start Airbyte (Docker)
docker-compose up -d

# Access web UI
# http://localhost:8000
# Default credentials: airbyte / password

# CLI usage (abctl)
abctl local up

# Check status
abctl local status

# View logs
abctl local logs
```

## Advanced Configuration
```yaml
- preset: airbyte
  with:
    state: present
  become: true
```

## Configuration
- **Web UI**: http://localhost:8000
- **API**: http://localhost:8001/api/v1/
- **Database**: PostgreSQL (bundled or external)
- **Storage**: Local filesystem or S3/GCS
- **Default credentials**: airbyte/password (change in production)

## Real-World Examples

### PostgreSQL to Snowflake Sync
```bash
# Configure via UI or API
curl -X POST http://localhost:8001/api/v1/connections/create \
  -H "Content-Type: application/json" \
  -d '{
    "sourceId": "postgres-source-id",
    "destinationId": "snowflake-dest-id",
    "schedule": {"units": 24, "timeUnit": "hours"}
  }'
```

### Custom Connector Development
```bash
# Install connector CDK
pip install airbyte-cdk

# Create new connector
airbyte-python-cdk generate source my-api

# Test connector
python main.py check --config config.json
python main.py discover --config config.json
```

### Infrastructure as Code
```yaml
# Deploy Airbyte with preset
- name: Deploy Airbyte
  preset: airbyte

- name: Configure source
  shell: |
    curl -X POST http://localhost:8001/api/v1/sources/create \
      -d @postgres-source.json
```

## Platform Support
- ✅ Linux (Docker, Kubernetes, apt)
- ✅ macOS (Docker, Homebrew)
- ❌ Windows (Docker Desktop only)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated data pipeline setup
- Connector provisioning
- Sync schedule management
- Data warehouse population
- API data extraction
- Database replication

## Troubleshooting

### Connector sync failing
Check source/destination credentials and connectivity.
```bash
# View connector logs
docker logs airbyte-worker

# Test connection
curl http://localhost:8001/api/v1/sources/check_connection \
  -d '{"sourceId": "source-id"}'
```

### Out of memory errors
Increase Docker memory allocation or worker resources.
```bash
# Edit docker-compose.yml
services:
  worker:
    environment:
      - JAVA_OPTS=-Xmx2g
```

### Database connection issues
Verify PostgreSQL is accessible and configured.
```bash
# Check database
docker exec airbyte-db psql -U airbyte -c '\l'

# Reset database
abctl local reset
```

## Uninstall
```yaml
- preset: airbyte
  with:
    state: absent
```

## Resources
- Official docs: https://docs.airbyte.com/
- Connector catalog: https://docs.airbyte.com/integrations/
- GitHub: https://github.com/airbytehq/airbyte
- Search: "airbyte tutorial", "airbyte connectors", "airbyte vs fivetran"

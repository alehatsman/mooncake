# loki-server - Grafana Loki Log Aggregation System

Horizontally-scalable, highly-available log aggregation system inspired by Prometheus, designed for storing and querying logs from all your applications and infrastructure.

## Quick Start
```yaml
- preset: loki-server
```

## Features
- **Label-based indexing**: Index logs by labels, not full-text
- **S3/GCS compatible**: Store logs in object storage
- **Grafana integration**: Native support in Grafana dashboards
- **LogQL**: Prometheus-like query language for logs
- **Multi-tenancy**: Isolate logs by tenant ID
- **Compression**: Efficient log storage with chunk compression

## Basic Usage
```bash
# Start Loki server
loki -config.file=/etc/loki/config.yml

# Start with different storage backend
loki -config.file=/etc/loki/config.yml -target=all

# Check version
loki --version

# Validate configuration
loki -config.file=/etc/loki/config.yml -verify-config

# Run specific component
loki -target=ingester -config.file=/etc/loki/config.yml
```

## Advanced Configuration
```yaml
- preset: loki-server
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove loki-server |

## Platform Support
- ✅ Linux (binary download, Docker)
- ✅ macOS (Homebrew, binary)
- ✅ Kubernetes (Helm charts)

## Configuration
- **Config file**: `/etc/loki/config.yml` (default)
- **Data directory**: `/var/lib/loki` (configurable)
- **HTTP API**: Port 3100 (default)
- **gRPC**: Port 9095 (default)

## Real-World Examples

### Basic Configuration
```yaml
# /etc/loki/config.yml
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9095

common:
  path_prefix: /var/lib/loki
  storage:
    filesystem:
      chunks_directory: /var/lib/loki/chunks
      rules_directory: /var/lib/loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
  ingestion_rate_mb: 10
  ingestion_burst_size_mb: 20
```

### S3 Storage Backend
```yaml
# /etc/loki/config.yml with S3
common:
  storage:
    s3:
      s3: s3://us-east-1/my-loki-bucket
      endpoint: s3.amazonaws.com
      access_key_id: ${AWS_ACCESS_KEY_ID}
      secret_access_key: ${AWS_SECRET_ACCESS_KEY}

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: s3
      schema: v11
      index:
        prefix: index_
        period: 24h
```

### Deployment with Promtail
```yaml
# Install Loki server
- name: Install Loki
  preset: loki-server

- name: Create Loki config
  copy:
    dest: /etc/loki/config.yml
    content: |
      auth_enabled: false
      server:
        http_listen_port: 3100
      common:
        path_prefix: /var/lib/loki
        storage:
          filesystem:
            chunks_directory: /var/lib/loki/chunks

- name: Start Loki service
  service:
    name: loki
    state: started
    enabled: true

# Install Promtail for log shipping
- name: Install Promtail
  preset: promtail
```

### Query Logs via LogQL
```bash
# Simple label query
curl -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query={job="varlogs"}'

# Query with filter
curl -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query={job="varlogs"} |= "error"'

# Aggregation
curl -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query=sum(rate({job="varlogs"}[5m]))'

# Time range query
curl -G "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={job="varlogs"}' \
  --data-urlencode 'start=2023-01-01T00:00:00Z' \
  --data-urlencode 'end=2023-01-01T12:00:00Z'
```

## Agent Use
- Centralized log aggregation for distributed systems
- Log retention and archival automation
- Multi-tenant log isolation
- Cost-effective log storage with object storage backends
- Integration with monitoring and alerting pipelines

## Troubleshooting

### Loki won't start
Check configuration syntax:
```bash
loki -config.file=/etc/loki/config.yml -verify-config
```

Check logs:
```bash
journalctl -u loki -f
```

### Storage permissions
Fix directory permissions:
```bash
sudo mkdir -p /var/lib/loki/chunks
sudo chown -R loki:loki /var/lib/loki
```

### High memory usage
Tune ingestion limits:
```yaml
# /etc/loki/config.yml
limits_config:
  ingestion_rate_mb: 4
  ingestion_burst_size_mb: 6
  max_streams_per_user: 0
  max_global_streams_per_user: 5000
```

### Query performance slow
Add query frontend:
```yaml
query_range:
  split_queries_by_interval: 24h
  cache_results: true
```

## Uninstall
```yaml
- preset: loki-server
  with:
    state: absent
```

**Note**: Does not remove data directory. Remove manually if needed:
```bash
sudo rm -rf /var/lib/loki
```

## Resources
- Official docs: https://grafana.com/docs/loki/
- GitHub: https://github.com/grafana/loki
- LogQL guide: https://grafana.com/docs/loki/latest/logql/
- Helm charts: https://github.com/grafana/helm-charts
- Search: "loki log aggregation", "loki grafana tutorial"

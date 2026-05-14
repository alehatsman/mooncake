# cortex - Scalable Prometheus for Kubernetes

Cortex provides horizontally scalable, highly available, multi-tenant, long-term storage for Prometheus metrics.

## Quick Start
```yaml
- preset: cortex
```

## Features
- **Horizontally scalable**: Shard and replicate for high throughput
- **Multi-tenant**: Isolated metrics per tenant with authentication
- **Long-term storage**: S3, GCS, Azure Blob support
- **Prometheus compatible**: Works with existing PromQL queries
- **High availability**: Replication and redundancy
- **Cost-effective**: Compress and deduplicate metrics

## Basic Usage
```bash
# Start Cortex (single binary mode)
cortex -target=all -config.file=cortex.yaml

# Query metrics
curl "http://localhost:9009/prometheus/api/v1/query?query=up"

# Push metrics (Prometheus remote_write)
# Configure in prometheus.yml:
# remote_write:
#   - url: http://cortex:9009/api/prom/push

# Check configuration
cortex -config.file=cortex.yaml -print-config

# Check Cortex cluster status
curl http://localhost:9009/services
```

## Advanced Configuration
```yaml
- preset: cortex
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove cortex |

## Platform Support
- ✅ Linux (binary download, Docker, Kubernetes)
- ✅ macOS (binary download, Docker)
- ❌ Windows (not yet supported)

## Configuration
- **Config file**: `cortex.yaml` (YAML format)
- **Data directory**: Specified per component (ingester, querier, etc.)
- **Default port**: 9009 (HTTP API)
- **Storage**: S3, GCS, Azure Blob, or local filesystem

## Real-World Examples

### Minimal Configuration (Single Binary)
```yaml
# cortex.yaml
auth_enabled: false

server:
  http_listen_port: 9009

distributor:
  ring:
    kvstore:
      store: inmemory

ingester:
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1

storage:
  engine: blocks

blocks_storage:
  backend: filesystem
  filesystem:
    dir: ./data/blocks

ruler_storage:
  backend: filesystem
  filesystem:
    dir: ./data/rules
```

### Production Configuration with S3
```yaml
# cortex-prod.yaml
auth_enabled: true

server:
  http_listen_port: 9009
  grpc_listen_port: 9095

distributor:
  ring:
    kvstore:
      store: consul
      consul:
        host: consul:8500

ingester:
  lifecycler:
    ring:
      kvstore:
        store: consul
        consul:
          host: consul:8500
      replication_factor: 3
  chunk_idle_period: 30m
  max_chunk_idle: 1h

storage:
  engine: blocks

blocks_storage:
  backend: s3
  s3:
    endpoint: s3.amazonaws.com
    bucket_name: cortex-metrics
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
  tsdb:
    dir: /data/tsdb
    retention_period: 30d
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cortex
spec:
  replicas: 3
  selector:
    matchLabels:
      app: cortex
  template:
    metadata:
      labels:
        app: cortex
    spec:
      containers:
      - name: cortex
        image: quay.io/cortexproject/cortex:latest
        args:
        - -target=all
        - -config.file=/etc/cortex/cortex.yaml
        ports:
        - containerPort: 9009
          name: http
        - containerPort: 9095
          name: grpc
        volumeMounts:
        - name: config
          mountPath: /etc/cortex
        - name: data
          mountPath: /data
      volumes:
      - name: config
        configMap:
          name: cortex-config
      - name: data
        persistentVolumeClaim:
          claimName: cortex-data
```

### Prometheus Remote Write Setup
```yaml
# prometheus.yml
remote_write:
  - url: http://cortex:9009/api/prom/push
    queue_config:
      capacity: 10000
      max_shards: 50
      min_shards: 1
      max_samples_per_send: 5000
      batch_send_deadline: 5s
    # Optional: tenant ID header
    headers:
      X-Scope-OrgID: tenant-1
```

### Multi-Tenant Configuration
```yaml
# Enable auth
auth_enabled: true

# Limits per tenant
limits:
  ingestion_rate: 100000
  ingestion_burst_size: 200000
  max_series_per_metric: 0
  max_series_per_query: 100000
  max_samples_per_query: 50000000

# Per-tenant overrides
limits_config:
  per_tenant_override_config: /etc/cortex/overrides.yaml
```

```yaml
# overrides.yaml
overrides:
  tenant-1:
    ingestion_rate: 200000
    ingestion_burst_size: 400000
  tenant-2:
    ingestion_rate: 50000
    ingestion_burst_size: 100000
```

### Grafana Integration
```yaml
# Add Cortex as datasource in Grafana
apiVersion: 1
datasources:
- name: Cortex
  type: prometheus
  access: proxy
  url: http://cortex:9009/prometheus
  jsonData:
    timeInterval: 30s
  # For multi-tenant setup
  httpHeaderName1: X-Scope-OrgID
  secureJsonData:
    httpHeaderValue1: tenant-1
```

## Microservices Mode
Run components independently for better scaling:

```bash
# Distributor (receives metrics)
cortex -target=distributor -config.file=cortex.yaml

# Ingester (stores in-memory metrics)
cortex -target=ingester -config.file=cortex.yaml

# Querier (queries metrics)
cortex -target=querier -config.file=cortex.yaml

# Query-frontend (caches and splits queries)
cortex -target=query-frontend -config.file=cortex.yaml

# Compactor (compacts blocks)
cortex -target=compactor -config.file=cortex.yaml

# Store-gateway (loads blocks for queries)
cortex -target=store-gateway -config.file=cortex.yaml

# Ruler (evaluates rules)
cortex -target=ruler -config.file=cortex.yaml
```

## Querying Metrics
```bash
# Instant query
curl -G "http://localhost:9009/prometheus/api/v1/query" \
  -H "X-Scope-OrgID: tenant-1" \
  --data-urlencode 'query=up'

# Range query
curl -G "http://localhost:9009/prometheus/api/v1/query_range" \
  -H "X-Scope-OrgID: tenant-1" \
  --data-urlencode 'query=rate(http_requests_total[5m])' \
  --data-urlencode 'start=2024-01-01T00:00:00Z' \
  --data-urlencode 'end=2024-01-01T01:00:00Z' \
  --data-urlencode 'step=15s'

# Label values
curl "http://localhost:9009/prometheus/api/v1/label/job/values" \
  -H "X-Scope-OrgID: tenant-1"

# Series metadata
curl -G "http://localhost:9009/prometheus/api/v1/series" \
  -H "X-Scope-OrgID: tenant-1" \
  --data-urlencode 'match[]=up'
```

## Monitoring Cortex
```bash
# Cortex metrics endpoint
curl http://localhost:9009/metrics

# Key metrics to monitor:
# - cortex_ingester_memory_series (in-memory series)
# - cortex_ingester_chunks_stored_total (chunks stored)
# - cortex_query_frontend_queries_total (query load)
# - cortex_distributor_received_samples_total (ingestion rate)
```

## Agent Use
- Centralized multi-cluster Prometheus storage
- Long-term metrics retention (months/years)
- Multi-tenant metrics isolation
- High-cardinality metrics at scale
- Cost-effective metrics storage
- Global query interface across clusters

## Troubleshooting

### Ingestion failures
Check distributor and ingester logs:
```bash
# Check distributor
curl http://localhost:9009/distributor/ring

# Check ingester ring
curl http://localhost:9009/ingester/ring

# View metrics
curl http://localhost:9009/metrics | grep cortex_distributor_received_samples
```

### Query performance
Optimize query splitting:
```yaml
# Increase query parallelism
query_range:
  split_queries_by_interval: 24h
  align_queries_with_step: true
  cache_results: true

# Tune query limits
limits:
  max_query_lookback: 720h
  max_query_length: 32d
  max_query_parallelism: 32
```

### Storage issues
Check compactor and store-gateway:
```bash
# Compactor status
curl http://localhost:9009/compactor/ring

# Store-gateway status
curl http://localhost:9009/store-gateway/ring

# Check block uploads
aws s3 ls s3://cortex-metrics/ --recursive
```

## Uninstall
```yaml
- preset: cortex
  with:
    state: absent
```

## Resources
- Official docs: https://cortexmetrics.io/docs/
- GitHub: https://github.com/cortexproject/cortex
- Architecture: https://cortexmetrics.io/docs/architecture/
- Search: "cortex prometheus", "cortex metrics storage"

# m3db - Distributed Time-Series Database

Distributed time-series database designed for storing and querying large volumes of metrics at scale, built by Uber for real-time monitoring and analytics.

## Quick Start
```yaml
- preset: m3db
```

## Features
- **Distributed architecture**: Horizontally scalable storage and query
- **High cardinality**: Handle millions of unique time series
- **Aggregation**: Downsampling and rollup policies
- **Prometheus compatible**: Native Prometheus remote read/write
- **Fast queries**: Optimized for time-range queries
- **Replication**: Configurable replication factor for durability

## Basic Usage
```bash
# Start M3DB coordinator
m3dbnode -f /etc/m3db/m3dbnode.yml

# Check cluster health
curl http://localhost:7201/health

# Query namespace info
curl http://localhost:7201/api/v1/services/m3db/namespace

# Bootstrap database
curl -X POST http://localhost:7201/api/v1/database/create -d '{
  "namespaceName": "default",
  "retentionTime": "48h"
}'

# Query metrics
curl -G http://localhost:7201/api/v1/query_range \
  --data-urlencode 'query=up{job="node"}' \
  --data-urlencode 'start=2023-01-01T00:00:00Z' \
  --data-urlencode 'end=2023-01-01T12:00:00Z' \
  --data-urlencode 'step=60s'
```

## Advanced Configuration
```yaml
- preset: m3db
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove m3db |

## Platform Support
- ✅ Linux (binary download, Docker)
- ✅ macOS (binary download)
- ✅ Kubernetes (Helm charts)

## Configuration
- **Config file**: `/etc/m3db/m3dbnode.yml`
- **Data directory**: `/var/lib/m3db` (configurable)
- **Coordinator port**: 7201 (default)
- **Client port**: 9000 (default)

## Real-World Examples

### Basic M3DB Configuration
```yaml
# /etc/m3db/m3dbnode.yml
db:
  listenAddress: 0.0.0.0:9000
  clusterListenAddress: 0.0.0.0:9001
  httpNodeListenAddress: 0.0.0.0:9002
  httpClusterListenAddress: 0.0.0.0:9003
  debugListenAddress: 0.0.0.0:9004

  hostID:
    resolver: config
    value: m3db_local

  client:
    writeConsistencyLevel: majority
    readConsistencyLevel: unstrict_majority

  gcPercentage: 100

  writeNewSeriesAsync: true
  writeNewSeriesBackoffDuration: 2ms

  commitlog:
    flushMaxBytes: 524288
    flushEvery: 1s
    queue:
      calculationType: fixed
      size: 2097152

  filesystem:
    filePathPrefix: /var/lib/m3db
    writeBufferSize: 65536
    dataReadBufferSize: 65536
    infoReadBufferSize: 128
    seekReadBufferSize: 4096
```

### Prometheus Integration
```yaml
# /etc/prometheus/prometheus.yml
remote_write:
  - url: http://localhost:7201/api/v1/prom/remote/write
    queue_config:
      capacity: 10000
      max_shards: 200
      min_shards: 1
      max_samples_per_send: 1000

remote_read:
  - url: http://localhost:7201/api/v1/prom/remote/read
    read_recent: true
```

### Multi-Node Cluster Setup
```yaml
# Deploy M3DB cluster
- name: Install M3DB on node 1
  preset: m3db
  delegate_to: m3db-node-1

- name: Install M3DB on node 2
  preset: m3db
  delegate_to: m3db-node-2

- name: Install M3DB on node 3
  preset: m3db
  delegate_to: m3db-node-3

- name: Create namespace with replication
  shell: |
    curl -X POST http://m3db-node-1:7201/api/v1/database/create -d '{
      "type": "cluster",
      "namespaceName": "metrics",
      "retentionTime": "48h",
      "replicationFactor": 3
    }'
```

### Aggregation and Downsampling
```yaml
# Configure downsampling policies
- name: Create aggregated namespace
  shell: |
    curl -X POST http://localhost:7201/api/v1/database/namespace/create -d '{
      "namespaceName": "metrics_5m",
      "retentionOptions": {
        "retentionPeriodNanos": "2592000000000000",
        "blockSizeNanos": "3600000000000"
      },
      "aggregationOptions": {
        "aggregations": [{
          "aggregated": true,
          "attributes": {
            "resolutionNanos": "300000000000",
            "downsampleOptions": {
              "all": false
            }
          }
        }]
      }
    }'
```

## Agent Use
- Large-scale metrics storage and querying
- Prometheus long-term storage backend
- Real-time monitoring dashboards
- Multi-tenant metrics isolation
- High-cardinality metrics handling

## Troubleshooting

### M3DB won't start
Check configuration syntax:
```bash
m3dbnode -f /etc/m3db/m3dbnode.yml -validate-config
```

Check logs:
```bash
journalctl -u m3dbnode -f
```

### Database initialization fails
Check if namespace already exists:
```bash
curl http://localhost:7201/api/v1/services/m3db/namespace
```

### High memory usage
Tune cache settings:
```yaml
db:
  cache:
    series:
      policy: lru
      size: 1048576
```

### Slow queries
Enable query tracing:
```bash
curl -G http://localhost:7201/api/v1/query_range \
  --data-urlencode 'query=up{job="node"}' \
  --data-urlencode 'debug=true'
```

### Replication lag
Check cluster health:
```bash
curl http://localhost:7201/api/v1/services/m3db/placement
```

## Uninstall
```yaml
- preset: m3db
  with:
    state: absent
```

**Note**: Does not remove data directory. Remove manually if needed:
```bash
sudo rm -rf /var/lib/m3db
```

## Resources
- Official docs: https://m3db.io/docs/
- GitHub: https://github.com/m3db/m3
- Operator: https://github.com/m3db/m3db-operator
- Helm charts: https://m3db.io/docs/operator/getting_started/
- Search: "m3db time series database", "m3db prometheus integration"

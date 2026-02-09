# Thanos - Prometheus Long-Term Storage

Highly available Prometheus setup with unlimited storage capacity. Scale Prometheus to months/years of retention, query across multiple Prometheus instances, and store metrics in object storage (S3, GCS, Azure).

## Quick Start
```yaml
- preset: thanos
```

## Features
- **Unlimited retention**: Store metrics in S3/GCS/Azure for years
- **Global query view**: Query across multiple Prometheus instances
- **Downsampling**: Automatic downsampling for long-term storage efficiency
- **High availability**: Prometheus federation with automatic deduplication
- **Cost-effective**: Object storage is 10x cheaper than persistent disks
- **Prometheus-compatible**: Drop-in replacement for Prometheus queries
- **Multi-tenant**: Separate data per tenant/environment
- **No vendor lock-in**: Use any S3-compatible object storage

## Architecture

### Components
```
┌─────────────────┐
│  Prometheus 1   │──┐
└────────┬────────┘  │
         │           │
    ┌────▼────┐      │
    │ Sidecar │──────┼──┐
    └─────────┘      │  │
                     │  │
┌─────────────────┐  │  │      ┌──────────────┐
│  Prometheus 2   │──┘  ├─────▶│ Object Store │
└────────┬────────┘     │      │  (S3/GCS)    │
         │              │      └──────────────┘
    ┌────▼────┐         │              │
    │ Sidecar │─────────┘              │
    └─────────┘                        │
                                       │
    ┌─────────────┐            ┌───────▼────────┐
    │ Query/API   │◀───────────│ Store Gateway  │
    └─────────────┘            └────────────────┘
         │
    ┌────▼──────┐
    │ Compactor │
    └───────────┘
```

### Component Roles
- **Sidecar**: Runs alongside Prometheus, uploads blocks to object storage
- **Query**: Provides Prometheus-compatible API, queries all data sources
- **Store Gateway**: Serves historical data from object storage
- **Compactor**: Downsamples and compacts blocks in object storage
- **Ruler**: Evaluates recording and alerting rules across data sources
- **Receiver**: Receives metrics via Prometheus remote write API

## Basic Usage

### Query data
```bash
# Query via Thanos API (Prometheus-compatible)
curl 'http://localhost:10902/api/v1/query?query=up'

# Query with time range
curl 'http://localhost:10902/api/v1/query_range?query=rate(requests_total[5m])&start=1609459200&end=1609545600&step=60'

# Check store status
curl http://localhost:10902/api/v1/stores
```

### Component commands
```bash
# Sidecar (alongside Prometheus)
thanos sidecar \
  --prometheus.url=http://localhost:9090 \
  --tsdb.path=/var/lib/prometheus \
  --objstore.config-file=/etc/thanos/bucket.yml

# Query (global view)
thanos query \
  --http-address=0.0.0.0:10902 \
  --store=sidecar-1:10901 \
  --store=sidecar-2:10901 \
  --store=store-gateway:10901

# Store Gateway (object storage)
thanos store \
  --data-dir=/var/thanos/store \
  --objstore.config-file=/etc/thanos/bucket.yml

# Compactor (maintenance)
thanos compact \
  --data-dir=/var/thanos/compact \
  --objstore.config-file=/etc/thanos/bucket.yml \
  --retention.resolution-raw=30d \
  --retention.resolution-5m=180d \
  --retention.resolution-1h=365d
```

## Advanced Configuration

### Sidecar with S3 backend
```yaml
- name: Install Thanos
  preset: thanos

- name: Configure S3 bucket
  template:
    src: thanos-bucket.yml.j2
    dest: /etc/thanos/bucket.yml
    mode: '0600'
  become: true

- name: Start Thanos Sidecar
  shell: |
    thanos sidecar \
      --prometheus.url=http://localhost:9090 \
      --tsdb.path=/var/lib/prometheus \
      --objstore.config-file=/etc/thanos/bucket.yml \
      --grpc-address=0.0.0.0:10901 \
      --http-address=0.0.0.0:10902
  async: true
  become: true
```

### Complete Thanos stack
```yaml
- name: Install Prometheus
  preset: prometheus

- name: Install Thanos
  preset: thanos

# Sidecar (per Prometheus instance)
- name: Start Thanos Sidecar
  service:
    name: thanos-sidecar
    state: started
    unit:
      content: |
        [Unit]
        Description=Thanos Sidecar
        After=prometheus.service

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/thanos sidecar \
          --prometheus.url=http://localhost:9090 \
          --tsdb.path=/var/lib/prometheus \
          --objstore.config-file=/etc/thanos/bucket.yml \
          --grpc-address=0.0.0.0:10901

        [Install]
        WantedBy=multi-user.target
  become: true

# Query (central)
- name: Start Thanos Query
  service:
    name: thanos-query
    state: started
    unit:
      content: |
        [Unit]
        Description=Thanos Query
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/thanos query \
          --http-address=0.0.0.0:10902 \
          --grpc-address=0.0.0.0:10901 \
          --store=sidecar-1.internal:10901 \
          --store=sidecar-2.internal:10901 \
          --store=store-gateway.internal:10901

        [Install]
        WantedBy=multi-user.target
  become: true

# Store Gateway (central)
- name: Start Thanos Store Gateway
  service:
    name: thanos-store
    state: started
    unit:
      content: |
        [Unit]
        Description=Thanos Store Gateway
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/thanos store \
          --data-dir=/var/thanos/store \
          --objstore.config-file=/etc/thanos/bucket.yml \
          --grpc-address=0.0.0.0:10901

        [Install]
        WantedBy=multi-user.target
  become: true

# Compactor (central, singleton)
- name: Start Thanos Compactor
  service:
    name: thanos-compact
    state: started
    unit:
      content: |
        [Unit]
        Description=Thanos Compactor
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/thanos compact \
          --data-dir=/var/thanos/compact \
          --objstore.config-file=/etc/thanos/bucket.yml \
          --retention.resolution-raw=30d \
          --retention.resolution-5m=180d \
          --retention.resolution-1h=365d \
          --wait

        [Install]
        WantedBy=multi-user.target
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Thanos |

## Platform Support
- ✅ Linux (all distributions) - via binary
- ✅ macOS (Homebrew)
- ✅ Docker (official images)
- ❌ Windows (use Docker)

## Object Storage Configuration

### S3 (AWS)
```yaml
# /etc/thanos/bucket.yml
type: S3
config:
  bucket: "my-thanos-bucket"
  endpoint: "s3.amazonaws.com"
  region: "us-east-1"
  access_key: "AKIAIOSFODNN7EXAMPLE"
  secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  insecure: false
  signature_version2: false
  sse_config:
    type: "SSE-S3"
```

### Google Cloud Storage (GCS)
```yaml
# /etc/thanos/bucket.yml
type: GCS
config:
  bucket: "my-thanos-bucket"
  service_account: |
    {
      "type": "service_account",
      "project_id": "my-project",
      "private_key_id": "...",
      "private_key": "...",
      "client_email": "thanos@my-project.iam.gserviceaccount.com"
    }
```

### Azure Blob Storage
```yaml
# /etc/thanos/bucket.yml
type: AZURE
config:
  storage_account: "mythanosstorageaccount"
  storage_account_key: "key"
  container: "thanos"
```

### MinIO (S3-compatible)
```yaml
# /etc/thanos/bucket.yml
type: S3
config:
  bucket: "thanos"
  endpoint: "minio.internal:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  insecure: true
  signature_version2: false
```

## Prometheus Configuration

### Enable external labels (required)
```yaml
# prometheus.yml
global:
  external_labels:
    cluster: "us-east-1"
    replica: "A"
```

### Enable Thanos sidecar (optional)
```yaml
# prometheus.yml
storage:
  tsdb:
    min-block-duration: 2h
    max-block-duration: 2h
```

### Remote write to Thanos Receiver (alternative)
```yaml
# prometheus.yml
remote_write:
  - url: http://thanos-receiver:19291/api/v1/receive
    queue_config:
      capacity: 10000
      max_shards: 50
```

## Querying

### Query API (Prometheus-compatible)
```bash
# Instant query
curl 'http://thanos-query:10902/api/v1/query?query=up'

# Range query
curl 'http://thanos-query:10902/api/v1/query_range?query=rate(http_requests_total[5m])&start=1609459200&end=1609545600&step=60'

# Label values
curl 'http://thanos-query:10902/api/v1/label/__name__/values'

# Series
curl 'http://thanos-query:10902/api/v1/series?match[]=up'
```

### Deduplication
```bash
# Thanos automatically deduplicates metrics from HA Prometheus pairs
# Use external labels to identify replicas:
# replica: "A" and replica: "B"
```

### Partial response
```bash
# Query with partial response (continues if one store fails)
curl 'http://thanos-query:10902/api/v1/query?query=up&partial_response=true'
```

## Grafana Integration

### Add Thanos as datasource
```yaml
- name: Configure Thanos datasource
  shell: |
    curl -X POST http://admin:admin@localhost:3000/api/datasources \
      -H "Content-Type: application/json" \
      -d '{
        "name": "Thanos",
        "type": "prometheus",
        "url": "http://thanos-query:10902",
        "access": "proxy",
        "isDefault": true,
        "jsonData": {
          "timeInterval": "30s",
          "queryTimeout": "300s"
        }
      }'
```

## Retention Policies

### Compactor retention
```bash
# Raw resolution (no downsampling): 30 days
--retention.resolution-raw=30d

# 5-minute downsampling: 180 days
--retention.resolution-5m=180d

# 1-hour downsampling: 365 days (1 year)
--retention.resolution-1h=365d
```

### Downsampling levels
- **Raw**: Original data (e.g., 15s scrape interval)
- **5m**: 5-minute resolution (1 sample per 5 minutes)
- **1h**: 1-hour resolution (1 sample per hour)

### Storage savings
```
Raw:  100 GB/month
5m:   10 GB/month (10x reduction)
1h:   1 GB/month (100x reduction)
```

## Use Cases

### Multi-Cluster Prometheus
```yaml
# Cluster 1 (us-east-1)
- name: Deploy Prometheus with Thanos
  hosts: us-east-1
  tasks:
    - preset: prometheus
    - preset: thanos

    - name: Configure external labels
      template:
        src: prometheus.yml.j2
        dest: /etc/prometheus/prometheus.yml
      vars:
        cluster: "us-east-1"
        replica: "A"

# Cluster 2 (eu-west-1)
- name: Deploy Prometheus with Thanos
  hosts: eu-west-1
  tasks:
    - preset: prometheus
    - preset: thanos

    - name: Configure external labels
      template:
        src: prometheus.yml.j2
        dest: /etc/prometheus/prometheus.yml
      vars:
        cluster: "eu-west-1"
        replica: "A"

# Central Query
- name: Deploy Thanos Query
  hosts: monitoring
  tasks:
    - name: Start Thanos Query
      shell: |
        thanos query \
          --http-address=0.0.0.0:10902 \
          --store=us-east-1-sidecar:10901 \
          --store=eu-west-1-sidecar:10901
```

### Long-Term Storage Migration
```yaml
- name: Install Thanos
  preset: thanos

- name: Configure S3 bucket
  template:
    src: bucket.yml.j2
    dest: /etc/thanos/bucket.yml

- name: Upload historical blocks
  shell: |
    thanos tools bucket upload \
      --objstore.config-file=/etc/thanos/bucket.yml \
      /var/lib/prometheus/data
```

### Receiver Mode (Remote Write)
```yaml
- name: Start Thanos Receiver
  shell: |
    thanos receive \
      --grpc-address=0.0.0.0:10901 \
      --http-address=0.0.0.0:10902 \
      --remote-write.address=0.0.0.0:19291 \
      --tsdb.path=/var/thanos/receive \
      --objstore.config-file=/etc/thanos/bucket.yml \
      --label=receive_replica="0"
  async: true
```

## Monitoring Thanos

### Key metrics
```promql
# Query performance
thanos_query_api_instant_query_duration_seconds
thanos_query_api_range_query_duration_seconds

# Store health
thanos_store_nodes_grpc_connections

# Compactor progress
thanos_compact_group_compactions_total
thanos_compact_group_compaction_runs_completed_total

# Sidecar upload
thanos_shipper_uploads_total
thanos_shipper_upload_failures_total
```

### Health checks
```bash
# Component health
curl http://thanos-query:10902/-/healthy
curl http://thanos-store:10902/-/healthy
curl http://thanos-sidecar:10902/-/healthy
curl http://thanos-compact:10902/-/healthy

# Ready status
curl http://thanos-query:10902/-/ready
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Thanos
  preset: thanos
```

### Production monitoring stack
```yaml
- name: Setup Thanos monitoring
  hosts: monitoring
  tasks:
    - preset: prometheus
    - preset: thanos
    - preset: grafana

    - name: Configure Thanos bucket
      template:
        src: bucket.yml.j2
        dest: /etc/thanos/bucket.yml
        mode: '0600'
      become: true

    - name: Deploy all Thanos components
      shell: |
        # Start sidecar
        systemctl start thanos-sidecar

        # Start query
        systemctl start thanos-query

        # Start store gateway
        systemctl start thanos-store

        # Start compactor
        systemctl start thanos-compact
      become: true
```

## Agent Use
- **Long-term storage**: Retain Prometheus metrics for months/years
- **Multi-cluster monitoring**: Query across all Kubernetes/cloud environments
- **HA Prometheus**: Federate multiple Prometheus instances with deduplication
- **Cost optimization**: Use cheap object storage instead of expensive disks
- **Downsampling**: Automatic data aggregation for historical queries
- **Global query view**: Single API for all metrics across infrastructure

## Troubleshooting

### Sidecar not uploading blocks
```bash
# Check sidecar logs
journalctl -u thanos-sidecar -f

# Verify Prometheus external labels
curl http://localhost:9090/api/v1/status/config | jq '.data.yaml' | grep external_labels

# Check S3 connectivity
aws s3 ls s3://my-thanos-bucket/

# Verify block uploads
curl http://localhost:10902/api/v1/status/tsdb
```

### Query slow or timing out
```bash
# Increase query timeout
thanos query --query.timeout=5m

# Check store connections
curl http://localhost:10902/api/v1/stores

# Verify Store Gateway health
curl http://store-gateway:10902/-/healthy

# Check for missing blocks
thanos tools bucket inspect --objstore.config-file=/etc/thanos/bucket.yml
```

### Compactor not running
```bash
# Check compactor logs
journalctl -u thanos-compact -f

# Verify only ONE compactor is running (must be singleton)
ps aux | grep "thanos compact"

# Check compactor progress
curl http://localhost:10902/metrics | grep thanos_compact
```

### High object storage costs
```bash
# Reduce retention
--retention.resolution-raw=15d
--retention.resolution-5m=90d
--retention.resolution-1h=180d

# Enable downsampling
--downsampling.disable=false

# Verify compaction is running
curl http://compactor:10902/metrics | grep thanos_compact_group_compactions_total
```

### Store Gateway memory issues
```bash
# Reduce index cache size
--index-cache-size=250MB

# Limit concurrent queries
--store.grpc.series-sample-limit=100000
--store.grpc.series-max-concurrency=20
```

## Best Practices
- **External labels**: Always set unique labels per Prometheus instance
- **HA pairs**: Use `replica` labels for Prometheus HA (A/B)
- **Singleton compactor**: Run only ONE compactor per object storage bucket
- **Query layer**: Use multiple Query instances for HA
- **Retention**: Balance cost vs query performance (raw/5m/1h)
- **Block duration**: Keep Prometheus block duration at 2h for Thanos
- **Networking**: Use gRPC for store-to-query communication (faster than HTTP)
- **Security**: Encrypt object storage credentials, use IAM roles where possible

## Comparison

| Feature | Prometheus | Thanos | Cortex | VictoriaMetrics |
|---------|-----------|--------|--------|-----------------|
| Storage | Local disk | Object storage | Object storage | Local + S3 |
| Retention | Limited by disk | Unlimited | Unlimited | Unlimited |
| Query federation | ✅ | ✅ | ✅ | ✅ |
| Downsampling | ❌ | ✅ | ✅ | ✅ |
| Multi-tenancy | ❌ | ✅ | ✅ | ✅ |
| Compatibility | Native | 100% compatible | 100% compatible | 100% compatible |

## Uninstall
```yaml
- preset: thanos
  with:
    state: absent
```

**Note**: Object storage data is NOT deleted. Remove manually from S3/GCS if needed.

## Resources
- Official: https://thanos.io/
- Documentation: https://thanos.io/tip/thanos/getting-started.md/
- GitHub: https://github.com/thanos-io/thanos
- Examples: https://github.com/thanos-io/kube-thanos
- Slack: https://slack.cncf.io/ (#thanos)
- Search: "thanos prometheus", "thanos s3", "thanos query"

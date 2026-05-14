# VictoriaMetrics - High-Performance Time-Series Database

Fast, cost-effective, and scalable time-series database (TSDB) compatible with Prometheus. Handles millions of metrics with 10x better compression, lower memory usage, and faster queries than Prometheus. Drop-in replacement for Prometheus with extended query capabilities.

## Quick Start
```yaml
- preset: victoriametrics
```

Start server: `victoria-metrics` (default port: 8428)
Prometheus remote write: `http://localhost:8428/api/v1/write`
Query API: `http://localhost:8428/prometheus/api/v1/query`

## Features
- **Prometheus compatible**: Drop-in replacement for Prometheus remote storage
- **10x storage compression**: Efficient data encoding and compression
- **Low memory usage**: 7x less RAM than Prometheus for same workload
- **Fast queries**: MetricsQL with enhanced functions and performance
- **Vertical scalability**: Single-node scales to millions of metrics
- **Horizontal scalability**: Cluster mode for unlimited scale
- **Multi-tenancy**: Isolated namespaces for different teams/projects
- **Long-term storage**: Years of data retention at low cost
- **Downsampling**: Automatic data aggregation for long-term storage
- **Deduplication**: Automatic removal of duplicate data points
- **Grafana integration**: Native data source plugin

## Basic Usage
```bash
# Start single-node VictoriaMetrics
victoria-metrics

# Start with custom data directory
victoria-metrics -storageDataPath=/var/lib/victoria-metrics

# Start with retention period
victoria-metrics -retentionPeriod=12

# Query metrics
curl 'http://localhost:8428/api/v1/query?query=up'

# Health check
curl http://localhost:8428/health

# Metrics
curl http://localhost:8428/metrics
```

## Architecture

### Single-Node VictoriaMetrics
```
┌──────────────────────────────────────────────────┐
│                  Data Ingestion                  │
│  ┌────────────┐ ┌─────────┐ ┌────────────────┐  │
│  │ Prometheus │ │ vmagent │ │ Other sources  │  │
│  │remote_write│ │         │ │(Graphite, etc) │  │
│  └──────┬─────┘ └────┬────┘ └────────┬───────┘  │
│         │            │               │          │
│         └────────────┼───────────────┘          │
│                      │                          │
│         ┌────────────▼──────────────┐           │
│         │   VictoriaMetrics        │           │
│         │   - Compression           │           │
│         │   - Deduplication         │           │
│         │   - Downsampling          │           │
│         │   - Storage Engine        │           │
│         └────────────┬──────────────┘           │
│                      │                          │
│         ┌────────────▼──────────────┐           │
│         │   Query API (PromQL)      │           │
│         │   - /api/v1/query         │           │
│         │   - /api/v1/query_range   │           │
│         │   - MetricsQL extensions  │           │
│         └────────────┬──────────────┘           │
│                      │                          │
│         ┌────────────▼──────────────┐           │
│         │    Grafana / Clients      │           │
│         └───────────────────────────┘           │
└──────────────────────────────────────────────────┘
```

### Cluster Architecture
```
┌─────────────────────────────────────────────────────┐
│                    VMCluster                        │
│                                                     │
│  ┌──────────────────────────────────────────────┐  │
│  │             VMInsert (stateless)              │  │
│  │          Load balancer for writes            │  │
│  └────────────┬─────────────┬──────────────────┘  │
│               │             │                      │
│  ┌────────────▼──┐  ┌───────▼──────┐              │
│  │  VMStorage 1  │  │ VMStorage 2  │ ...          │
│  │  (stateful)   │  │  (stateful)  │              │
│  └────────────┬──┘  └───────┬──────┘              │
│               │             │                      │
│  ┌────────────▼─────────────▼──────────────────┐  │
│  │             VMSelect (stateless)              │  │
│  │          Load balancer for queries           │  │
│  └──────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

## Advanced Configuration

### Single-node production setup
```yaml
- name: Install VictoriaMetrics
  preset: victoriametrics

- name: Create data directory
  file:
    path: /var/lib/victoria-metrics
    state: directory
    owner: victoriametrics
    group: victoriametrics
    mode: '0755'
  become: true

- name: Configure VictoriaMetrics service
  service:
    name: victoria-metrics
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=VictoriaMetrics Time-Series Database
        After=network.target

        [Service]
        Type=simple
        User=victoriametrics
        Group=victoriametrics
        ExecStart=/usr/local/bin/victoria-metrics \
          -storageDataPath=/var/lib/victoria-metrics \
          -retentionPeriod=12 \
          -httpListenAddr=:8428 \
          -memory.allowedPercent=80
        Restart=always
        LimitNOFILE=65535

        [Install]
        WantedBy=multi-user.target
  become: true
```

### Multi-tenant configuration
```yaml
- name: Start VictoriaMetrics with multi-tenancy
  shell: |
    victoria-metrics \
      -storageDataPath=/var/lib/victoria-metrics \
      -retentionPeriod=12
  async: true

# Write to tenant 'team-a' (accountID=1)
- name: Configure Prometheus remote write for team-a
  template:
    src: prometheus-team-a.yml.j2
    dest: /etc/prometheus/prometheus.yml
  vars:
    remote_write_url: http://victoria-metrics:8428/api/v1/write
    tenant_id: "1:0"  # accountID:projectID

# Query from tenant 'team-a'
- name: Query team-a metrics
  shell: curl 'http://localhost:8428/select/1/prometheus/api/v1/query?query=up'
```

### Cluster deployment
```yaml
# VMStorage nodes (3 replicas)
- name: Deploy VMStorage
  shell: |
    vmstorage \
      -storageDataPath=/var/lib/vmstorage \
      -retentionPeriod=12 \
      -httpListenAddr=:8482 \
      -vminsertAddr=:8400 \
      -vmselectAddr=:8401
  async: true
  loop: "{{ range(3) }}"

# VMInsert nodes (2 replicas for HA)
- name: Deploy VMInsert
  shell: |
    vminsert \
      -storageNode=vmstorage-1:8400,vmstorage-2:8400,vmstorage-3:8400 \
      -httpListenAddr=:8480 \
      -replicationFactor=2
  async: true
  loop: "{{ range(2) }}"

# VMSelect nodes (2 replicas for HA)
- name: Deploy VMSelect
  shell: |
    vmselect \
      -storageNode=vmstorage-1:8401,vmstorage-2:8401,vmstorage-3:8401 \
      -httpListenAddr=:8481
  async: true
  loop: "{{ range(2) }}"
```

## Prometheus Integration

### Remote write configuration
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

remote_write:
  - url: http://victoria-metrics:8428/api/v1/write
    queue_config:
      max_samples_per_send: 10000
      batch_send_deadline: 5s
      max_shards: 30

scrape_configs:
  - job_name: 'my-app'
    static_configs:
      - targets: ['localhost:9090']
```

### Migrate from Prometheus
```yaml
- name: Install VictoriaMetrics
  preset: victoriametrics

- name: Install vmctl (migration tool)
  shell: |
    wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.93.0/vmutils-linux-amd64-v1.93.0.tar.gz
    tar xzf vmutils-linux-amd64-v1.93.0.tar.gz
    mv vmctl-prod /usr/local/bin/vmctl
  become: true

- name: Migrate Prometheus data
  shell: |
    vmctl prometheus \
      --prom-snapshot=/var/lib/prometheus/snapshots/20240101T000000Z-1234567890abcdef \
      --vm-addr=http://localhost:8428

- name: Update Prometheus to remote write
  template:
    src: prometheus.yml.j2
    dest: /etc/prometheus/prometheus.yml
  vars:
    remote_write_enabled: true
```

## Data Ingestion

### VMAgent (lightweight agent)
```yaml
- name: Install vmagent
  shell: |
    wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.93.0/victoria-metrics-linux-amd64-v1.93.0.tar.gz
    tar xzf victoria-metrics-linux-amd64-v1.93.0.tar.gz
    mv vmagent-prod /usr/local/bin/vmagent
  become: true

- name: Configure vmagent
  template:
    src: vmagent.yml.j2
    dest: /etc/vmagent/vmagent.yml

- name: Start vmagent
  shell: |
    vmagent \
      -promscrape.config=/etc/vmagent/vmagent.yml \
      -remoteWrite.url=http://victoria-metrics:8428/api/v1/write
  async: true
```

### Graphite protocol
```yaml
- name: Enable Graphite ingestion
  shell: |
    victoria-metrics \
      -graphiteListenAddr=:2003
  async: true

- name: Send Graphite metrics
  shell: echo "local.random.diceroll 4 `date +%s`" | nc localhost 2003
```

### InfluxDB line protocol
```yaml
- name: Enable InfluxDB ingestion
  shell: |
    victoria-metrics \
      -influxListenAddr=:8089
  async: true

- name: Send InfluxDB metrics
  shell: |
    curl -d 'measurement,tag1=value1 field1=123' \
      http://localhost:8089/write
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove VictoriaMetrics |

## Platform Support
- ✅ Linux (all distributions) - native binary
- ✅ macOS (Homebrew, native binary)
- ✅ Docker (official images)
- ❌ Windows (use WSL2 or Docker)

## Configuration Options

### Command-line flags
```bash
# Storage
-storageDataPath=/var/lib/victoria-metrics  # Data directory
-retentionPeriod=12                          # Retention in months
-storage.minFreeDiskSpaceBytes=10G           # Minimum free disk space

# Performance
-memory.allowedPercent=80                    # Max memory usage %
-search.maxQueryDuration=60s                 # Query timeout
-search.maxConcurrentRequests=8              # Concurrent queries

# HTTP
-httpListenAddr=:8428                        # HTTP listen address
-tls                                         # Enable TLS
-tlsCertFile=/path/to/cert.pem
-tlsKeyFile=/path/to/key.pem

# Deduplication
-dedup.minScrapeInterval=60s                 # Dedupe interval

# Downsampling
-downsampling.period=30d:5m                  # Downsample after 30d to 5m resolution
```

## Query Language

### PromQL compatibility
```bash
# Standard PromQL queries work
curl 'http://localhost:8428/api/v1/query?query=up'
curl 'http://localhost:8428/api/v1/query?query=rate(http_requests_total[5m])'

# Range queries
curl 'http://localhost:8428/api/v1/query_range?query=cpu_usage&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&step=1m'
```

### MetricsQL extensions
```bash
# Rollup functions
curl 'http://localhost:8428/api/v1/query?query=rollup_rate(http_requests_total[5m])'

# WITH expressions
curl 'http://localhost:8428/api/v1/query?query=WITH (commonFilters = {job="app"}) sum(rate(requests_total{commonFilters}[5m]))'

# Subqueries
curl 'http://localhost:8428/api/v1/query?query=max_over_time((avg(cpu_usage) by (instance))[1h:5m])'

# Histogram functions
curl 'http://localhost:8428/api/v1/query?query=histogram_quantile(0.99, http_request_duration_seconds_bucket)'
```

## Grafana Integration

### Add VictoriaMetrics data source
```yaml
- name: Configure Grafana data source
  shell: |
    curl -X POST http://admin:admin@grafana:3000/api/datasources \
      -H "Content-Type: application/json" \
      -d '{
        "name": "VictoriaMetrics",
        "type": "prometheus",
        "url": "http://victoria-metrics:8428",
        "access": "proxy",
        "isDefault": true
      }'

# Or use native VictoriaMetrics plugin
- name: Install VictoriaMetrics plugin
  shell: grafana-cli plugins install victoriametrics-datasource
```

### Import dashboards
```yaml
- name: Import VictoriaMetrics dashboard
  shell: |
    curl -X POST http://admin:admin@grafana:3000/api/dashboards/import \
      -H "Content-Type: application/json" \
      -d @/path/to/victoria-metrics-dashboard.json
```

## Use Cases

### Prometheus replacement
```yaml
- name: Install VictoriaMetrics
  preset: victoriametrics

- name: Start VictoriaMetrics
  shell: |
    victoria-metrics \
      -storageDataPath=/var/lib/victoria-metrics \
      -retentionPeriod=24
  async: true

- name: Configure Prometheus remote write
  template:
    src: prometheus.yml.j2
    dest: /etc/prometheus/prometheus.yml
  vars:
    remote_write_url: http://localhost:8428/api/v1/write

- name: Restart Prometheus
  service:
    name: prometheus
    state: restarted
```

### Multi-cluster monitoring
```yaml
- name: Deploy central VictoriaMetrics
  preset: victoriametrics

- name: Configure cluster1 Prometheus
  template:
    src: prometheus.yml.j2
    dest: /etc/prometheus/prometheus.yml
  vars:
    remote_write_url: http://victoria-metrics.central:8428/api/v1/write
    external_labels:
      cluster: cluster1

- name: Configure cluster2 Prometheus
  template:
    src: prometheus.yml.j2
    dest: /etc/prometheus/prometheus.yml
  vars:
    remote_write_url: http://victoria-metrics.central:8428/api/v1/write
    external_labels:
      cluster: cluster2
```

### Long-term storage with downsampling
```yaml
- name: Configure VictoriaMetrics with downsampling
  shell: |
    victoria-metrics \
      -storageDataPath=/var/lib/victoria-metrics \
      -retentionPeriod=60 \
      -downsampling.period=30d:5m,180d:1h
  async: true

# After 30 days: 5-minute resolution
# After 180 days: 1-hour resolution
```

## CLI Commands

### Query operations
```bash
# Instant query
curl 'http://localhost:8428/api/v1/query?query=up'

# Range query
curl 'http://localhost:8428/api/v1/query_range?query=cpu_usage&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&step=1m'

# Label values
curl 'http://localhost:8428/api/v1/label/__name__/values'

# Series metadata
curl 'http://localhost:8428/api/v1/series?match[]=up'
```

### Management operations
```bash
# Health check
curl http://localhost:8428/health

# Metrics
curl http://localhost:8428/metrics

# Force snapshot
curl -X POST http://localhost:8428/snapshot/create

# List snapshots
curl http://localhost:8428/snapshot/list

# Delete old data
curl -X POST 'http://localhost:8428/api/v1/admin/tsdb/delete_series?match[]=old_metric'
```

## Monitoring

### Key metrics
```bash
# Storage metrics
vm_data_size_bytes           # Data size on disk
vm_free_disk_space_bytes     # Available disk space
vm_rows                      # Number of data points

# Performance metrics
vm_http_requests_total       # Request count
vm_http_request_duration_seconds  # Request latency
vm_active_merges             # Background merge operations
vm_slow_queries_total        # Slow query count

# Memory metrics
process_resident_memory_bytes  # Memory usage
vm_cache_entries             # Cache size
```

### Prometheus alerts
```yaml
# prometheus-alerts.yml
groups:
  - name: victoriametrics
    rules:
      - alert: VictoriaMetricsDown
        expr: up{job="victoria-metrics"} == 0
        for: 5m

      - alert: VictoriaMetricsDiskSpaceLow
        expr: vm_free_disk_space_bytes < 10e9
        for: 5m

      - alert: VictoriaMetricsHighMemory
        expr: process_resident_memory_bytes > 8e9
        for: 10m

      - alert: VictoriaMetricsSlowQueries
        expr: rate(vm_slow_queries_total[5m]) > 0.1
        for: 5m
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install VictoriaMetrics
  preset: victoriametrics
```

### Single-node production
```yaml
- name: Install VictoriaMetrics
  preset: victoriametrics

- name: Create victoria-metrics user
  shell: useradd -r -s /bin/false victoriametrics
  become: true

- name: Create data directory
  file:
    path: /var/lib/victoria-metrics
    state: directory
    owner: victoriametrics
    group: victoriametrics
    mode: '0755'
  become: true

- name: Start VictoriaMetrics
  service:
    name: victoria-metrics
    state: started
    enabled: true
  become: true

- name: Wait for VictoriaMetrics
  shell: |
    for i in {1..30}; do
      curl -f http://localhost:8428/health && break
      sleep 1
    done
```

### Cluster deployment
```yaml
- name: Install VictoriaMetrics cluster
  preset: victoriametrics

- name: Deploy VMStorage
  service:
    name: vmstorage
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=VMStorage
        [Service]
        ExecStart=/usr/local/bin/vmstorage -storageDataPath=/var/lib/vmstorage -retentionPeriod=12
  become: true

- name: Deploy VMInsert
  service:
    name: vminsert
    state: started
    enabled: true
  become: true

- name: Deploy VMSelect
  service:
    name: vmselect
    state: started
    enabled: true
  become: true
```

## Agent Use
- **Prometheus replacement**: Drop-in replacement with better performance and compression
- **Long-term storage**: Store years of metrics at low cost
- **Multi-cluster monitoring**: Centralized metrics from multiple Prometheus instances
- **High-cardinality metrics**: Handle millions of unique time series
- **Cost optimization**: Reduce storage and memory costs by 10x
- **Multi-tenancy**: Isolated metrics for different teams
- **Downsampling**: Automatic data aggregation for old metrics

## Troubleshooting

### High memory usage
```bash
# Check memory stats
curl http://localhost:8428/metrics | grep process_resident_memory

# Reduce memory limit
victoria-metrics -memory.allowedPercent=60

# Enable data compression
victoria-metrics -storageDataPath=/var/lib/victoria-metrics -storage.minFreeDiskSpaceBytes=5G
```

### Slow queries
```bash
# Check slow query log
curl http://localhost:8428/metrics | grep vm_slow_queries_total

# Reduce query timeout
victoria-metrics -search.maxQueryDuration=30s

# Limit concurrent queries
victoria-metrics -search.maxConcurrentRequests=4

# Check query in logs
tail -f /var/log/victoria-metrics/victoria-metrics.log | grep slow
```

### Disk space issues
```bash
# Check disk usage
df -h /var/lib/victoria-metrics

# Reduce retention
victoria-metrics -retentionPeriod=6

# Force merge
curl -X POST http://localhost:8428/internal/force_merge

# Delete old series
curl -X POST 'http://localhost:8428/api/v1/admin/tsdb/delete_series?match[]=old_metric&start=2023-01-01T00:00:00Z&end=2023-12-31T23:59:59Z'
```

### Ingestion failures
```bash
# Check Prometheus remote write errors
curl http://prometheus:9090/metrics | grep prometheus_remote_storage_failed_samples_total

# Check VictoriaMetrics ingestion
curl http://localhost:8428/metrics | grep vm_rows_inserted_total

# Verify network connectivity
telnet victoria-metrics 8428

# Check VictoriaMetrics logs
journalctl -u victoria-metrics -f
```

## Best Practices

1. **Size storage appropriately**: 1GB RAM per 1M active time series
2. **Use deduplication**: Set `-dedup.minScrapeInterval` to scrape interval
3. **Enable downsampling**: Reduce storage for old metrics with `-downsampling.period`
4. **Monitor disk space**: Keep at least 20% free space for merges
5. **Use vmagent**: Lightweight replacement for Prometheus for scraping
6. **Cluster for scale**: Use cluster mode for >10M active time series
7. **Backup regularly**: Use `/snapshot/create` API for backups
8. **Optimize queries**: Use MetricsQL rollup functions for better performance
9. **Set retention**: Use `-retentionPeriod` to automatically delete old data
10. **Multi-tenancy**: Use tenant IDs to isolate metrics between teams

## Backup and Restore

### Create snapshot
```yaml
- name: Create VictoriaMetrics snapshot
  shell: curl -X POST http://localhost:8428/snapshot/create
  register: snapshot

- name: Backup snapshot
  shell: |
    rsync -av /var/lib/victoria-metrics/snapshots/{{ snapshot.stdout }} \
      /backup/victoria-metrics/
```

### Restore from snapshot
```yaml
- name: Stop VictoriaMetrics
  service:
    name: victoria-metrics
    state: stopped

- name: Restore snapshot
  shell: |
    rm -rf /var/lib/victoria-metrics/data
    cp -r /backup/victoria-metrics/snapshot-20240101/* /var/lib/victoria-metrics/data/

- name: Start VictoriaMetrics
  service:
    name: victoria-metrics
    state: started
```

## Uninstall
```yaml
- name: Stop VictoriaMetrics
  service:
    name: victoria-metrics
    state: stopped

- name: Remove VictoriaMetrics
  preset: victoriametrics
  with:
    state: absent

- name: Remove data
  file:
    path: /var/lib/victoria-metrics
    state: absent
  become: true
```

**Note**: Uninstalling does not remove data directory. Delete `/var/lib/victoria-metrics` manually if needed.

## Resources
- Official: https://victoriametrics.com/
- Documentation: https://docs.victoriametrics.com/
- GitHub: https://github.com/VictoriaMetrics/VictoriaMetrics
- Blog: https://victoriametrics.com/blog/
- Community: https://slack.victoriametrics.com/
- Grafana dashboards: https://grafana.com/grafana/dashboards/?search=victoriametrics
- Comparison: https://docs.victoriametrics.com/Articles.html#comparing-with-other-solutions
- Search: "victoriametrics vs prometheus", "victoriametrics benchmarks", "victoriametrics migration"

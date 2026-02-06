# Prometheus - Metrics and Monitoring

Open-source systems monitoring and alerting toolkit. Time-series database with powerful query language and visualization.

## Quick Start
```yaml
- preset: prometheus-server
```

## Features
- **Time-series database**: Store metrics with timestamps
- **PromQL**: Powerful query language for metrics analysis
- **Pull model**: Scrape metrics from targets automatically
- **Service discovery**: Auto-discover targets via Kubernetes, Consul, etc.
- **Alerting**: Rule-based alerts with Alertmanager integration
- **Exporters**: 100+ exporters for various systems

## Basic Usage
```bash
# Start Prometheus (after installation)
prometheus --config.file=/etc/prometheus/prometheus.yml

# Check targets
curl http://localhost:9090/api/v1/targets

# Query metrics
curl 'http://localhost:9090/api/v1/query?query=up'

# Health check
curl http://localhost:9090/-/healthy

# Reload config
curl -X POST http://localhost:9090/-/reload
```

## Advanced Configuration

### Basic installation
```yaml
- name: Install Prometheus
  preset: prometheus-server
  become: true

- name: Verify Prometheus is running
  assert:
    http:
      url: http://localhost:9090/-/healthy
      status: 200
```

### Custom configuration
```yaml
- name: Install Prometheus
  preset: prometheus-server
  become: true

- name: Configure Prometheus
  copy:
    dest: /etc/prometheus/prometheus.yml
    content: |
      global:
        scrape_interval: 15s
        evaluation_interval: 15s

      scrape_configs:
        - job_name: 'prometheus'
          static_configs:
            - targets: ['localhost:9090']

        - job_name: 'node_exporter'
          static_configs:
            - targets: ['localhost:9100']
  become: true

- name: Reload Prometheus
  shell: curl -X POST http://localhost:9090/-/reload
```

### Kubernetes service discovery
```yaml
- name: Configure Kubernetes SD
  template:
    dest: /etc/prometheus/prometheus.yml
    content: |
      global:
        scrape_interval: 30s

      scrape_configs:
        - job_name: 'kubernetes-pods'
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
              action: keep
              regex: true
            - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
              action: replace
              target_label: __metrics_path__
              regex: (.+)
            - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
              action: replace
              regex: ([^:]+)(?::\d+)?;(\d+)
              replacement: $1:$2
              target_label: __address__
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Prometheus |

## Platform Support
- ✅ Linux (systemd service, apt, dnf, yum)
- ✅ macOS (Homebrew, binary)
- ✅ Docker (official images)
- ✅ Kubernetes (Helm charts, operator)

## Configuration
- **Config file**: `/etc/prometheus/prometheus.yml`
- **Data directory**: `/var/lib/prometheus/` (Linux)
- **Web UI**: `http://localhost:9090`
- **API**: `http://localhost:9090/api/v1/`
- **Service**: systemd (Linux), launchd (macOS)

## Real-World Examples

### Monitor Node.js application
```yaml
# Install Prometheus and Node Exporter
- name: Install Prometheus
  preset: prometheus-server
  become: true

- name: Configure scrape targets
  copy:
    dest: /etc/prometheus/prometheus.yml
    content: |
      global:
        scrape_interval: 10s

      scrape_configs:
        - job_name: 'node_app'
          static_configs:
            - targets: ['localhost:3000']
          metrics_path: /metrics
  become: true
```

```javascript
// Node.js app with metrics
const express = require('express');
const client = require('prom-client');

const app = express();
const register = new client.Registry();
client.collectDefaultMetrics({ register });

const httpRequestsTotal = new client.Counter({
  name: 'http_requests_total',
  help: 'Total HTTP requests',
  labelNames: ['method', 'route', 'status_code'],
  registers: [register]
});

app.get('/metrics', async (req, res) => {
  res.set('Content-Type', register.contentType);
  res.send(await register.metrics());
});
```

### Multi-target monitoring
```yaml
- name: Configure multiple targets
  copy:
    dest: /etc/prometheus/prometheus.yml
    content: |
      global:
        scrape_interval: 15s

      scrape_configs:
        - job_name: 'api-servers'
          static_configs:
            - targets:
              - 'api1.example.com:8080'
              - 'api2.example.com:8080'
              - 'api3.example.com:8080'

        - job_name: 'databases'
          static_configs:
            - targets:
              - 'db1.example.com:9104'  # MySQL exporter
              - 'db2.example.com:9187'  # PostgreSQL exporter

        - job_name: 'node_exporters'
          static_configs:
            - targets:
              - 'server1:9100'
              - 'server2:9100'
  become: true
```

### Alerting rules
```yaml
- name: Configure alert rules
  copy:
    dest: /etc/prometheus/alerts.yml
    content: |
      groups:
        - name: example_alerts
          interval: 30s
          rules:
            - alert: HighMemoryUsage
              expr: node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes < 0.1
              for: 5m
              labels:
                severity: warning
              annotations:
                summary: "High memory usage on {{ $labels.instance }}"

            - alert: ServiceDown
              expr: up == 0
              for: 1m
              labels:
                severity: critical
              annotations:
                summary: "Service {{ $labels.job }} is down"
  become: true

- name: Update Prometheus config
  copy:
    dest: /etc/prometheus/prometheus.yml
    content: |
      global:
        scrape_interval: 15s

      rule_files:
        - /etc/prometheus/alerts.yml

      alerting:
        alertmanagers:
          - static_configs:
            - targets: ['localhost:9093']

      scrape_configs:
        - job_name: 'prometheus'
          static_configs:
            - targets: ['localhost:9090']
  become: true
```

## Configuration File

### prometheus.yml
```yaml
# Global configuration
global:
  scrape_interval: 15s          # How often to scrape targets
  evaluation_interval: 15s       # How often to evaluate rules
  external_labels:
    cluster: 'production'
    region: 'us-east-1'

# Alertmanager configuration
alerting:
  alertmanagers:
    - static_configs:
        - targets: ['localhost:9093']

# Load rules once and periodically evaluate them
rule_files:
  - '/etc/prometheus/rules/*.yml'

# Scrape configuration
scrape_configs:
  # Prometheus itself
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  # Application servers
  - job_name: 'applications'
    static_configs:
      - targets:
        - 'app1.example.com:8080'
        - 'app2.example.com:8080'
    metrics_path: /metrics
    scrape_interval: 10s

  # File-based service discovery
  - job_name: 'file_sd'
    file_sd_configs:
      - files:
        - '/etc/prometheus/targets/*.json'
        refresh_interval: 5m
```

## PromQL Examples

### Basic queries
```promql
# Check if services are up
up

# CPU usage
rate(process_cpu_seconds_total[5m])

# Memory usage
process_resident_memory_bytes

# HTTP request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

### Advanced queries
```promql
# Top 10 endpoints by request count
topk(10, sum by (endpoint) (rate(http_requests_total[5m])))

# Service availability over 30 days
avg_over_time(up{job="api"}[30d]) * 100

# Memory usage percentage
(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes * 100

# Disk space remaining
(node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100
```

## Common Exporters

### Node Exporter (system metrics)
```bash
# Install
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
tar xvf node_exporter-1.7.0.linux-amd64.tar.gz
sudo cp node_exporter-1.7.0.linux-amd64/node_exporter /usr/local/bin/

# Run
node_exporter
```

### Blackbox Exporter (probing)
```yaml
# /etc/prometheus/blackbox.yml
modules:
  http_2xx:
    prober: http
    timeout: 5s
    http:
      preferred_ip_protocol: ip4
```

### MySQL Exporter
```bash
mysqld_exporter --config.my-cnf=/etc/mysql/.my.cnf
```

### PostgreSQL Exporter
```bash
DATA_SOURCE_NAME="postgresql://user:pass@localhost:5432/database?sslmode=disable" postgres_exporter
```

## Service Discovery

### File-based
```json
// /etc/prometheus/targets/web-servers.json
[
  {
    "targets": ["web1:8080", "web2:8080"],
    "labels": {
      "env": "production",
      "job": "web"
    }
  }
]
```

### Consul
```yaml
scrape_configs:
  - job_name: 'consul'
    consul_sd_configs:
      - server: 'localhost:8500'
        services: ['web', 'api']
```

### Kubernetes
```yaml
scrape_configs:
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
```

## API Usage
```bash
# Query current value
curl 'http://localhost:9090/api/v1/query?query=up'

# Query range
curl 'http://localhost:9090/api/v1/query_range?query=up&start=1609459200&end=1609545600&step=60'

# Get series
curl 'http://localhost:9090/api/v1/series?match[]=up'

# Get targets
curl 'http://localhost:9090/api/v1/targets'

# Get rules
curl 'http://localhost:9090/api/v1/rules'

# Get alerts
curl 'http://localhost:9090/api/v1/alerts'
```

## Storage

### Retention
```bash
# Keep 30 days of data
prometheus --storage.tsdb.retention.time=30d

# Keep max 100GB
prometheus --storage.tsdb.retention.size=100GB
```

### Remote storage
```yaml
# Write to remote storage
remote_write:
  - url: http://remote-storage:8080/api/v1/write

# Read from remote storage
remote_read:
  - url: http://remote-storage:8080/api/v1/read
```

## Agent Use
- Monitor application health and performance
- Track infrastructure metrics
- Alert on anomalies and failures
- Capacity planning with historical data
- SLA monitoring and reporting
- Auto-scaling decisions based on metrics
- Cost optimization via resource monitoring

## Troubleshooting

### High memory usage
```bash
# Reduce scrape interval
scrape_interval: 30s

# Decrease retention
--storage.tsdb.retention.time=15d
```

### Targets not scraped
```bash
# Check target status
curl http://localhost:9090/api/v1/targets

# Verify network connectivity
telnet target-host 9100

# Check logs
journalctl -u prometheus -f
```

### Query timeout
```bash
# Increase timeout
--query.timeout=2m

# Optimize queries
# Use rate() instead of increase()
# Add time range limits
```

### Config errors
```bash
# Validate config
promtool check config /etc/prometheus/prometheus.yml

# Check rules
promtool check rules /etc/prometheus/alerts.yml
```

## Best Practices
- **Instrument code**: Add metrics to applications
- **Use labels wisely**: High cardinality = memory issues
- **Set retention**: Balance storage vs history needs
- **Alert on symptoms**: Not on causes
- **Use recording rules**: Pre-compute expensive queries
- **Monitor Prometheus**: Track its own metrics
- **Backup TSDB**: Regular snapshots of data
- **Use exporters**: Don't reinvent the wheel

## Comparison

| Tool | Type | Storage | Query Language | Cost |
|------|------|---------|----------------|------|
| Prometheus | Pull | Local TSDB | PromQL | Free |
| Datadog | Push | SaaS | Custom | $$$$ |
| New Relic | Push | SaaS | NRQL | $$$$ |
| InfluxDB | Push | Local/Cloud | InfluxQL/Flux | Free/Paid |
| Grafana Cloud | Push | SaaS | PromQL | $$$ |

## Uninstall
```yaml
- preset: prometheus-server
  with:
    state: absent
```

**Note**: This removes Prometheus but keeps data at `/var/lib/prometheus/`.

## Resources
- Official docs: https://prometheus.io/docs/
- GitHub: https://github.com/prometheus/prometheus
- Exporters: https://prometheus.io/docs/instrumenting/exporters/
- PromQL: https://prometheus.io/docs/prometheus/latest/querying/basics/
- Search: "prometheus tutorial", "promql examples", "prometheus alerting"

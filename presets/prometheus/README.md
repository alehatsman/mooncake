# prometheus - Monitoring and Alerting System

Open-source systems monitoring and alerting toolkit with powerful metrics collection and querying.

## Quick Start
```yaml
- preset: prometheus
```

## Features
- **Dimensional data**: Time series identified by metric name and key-value pairs
- **Powerful queries**: PromQL for slicing and dicing data
- **Pull model**: Scrapes metrics from instrumented services
- **Service discovery**: Dynamic target discovery for cloud environments
- **Alerting**: Flexible alert rules with Alertmanager integration
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Access web UI
open http://localhost:9090

# Check status
curl http://localhost:9090/-/ready

# Query metrics API
curl 'http://localhost:9090/api/v1/query?query=up'

# Check configuration
curl http://localhost:9090/api/v1/status/config

# View targets
curl http://localhost:9090/api/v1/targets
```

## Advanced Configuration
```yaml
- preset: prometheus
  with:
    state: present
    start_service: true        # Start as system service
    port: "9090"              # Web UI port
    data_dir: "/var/lib/prometheus"
    retention: "15d"          # Data retention period
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove prometheus |
| start_service | bool | true | Start Prometheus service |
| port | string | 9090 | Web UI and API port |
| data_dir | string | /var/lib/prometheus | Data storage directory |
| retention | string | 15d | Data retention period |

## Platform Support
- ✅ Linux (systemd service)
- ✅ macOS (launchd service via Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/etc/prometheus/prometheus.yml` (Linux), `/opt/homebrew/etc/prometheus.yml` (macOS)
- **Data directory**: `/var/lib/prometheus/` (default)
- **Web UI**: `http://localhost:9090`
- **Reload endpoint**: `POST http://localhost:9090/-/reload`

## Real-World Examples

### Basic Monitoring Setup
```yaml
- preset: prometheus
  with:
    start_service: true

- name: Create basic config
  copy:
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
    dest: /etc/prometheus/prometheus.yml
  become: true

- name: Reload configuration
  shell: curl -X POST http://localhost:9090/-/reload
```

### Application Monitoring
```yaml
# prometheus.yml
global:
  scrape_interval: 10s

scrape_configs:
  # Scrape own metrics
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  # Scrape application metrics
  - job_name: 'myapp'
    static_configs:
      - targets:
          - 'app1.example.com:8080'
          - 'app2.example.com:8080'
    metrics_path: '/metrics'

  # Kubernetes service discovery
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
```

### Alert Rules
```yaml
# /etc/prometheus/alerts.yml
groups:
  - name: example
    rules:
      - alert: InstanceDown
        expr: up == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Instance {{ $labels.instance }} down"

      - alert: HighCPU
        expr: rate(cpu_usage[5m]) > 0.8
        for: 10m
        labels:
          severity: warning
```

### CI/CD Monitoring
```yaml
- preset: prometheus

- name: Deploy application
  shell: kubectl apply -f app.yaml

- name: Wait for metrics
  shell: sleep 10

- name: Verify metrics endpoint
  assert:
    http:
      url: "http://app.example.com:8080/metrics"
      status: 200

- name: Query Prometheus for app
  shell: |
    curl -s 'http://localhost:9090/api/v1/query?query=up{job="myapp"}' | \
    jq -r '.data.result[0].value[1]'
  register: app_up

- name: Verify app is up
  assert:
    command:
      cmd: "[ {{ app_up.stdout }} = '1' ]"
      exit_code: 0
```

## PromQL Query Examples
```promql
# Current CPU usage
rate(node_cpu_seconds_total{mode="user"}[5m])

# Memory usage percentage
(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100

# HTTP request rate
rate(http_requests_total[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Disk usage
(node_filesystem_size_bytes - node_filesystem_free_bytes) / node_filesystem_size_bytes

# Alert for high error rate
rate(http_requests_total{status=~"5.."}[5m]) > 0.05
```

## Agent Use
- Monitor infrastructure and application metrics
- Set up alerting for service degradation
- Track SLIs and SLOs
- Capacity planning with historical data
- Debug performance issues with detailed metrics
- Implement service-level monitoring

## Service Management
```bash
# Linux (systemd)
sudo systemctl status prometheus
sudo systemctl restart prometheus
sudo systemctl stop prometheus

# macOS (Homebrew services)
brew services list
brew services restart prometheus
brew services stop prometheus

# Reload config without restart
curl -X POST http://localhost:9090/-/reload
```

## Troubleshooting

### Service won't start
Check logs:
```bash
# Linux
sudo journalctl -u prometheus -f

# macOS
tail -f /opt/homebrew/var/log/prometheus.log
```

### Configuration errors
Validate config:
```bash
promtool check config /etc/prometheus/prometheus.yml
```

### Targets not discovered
Check service discovery:
```bash
curl http://localhost:9090/api/v1/targets
```

### High memory usage
Reduce retention or increase resources:
```yaml
- preset: prometheus
  with:
    retention: "7d"  # Reduce from 15d
```

## Uninstall
```yaml
- preset: prometheus
  with:
    state: absent
```

**Note**: Data directory is preserved after uninstall for safety.

## Resources
- Official docs: https://prometheus.io/docs/
- GitHub: https://github.com/prometheus/prometheus
- PromQL guide: https://prometheus.io/docs/prometheus/latest/querying/basics/
- Search: "prometheus monitoring", "promql tutorial"

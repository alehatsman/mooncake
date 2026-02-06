# Grafana Agent - Telemetry Collector

Lightweight telemetry collector for metrics, logs, and traces. Vendor-neutral observability agent compatible with Prometheus, Loki, and Tempo.

## Quick Start
```yaml
- preset: grafana-agent
```

## Features
- **All signals**: Collect metrics, logs, and traces in one agent
- **Prometheus compatible**: Drop-in replacement for Prometheus agent
- **Low resource**: Uses less memory and CPU than full Prometheus
- **Vendor neutral**: Send data to Grafana Cloud, or any backend
- **Dynamic config**: Reload configuration without restart
- **Service discovery**: Kubernetes, Consul, EC2 auto-discovery

## Basic Usage
```bash
# Start agent
grafana-agent --config.file=config.yaml

# Validate config
grafana-agent --config.file=config.yaml --config.expand-env --dry-run

# Check version
grafana-agent --version

# View metrics
curl http://localhost:12345/metrics
```

## Advanced Configuration
```yaml
- preset: grafana-agent
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Grafana Agent |

## Platform Support
- ✅ Linux (apt, dnf, yum, binary download)
- ✅ macOS (Homebrew, binary download)
- ✅ Windows (binary download, service install)

## Configuration
- **Config file**: `/etc/grafana-agent.yaml` (Linux), `/usr/local/etc/grafana-agent/config.yaml` (macOS)
- **Metrics port**: 12345 (agent self-metrics)
- **Data directory**: `/var/lib/grafana-agent/`
- **Log level**: info (default)

## Real-World Examples

### Basic Metrics Collection
```yaml
# config.yaml
server:
  log_level: info

metrics:
  global:
    scrape_interval: 15s
    remote_write:
      - url: https://prometheus.example.com/api/v1/write
        basic_auth:
          username: user
          password: pass

  configs:
    - name: agent
      scrape_configs:
        - job_name: 'node'
          static_configs:
            - targets: ['localhost:9100']
```

### Kubernetes Monitoring
```yaml
metrics:
  configs:
    - name: kubernetes
      scrape_configs:
        # Scrape kubelet metrics
        - job_name: 'kubernetes-nodes'
          kubernetes_sd_configs:
            - role: node
          relabel_configs:
            - source_labels: [__address__]
              regex: '(.*):10250'
              replacement: '${1}:10255'
              target_label: __address__

        # Scrape pod metrics
        - job_name: 'kubernetes-pods'
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
              action: keep
              regex: true
```

### Logs Collection
```yaml
logs:
  configs:
    - name: default
      positions:
        filename: /var/lib/grafana-agent/positions.yaml
      clients:
        - url: https://loki.example.com/loki/api/v1/push
          basic_auth:
            username: user
            password: pass

      scrape_configs:
        - job_name: system
          static_configs:
            - targets: [localhost]
              labels:
                job: varlogs
                __path__: /var/log/*.log

        - job_name: containers
          docker_sd_configs:
            - host: unix:///var/run/docker.sock
          relabel_configs:
            - source_labels: [__meta_docker_container_name]
              target_label: container
```

### Traces Collection
```yaml
traces:
  configs:
    - name: default
      receivers:
        jaeger:
          protocols:
            thrift_http:
              endpoint: 0.0.0.0:14268
        otlp:
          protocols:
            grpc:
              endpoint: 0.0.0.0:4317
            http:
              endpoint: 0.0.0.0:4318

      remote_write:
        - endpoint: tempo.example.com:443
          insecure: false
          headers:
            authorization: Bearer ${TEMPO_TOKEN}
```

### Full Observability Stack
```yaml
# Complete config: metrics, logs, traces
server:
  log_level: info

metrics:
  global:
    scrape_interval: 30s
    remote_write:
      - url: ${PROMETHEUS_URL}
        basic_auth:
          username: ${PROM_USER}
          password: ${PROM_PASS}
  configs:
    - name: integrations
      scrape_configs:
        - job_name: 'agent'
          static_configs:
            - targets: ['localhost:12345']

logs:
  configs:
    - name: default
      clients:
        - url: ${LOKI_URL}
          basic_auth:
            username: ${LOKI_USER}
            password: ${LOKI_PASS}
      scrape_configs:
        - job_name: app-logs
          static_configs:
            - targets: [localhost]
              labels:
                __path__: /var/log/myapp/*.log

traces:
  configs:
    - name: default
      receivers:
        otlp:
          protocols:
            grpc:
      remote_write:
        - endpoint: ${TEMPO_URL}
          headers:
            authorization: Bearer ${TEMPO_TOKEN}
```

## Agent Use
- Collect metrics from infrastructure and applications
- Forward logs from multiple sources to Loki
- Receive and forward distributed traces
- Monitor Kubernetes clusters with service discovery
- Implement unified observability pipeline
- Replace heavyweight monitoring agents

## Troubleshooting

### Agent not starting
```bash
# Check config syntax
grafana-agent --config.file=config.yaml --dry-run

# View logs
journalctl -u grafana-agent -f  # Linux
tail -f /var/log/grafana-agent.log

# Check port conflicts
netstat -an | grep 12345

# Validate remote write endpoint
curl ${PROMETHEUS_URL}
```

### No metrics collected
```bash
# Check scrape targets
curl http://localhost:12345/agent/api/v1/metrics/targets

# View agent metrics
curl http://localhost:12345/metrics | grep agent_

# Verify target is accessible
curl http://target:9100/metrics

# Check service discovery
curl http://localhost:12345/agent/api/v1/metrics/sd_configs
```

### Logs not forwarding
```bash
# Check positions file
cat /var/lib/grafana-agent/positions.yaml

# Verify file permissions
ls -la /var/log/*.log

# Test Loki endpoint
curl ${LOKI_URL}/ready

# Check log scraping stats
curl http://localhost:12345/metrics | grep loki_
```

### High memory usage
```yaml
# Limit metrics retention
metrics:
  wal_directory: /tmp/agent/wal
  global:
    wal_truncate_frequency: 1m

# Reduce scrape frequency
scrape_interval: 60s

# Enable metric relabeling to drop unused metrics
relabel_configs:
  - source_labels: [__name__]
    regex: 'unwanted_metric_.*'
    action: drop
```

## Uninstall
```yaml
- preset: grafana-agent
  with:
    state: absent
```

## Resources
- Official docs: https://grafana.com/docs/agent/
- GitHub: https://github.com/grafana/agent
- Configuration: https://grafana.com/docs/agent/latest/configuration/
- Examples: https://github.com/grafana/agent/tree/main/example
- Search: "grafana agent", "grafana agent kubernetes", "grafana observability"

# Prometheus Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Access web UI
open http://localhost:9090  # macOS
xdg-open http://localhost:9090  # Linux

# Check status
curl http://localhost:9090/-/ready

# Check health
curl http://localhost:9090/-/healthy
```

## Configuration

- **Config file:** `/etc/prometheus/prometheus.yml`
- **Data directory:** `/var/lib/prometheus` (default)
- **Web UI port:** 9090 (default)

## Common Operations

```bash
# Restart Prometheus
sudo systemctl restart prometheus  # Linux
brew services restart prometheus  # macOS

# Reload config (without restart)
curl -X POST http://localhost:9090/-/reload

# Check targets
curl http://localhost:9090/api/v1/targets

# Query metrics
curl 'http://localhost:9090/api/v1/query?query=up'
```

## Example prometheus.yml

```yaml
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
```

## Adding Exporters

```bash
# Install Node Exporter for system metrics
- preset: node_exporter  # if available
```

## Query Examples

```promql
# CPU usage
rate(node_cpu_seconds_total[5m])

# Memory usage
node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes

# HTTP requests per second
rate(http_requests_total[5m])
```

## Uninstall

```yaml
- preset: prometheus
  with:
    state: absent
```

**Note:** Data directory is preserved after uninstall.

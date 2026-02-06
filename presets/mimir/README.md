# Mimir - Prometheus-Compatible Metrics Backend

Mimir is a horizontally scalable, highly available, multi-tenant time-series database that provides long-term storage for Prometheus metrics. It replaces Cortex with improved performance, reliability, and operational simplicity while maintaining full Prometheus compatibility.

## Quick Start

```yaml
- preset: mimir
```

## Features

- **Prometheus compatible**: Drop-in replacement with existing Prometheus scrape configs
- **Horizontally scalable**: Distributed architecture handles billions of metrics
- **Multi-tenant**: Isolate metrics across different teams/applications with single deployment
- **Long-term storage**: Efficiently store metrics for months or years
- **High availability**: Replication and redundancy built-in
- **Query acceleration**: Built-in caching and query optimization for fast retrieval
- **Production hardened**: Powers Grafana Cloud and enterprise deployments

## Basic Usage

```bash
# Check Mimir version
mimir --version

# View help and configuration options
mimir --help

# Common Prometheus remote write configuration
# In prometheus.yml:
remote_write:
  - url: http://mimir:9009/api/prom/push
    queue_config:
      max_samples_per_send: 1000
```

## Advanced Configuration

```yaml
- preset: mimir
```

**Note**: Mimir configuration typically happens through YAML config files or environment variables during deployment. See Configuration section for file locations.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Mimir |

## Platform Support

- ✅ Linux (all distributions with binary or package manager)
- ✅ macOS (Homebrew)
- ✅ Kubernetes (Helm, manifests)
- ✅ Docker (container deployments)
- ⚠️ Windows (Docker primarily)

## Configuration

- **Config file**: `/etc/mimir/mimir.yaml`
- **Default port**: 9009 (gRPC)
- **HTTP port**: 8080
- **Data directory**: `/var/lib/mimir/`
- **Logs**: `/var/log/mimir/`

## Real-World Examples

### Prometheus Remote Storage Setup

```yaml
# prometheus.yml configuration for Mimir backend
global:
  scrape_interval: 15s
  evaluation_interval: 15s

remote_write:
  - url: http://mimir:9009/api/prom/push
    write_relabel_configs:
      - source_labels: [__name__]
        regex: 'go_.*'
        action: drop

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
  - job_name: 'node'
    static_configs:
      - targets: ['localhost:9100']
```

### Kubernetes Deployment with Helm

```bash
# Add Grafana Helm repository
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

# Install Mimir in Kubernetes
helm install mimir grafana/mimir \
  --namespace monitoring \
  --values mimir-values.yaml
```

## Agent Use

- Aggregate metrics from multiple Prometheus instances for centralized monitoring
- Implement long-term metric retention (compliance, historical analysis)
- Enable multi-team metric segregation with tenant isolation
- Query historical metrics for trend analysis and capacity planning
- Build cost-effective metrics backend for large-scale deployments
- Integrate with alerting systems for metric-based incident detection

## Troubleshooting

### High disk usage

Monitor and tune compression settings in mimir.yaml:

```bash
# Check block sizes
du -sh /var/lib/mimir/tsdb/*

# Verify compaction settings
grep -A 5 compaction /etc/mimir/mimir.yaml
```

### Remote write failures from Prometheus

Check network connectivity and Mimir logs:

```bash
# Verify Mimir listening
netstat -tuln | grep 9009

# Check Prometheus remote write queue
curl http://localhost:9090/api/v1/status/runtimeinfo | jq .
```

### Out of memory (OOM) errors

Adjust cache settings and ingestion limits:

```bash
# Edit mimir.yaml to reduce:
# - query_cache_config.memcached_client.max_idle_conns
# - query_engine_cache_size
# - ingestion_rate_mb
```

## Uninstall

```yaml
- preset: mimir
  with:
    state: absent
```

## Resources

- Official docs: https://grafana.com/docs/mimir/latest/
- GitHub: https://github.com/grafana/mimir
- Helm Charts: https://github.com/grafana/helm-charts
- Search: "Mimir Prometheus remote storage", "Mimir multi-tenancy setup", "Mimir high availability"

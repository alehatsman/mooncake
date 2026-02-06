# Elasticsearch Preset

Install and configure Elasticsearch - a distributed search and analytics engine.

## Quick Start

```yaml
- preset: elasticsearch
  with:
    start_service: true
    heap_size: "2g"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `8.x` | Version (7.x, 8.x, or specific) |
| `start_service` | bool | `true` | Start service after install |
| `cluster_name` | string | `elasticsearch` | Cluster name |
| `node_name` | string | hostname | Node name |
| `http_port` | string | `9200` | HTTP API port |
| `transport_port` | string | `9300` | Transport port |
| `data_dir` | string | `/var/lib/elasticsearch` | Data directory |
| `heap_size` | string | `1g` | JVM heap size |

## Usage

### Basic Installation
```yaml
- preset: elasticsearch
```

### Production Setup
```yaml
- preset: elasticsearch
  with:
    heap_size: "4g"
    cluster_name: "production"
    data_dir: "/mnt/elasticsearch/data"
```

### Development Setup
```yaml
- preset: elasticsearch
  with:
    heap_size: "512m"
    http_port: "9200"
```

## Verify Installation

```bash
# Check cluster health
curl http://localhost:9200/_cluster/health?pretty

# Check node info
curl http://localhost:9200/_nodes?pretty

# List indices
curl http://localhost:9200/_cat/indices?v
```

## Common Operations

```bash
# Create index
curl -X PUT http://localhost:9200/myindex

# Index document
curl -X POST http://localhost:9200/myindex/_doc \
  -H 'Content-Type: application/json' \
  -d '{"field": "value"}'

# Search
curl http://localhost:9200/myindex/_search?q=field:value

# Delete index
curl -X DELETE http://localhost:9200/myindex

# Restart service
sudo systemctl restart elasticsearch  # Linux
brew services restart elasticsearch   # macOS
```

## Configuration Files

- **Linux**: `/etc/elasticsearch/elasticsearch.yml`
- **macOS**: `/opt/homebrew/etc/elasticsearch/elasticsearch.yml`
- **Data**: `/var/lib/elasticsearch/` (default)
- **Logs**: `/var/log/elasticsearch/`

## Heap Size

Set to 50% of available RAM, but not more than 31GB:
- 4GB RAM → heap_size: "2g"
- 8GB RAM → heap_size: "4g"
- 64GB RAM → heap_size: "31g"

## Uninstall

```yaml
- preset: elasticsearch
  with:
    state: absent
```

**Note:** Data directory preserved after uninstall.

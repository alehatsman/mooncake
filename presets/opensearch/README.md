# OpenSearch - Search and Analytics Engine

Open-source distributed search and analytics engine. Fork of Elasticsearch 7.10, designed for log analytics, full-text search, and real-time application monitoring.

## Quick Start
```yaml
- preset: opensearch
```

## Features
- **Full-text search**: Advanced search capabilities with relevance scoring
- **Log analytics**: Real-time log aggregation and analysis
- **Distributed**: Horizontal scaling across multiple nodes
- **RESTful API**: HTTP interface for all operations
- **Query DSL**: Powerful JSON-based query language
- **Dashboards**: Built-in visualization with OpenSearch Dashboards
- **Security**: Authentication, authorization, and encryption
- **Cross-platform**: Linux, macOS, Docker

## Basic Usage
```bash
# Check cluster health
curl -X GET "localhost:9200/_cluster/health?pretty"

# List indices
curl -X GET "localhost:9200/_cat/indices?v"

# Create index
curl -X PUT "localhost:9200/myindex"

# Index document
curl -X POST "localhost:9200/myindex/_doc" \
  -H 'Content-Type: application/json' \
  -d '{"title": "Example", "content": "Hello world"}'

# Search documents
curl -X GET "localhost:9200/myindex/_search?q=hello"

# Delete index
curl -X DELETE "localhost:9200/myindex"
```

## Advanced Configuration
```yaml
# Install OpenSearch (default)
- preset: opensearch

# Uninstall OpenSearch
- preset: opensearch
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, yum, dnf, tar.gz)
- ✅ macOS (Homebrew, tar.gz)
- ✅ Docker (official images available)

## Configuration
- **Config file**: `/etc/opensearch/opensearch.yml` (Linux)
- **Data directory**: `/var/lib/opensearch/`
- **Log directory**: `/var/log/opensearch/`
- **Default port**: 9200 (HTTP), 9300 (transport)
- **JVM options**: `/etc/opensearch/jvm.options`

## Index Management
```bash
# Create index with settings
curl -X PUT "localhost:9200/myindex" \
  -H 'Content-Type: application/json' \
  -d '{
    "settings": {
      "number_of_shards": 3,
      "number_of_replicas": 2
    }
  }'

# Create index with mapping
curl -X PUT "localhost:9200/products" \
  -H 'Content-Type: application/json' \
  -d '{
    "mappings": {
      "properties": {
        "name": {"type": "text"},
        "price": {"type": "float"},
        "created": {"type": "date"}
      }
    }
  }'

# Update index settings
curl -X PUT "localhost:9200/myindex/_settings" \
  -H 'Content-Type: application/json' \
  -d '{"number_of_replicas": 1}'

# Get index info
curl -X GET "localhost:9200/myindex"

# Reindex data
curl -X POST "localhost:9200/_reindex" \
  -H 'Content-Type: application/json' \
  -d '{
    "source": {"index": "old_index"},
    "dest": {"index": "new_index"}
  }'
```

## Document Operations
```bash
# Index document with ID
curl -X PUT "localhost:9200/products/_doc/1" \
  -H 'Content-Type: application/json' \
  -d '{"name": "Laptop", "price": 999.99}'

# Get document by ID
curl -X GET "localhost:9200/products/_doc/1"

# Update document
curl -X POST "localhost:9200/products/_update/1" \
  -H 'Content-Type: application/json' \
  -d '{"doc": {"price": 899.99}}'

# Delete document
curl -X DELETE "localhost:9200/products/_doc/1"

# Bulk operations
curl -X POST "localhost:9200/_bulk" \
  -H 'Content-Type: application/x-ndjson' \
  --data-binary @bulk.json
```

## Search Queries
```bash
# Simple query string search
curl -X GET "localhost:9200/products/_search?q=laptop"

# Match query
curl -X GET "localhost:9200/products/_search" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "match": {"name": "laptop"}
    }
  }'

# Bool query with filters
curl -X GET "localhost:9200/products/_search" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "bool": {
        "must": [{"match": {"name": "laptop"}}],
        "filter": [{"range": {"price": {"gte": 500, "lte": 1500}}}]
      }
    }
  }'

# Aggregations
curl -X GET "localhost:9200/products/_search" \
  -H 'Content-Type: application/json' \
  -d '{
    "size": 0,
    "aggs": {
      "avg_price": {"avg": {"field": "price"}}
    }
  }'

# Search with highlighting
curl -X GET "localhost:9200/products/_search" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {"match": {"name": "laptop"}},
    "highlight": {"fields": {"name": {}}}
  }'
```

## Cluster Management
```bash
# Cluster health
curl -X GET "localhost:9200/_cluster/health?pretty"

# Cluster stats
curl -X GET "localhost:9200/_cluster/stats?pretty"

# Node info
curl -X GET "localhost:9200/_nodes?pretty"

# Node stats
curl -X GET "localhost:9200/_nodes/stats?pretty"

# Cluster settings
curl -X GET "localhost:9200/_cluster/settings?pretty"

# Update cluster settings
curl -X PUT "localhost:9200/_cluster/settings" \
  -H 'Content-Type: application/json' \
  -d '{
    "persistent": {
      "cluster.routing.allocation.enable": "all"
    }
  }'
```

## Index Templates
```bash
# Create index template
curl -X PUT "localhost:9200/_index_template/logs_template" \
  -H 'Content-Type: application/json' \
  -d '{
    "index_patterns": ["logs-*"],
    "template": {
      "settings": {
        "number_of_shards": 1,
        "number_of_replicas": 1
      },
      "mappings": {
        "properties": {
          "timestamp": {"type": "date"},
          "message": {"type": "text"},
          "level": {"type": "keyword"}
        }
      }
    }
  }'

# List templates
curl -X GET "localhost:9200/_cat/templates?v"

# Delete template
curl -X DELETE "localhost:9200/_index_template/logs_template"
```

## Real-World Examples

### Log Aggregation Pipeline
```yaml
- name: Install OpenSearch
  preset: opensearch
  become: true

- name: Create logs index template
  shell: |
    curl -X PUT "localhost:9200/_index_template/logs" \
      -H 'Content-Type: application/json' \
      -d '{
        "index_patterns": ["logs-*"],
        "template": {
          "settings": {"number_of_shards": 3},
          "mappings": {
            "properties": {
              "timestamp": {"type": "date"},
              "level": {"type": "keyword"},
              "message": {"type": "text"}
            }
          }
        }
      }'
```

### Application Search Backend
```bash
# Create products index
curl -X PUT "localhost:9200/products" \
  -H 'Content-Type: application/json' \
  -d '{
    "settings": {
      "analysis": {
        "analyzer": {
          "autocomplete": {
            "type": "custom",
            "tokenizer": "standard",
            "filter": ["lowercase", "autocomplete_filter"]
          }
        },
        "filter": {
          "autocomplete_filter": {
            "type": "edge_ngram",
            "min_gram": 2,
            "max_gram": 20
          }
        }
      }
    },
    "mappings": {
      "properties": {
        "name": {
          "type": "text",
          "analyzer": "autocomplete",
          "search_analyzer": "standard"
        }
      }
    }
  }'
```

### Monitoring and Alerting
```bash
# Create monitoring index
curl -X PUT "localhost:9200/metrics-$(date +%Y.%m.%d)" \
  -H 'Content-Type: application/json' \
  -d '{
    "mappings": {
      "properties": {
        "timestamp": {"type": "date"},
        "cpu_usage": {"type": "float"},
        "memory_usage": {"type": "float"},
        "hostname": {"type": "keyword"}
      }
    }
  }'

# Query recent high CPU usage
curl -X GET "localhost:9200/metrics-*/_search" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "bool": {
        "must": [
          {"range": {"timestamp": {"gte": "now-5m"}}},
          {"range": {"cpu_usage": {"gte": 80}}}
        ]
      }
    }
  }'
```

## Agent Use
- Log aggregation and analysis
- Application search backends
- Real-time analytics dashboards
- System monitoring and alerting
- Document indexing and retrieval
- Time-series data storage
- Security event analysis (SIEM)

## Troubleshooting

### Service won't start
Check logs and increase heap size:
```bash
# Linux
journalctl -u opensearch -f
# Increase heap in /etc/opensearch/jvm.options
-Xms2g
-Xmx2g
```

### Index is read-only
Clear read-only flag:
```bash
curl -X PUT "localhost:9200/_all/_settings" \
  -H 'Content-Type: application/json' \
  -d '{"index.blocks.read_only_allow_delete": null}'
```

### Cluster status yellow
Add replicas or reduce replica count:
```bash
curl -X PUT "localhost:9200/_settings" \
  -H 'Content-Type: application/json' \
  -d '{"number_of_replicas": 0}'
```

### Out of memory
Increase JVM heap size in `/etc/opensearch/jvm.options`:
```
-Xms4g
-Xmx4g
```

## Uninstall
```yaml
- preset: opensearch
  with:
    state: absent
```

## Resources
- Official docs: https://opensearch.org/docs/
- API reference: https://opensearch.org/docs/latest/api-reference/
- GitHub: https://github.com/opensearch-project/OpenSearch
- Community: https://forum.opensearch.org/
- Search: "opensearch tutorial", "opensearch query examples", "opensearch getting started"

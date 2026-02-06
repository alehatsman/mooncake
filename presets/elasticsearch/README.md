# Elasticsearch - Search and Analytics Engine

Distributed search and analytics engine built on Apache Lucene for full-text search, structured search, and analytics.

## Quick Start
```yaml
- preset: elasticsearch
  with:
    start_service: true
    heap_size: "2g"
```

## Features
- **Distributed**: Horizontal scaling across multiple nodes
- **Full-text search**: Powerful text analysis and relevance scoring
- **Real-time**: Near real-time indexing and search
- **RESTful API**: Simple HTTP JSON API
- **Schema-free**: Dynamic mapping and flexible data models
- **Analytics**: Aggregations for data analysis and visualization

## Basic Usage
```bash
# Check cluster health
curl http://localhost:9200/_cluster/health?pretty

# Check node info
curl http://localhost:9200/_nodes?pretty

# List indices
curl http://localhost:9200/_cat/indices?v

# Create index
curl -X PUT http://localhost:9200/myindex

# Index document
curl -X POST http://localhost:9200/myindex/_doc \
  -H 'Content-Type: application/json' \
  -d '{"title": "Example", "content": "Hello World"}'

# Search
curl http://localhost:9200/myindex/_search?q=Hello

# Delete index
curl -X DELETE http://localhost:9200/myindex
```

## Advanced Configuration
```yaml
# Basic installation
- preset: elasticsearch

# Production setup
- preset: elasticsearch
  with:
    heap_size: "4g"
    cluster_name: "production"
    data_dir: "/mnt/elasticsearch/data"
    http_port: "9200"
    start_service: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) |
| version | string | 8.x | Version (7.x, 8.x, or specific like 8.11.0) |
| start_service | bool | true | Start service after installation |
| cluster_name | string | elasticsearch | Cluster name |
| node_name | string | hostname | Node name |
| http_port | string | 9200 | HTTP API port |
| transport_port | string | 9300 | Cluster transport port |
| data_dir | string | /var/lib/elasticsearch | Data directory |
| heap_size | string | 1g | JVM heap size (e.g., 2g, 4g) |

## Platform Support
- ✅ Linux (apt, dnf, yum, Homebrew)
- ✅ macOS (Homebrew)
- ❌ Windows

## Configuration
- **Linux**: `/etc/elasticsearch/elasticsearch.yml`
- **macOS**: `/opt/homebrew/etc/elasticsearch/elasticsearch.yml`
- **Data**: `/var/lib/elasticsearch/` (default)
- **Logs**: `/var/log/elasticsearch/`

## Heap Size Guidelines
Set to 50% of available RAM, but not more than 31GB:
- 4GB RAM → `heap_size: "2g"`
- 8GB RAM → `heap_size: "4g"`
- 16GB RAM → `heap_size: "8g"`
- 64GB RAM → `heap_size: "31g"` (max recommended)

## Real-World Examples

### Application Logs Search
```bash
# Index log entry
curl -X POST http://localhost:9200/logs/_doc \
  -H 'Content-Type: application/json' \
  -d '{
    "timestamp": "2024-01-15T10:30:00",
    "level": "ERROR",
    "service": "api",
    "message": "Database connection failed"
  }'

# Search errors
curl 'http://localhost:9200/logs/_search?q=level:ERROR&pretty'

# Time range query
curl -X POST http://localhost:9200/logs/_search?pretty \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "range": {
        "timestamp": {
          "gte": "now-1h"
        }
      }
    }
  }'
```

### Product Catalog Search
```bash
# Index product
curl -X POST http://localhost:9200/products/_doc \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Laptop",
    "price": 999.99,
    "category": "Electronics",
    "tags": ["computer", "portable"]
  }'

# Full-text search
curl 'http://localhost:9200/products/_search?q=laptop&pretty'

# Filtered search
curl -X POST http://localhost:9200/products/_search?pretty \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "bool": {
        "must": [
          {"match": {"category": "Electronics"}}
        ],
        "filter": [
          {"range": {"price": {"lte": 1000}}}
        ]
      }
    }
  }'
```

### Aggregations for Analytics
```bash
# Count by category
curl -X POST http://localhost:9200/products/_search?pretty \
  -H 'Content-Type: application/json' \
  -d '{
    "size": 0,
    "aggs": {
      "categories": {
        "terms": {"field": "category.keyword"}
      }
    }
  }'

# Average price
curl -X POST http://localhost:9200/products/_search?pretty \
  -H 'Content-Type: application/json' \
  -d '{
    "size": 0,
    "aggs": {
      "avg_price": {
        "avg": {"field": "price"}
      }
    }
  }'
```

## Index Management

### Create Index with Mapping
```bash
curl -X PUT http://localhost:9200/articles \
  -H 'Content-Type: application/json' \
  -d '{
    "mappings": {
      "properties": {
        "title": {"type": "text"},
        "author": {"type": "keyword"},
        "published": {"type": "date"},
        "views": {"type": "integer"}
      }
    }
  }'
```

### Index Settings
```bash
curl -X PUT http://localhost:9200/myindex \
  -H 'Content-Type: application/json' \
  -d '{
    "settings": {
      "number_of_shards": 2,
      "number_of_replicas": 1,
      "refresh_interval": "30s"
    }
  }'
```

### Bulk Operations
```bash
curl -X POST http://localhost:9200/_bulk \
  -H 'Content-Type: application/json' \
  -d '
{"index":{"_index":"products","_id":"1"}}
{"name":"Product 1","price":10.99}
{"index":{"_index":"products","_id":"2"}}
{"name":"Product 2","price":20.99}
'
```

## Search Queries

### Match Query
```bash
curl -X POST http://localhost:9200/articles/_search?pretty \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "match": {
        "title": "elasticsearch tutorial"
      }
    }
  }'
```

### Boolean Query
```bash
curl -X POST http://localhost:9200/articles/_search?pretty \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "bool": {
        "must": [
          {"match": {"title": "elasticsearch"}}
        ],
        "filter": [
          {"range": {"published": {"gte": "2024-01-01"}}}
        ],
        "must_not": [
          {"term": {"status": "draft"}}
        ]
      }
    }
  }'
```

## Service Management
```bash
# Linux (systemd)
sudo systemctl start elasticsearch
sudo systemctl enable elasticsearch
sudo systemctl status elasticsearch
sudo journalctl -u elasticsearch -f

# macOS (Homebrew)
brew services start elasticsearch
brew services list
tail -f /opt/homebrew/var/log/elasticsearch.log
```

## Agent Use
- Index and search application logs
- Build full-text search for websites and applications
- Store and analyze metrics and time-series data
- Create product catalogs with advanced search
- Implement auto-complete and suggestions
- Analyze user behavior and analytics
- Monitor infrastructure and application performance
- Power business intelligence dashboards

## Troubleshooting

### Service won't start
```bash
# Check logs
sudo journalctl -u elasticsearch -n 50

# Check Java version
java -version  # Requires Java 11+

# Check heap size
grep Xms /etc/elasticsearch/jvm.options
```

### Cluster health red/yellow
```bash
# Check cluster health
curl http://localhost:9200/_cluster/health?pretty

# Check unassigned shards
curl http://localhost:9200/_cat/shards?v

# Explain allocation
curl http://localhost:9200/_cluster/allocation/explain?pretty
```

### Out of memory
```bash
# Increase heap size (50% of RAM, max 31GB)
# Edit /etc/elasticsearch/jvm.options
-Xms4g
-Xmx4g
```

### Port already in use
```bash
# Check what's using port 9200
sudo lsof -i :9200
sudo netstat -tulpn | grep 9200

# Change port in elasticsearch.yml
http.port: 9201
```

## Security (Elasticsearch 8.x+)
```bash
# Get enrollment token for new nodes
sudo /usr/share/elasticsearch/bin/elasticsearch-create-enrollment-token -s node

# Reset elastic user password
sudo /usr/share/elasticsearch/bin/elasticsearch-reset-password -u elastic

# Create API key
curl -X POST "http://localhost:9200/_security/api_key" \
  -u elastic:password \
  -H 'Content-Type: application/json' \
  -d '{"name": "my-api-key","expiration": "7d"}'
```

## Uninstall
```yaml
- preset: elasticsearch
  with:
    state: absent
```

**Note:** Data directory is preserved after uninstall.

## Resources
- Official docs: https://www.elastic.co/guide/en/elasticsearch/reference/current/index.html
- Getting started: https://www.elastic.co/guide/en/elasticsearch/reference/current/getting-started.html
- Query DSL: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html
- Search: "elasticsearch tutorial", "elasticsearch queries", "elastic stack"

# Zinc - Lightweight Search Engine

A lightweight alternative to Elasticsearch with full-text search, aggregations, and simple deployment requiring minimal resources.

## Quick Start
```yaml
- preset: zinc
```

## Features
- **Lightweight**: ~100MB memory footprint vs multi-GB for Elasticsearch
- **Easy to deploy**: Single binary, no external dependencies
- **Full-text search**: Index and search text with relevance scoring
- **Web UI**: Built-in interface for index management and searching
- **S3 storage**: Optional S3 backend for index storage
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Start Zinc server (default port 4080)
zinc

# Start with custom port
ZINC_FIRST_ADMIN_USER=admin \
ZINC_FIRST_ADMIN_PASSWORD=password \
zinc

# Start with custom data directory
ZINC_DATA_PATH=/var/lib/zinc zinc

# Check version
zinc --version
```

## Web UI

Access the web interface at `http://localhost:4080`

Default credentials (first run):
- Username: Set via `ZINC_FIRST_ADMIN_USER`
- Password: Set via `ZINC_FIRST_ADMIN_PASSWORD`

## API Usage

### Create Index
```bash
curl -X PUT "http://localhost:4080/api/index" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "name": "myindex",
    "storage_type": "disk"
  }'
```

### Index Document
```bash
curl -X POST "http://localhost:4080/api/myindex/_doc" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Sample Document",
    "content": "This is a test document",
    "timestamp": "2024-01-01T00:00:00Z"
  }'
```

### Search Documents
```bash
curl -X POST "http://localhost:4080/api/myindex/_search" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "search_type": "match",
    "query": {
      "term": "test"
    }
  }'
```

## Advanced Configuration
```yaml
# Basic installation
- preset: zinc
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (binary download, Homebrew)
- ✅ Windows (binary download)

## Configuration

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `ZINC_DATA_PATH` | `./data` | Data storage directory |
| `ZINC_SERVER_PORT` | `4080` | HTTP server port |
| `ZINC_FIRST_ADMIN_USER` | - | Initial admin username (required on first run) |
| `ZINC_FIRST_ADMIN_PASSWORD` | - | Initial admin password (required on first run) |
| `ZINC_PROMETHEUS_ENABLE` | `false` | Enable Prometheus metrics |
| `ZINC_TELEMETRY` | `true` | Enable anonymous telemetry |

### File Locations
- **Data directory**: `./data/` (default), configurable via `ZINC_DATA_PATH`
- **Binary**: `/usr/local/bin/zinc`

## Real-World Examples

### Application Log Search
```bash
# Index application logs
curl -X POST "http://localhost:4080/api/logs/_doc" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "level": "ERROR",
    "message": "Database connection timeout",
    "service": "api-server",
    "timestamp": "2024-01-15T10:30:00Z"
  }'

# Search for errors
curl -X POST "http://localhost:4080/api/logs/_search" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "term": "ERROR"
    },
    "from": 0,
    "size": 100
  }'
```

### Product Catalog Search
```bash
# Index products
curl -X POST "http://localhost:4080/api/products/_doc" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Wireless Keyboard",
    "description": "Ergonomic wireless keyboard with backlit keys",
    "price": 79.99,
    "category": "electronics"
  }'

# Search products
curl -X POST "http://localhost:4080/api/products/_search" \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "term": "wireless keyboard"
    }
  }'
```

### CI/CD Build Logs
```yaml
# In mooncake playbook
- name: Install Zinc
  preset: zinc

- name: Start Zinc server
  shell: |
    ZINC_FIRST_ADMIN_USER=ci \
    ZINC_FIRST_ADMIN_PASSWORD=secret \
    ZINC_DATA_PATH=/var/lib/zinc \
    zinc &
  become: true

- name: Index build logs
  shell: |
    curl -X POST "http://localhost:4080/api/builds/_doc" \
      -u ci:secret \
      -H "Content-Type: application/json" \
      -d '{"build_id": "123", "status": "success", "duration": 120}'
```

## Agent Use
- Index and search application logs in automated pipelines
- Build lightweight search functionality into applications
- Create searchable documentation indexes
- Monitor and analyze build/deployment logs
- Implement full-text search without Elasticsearch overhead

## Troubleshooting

### Zinc fails to start
Check if port 4080 is available:
```bash
lsof -i :4080
# If in use, set custom port
ZINC_SERVER_PORT=8080 zinc
```

### Authentication errors
Ensure admin credentials are set on first run:
```bash
ZINC_FIRST_ADMIN_USER=admin \
ZINC_FIRST_ADMIN_PASSWORD=mypassword \
zinc
```

### Data directory permission errors
Ensure Zinc has write access:
```bash
mkdir -p /var/lib/zinc
chown zinc:zinc /var/lib/zinc
ZINC_DATA_PATH=/var/lib/zinc zinc
```

### High memory usage
Zinc's memory usage scales with index size. For large datasets, consider:
- Using S3 backend storage
- Reducing index size
- Increasing system resources

## Uninstall
```yaml
- preset: zinc
  with:
    state: absent
```

**Manual cleanup:**
```bash
# Remove data directory
rm -rf ./data/  # or custom ZINC_DATA_PATH
```

## Resources
- GitHub: https://github.com/zincsearch/zincsearch
- Documentation: https://zincsearch-docs.zinc.dev
- API Reference: https://zincsearch-docs.zinc.dev/api-reference/
- Search: "zinc search engine", "zinc vs elasticsearch", "zinc tutorial"

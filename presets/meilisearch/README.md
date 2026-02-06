# Meilisearch - Fast, Open-Source Search Engine

Meilisearch is a fast, open-source search engine with typo tolerance and instant search results. It provides a RESTful API for indexing and searching documents with lightning-fast performance, making it ideal for building search experiences in applications.

## Quick Start

```yaml
- preset: meilisearch
```

## Features

- **Typo Tolerance**: Find results even with spelling mistakes
- **Instant Search**: Sub-millisecond response times for real-time search
- **RESTful API**: Simple HTTP API for indexing and querying
- **Multi-Lingual**: Support for 100+ languages with proper stemming
- **Full-Text Search**: Advanced search capabilities with filters and sorting
- **JSON Documents**: Store and search JSON documents natively
- **Cross-Platform**: Runs on Linux and macOS

## Basic Usage

```bash
# Check version
meilisearch --version

# Get help
meilisearch --help

# Start Meilisearch server (default port 7700)
meilisearch

# Start with custom host and port
meilisearch --http-addr 0.0.0.0:7700
```

## REST API Examples

### Create an index and add documents

```bash
# Create index
curl -X POST http://localhost:7700/indexes \
  -H "Content-Type: application/json" \
  -d '{"uid":"books","primaryKey":"id"}'

# Add documents
curl -X POST http://localhost:7700/indexes/books/documents \
  -H "Content-Type: application/json" \
  -d '[
    {"id":1,"title":"The Great Gatsby","author":"F. Scott Fitzgerald"},
    {"id":2,"title":"To Kill a Mockingbird","author":"Harper Lee"}
  ]'
```

### Search documents

```bash
# Basic search
curl http://localhost:7700/indexes/books/search \
  -H "Content-Type: application/json" \
  -d '{"q":"gatsby"}'

# Search with filters
curl http://localhost:7700/indexes/books/search \
  -H "Content-Type: application/json" \
  -d '{"q":"mockingbird","filter":"author = \"Harper Lee\""}'
```

## Advanced Configuration

```yaml
- preset: meilisearch
  with:
    state: present
    version: latest
    service: true
    port: 7700
    data_dir: /var/lib/meilisearch
    api_key: your-secret-key
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove meilisearch |
| version | string | latest | Version to install |
| service | bool | true | Configure as system service |
| port | number | 7700 | API server port |
| data_dir | string | /var/lib/meilisearch | Data storage directory |
| api_key | string | - | API key for authentication |
| http_addr | string | 127.0.0.1:7700 | HTTP server address |

## Configuration

- **Config directory**: `/etc/meilisearch/` (Linux), `~/Library/Application Support/meilisearch/` (macOS)
- **Data directory**: `/var/lib/meilisearch/` (Linux), `~/Library/Application Support/meilisearch/data/` (macOS)
- **Default port**: 7700
- **API documentation**: http://localhost:7700/docs (when running)

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, Homebrew)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Real-World Examples

### Deploy search engine for documentation

```yaml
- name: Setup documentation search
  preset: meilisearch
  with:
    state: present
    service: true
    port: 7700
    api_key: "{{ doc_search_key }}"
  become: true

- name: Index documentation
  shell: |
    curl -X POST http://localhost:7700/indexes/docs/documents \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer {{ doc_search_key }}" \
      -d @/var/lib/docs/index.json
```

### Production deployment with monitoring

```yaml
- name: Deploy Meilisearch production instance
  preset: meilisearch
  with:
    state: present
    service: true
    port: 7700
    data_dir: /opt/meilisearch/data
    api_key: "{{ lookup('env', 'MEILISEARCH_API_KEY') }}"
  become: true

- name: Verify service is running
  assert:
    http:
      url: "http://localhost:7700/health"
      status: 200
```

### Development setup with custom indexing

```yaml
- name: Setup Meilisearch for development
  preset: meilisearch
  with:
    state: present
    service: false
    port: 7700

- name: Create search indices
  shell: |
    # Products index
    curl -X POST http://localhost:7700/indexes \
      -H "Content-Type: application/json" \
      -d '{"uid":"products","primaryKey":"id"}'

    # Users index
    curl -X POST http://localhost:7700/indexes \
      -H "Content-Type: application/json" \
      -d '{"uid":"users","primaryKey":"id"}'
```

## Agent Use

AI agents can leverage Meilisearch for:

- **Full-text search**: Index documents and provide search capabilities to applications
- **Data integration**: Ingest data from APIs, databases, or files for searchability
- **Search API management**: Create and manage search indices programmatically
- **Performance monitoring**: Query response time metrics and search analytics
- **Content discovery**: Find and recommend relevant documents based on search queries
- **Automated indexing**: Continuously update indices with new data from external sources

## Troubleshooting

### Service won't start

Check if the service is properly configured and running:

```bash
# Linux - Check systemd status
sudo systemctl status meilisearch
sudo journalctl -u meilisearch -f

# macOS - Check launchd status
launchctl list | grep meilisearch
tail -f ~/Library/Logs/meilisearch.log
```

### Port already in use

If port 7700 is already in use, specify a different port:

```yaml
- preset: meilisearch
  with:
    port: 7701
    service: true
```

### API connection refused

Verify Meilisearch is running and listening on the correct address:

```bash
# Check if service is running
curl http://localhost:7700/health

# Check listening ports
sudo lsof -i :7700  # Linux/macOS
netstat -antp | grep 7700  # Linux
```

### Memory issues

If Meilisearch uses too much memory, check your data size and consider implementing pagination:

```bash
# Check available memory
free -h  # Linux
vm_stat  # macOS

# Monitor Meilisearch memory usage
ps aux | grep meilisearch
```

## Uninstall

```yaml
- preset: meilisearch
  with:
    state: absent
  become: true
```

## Resources

- Official documentation: https://docs.meilisearch.com/
- GitHub repository: https://github.com/meilisearch/meilisearch
- API reference: https://docs.meilisearch.com/reference/api/
- Search: "meilisearch tutorial", "meilisearch API examples", "meilisearch setup guide"

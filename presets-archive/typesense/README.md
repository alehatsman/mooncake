# Typesense - Fast Typo-Tolerant Search Engine

Lightning-fast search engine optimized for instant search experiences. Built-in typo tolerance, faceting, filtering, and geo search. Simple API, minimal configuration, and developer-friendly. Open-source alternative to Algolia and Elasticsearch for search.

## Quick Start
```yaml
- preset: typesense
```

Start server: `typesense-server --data-dir=/var/lib/typesense --api-key=xyz`

API endpoint: `http://localhost:8108`
Default port: 8108

## Features
- **Typo tolerance**: Automatically handles typos and misspellings
- **Instant search**: Sub-50ms search latency for real-time as-you-type search
- **Faceted search**: Dynamic filtering with facets and counts
- **Geo search**: Location-based search with radius filtering
- **Vector search**: Semantic search with embeddings (ML integration)
- **Simple API**: RESTful API with straightforward indexing and querying
- **Low memory**: 512MB RAM for 1M documents
- **High availability**: Multi-node clustering with automatic failover
- **Exact matching**: Prefix, infix, and fuzzy matching options
- **Result ranking**: Customizable ranking with boost fields

## Basic Usage
```bash
# Start Typesense server
typesense-server \
  --data-dir=/var/lib/typesense \
  --api-key=your-secret-key \
  --enable-cors

# Create collection
curl -X POST \
  http://localhost:8108/collections \
  -H "X-TYPESENSE-API-KEY: your-secret-key" \
  -d '{
    "name": "products",
    "fields": [
      {"name": "name", "type": "string"},
      {"name": "price", "type": "float"},
      {"name": "category", "type": "string", "facet": true}
    ]
  }'

# Index document
curl -X POST \
  http://localhost:8108/collections/products/documents \
  -H "X-TYPESENSE-API-KEY: your-secret-key" \
  -d '{
    "id": "1",
    "name": "Laptop",
    "price": 999.99,
    "category": "Electronics"
  }'

# Search
curl "http://localhost:8108/collections/products/documents/search?q=loptop&query_by=name" \
  -H "X-TYPESENSE-API-KEY: your-secret-key"
# Returns "Laptop" despite typo!
```

## Architecture

### Single-Node Setup
```
┌─────────────────────────────────────────┐
│         Typesense Server                │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │     HTTP API Server               │ │
│  │     (RESTful, port 8108)          │ │
│  └─────────────┬─────────────────────┘ │
│                │                        │
│  ┌─────────────▼─────────────────────┐ │
│  │     Search Engine Core            │ │
│  │  - Tokenization                   │ │
│  │  - Typo tolerance                 │ │
│  │  - Ranking algorithm              │ │
│  └─────────────┬─────────────────────┘ │
│                │                        │
│  ┌─────────────▼─────────────────────┐ │
│  │     Index Storage                 │ │
│  │  (In-memory + disk persistence)   │ │
│  └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### Clustered Setup
```
┌───────────────────────────────────────────┐
│            Load Balancer                  │
└───────┬───────────────────┬───────────────┘
        │                   │
┌───────▼────────┐  ┌───────▼────────┐
│  Typesense 1   │  │  Typesense 2   │
│  (Primary)     │  │  (Replica)     │
└───────┬────────┘  └───────┬────────┘
        │                   │
        └───────┬───────────┘
                │
        ┌───────▼────────┐
        │  Typesense 3   │
        │  (Replica)     │
        └────────────────┘
```

## Advanced Configuration

### Production server setup
```yaml
- name: Install Typesense
  preset: typesense

- name: Create typesense user
  command:
    cmd: useradd
    argv:
      - -r
      - -s
      - /bin/false
      - typesense
  become: true

- name: Create data directory
  file:
    path: /var/lib/typesense
    state: directory
    owner: typesense
    group: typesense
    mode: '0755'
  become: true

- name: Generate API key
  shell: openssl rand -hex 32
  register: typesense_api_key

- name: Start Typesense service
  service:
    name: typesense
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=Typesense Search Engine
        After=network.target

        [Service]
        Type=simple
        User=typesense
        Group=typesense
        ExecStart=/usr/local/bin/typesense-server \
          --data-dir=/var/lib/typesense \
          --api-key={{ typesense_api_key.stdout }} \
          --enable-cors=true
        Restart=always

        [Install]
        WantedBy=multi-user.target
  become: true
```

### High availability cluster
```yaml
# Node 1 (Primary)
- name: Start Typesense node 1
  shell: |
    typesense-server \
      --data-dir=/var/lib/typesense \
      --api-key={{ api_key }} \
      --peering-address=node1.example.com:8107:8108 \
      --nodes=node1.example.com:8107:8108,node2.example.com:8107:8108,node3.example.com:8107:8108
  async: true

# Node 2 (Replica)
- name: Start Typesense node 2
  shell: |
    typesense-server \
      --data-dir=/var/lib/typesense \
      --api-key={{ api_key }} \
      --peering-address=node2.example.com:8107:8108 \
      --nodes=node1.example.com:8107:8108,node2.example.com:8107:8108,node3.example.com:8107:8108
  async: true

# Node 3 (Replica)
- name: Start Typesense node 3
  shell: |
    typesense-server \
      --data-dir=/var/lib/typesense \
      --api-key={{ api_key }} \
      --peering-address=node3.example.com:8107:8108 \
      --nodes=node1.example.com:8107:8108,node2.example.com:8107:8108,node3.example.com:8107:8108
  async: true
```

### Docker deployment
```yaml
- name: Deploy Typesense with Docker
  shell: |
    docker run -d \
      --name typesense \
      -p 8108:8108 \
      -v /var/lib/typesense:/data \
      -e TYPESENSE_DATA_DIR=/data \
      -e TYPESENSE_API_KEY={{ api_key }} \
      -e TYPESENSE_ENABLE_CORS=true \
      typesense/typesense:latest
```

## Collections and Schema

### Create collection
```bash
curl -X POST \
  http://localhost:8108/collections \
  -H "X-TYPESENSE-API-KEY: xyz" \
  -d '{
    "name": "books",
    "fields": [
      {"name": "title", "type": "string"},
      {"name": "author", "type": "string", "facet": true},
      {"name": "publication_year", "type": "int32", "facet": true},
      {"name": "rating", "type": "float"},
      {"name": "genres", "type": "string[]", "facet": true},
      {"name": "in_stock", "type": "bool"}
    ],
    "default_sorting_field": "rating"
  }'
```

### Field types
- `string` - Text field
- `int32`, `int64` - Integer numbers
- `float` - Floating point numbers
- `bool` - Boolean
- `string[]` - Array of strings
- `geopoint` - Lat/lng coordinates `[lat, lng]`
- `object` - Nested object
- `object[]` - Array of objects
- `auto` - Automatic type detection

## Indexing Documents

### Index single document
```bash
curl -X POST \
  http://localhost:8108/collections/books/documents \
  -H "X-TYPESENSE-API-KEY: xyz" \
  -d '{
    "id": "1",
    "title": "The Great Gatsby",
    "author": "F. Scott Fitzgerald",
    "publication_year": 1925,
    "rating": 4.5,
    "genres": ["Fiction", "Classic"],
    "in_stock": true
  }'
```

### Bulk import
```yaml
- name: Import documents in bulk
  shell: |
    curl -X POST \
      http://localhost:8108/collections/books/documents/import \
      -H "X-TYPESENSE-API-KEY: xyz" \
      -d '
      {"id":"1","title":"Book 1","author":"Author A","rating":4.5}
      {"id":"2","title":"Book 2","author":"Author B","rating":4.2}
      {"id":"3","title":"Book 3","author":"Author C","rating":4.8}
      '
```

### Update document
```bash
curl -X PATCH \
  http://localhost:8108/collections/books/documents/1 \
  -H "X-TYPESENSE-API-KEY: xyz" \
  -d '{"rating": 4.7}'
```

### Delete document
```bash
curl -X DELETE \
  http://localhost:8108/collections/books/documents/1 \
  -H "X-TYPESENSE-API-KEY: xyz"
```

## Search Queries

### Basic search
```bash
# Simple query
curl "http://localhost:8108/collections/books/documents/search?q=gatsby&query_by=title" \
  -H "X-TYPESENSE-API-KEY: xyz"
```

### Typo tolerance
```bash
# Automatically handles typos (up to 2 characters by default)
curl "http://localhost:8108/collections/books/documents/search?q=gatsbee&query_by=title" \
  -H "X-TYPESENSE-API-KEY: xyz"
# Still finds "The Great Gatsby"
```

### Faceted search
```bash
# Search with facets
curl "http://localhost:8108/collections/books/documents/search?q=*&query_by=title&facet_by=author,genres&filter_by=publication_year:>1900" \
  -H "X-TYPESENSE-API-KEY: xyz"

# Returns:
# {
#   "facet_counts": [
#     {"field_name": "author", "counts": [{"value": "F. Scott Fitzgerald", "count": 5}]},
#     {"field_name": "genres", "counts": [{"value": "Fiction", "count": 12}]}
#   ],
#   "hits": [...]
# }
```

### Filtering
```bash
# Filter by field
curl "http://localhost:8108/collections/books/documents/search?q=*&query_by=title&filter_by=rating:>4.0 && in_stock:true" \
  -H "X-TYPESENSE-API-KEY: xyz"

# Multiple filters
curl "http://localhost:8108/collections/books/documents/search?q=*&query_by=title&filter_by=genres:Fiction && publication_year:[1920..1950]" \
  -H "X-TYPESENSE-API-KEY: xyz"
```

### Geo search
```bash
# Create collection with geo field
curl -X POST http://localhost:8108/collections \
  -d '{"name":"places","fields":[
    {"name":"name","type":"string"},
    {"name":"location","type":"geopoint"}
  ]}'

# Index with lat/lng
curl -X POST http://localhost:8108/collections/places/documents \
  -d '{"id":"1","name":"Coffee Shop","location":[40.7128,-74.0060]}'

# Search within radius (5km)
curl "http://localhost:8108/collections/places/documents/search?q=*&query_by=name&filter_by=location:(40.7128,-74.0060,5 km)" \
  -H "X-TYPESENSE-API-KEY: xyz"
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Typesense |

## Platform Support
- ✅ Linux (DEB, RPM, binary)
- ✅ macOS (Homebrew, binary)
- ✅ Docker (official images)
- ❌ Windows (use WSL2 or Docker)

## Configuration Options

### Command-line flags
```bash
# Required
--data-dir=/var/lib/typesense          # Data storage directory
--api-key=secret                       # API key for authentication

# Optional
--api-port=8108                        # HTTP API port
--peering-port=8107                    # Cluster peering port
--enable-cors=true                     # Enable CORS
--log-dir=/var/log/typesense          # Log directory
--reset-peers-on-error=true           # Reset cluster on error
```

## API Keys

### Admin key (full access)
```bash
# Set via environment or flag
--api-key=admin-secret-key
```

### Scoped API keys
```bash
# Create search-only key
curl -X POST \
  http://localhost:8108/keys \
  -H "X-TYPESENSE-API-KEY: admin-key" \
  -d '{
    "description": "Search-only key",
    "actions": ["documents:search"],
    "collections": ["*"]
  }'

# Create collection-specific key
curl -X POST \
  http://localhost:8108/keys \
  -H "X-TYPESENSE-API-KEY: admin-key" \
  -d '{
    "description": "Books search key",
    "actions": ["documents:search"],
    "collections": ["books"]
  }'
```

## Client Libraries

### JavaScript/TypeScript
```yaml
- name: Install Typesense client
  shell: npm install typesense

# client.js
const Typesense = require('typesense');

const client = new Typesense.Client({
  nodes: [{
    host: 'localhost',
    port: '8108',
    protocol: 'http'
  }],
  apiKey: 'xyz',
  connectionTimeoutSeconds: 2
});

// Search
const searchResults = await client.collections('books')
  .documents()
  .search({
    q: 'gatsby',
    query_by: 'title',
    filter_by: 'rating:>4.0'
  });
```

### Python
```yaml
- name: Install Typesense client
  shell: pip install typesense

# client.py
import typesense

client = typesense.Client({
  'nodes': [{
    'host': 'localhost',
    'port': '8108',
    'protocol': 'http'
  }],
  'api_key': 'xyz',
  'connection_timeout_seconds': 2
})

# Search
search_results = client.collections['books'].documents.search({
  'q': 'gatsby',
  'query_by': 'title',
  'filter_by': 'rating:>4.0'
})
```

### Go
```yaml
- name: Install Typesense client
  shell: go get github.com/typesense/typesense-go

// client.go
package main

import (
    "github.com/typesense/typesense-go/typesense"
    "github.com/typesense/typesense-go/typesense/api"
)

func main() {
    client := typesense.NewClient(
        typesense.WithServer("http://localhost:8108"),
        typesense.WithAPIKey("xyz"),
    )

    searchParams := &api.SearchCollectionParams{
        Q:       "gatsby",
        QueryBy: "title",
    }

    results, _ := client.Collection("books").Documents().Search(searchParams)
}
```

## Use Cases

### E-commerce product search
```yaml
- name: Create products collection
  shell: |
    curl -X POST http://localhost:8108/collections \
      -H "X-TYPESENSE-API-KEY: xyz" \
      -d '{
        "name": "products",
        "fields": [
          {"name": "name", "type": "string"},
          {"name": "description", "type": "string"},
          {"name": "price", "type": "float", "facet": true},
          {"name": "category", "type": "string", "facet": true},
          {"name": "brand", "type": "string", "facet": true},
          {"name": "in_stock", "type": "bool"},
          {"name": "rating", "type": "float"}
        ],
        "default_sorting_field": "rating"
      }'

- name: Implement autocomplete
  shell: |
    # Prefix search for autocomplete
    curl "http://localhost:8108/collections/products/documents/search?q=lapt&query_by=name&prefix=true" \
      -H "X-TYPESENSE-API-KEY: xyz"
```

### Documentation search
```yaml
- name: Index documentation
  shell: |
    curl -X POST http://localhost:8108/collections \
      -d '{
        "name": "docs",
        "fields": [
          {"name": "title", "type": "string"},
          {"name": "content", "type": "string"},
          {"name": "category", "type": "string", "facet": true},
          {"name": "url", "type": "string"}
        ]
      }'

- name: Import docs
  shell: |
    curl -X POST http://localhost:8108/collections/docs/documents/import \
      -H "X-TYPESENSE-API-KEY: xyz" \
      --data-binary @docs.jsonl
```

### Semantic search with vectors
```yaml
- name: Create collection with vector field
  shell: |
    curl -X POST http://localhost:8108/collections \
      -d '{
        "name": "articles",
        "fields": [
          {"name": "title", "type": "string"},
          {"name": "content", "type": "string"},
          {"name": "embedding", "type": "float[]", "num_dim": 384}
        ]
      }'

- name: Vector search
  shell: |
    curl "http://localhost:8108/collections/articles/documents/search" \
      -d '{
        "q": "*",
        "vector_query": "embedding:([0.1, 0.2, ...], k:10)"
      }'
```

## CLI Commands

### Collection management
```bash
# List collections
curl http://localhost:8108/collections -H "X-TYPESENSE-API-KEY: xyz"

# Get collection schema
curl http://localhost:8108/collections/books -H "X-TYPESENSE-API-KEY: xyz"

# Delete collection
curl -X DELETE http://localhost:8108/collections/books -H "X-TYPESENSE-API-KEY: xyz"
```

### Server operations
```bash
# Health check
curl http://localhost:8108/health

# Server metrics
curl http://localhost:8108/metrics.json -H "X-TYPESENSE-API-KEY: xyz"

# Create snapshot
curl -X POST http://localhost:8108/operations/snapshot -H "X-TYPESENSE-API-KEY: xyz"
```

## Monitoring

### Metrics endpoint
```bash
curl http://localhost:8108/metrics.json -H "X-TYPESENSE-API-KEY: xyz"

# Returns:
# {
#   "system_cpu_active_percentage": 12.5,
#   "system_memory_used_bytes": 1073741824,
#   "typesense_memory_used_bytes": 536870912,
#   "system_disk_used_bytes": 10737418240
# }
```

### Prometheus integration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'typesense'
    static_configs:
      - targets: ['localhost:8108']
    metrics_path: /metrics.json
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Typesense
  preset: typesense
```

### Production deployment
```yaml
- name: Install Typesense
  preset: typesense

- name: Create typesense user
  command:
    cmd: useradd
    argv: ["-r", "-s", "/bin/false", "typesense"]
  become: true

- name: Create directories
  file:
    path: "{{ item }}"
    state: directory
    owner: typesense
    group: typesense
    mode: '0755'
  loop:
    - /var/lib/typesense
    - /var/log/typesense
  become: true

- name: Generate API key
  shell: openssl rand -hex 32
  register: api_key

- name: Start Typesense
  service:
    name: typesense
    state: started
    enabled: true
  become: true
```

### Kubernetes deployment
```yaml
- name: Deploy Typesense StatefulSet
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: apps/v1
    kind: StatefulSet
    metadata:
      name: typesense
    spec:
      serviceName: typesense
      replicas: 3
      selector:
        matchLabels:
          app: typesense
      template:
        metadata:
          labels:
            app: typesense
        spec:
          containers:
          - name: typesense
            image: typesense/typesense:latest
            ports:
            - containerPort: 8108
            - containerPort: 8107
            env:
            - name: TYPESENSE_API_KEY
              valueFrom:
                secretKeyRef:
                  name: typesense-secret
                  key: api-key
            - name: TYPESENSE_DATA_DIR
              value: /data
            volumeMounts:
            - name: data
              mountPath: /data
      volumeClaimTemplates:
      - metadata:
          name: data
        spec:
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi
    EOF
```

## Agent Use
- **Product search**: Fast e-commerce search with typo tolerance
- **Documentation search**: Instant developer documentation search
- **Autocomplete**: Real-time search suggestions
- **Geo search**: Location-based business search
- **Site search**: General website search
- **Semantic search**: ML-powered contextual search with embeddings
- **Log search**: Fast log filtering and analysis

## Troubleshooting

### High memory usage
```bash
# Check memory stats
curl http://localhost:8108/metrics.json -H "X-TYPESENSE-API-KEY: xyz"

# Reduce memory with smaller cache
typesense-server --data-dir=/var/lib/typesense --api-key=xyz --memory-cache-size-mb=512
```

### Slow queries
```bash
# Check query performance
curl "http://localhost:8108/collections/books/documents/search?q=gatsby&query_by=title&x-typesense-query-profile=true" \
  -H "X-TYPESENSE-API-KEY: xyz"

# Optimize: use specific fields in query_by
# BAD: query_by=title,description,content
# GOOD: query_by=title
```

### Cluster sync issues
```bash
# Check cluster status
curl http://localhost:8108/debug -H "X-TYPESENSE-API-KEY: xyz"

# Restart with peer reset
typesense-server --data-dir=/var/lib/typesense --api-key=xyz --reset-peers-on-error=true
```

## Best Practices

1. **Use facets sparingly**: Only mark filterable fields as facets
2. **Optimize query_by**: Search fewer fields for better performance
3. **Set default_sorting_field**: Improves result relevance
4. **Use prefix search** for autocomplete (add `prefix=true`)
5. **Index numeric fields** as int32/float, not strings
6. **Enable clustering** for high availability (3+ nodes)
7. **Use scoped API keys** to restrict client access
8. **Batch imports**: Use bulk import for large datasets
9. **Monitor memory**: Set appropriate cache size for workload
10. **Regular backups**: Create snapshots of data directory

## Backup and Restore

### Create snapshot
```yaml
- name: Create Typesense snapshot
  shell: curl -X POST http://localhost:8108/operations/snapshot -H "X-TYPESENSE-API-KEY: xyz"
  register: snapshot

- name: Backup data
  shell: rsync -av /var/lib/typesense/ /backup/typesense/
```

### Restore from backup
```yaml
- name: Stop Typesense
  service:
    name: typesense
    state: stopped

- name: Restore data
  shell: rsync -av /backup/typesense/ /var/lib/typesense/

- name: Start Typesense
  service:
    name: typesense
    state: started
```

## Uninstall
```yaml
- name: Stop Typesense
  service:
    name: typesense
    state: stopped

- name: Remove Typesense
  preset: typesense
  with:
    state: absent

- name: Remove data
  file:
    path: /var/lib/typesense
    state: absent
  become: true
```

**Note**: Uninstalling does not remove data directory. Delete `/var/lib/typesense` manually if needed.

## Resources
- Official: https://typesense.org/
- Documentation: https://typesense.org/docs/
- GitHub: https://github.com/typesense/typesense
- Community: https://typesense.org/community
- Guide: https://typesense.org/docs/guide/
- API Reference: https://typesense.org/docs/latest/api/
- Showcase: https://typesense.org/showcase/
- Search: "typesense vs elasticsearch", "typesense vs algolia", "typesense search engine"

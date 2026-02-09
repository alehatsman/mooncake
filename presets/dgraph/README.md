# Dgraph - Distributed Graph Database

Native GraphQL database with graph backend, designed for complex relationships and deep traversals. Horizontally scalable with sharding, replication, and ACID transactions. Built-in vector search for semantic queries. Open-source alternative to Neo4j.

## Quick Start
```yaml
- preset: dgraph
```

Start Dgraph: `dgraph alpha --lru_mb=2048`
GraphQL endpoint: `http://localhost:8080/graphql`
Admin endpoint: `http://localhost:8080/admin`

## Features
- **Native GraphQL**: First-class GraphQL support with schema management
- **Graph backend**: Optimized for deep traversals and relationships
- **Distributed**: Horizontal scaling with automatic sharding
- **ACID transactions**: Full transactional guarantees
- **Vector search**: Semantic search with embeddings (ML integration)
- **Real-time subscriptions**: Live GraphQL subscriptions
- **Multi-tenancy**: Namespace isolation for multiple tenants
- **Dgraph Query Language (DQL)**: Powerful graph query language
- **Flexible schema**: Schema-first or schemaless modes
- **High availability**: Automatic failover with Raft consensus

## Basic Usage
```bash
# Start Dgraph Zero (cluster coordinator)
dgraph zero

# Start Dgraph Alpha (data node)
dgraph alpha --zero localhost:5080

# Access Ratel UI (admin interface)
# Navigate to http://localhost:8000

# Query via GraphQL
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "{ queryPerson { name email } }"
  }'
```

## Architecture

### Components
```
┌──────────────────────────────────────────────────┐
│              Dgraph Cluster                      │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │          Dgraph Zero (Coordinator)         │ │
│  │  - Cluster membership                      │ │
│  │  - Shard assignment                        │ │
│  │  - Transaction timestamps                  │ │
│  └────────────────┬───────────────────────────┘ │
│                   │                              │
│  ┌────────────────▼───────────────────────────┐ │
│  │         Dgraph Alpha (Data Nodes)         │ │
│  │                                            │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐│ │
│  │  │ Alpha 1  │  │ Alpha 2  │  │ Alpha 3  ││ │
│  │  │          │  │          │  │          ││ │
│  │  │ Group 1  │  │ Group 2  │  │ Group 3  ││ │
│  │  │(Shard A) │  │(Shard B) │  │(Shard C) ││ │
│  │  └──────────┘  └──────────┘  └──────────┘│ │
│  └────────────────────────────────────────────┘ │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │         Client Applications                │ │
│  │  - GraphQL queries                         │ │
│  │  - DQL queries                             │ │
│  │  - Mutations                               │ │
│  └────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

### Key Concepts
- **Dgraph Zero**: Cluster coordinator managing membership and sharding
- **Dgraph Alpha**: Data nodes storing and serving graph data
- **Predicate**: Property/edge in the graph (like a field)
- **Node**: Vertex in the graph
- **Edge**: Relationship between nodes
- **Group**: Shard of data managed by an Alpha
- **Namespace**: Logical isolation for multi-tenancy

## Advanced Configuration

### Single-node setup
```yaml
- name: Install Dgraph
  preset: dgraph

- name: Start Dgraph Zero
  shell: dgraph zero
  async: true

- name: Wait for Zero
  shell: |
    for i in {1..30}; do
      curl -f http://localhost:6080/health && break
      sleep 1
    done

- name: Start Dgraph Alpha
  shell: |
    dgraph alpha \
      --zero localhost:5080 \
      --lru_mb=2048
  async: true

- name: Wait for Alpha
  shell: |
    for i in {1..30}; do
      curl -f http://localhost:8080/health && break
      sleep 1
    done
```

### Production cluster (3-node)
```yaml
# Node 1: Zero + Alpha
- name: Start Zero on node1
  shell: |
    dgraph zero \
      --idx=1 \
      --replicas=3 \
      --my=node1:5080 \
      --peer=node2:5080,node3:5080
  async: true

- name: Start Alpha on node1
  shell: |
    dgraph alpha \
      --zero=node1:5080,node2:5080,node3:5080 \
      --my=node1:7080 \
      --lru_mb=4096
  async: true

# Node 2: Zero + Alpha
- name: Start Zero on node2
  shell: |
    dgraph zero \
      --idx=2 \
      --replicas=3 \
      --my=node2:5080 \
      --peer=node1:5080,node3:5080
  async: true

- name: Start Alpha on node2
  shell: |
    dgraph alpha \
      --zero=node1:5080,node2:5080,node3:5080 \
      --my=node2:7080 \
      --lru_mb=4096
  async: true

# Node 3: Zero + Alpha
- name: Start Zero on node3
  shell: |
    dgraph zero \
      --idx=3 \
      --replicas=3 \
      --my=node3:5080 \
      --peer=node1:5080,node2:5080
  async: true

- name: Start Alpha on node3
  shell: |
    dgraph alpha \
      --zero=node1:5080,node2:5080,node3:5080 \
      --my=node3:7080 \
      --lru_mb=4096
  async: true
```

### Docker Compose
```yaml
- name: Deploy Dgraph with Docker Compose
  shell: |
    cat > docker-compose.yml <<EOF
    version: "3.8"
    services:
      zero:
        image: dgraph/dgraph:latest
        command: dgraph zero --my=zero:5080
        ports:
          - 5080:5080
          - 6080:6080

      alpha:
        image: dgraph/dgraph:latest
        command: dgraph alpha --zero=zero:5080 --my=alpha:7080
        ports:
          - 8080:8080
          - 9080:9080
        depends_on:
          - zero

      ratel:
        image: dgraph/ratel:latest
        ports:
          - 8000:8000
    EOF
    docker compose up -d
```

## GraphQL Schema and Queries

### Define schema
```graphql
# Upload schema
curl -X POST http://localhost:8080/admin/schema \
  -H "Content-Type: application/graphql" \
  -d '
type Person {
  id: ID!
  name: String! @search(by: [term, fulltext])
  email: String @search(by: [hash])
  age: Int @search
  friends: [Person] @hasInverse(field: friends)
  posts: [Post]
}

type Post {
  id: ID!
  title: String! @search(by: [term, fulltext])
  content: String @search(by: [fulltext])
  author: Person!
  tags: [String] @search(by: [exact])
  createdAt: DateTime
}
'
```

### Create data (mutations)
```graphql
mutation {
  addPerson(input: [
    {
      name: "Alice"
      email: "alice@example.com"
      age: 30
      posts: [
        {
          title: "My First Post"
          content: "Hello world!"
          tags: ["intro", "hello"]
          createdAt: "2024-01-01T00:00:00Z"
        }
      ]
    }
  ]) {
    person {
      id
      name
    }
  }
}
```

### Query data
```graphql
# Simple query
query {
  queryPerson {
    name
    email
    posts {
      title
    }
  }
}

# Query with filters
query {
  queryPerson(filter: { age: { gt: 25 } }) {
    name
    age
  }
}

# Deep traversal
query {
  queryPerson(filter: { name: { anyofterms: "Alice" } }) {
    name
    friends {
      name
      friends {
        name
      }
    }
  }
}

# Aggregation
query {
  aggregatePerson {
    count
    ageAvg: ageAvg
    ageMax: ageMax
  }
}
```

## DQL (Dgraph Query Language)

### Query with DQL
```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/dql" \
  -d '{
    people(func: type(Person)) {
      uid
      name
      email
      friends {
        name
      }
    }
  }'
```

### Advanced DQL features
```dql
# Facets (edge properties)
{
  people(func: type(Person)) {
    name
    friends @facets(since, closeness) {
      name
    }
  }
}

# Variables
{
  var(func: type(Person)) {
    name
    a as age
  }

  avgAge() {
    avg(val(a))
  }
}

# Shortest path
{
  path as shortest(from: 0x1, to: 0x2) {
    friends
  }

  path(func: uid(path)) {
    name
  }
}
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Dgraph |

## Platform Support
- ✅ Linux (all distributions) - native binary
- ✅ macOS (Homebrew, native binary)
- ✅ Docker (official images)
- ✅ Kubernetes (Helm charts)
- ❌ Windows (use WSL2 or Docker)

## Configuration

### Command-line flags
```bash
# Dgraph Zero
dgraph zero \
  --idx=1 \                      # Node index
  --replicas=3 \                 # Replication factor
  --my=localhost:5080 \          # This node's address
  --wal=/dgraph/zero-wal         # Write-ahead log directory

# Dgraph Alpha
dgraph alpha \
  --zero=localhost:5080 \        # Zero addresses
  --lru_mb=4096 \                # Cache size (MB)
  --badger.compression=zstd \    # Compression algorithm
  --badger.vlog=disk \           # Value log mode
  --postings=/dgraph/p \         # Posting lists directory
  --wal=/dgraph/wal              # Write-ahead log directory
```

### Security
```yaml
# Enable access control
- name: Start Alpha with ACL
  shell: |
    dgraph alpha \
      --zero=localhost:5080 \
      --acl "secret-file=/path/to/secret.txt"

# Create admin user
- name: Create admin user
  shell: |
    dgraph acl add -u admin -p adminpassword
```

## Vector Search

### Schema with vectors
```graphql
type Article {
  id: ID!
  title: String! @search(by: [term])
  content: String
  embedding: [Float] @embedding
}
```

### Store vectors
```graphql
mutation {
  addArticle(input: [
    {
      title: "Machine Learning Basics"
      content: "Introduction to ML..."
      embedding: [0.1, 0.2, 0.3, ..., 0.n]  # 384-dim vector
    }
  ]) {
    article {
      id
      title
    }
  }
}
```

### Vector similarity search
```graphql
query {
  querySimilarArticles(
    by: EMBEDDING,
    topK: 10,
    vector: [0.15, 0.25, 0.35, ..., 0.n]
  ) {
    title
    content
  }
}
```

## Indexing

### Search indexes
```graphql
# Term index (tokenization)
name: String @search(by: [term])

# Full-text search
content: String @search(by: [fulltext])

# Hash index (exact match)
email: String @search(by: [hash])

# Exact match (for arrays)
tags: [String] @search(by: [exact])

# Numeric indexes
age: Int @search
price: Float @search

# Geo index
location: GeoPoint @search
```

## Use Cases

### Social network
```yaml
- name: Deploy Dgraph
  preset: dgraph

- name: Create social network schema
  shell: |
    curl -X POST http://localhost:8080/admin/schema \
      -H "Content-Type: application/graphql" \
      -d '
    type User {
      id: ID!
      username: String! @search(by: [hash, term])
      name: String @search(by: [fulltext])
      bio: String
      followers: [User] @hasInverse(field: following)
      following: [User]
      posts: [Post]
    }

    type Post {
      id: ID!
      content: String @search(by: [fulltext])
      author: User!
      likes: [User]
      comments: [Comment]
      createdAt: DateTime
    }

    type Comment {
      id: ID!
      text: String
      author: User!
      post: Post!
    }
    '

- name: Query user network
  shell: |
    curl -X POST http://localhost:8080/graphql \
      -d '{
        "query": "{ queryUser(filter: {username: {eq: \"alice\"}}) { name followers { name followers { name } } } }"
      }'
```

### Knowledge graph
```yaml
- name: Create knowledge graph schema
  shell: |
    curl -X POST http://localhost:8080/admin/schema \
      -H "Content-Type: application/graphql" \
      -d '
    type Entity {
      id: ID!
      name: String! @search(by: [term, fulltext])
      type: String @search(by: [exact])
      relatedTo: [Relationship]
    }

    type Relationship {
      id: ID!
      type: String! @search(by: [exact])
      from: Entity!
      to: Entity!
      properties: String
    }
    '

- name: Query relationships
  shell: |
    curl -X POST http://localhost:8080/graphql \
      -d '{
        "query": "{ queryEntity(filter: {name: {anyofterms: \"Einstein\"}}) { name relatedTo { type to { name } } } }"
      }'
```

### Recommendation system
```yaml
- name: Create recommendation schema
  shell: |
    curl -X POST http://localhost:8080/admin/schema \
      -H "Content-Type: application/graphql" \
      -d '
    type Product {
      id: ID!
      name: String! @search(by: [term, fulltext])
      category: String @search(by: [exact])
      price: Float @search
      embedding: [Float] @embedding
      purchasedBy: [User]
    }

    type User {
      id: ID!
      name: String
      purchased: [Product]
    }
    '

- name: Find similar products
  shell: |
    curl -X POST http://localhost:8080/graphql \
      -d '{
        "query": "{ querySimilarProducts(by: EMBEDDING, topK: 5, vector: [...]) { name price } }"
      }'
```

## CLI Commands

### Data operations
```bash
# Export data
dgraph export -a localhost:9080

# Import data (RDF format)
dgraph live -f data.rdf -a localhost:9080

# Backup
dgraph backup -a localhost:9080 -d /backups

# Restore
dgraph restore -l /backups -a localhost:9080
```

### Cluster management
```bash
# Check cluster health
curl http://localhost:6080/health

# Remove node from cluster
dgraph zero --idx=1 --peer=node2:5080 --rebalance_interval=0

# Move tablet (shard rebalancing)
curl http://localhost:6080/moveTablet?tablet=name&group=2
```

## Monitoring

### Metrics endpoint
```bash
# Alpha metrics
curl http://localhost:8080/debug/prometheus_metrics

# Zero metrics
curl http://localhost:6080/debug/prometheus_metrics
```

### Prometheus integration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'dgraph-alpha'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /debug/prometheus_metrics

  - job_name: 'dgraph-zero'
    static_configs:
      - targets: ['localhost:6080']
    metrics_path: /debug/prometheus_metrics
```

### Key metrics
```promql
# Query latency
dgraph_latency_bucket

# Memory usage
dgraph_memory_alloc_bytes

# Pending proposals (Raft)
dgraph_pending_proposals_total

# Disk usage
dgraph_disk_usage_bytes
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Dgraph
  preset: dgraph
```

### Production deployment
```yaml
- name: Install Dgraph
  preset: dgraph

- name: Create dgraph user
  command:
    cmd: useradd
    argv: ["-r", "-s", "/bin/false", "dgraph"]
  become: true

- name: Create directories
  file:
    path: "{{ item }}"
    state: directory
    owner: dgraph
    group: dgraph
    mode: '0755'
  loop:
    - /var/lib/dgraph/zero
    - /var/lib/dgraph/alpha
    - /var/log/dgraph
  become: true

- name: Start Dgraph Zero
  service:
    name: dgraph-zero
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=Dgraph Zero
        After=network.target

        [Service]
        Type=simple
        User=dgraph
        Group=dgraph
        ExecStart=/usr/local/bin/dgraph zero --wal /var/lib/dgraph/zero
        Restart=always

        [Install]
        WantedBy=multi-user.target
  become: true

- name: Start Dgraph Alpha
  service:
    name: dgraph-alpha
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=Dgraph Alpha
        After=network.target dgraph-zero.service

        [Service]
        Type=simple
        User=dgraph
        Group=dgraph
        ExecStart=/usr/local/bin/dgraph alpha --zero localhost:5080 --postings /var/lib/dgraph/alpha --lru_mb=4096
        Restart=always

        [Install]
        WantedBy=multi-user.target
  become: true
```

### Kubernetes deployment
```yaml
- name: Add Dgraph Helm repository
  shell: |
    helm repo add dgraph https://charts.dgraph.io
    helm repo update

- name: Install Dgraph
  shell: |
    helm install dgraph dgraph/dgraph \
      --namespace dgraph \
      --create-namespace \
      --set zero.replicaCount=3 \
      --set alpha.replicaCount=3 \
      --set alpha.persistence.size=100Gi
```

## Agent Use
- **Social networks**: Store users, relationships, and interactions
- **Knowledge graphs**: Entity relationships and semantic search
- **Recommendation systems**: Product similarities and user preferences
- **Fraud detection**: Transaction patterns and anomaly detection
- **Content management**: Hierarchical content with rich relationships
- **Access control**: Complex permission graphs
- **Network topology**: Infrastructure and dependency mapping

## Troubleshooting

### Dgraph won't start
```bash
# Check logs
journalctl -u dgraph-alpha -f
journalctl -u dgraph-zero -f

# Check Zero is running
curl http://localhost:6080/health

# Check port availability
netstat -tuln | grep -E "5080|8080"

# Check disk space
df -h /var/lib/dgraph
```

### Query performance issues
```bash
# Check query plan
curl -X POST http://localhost:8080/graphql?debug=true \
  -d '{"query": "{ queryPerson { name } }"}'

# Check for missing indexes
# Review schema for @search directives

# Increase cache
dgraph alpha --lru_mb=8192

# Check metrics
curl http://localhost:8080/debug/prometheus_metrics | grep latency
```

### Data corruption
```bash
# Check badger files
badger info --dir=/var/lib/dgraph/alpha/p

# Restore from backup
dgraph restore -l /backups -a localhost:9080
```

### High memory usage
```bash
# Reduce cache size
dgraph alpha --lru_mb=2048

# Enable compression
dgraph alpha --badger.compression=zstd

# Check memory metrics
curl http://localhost:8080/debug/prometheus_metrics | grep memory
```

## Best Practices

1. **Use GraphQL** for most use cases (simpler than DQL)
2. **Add indexes** with `@search` for filtered queries
3. **Deploy 3+ node cluster** for production high availability
4. **Set replication factor** to 3 for data durability
5. **Use vector search** for semantic similarity queries
6. **Monitor memory usage** and adjust LRU cache size
7. **Regular backups** with `dgraph backup`
8. **Use namespaces** for multi-tenancy isolation
9. **Enable ACLs** for production security
10. **Optimize schema** with appropriate data types and indexes

## Backup and Restore

### Backup
```yaml
- name: Create Dgraph backup
  shell: dgraph backup -a localhost:9080 -d /backup/dgraph/$(date +%Y%m%d)

- name: Copy backup to remote
  shell: rsync -av /backup/dgraph/ s3://my-bucket/dgraph-backups/
```

### Restore
```yaml
- name: Stop Dgraph
  service:
    name: dgraph-alpha
    state: stopped

- name: Restore from backup
  shell: dgraph restore -l /backup/dgraph/20240101 -a localhost:9080

- name: Start Dgraph
  service:
    name: dgraph-alpha
    state: started
```

## Uninstall
```yaml
- name: Stop Dgraph
  service:
    name: "{{ item }}"
    state: stopped
  loop:
    - dgraph-alpha
    - dgraph-zero

- name: Remove Dgraph
  preset: dgraph
  with:
    state: absent

- name: Remove data
  file:
    path: /var/lib/dgraph
    state: absent
  become: true
```

**Note**: Uninstalling does not remove data directory. Delete `/var/lib/dgraph` manually if needed.

## Resources
- Official: https://dgraph.io/
- Documentation: https://dgraph.io/docs/
- GitHub: https://github.com/dgraph-io/dgraph
- GraphQL: https://dgraph.io/docs/graphql/
- DQL: https://dgraph.io/docs/dql/
- Community: https://discuss.dgraph.io/
- Cloud: https://dgraph.io/cloud
- Search: "dgraph tutorial", "dgraph graphql", "dgraph vs neo4j"

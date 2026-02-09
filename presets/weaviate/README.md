# weaviate

AI-native vector database for semantic search, RAG, and embeddings

## Quick Start

```yaml
- preset: weaviate
```

## What is Weaviate?

**Weaviate** is an open-source vector database designed for AI applications. It stores data objects and vector embeddings, enabling semantic search, recommendation systems, and Retrieval Augmented Generation (RAG) for LLMs.

**Key Features:**
- **Vector Search**: Sub-100ms queries on billions of objects
- **Hybrid Search**: Combines vector and keyword search with BM25
- **Multi-Modal**: Text, images, video embeddings in one database
- **Modular**: Pluggable vectorizers (OpenAI, Cohere, HuggingFace, custom)
- **GraphQL API**: Native GraphQL interface with RESTful fallback
- **HNSW Index**: Hierarchical Navigable Small World graphs for speed
- **Multi-Tenancy**: Isolated data partitions for SaaS applications
- **Scalability**: Horizontal scaling with replication and sharding

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Client Applications                │
│     (Python, JS, Go, Java, REST, GraphQL)      │
└─────────────────┬───────────────────────────────┘
                  │
         ┌────────▼────────┐
         │  Weaviate API   │
         │   (Port 8080)   │
         └────────┬────────┘
                  │
    ┌─────────────┴─────────────┐
    │                           │
┌───▼────────┐         ┌────────▼───┐
│  Vectorizer│         │   Vector   │
│  Modules   │         │   Storage  │
│ (OpenAI,   │         │   (HNSW)   │
│  Cohere)   │         └────────────┘
└────────────┘
```

**Components:**
- **Core Database**: CRUD operations, indexing, persistence
- **Vector Index**: HNSW algorithm for approximate nearest neighbor search
- **Vectorizer Modules**: Text2vec, img2vec, multi2vec modules
- **Schema Manager**: Class definitions, properties, cross-references
- **Query Engine**: GraphQL and REST APIs for data retrieval

## Usage

### Basic Operations

```bash
# Check version
weaviate --version

# Get help
weaviate --help

# View configuration
weaviate show-config
```

### Docker Deployment

```yaml
- name: Deploy Weaviate with Docker Compose
  shell: |
    cat <<EOF > docker-compose.yml
    version: '3.4'
    services:
      weaviate:
        image: semitechnologies/weaviate:1.24.0
        restart: on-failure:0
        ports:
         - "8080:8080"
         - "50051:50051"
        environment:
          QUERY_DEFAULTS_LIMIT: 25
          AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: 'true'
          PERSISTENCE_DATA_PATH: '/var/lib/weaviate'
          DEFAULT_VECTORIZER_MODULE: 'none'
          ENABLE_MODULES: 'text2vec-openai,text2vec-cohere,text2vec-huggingface'
          CLUSTER_HOSTNAME: 'node1'
        volumes:
          - weaviate_data:/var/lib/weaviate
    volumes:
      weaviate_data:
    EOF
    docker-compose up -d

- name: Wait for Weaviate to be ready
  shell: |
    until curl -sf http://localhost:8080/v1/meta > /dev/null; do
      echo "Waiting for Weaviate..."
      sleep 2
    done
```

### Kubernetes Deployment

```yaml
- name: Install Weaviate with Helm
  shell: |
    helm repo add weaviate https://weaviate.github.io/weaviate-helm
    helm repo update

    helm install weaviate weaviate/weaviate \
      --namespace weaviate --create-namespace \
      --set replicas=3 \
      --set resources.requests.memory=4Gi \
      --set resources.requests.cpu=1000m \
      --set service.type=LoadBalancer

- name: Get Weaviate endpoint
  shell: kubectl get svc -n weaviate weaviate -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
  register: weaviate_endpoint

- name: Display endpoint
  print: "Weaviate available at http://{{ weaviate_endpoint.stdout }}:8080"
```

## Configuration

### Environment Variables

```yaml
# Core settings
QUERY_DEFAULTS_LIMIT: "25"                    # Default query limit
QUERY_MAXIMUM_RESULTS: "10000"                # Max results per query
AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: "true"  # Allow anonymous access
AUTHENTICATION_APIKEY_ENABLED: "true"         # Enable API key auth
AUTHENTICATION_APIKEY_ALLOWED_KEYS: "key1,key2"

# Persistence
PERSISTENCE_DATA_PATH: "/var/lib/weaviate"   # Data directory
PERSISTENCE_LSM_ACCESS_STRATEGY: "mmap"      # Memory mapping strategy
BACKUP_FILESYSTEM_PATH: "/var/weaviate/backups"

# Vectorizer modules
DEFAULT_VECTORIZER_MODULE: "text2vec-openai"
ENABLE_MODULES: "text2vec-openai,text2vec-cohere,text2vec-huggingface,generative-openai"
OPENAI_APIKEY: "sk-..."                      # OpenAI API key
COHERE_APIKEY: "..."                         # Cohere API key

# Performance
GOMAXPROCS: "8"                              # Go max processors
LIMIT_RESOURCES: "true"                      # Limit resource usage
DISK_USE_WARNING_PERCENTAGE: 80
DISK_USE_READONLY_PERCENTAGE: 90

# Cluster settings
CLUSTER_HOSTNAME: "node1"
CLUSTER_JOIN: "node2:7946,node3:7946"        # Join cluster
CLUSTER_DATA_BIND_PORT: 7946
RAFT_JOIN: "node1,node2,node3"               # Raft consensus nodes
```

## Schema Management

### Creating a Class (Schema)

```python
import weaviate

client = weaviate.Client("http://localhost:8080")

# Define a class for articles
class_schema = {
    "class": "Article",
    "description": "News articles with embeddings",
    "vectorizer": "text2vec-openai",
    "moduleConfig": {
        "text2vec-openai": {
            "model": "text-embedding-ada-002",
            "modelVersion": "002",
            "type": "text"
        }
    },
    "properties": [
        {
            "name": "title",
            "dataType": ["text"],
            "description": "Article title",
            "moduleConfig": {
                "text2vec-openai": {
                    "skip": False,
                    "vectorizePropertyName": False
                }
            }
        },
        {
            "name": "content",
            "dataType": ["text"],
            "description": "Article body"
        },
        {
            "name": "author",
            "dataType": ["text"],
            "description": "Author name"
        },
        {
            "name": "publishedDate",
            "dataType": ["date"],
            "description": "Publication date"
        },
        {
            "name": "category",
            "dataType": ["text"],
            "description": "Article category"
        },
        {
            "name": "tags",
            "dataType": ["text[]"],
            "description": "Article tags"
        }
    ]
}

client.schema.create_class(class_schema)
```

### Adding Data with Vectors

```python
# Import with automatic vectorization
article = {
    "title": "The Future of AI",
    "content": "Artificial intelligence is transforming industries...",
    "author": "Jane Doe",
    "publishedDate": "2024-01-15T10:00:00Z",
    "category": "Technology",
    "tags": ["AI", "Machine Learning", "Innovation"]
}

# Weaviate automatically generates embeddings
client.data_object.create(
    data_object=article,
    class_name="Article"
)

# Batch import for performance
with client.batch as batch:
    batch.batch_size = 100
    for article in articles:
        batch.add_data_object(article, "Article")
```

### Vector Search

```python
# Semantic search by meaning
response = client.query.get(
    "Article",
    ["title", "content", "author", "publishedDate"]
).with_near_text({
    "concepts": ["machine learning applications"],
    "distance": 0.7
}).with_limit(5).do()

# Hybrid search (vector + keyword)
response = client.query.get(
    "Article",
    ["title", "content", "author"]
).with_hybrid(
    query="AI ethics",
    alpha=0.5  # 0=keyword only, 1=vector only, 0.5=balanced
).with_limit(10).do()

# Filtered search
response = client.query.get(
    "Article",
    ["title", "content"]
).with_near_text({
    "concepts": ["neural networks"]
}).with_where({
    "path": ["category"],
    "operator": "Equal",
    "valueText": "Technology"
}).do()
```

### GraphQL Queries

```graphql
{
  Get {
    Article(
      nearText: {
        concepts: ["climate change solutions"]
        distance: 0.6
      }
      limit: 5
    ) {
      title
      content
      author
      publishedDate
      _additional {
        distance
        certainty
        id
      }
    }
  }
}
```

## Advanced Usage

### RAG with Generative Module

```python
# Configure generative-openai module
response = client.query.get(
    "Article",
    ["title", "content"]
).with_near_text({
    "concepts": ["quantum computing"]
}).with_generate(
    single_prompt="Summarize this article in one sentence: {content}"
).with_limit(3).do()

# Access generated summaries
for article in response['data']['Get']['Article']:
    print(f"Title: {article['title']}")
    print(f"Summary: {article['_additional']['generate']['singleResult']}")
```

### Multi-Tenancy

```python
# Create multi-tenant class
client.schema.create_class({
    "class": "Document",
    "multiTenancyConfig": {"enabled": True},
    "properties": [
        {"name": "content", "dataType": ["text"]}
    ]
})

# Add tenant
client.schema.add_class_tenants(
    class_name="Document",
    tenants=[{"name": "tenant_acme"}, {"name": "tenant_globex"}]
)

# Query specific tenant
response = client.query.get(
    "Document",
    ["content"]
).with_tenant("tenant_acme").with_near_text({
    "concepts": ["contract"]
}).do()
```

### Cross-References

```python
# Create Author class
client.schema.create_class({
    "class": "Author",
    "properties": [
        {"name": "name", "dataType": ["text"]},
        {"name": "bio", "dataType": ["text"]}
    ]
})

# Add cross-reference to Article
client.schema.property.create(
    "Article",
    {
        "name": "writtenBy",
        "dataType": ["Author"]
    }
)

# Query with cross-references
response = client.query.get(
    "Article",
    ["title", "writtenBy { ... on Author { name bio } }"]
).with_near_text({
    "concepts": ["blockchain"]
}).do()
```

### Custom Vectors

```python
# Import with your own vectors (e.g., from custom model)
import numpy as np

vector = np.random.rand(1536).tolist()  # OpenAI ada-002 dimension

client.data_object.create(
    data_object={
        "title": "Custom Embeddings Example",
        "content": "Using pre-computed vectors"
    },
    class_name="Article",
    vector=vector
)

# Search with custom vector
query_vector = np.random.rand(1536).tolist()

response = client.query.get(
    "Article",
    ["title", "content"]
).with_near_vector({
    "vector": query_vector
}).do()
```

## Use Cases

### Semantic Search Engine

```yaml
- name: Deploy semantic search for documentation
  shell: |
    python3 <<EOF
    import weaviate

    client = weaviate.Client("http://localhost:8080")

    # Create schema
    client.schema.create_class({
        "class": "Documentation",
        "vectorizer": "text2vec-openai",
        "properties": [
            {"name": "title", "dataType": ["text"]},
            {"name": "section", "dataType": ["text"]},
            {"name": "content", "dataType": ["text"]},
            {"name": "url", "dataType": ["text"]}
        ]
    })

    # Import docs
    docs = [
        {
            "title": "Getting Started",
            "section": "Introduction",
            "content": "This guide helps you get started...",
            "url": "/docs/intro"
        },
        # More documents...
    ]

    with client.batch as batch:
        for doc in docs:
            batch.add_data_object(doc, "Documentation")

    print("Documentation search ready")
    EOF
```

### RAG for LLM Applications

```python
def rag_query(question: str) -> str:
    """Retrieval Augmented Generation with Weaviate."""

    # Search relevant context
    response = client.query.get(
        "Article",
        ["content"]
    ).with_near_text({
        "concepts": [question],
        "distance": 0.7
    }).with_limit(5).with_generate(
        grouped_task=f"Answer this question: {question}"
    ).do()

    # Return generated answer
    return response['data']['Get']['Article'][0]['_additional']['generate']['groupedResult']

# Example
answer = rag_query("What are the benefits of vector databases?")
print(answer)
```

### Recommendation System

```yaml
- name: Build product recommendation system
  shell: |
    python3 <<EOF
    import weaviate

    client = weaviate.Client("http://localhost:8080")

    # Schema for products
    client.schema.create_class({
        "class": "Product",
        "vectorizer": "text2vec-openai",
        "properties": [
            {"name": "name", "dataType": ["text"]},
            {"name": "description", "dataType": ["text"]},
            {"name": "category", "dataType": ["text"]},
            {"name": "price", "dataType": ["number"]},
            {"name": "imageUrl", "dataType": ["text"]}
        ]
    })

    # Find similar products
    def recommend(product_id: str, limit: int = 5):
        # Get product vector
        product = client.data_object.get_by_id(product_id, with_vector=True)

        # Find similar
        return client.query.get(
            "Product",
            ["name", "description", "price"]
        ).with_near_vector({
            "vector": product["vector"]
        }).with_limit(limit).do()

    print("Recommendation system ready")
    EOF
```

### Image Search

```python
# Configure multi-modal vectorizer
client.schema.create_class({
    "class": "Image",
    "vectorizer": "multi2vec-clip",
    "moduleConfig": {
        "multi2vec-clip": {
            "imageFields": ["image"],
            "textFields": ["description"]
        }
    },
    "properties": [
        {
            "name": "image",
            "dataType": ["blob"]
        },
        {
            "name": "description",
            "dataType": ["text"]
        },
        {
            "name": "tags",
            "dataType": ["text[]"]
        }
    ]
})

# Search images by text
response = client.query.get(
    "Image",
    ["image", "description"]
).with_near_text({
    "concepts": ["sunset over mountains"]
}).do()
```

## Mooncake Integration

### Complete Deployment

```yaml
- name: Deploy Weaviate cluster
  vars:
    weaviate_version: "1.24.0"
    weaviate_replicas: 3
    openai_key: "{{ lookup('env', 'OPENAI_API_KEY') }}"

- name: Create namespace
  shell: kubectl create namespace weaviate --dry-run=client -o yaml | kubectl apply -f -

- name: Deploy Weaviate with Helm
  shell: |
    helm repo add weaviate https://weaviate.github.io/weaviate-helm
    helm repo update

    helm upgrade --install weaviate weaviate/weaviate \
      --namespace weaviate \
      --set replicas={{ weaviate_replicas }} \
      --set image.tag={{ weaviate_version }} \
      --set service.type=LoadBalancer \
      --set resources.requests.memory=4Gi \
      --set resources.requests.cpu=1000m \
      --set authentication.apikey.enabled=true \
      --set authentication.apikey.allowed_keys="admin-key" \
      --set modules.text2vec-openai.enabled=true \
      --set modules.text2vec-openai.apiKey="{{ openai_key }}" \
      --set modules.generative-openai.enabled=true \
      --set persistence.enabled=true \
      --set persistence.size=100Gi
  register: helm_deploy

- name: Wait for deployment
  shell: kubectl rollout status deployment/weaviate -n weaviate --timeout=5m

- name: Get service endpoint
  shell: kubectl get svc -n weaviate weaviate -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
  register: endpoint

- name: Health check
  assert:
    http:
      url: "http://{{ endpoint.stdout }}:8080/v1/meta"
      status: 200

- name: Display info
  print: |
    Weaviate deployed successfully!
    Endpoint: http://{{ endpoint.stdout }}:8080
    GraphQL: http://{{ endpoint.stdout }}:8080/v1/graphql
```

### Schema Initialization

```yaml
- name: Initialize schema
  shell: |
    cat <<EOF > schema.json
    {
      "classes": [
        {
          "class": "Document",
          "vectorizer": "text2vec-openai",
          "properties": [
            {"name": "title", "dataType": ["text"]},
            {"name": "content", "dataType": ["text"]},
            {"name": "metadata", "dataType": ["object"]}
          ]
        }
      ]
    }
    EOF

    curl -X POST "http://{{ endpoint.stdout }}:8080/v1/schema" \
      -H "Content-Type: application/json" \
      -d @schema.json

- name: Verify schema
  shell: curl -s "http://{{ endpoint.stdout }}:8080/v1/schema" | jq '.classes[].class'
  register: classes

- name: Display classes
  print: "Created classes: {{ classes.stdout }}"
```

## Agent Use

Weaviate is automation-friendly with:
- **RESTful API**: Full CRUD operations via HTTP
- **GraphQL API**: Flexible queries with type safety
- **Client Libraries**: Python, JavaScript, Go, Java with idiomatic APIs
- **Batch Operations**: Efficient bulk imports and updates
- **Health Checks**: `/v1/meta` endpoint for readiness probes
- **Metrics**: Prometheus metrics on `/metrics` endpoint
- **Backup/Restore**: API-driven backup operations

### Monitoring

```yaml
- name: Check Weaviate health
  assert:
    http:
      url: "http://localhost:8080/v1/meta"
      status: 200
  register: health

- name: Get cluster status
  shell: curl -s http://localhost:8080/v1/nodes | jq '.nodes[].status'
  register: cluster_status

- name: Verify all nodes healthy
  assert:
    command:
      cmd: echo "{{ cluster_status.stdout }}"
      exit_code: 0
  when: cluster_status.stdout is search("HEALTHY")

- name: Check object count
  shell: |
    curl -s http://localhost:8080/v1/graphql -X POST \
      -H "Content-Type: application/json" \
      -d '{"query": "{ Aggregate { Article { meta { count } } } }"}' \
      | jq '.data.Aggregate.Article[0].meta.count'
  register: object_count

- name: Display metrics
  print: "Total objects: {{ object_count.stdout }}"
```

## CLI Commands

### Core Operations

```bash
# Health check
curl http://localhost:8080/v1/meta

# Get schema
curl http://localhost:8080/v1/schema

# List classes
curl http://localhost:8080/v1/schema | jq '.classes[].class'

# Create object
curl -X POST http://localhost:8080/v1/objects \
  -H "Content-Type: application/json" \
  -d '{
    "class": "Article",
    "properties": {
      "title": "Sample Article",
      "content": "This is sample content"
    }
  }'

# Get object by ID
curl http://localhost:8080/v1/objects/Article/{id}

# Delete object
curl -X DELETE http://localhost:8080/v1/objects/Article/{id}

# Vector search via REST
curl -X POST http://localhost:8080/v1/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "{ Get { Article(nearText: {concepts: [\"AI\"]}) { title } } }"
  }'
```

### Backup and Restore

```bash
# Create backup
curl -X POST http://localhost:8080/v1/backups/filesystem \
  -H "Content-Type: application/json" \
  -d '{
    "id": "backup-2024-01-15",
    "include": ["Article", "Author"]
  }'

# Check backup status
curl http://localhost:8080/v1/backups/filesystem/backup-2024-01-15

# Restore backup
curl -X POST http://localhost:8080/v1/backups/filesystem/backup-2024-01-15/restore
```

### Performance Tuning

```bash
# Get vector index configuration
curl http://localhost:8080/v1/schema/Article | jq '.vectorIndexConfig'

# Update HNSW parameters
curl -X PUT http://localhost:8080/v1/schema/Article \
  -H "Content-Type: application/json" \
  -d '{
    "vectorIndexConfig": {
      "ef": 64,
      "efConstruction": 128,
      "maxConnections": 64
    }
  }'
```

## Troubleshooting

### Connection Issues

```yaml
- name: Test connectivity
  shell: curl -f http://localhost:8080/v1/meta
  register: health_check
  failed_when: false

- name: Check if port is open
  shell: nc -zv localhost 8080
  when: health_check.rc != 0

- name: View Weaviate logs
  shell: docker logs weaviate --tail 50
  when: health_check.rc != 0
```

### Performance Problems

```yaml
- name: Check resource usage
  shell: docker stats weaviate --no-stream --format "table {{.CPUPerc}}\t{{.MemUsage}}"
  register: stats

- name: Monitor query performance
  shell: |
    curl -s http://localhost:8080/v1/graphql -X POST \
      -H "Content-Type: application/json" \
      -d '{"query": "{ Aggregate { Article { meta { count } } } }"}' \
      -w "\nTime: %{time_total}s\n"
  register: query_time

- name: Check vector index size
  shell: |
    curl -s http://localhost:8080/v1/schema/Article | \
      jq '.vectorIndexConfig'
```

### Memory Issues

```yaml
- name: Check GOMEMLIMIT
  shell: docker exec weaviate printenv GOMEMLIMIT
  register: mem_limit

- name: Increase memory limit
  shell: |
    docker stop weaviate
    docker run -d --name weaviate \
      -e GOMEMLIMIT=8GiB \
      -e LIMIT_RESOURCES=true \
      -p 8080:8080 \
      semitechnologies/weaviate:1.24.0
  when: mem_limit.stdout | int < 8000000000
```

## Best Practices

### Schema Design

1. **Choose appropriate vectorizers**: Use `text2vec-openai` for production, `text2vec-transformers` for privacy
2. **Set vectorizePropertyName**: Skip vectorizing property names to reduce noise
3. **Use inverted indexes**: Enable for properties used in filters
4. **Configure HNSW properly**:
   - Higher `ef` (64-512) = better recall, slower queries
   - Higher `efConstruction` (128-512) = better quality, slower indexing
   - Higher `maxConnections` (32-128) = more memory, faster queries

### Performance

```yaml
# Optimize for write-heavy workloads
vectorIndexConfig:
  ef: 64                    # Lower for faster writes
  efConstruction: 128
  maxConnections: 32
  vectorCacheMaxObjects: 500000

# Optimize for read-heavy workloads
vectorIndexConfig:
  ef: 128                   # Higher for better recall
  efConstruction: 256
  maxConnections: 64
  vectorCacheMaxObjects: 1000000
```

### Batch Imports

```python
# Use batch mode for bulk imports
with client.batch(
    batch_size=100,           # Adjust based on object size
    dynamic=True,             # Auto-adjust batch size
    timeout_retries=3,
    connection_error_retries=3
) as batch:
    for obj in objects:
        batch.add_data_object(obj, "Article")
```

### Security

```yaml
- name: Enable authentication
  shell: |
    docker run -d --name weaviate \
      -e AUTHENTICATION_APIKEY_ENABLED=true \
      -e AUTHENTICATION_APIKEY_ALLOWED_KEYS="admin-secret-key,readonly-key" \
      -e AUTHENTICATION_APIKEY_USERS="admin,readonly" \
      -e AUTHORIZATION_ADMINLIST_ENABLED=true \
      -e AUTHORIZATION_ADMINLIST_USERS="admin" \
      -p 8080:8080 \
      semitechnologies/weaviate:1.24.0

- name: Use API key in requests
  shell: |
    curl http://localhost:8080/v1/meta \
      -H "Authorization: Bearer admin-secret-key"
```

### Monitoring

```yaml
- name: Enable Prometheus metrics
  shell: |
    docker run -d --name weaviate \
      -e PROMETHEUS_MONITORING_ENABLED=true \
      -e PROMETHEUS_MONITORING_PORT=2112 \
      -p 8080:8080 -p 2112:2112 \
      semitechnologies/weaviate:1.24.0

- name: Scrape metrics
  shell: curl http://localhost:2112/metrics | grep weaviate
  register: metrics

- name: Monitor key metrics
  shell: |
    curl -s http://localhost:2112/metrics | \
      grep -E "weaviate_object_count|weaviate_vector_index_size|weaviate_batch_durations"
```

## Backup Strategies

### Filesystem Backup

```yaml
- name: Create filesystem backup
  shell: |
    curl -X POST http://localhost:8080/v1/backups/filesystem \
      -H "Content-Type: application/json" \
      -d '{
        "id": "backup-{{ ansible_date_time.epoch }}",
        "include": ["Article", "Author"]
      }'
  register: backup

- name: Wait for backup completion
  shell: |
    while true; do
      status=$(curl -s http://localhost:8080/v1/backups/filesystem/{{ backup.json.id }} | jq -r '.status')
      [ "$status" = "SUCCESS" ] && break
      sleep 5
    done

- name: Copy backup to S3
  shell: |
    aws s3 cp /var/lib/weaviate/backups/{{ backup.json.id }} \
      s3://my-backups/weaviate/ --recursive
```

### Point-in-Time Recovery

```yaml
- name: Schedule regular backups
  shell: |
    cat <<EOF > /etc/cron.d/weaviate-backup
    0 2 * * * root /usr/local/bin/weaviate-backup.sh
    EOF

- name: Create backup script
  copy:
    dest: /usr/local/bin/weaviate-backup.sh
    mode: '0755'
    content: |
      #!/bin/bash
      BACKUP_ID="backup-$(date +%Y%m%d-%H%M%S)"
      curl -X POST http://localhost:8080/v1/backups/filesystem \
        -H "Content-Type: application/json" \
        -d "{\"id\": \"$BACKUP_ID\"}"
```

## Uninstall

```yaml
- preset: weaviate
  with:
    state: absent
```

**Manual cleanup:**

```bash
# Stop and remove Docker container
docker stop weaviate
docker rm weaviate

# Remove data volume
docker volume rm weaviate_data

# Uninstall Helm chart
helm uninstall weaviate -n weaviate

# Delete namespace
kubectl delete namespace weaviate

# Remove backup files
rm -rf /var/lib/weaviate/backups
```

## Resources

- **Official Website**: https://weaviate.io
- **Documentation**: https://weaviate.io/developers/weaviate
- **GitHub**: https://github.com/weaviate/weaviate
- **Python Client**: https://weaviate-python-client.readthedocs.io
- **Examples**: https://github.com/weaviate/weaviate-examples
- **Community Slack**: https://weaviate.io/slack
- **Blog**: https://weaviate.io/blog
- **Awesome Weaviate**: https://github.com/weaviate/awesome-weaviate

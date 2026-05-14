# Qdrant - Vector Database

High-performance vector database for AI applications with advanced filtering and full-text search capabilities.

## Quick Start

```yaml
- preset: qdrant
```

## Features

- **Vector similarity search**: Fast nearest neighbor search with multiple distance metrics
- **Hybrid search**: Combines vector and traditional filtering
- **Payload filtering**: Filter vectors by metadata before similarity search
- **Distributed**: Horizontal scaling with sharding and replication
- **CRUD API**: Full REST and gRPC APIs for vector management
- **Multiple languages**: Official clients for Python, Rust, Go, TypeScript, .NET
- **Persistent storage**: On-disk and in-memory storage with snapshots
- **Cross-platform**: Linux, macOS, Docker support

## Basic Usage

```bash
# Check version
qdrant --version

# Start server (default port 6333)
qdrant

# Health check
curl http://localhost:6333/health

# Create collection
curl -X PUT http://localhost:6333/collections/my_collection \
  -H 'Content-Type: application/json' \
  -d '{
    "vectors": {
      "size": 384,
      "distance": "Cosine"
    }
  }'

# Insert vectors
curl -X PUT http://localhost:6333/collections/my_collection/points \
  -H 'Content-Type: application/json' \
  -d '{
    "points": [
      {
        "id": 1,
        "vector": [0.1, 0.2, 0.3, ...],
        "payload": {"city": "London"}
      }
    ]
  }'

# Search
curl -X POST http://localhost:6333/collections/my_collection/points/search \
  -H 'Content-Type: application/json' \
  -d '{
    "vector": [0.1, 0.2, 0.3, ...],
    "limit": 5
  }'
```

## Advanced Configuration

```yaml
# Install Qdrant
- preset: qdrant
  register: qdrant_result
  become: true

# Configure as service
- name: Start Qdrant with custom settings
  shell: |
    qdrant \
      --storage-path /var/lib/qdrant/storage \
      --http-port 6333 \
      --grpc-port 6334
  become: true

# Verify installation
- name: Wait for Qdrant to be ready
  assert:
    http:
      url: http://localhost:6333/health
      status: 200
  retries: 10
  delay: 2
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (`present` or `absent`) |

## Platform Support

- ✅ Linux (apt, dnf, yum, binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Docker (official images available)
- ❌ Windows (use Docker or WSL)

## Configuration

- **Config file**: `config/config.yaml` (in storage directory)
- **Data directory**: `/var/lib/qdrant/storage/` or `./storage/`
- **HTTP API port**: 6333
- **gRPC API port**: 6334
- **Web dashboard**: http://localhost:6333/dashboard
- **Binary location**: `/usr/local/bin/qdrant`

## Real-World Examples

### Semantic Search System

```yaml
# Deploy Qdrant for semantic search
- preset: qdrant
  become: true

# Configure systemd service
- name: Create Qdrant service
  service:
    name: qdrant
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=Qdrant Vector Database
        After=network.target

        [Service]
        Type=simple
        User=qdrant
        WorkingDirectory=/var/lib/qdrant
        ExecStart=/usr/local/bin/qdrant --storage-path=/var/lib/qdrant/storage
        Restart=always
        RestartSec=10
        LimitNOFILE=65536

        [Install]
        WantedBy=multi-user.target
  when: os == "linux"
  become: true

# Create collection for documents
- name: Initialize semantic search collection
  shell: |
    curl -X PUT http://localhost:6333/collections/documents \
      -H 'Content-Type: application/json' \
      -d '{
        "vectors": {
          "size": 768,
          "distance": "Cosine"
        },
        "optimizers_config": {
          "indexing_threshold": 10000
        }
      }'
```

Python client example:

```python
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, VectorParams, PointStruct

# Connect to Qdrant
client = QdrantClient(host="localhost", port=6333)

# Create collection
client.create_collection(
    collection_name="documents",
    vectors_config=VectorParams(size=768, distance=Distance.COSINE)
)

# Insert documents
client.upsert(
    collection_name="documents",
    points=[
        PointStruct(
            id=1,
            vector=[0.1] * 768,  # Your embedding vector
            payload={"title": "Document 1", "category": "tech"}
        )
    ]
)

# Search
results = client.search(
    collection_name="documents",
    query_vector=[0.1] * 768,
    limit=5,
    query_filter={
        "must": [{"key": "category", "match": {"value": "tech"}}]
    }
)
```

### RAG System for LLM

```yaml
# Install Qdrant for RAG application
- preset: qdrant
  become: true

# Configure with increased memory
- name: Start Qdrant with optimized settings
  shell: |
    qdrant \
      --storage-path /var/lib/qdrant/storage \
      --log-level INFO
  environment:
    QDRANT__STORAGE__OPTIMIZERS__INDEXING_THRESHOLD: "20000"
    QDRANT__STORAGE__PERFORMANCE__MAX_OPTIMIZATION_THREADS: "4"
  become: true
```

RAG implementation:

```python
from qdrant_client import QdrantClient
from sentence_transformers import SentenceTransformer

# Initialize
client = QdrantClient("localhost", port=6333)
encoder = SentenceTransformer("all-MiniLM-L6-v2")

# Create knowledge base collection
client.create_collection(
    collection_name="knowledge_base",
    vectors_config={"size": 384, "distance": "Cosine"}
)

# Index documents
documents = ["AI is transforming...", "Machine learning..."]
vectors = encoder.encode(documents)

client.upload_collection(
    collection_name="knowledge_base",
    vectors=vectors,
    payload=[{"text": doc, "source": f"doc_{i}"} for i, doc in enumerate(documents)]
)

# Retrieve context for LLM
query = "What is AI?"
query_vector = encoder.encode([query])[0]

results = client.search(
    collection_name="knowledge_base",
    query_vector=query_vector,
    limit=3
)

# Use results as context for LLM
context = "\n".join([hit.payload["text"] for hit in results])
```

### Recommendation System

```yaml
# Deploy Qdrant cluster for recommendations
- preset: qdrant
  hosts: qdrant-servers
  become: true

# Configure distributed setup
- name: Setup Qdrant cluster node
  shell: |
    qdrant \
      --uri http://{{ ansible_host }}:6333 \
      --bootstrap http://{{ leader_host }}:6333
  when: inventory_hostname != leader_host
  become: true
```

### Image Similarity Search

```python
from qdrant_client import QdrantClient
import clip
import torch

# Setup
client = QdrantClient("localhost", port=6333)
device = "cuda" if torch.cuda.is_available() else "cpu"
model, preprocess = clip.load("ViT-B/32", device=device)

# Create collection for images
client.create_collection(
    collection_name="images",
    vectors_config={"size": 512, "distance": "Cosine"}
)

# Index images
from PIL import Image
image = preprocess(Image.open("photo.jpg")).unsqueeze(0).to(device)
with torch.no_grad():
    image_features = model.encode_image(image).cpu().numpy()[0]

client.upsert(
    collection_name="images",
    points=[{
        "id": 1,
        "vector": image_features.tolist(),
        "payload": {"url": "photo.jpg", "tags": ["nature", "landscape"]}
    }]
)

# Search by text
text = clip.tokenize(["beautiful sunset"]).to(device)
with torch.no_grad():
    text_features = model.encode_text(text).cpu().numpy()[0]

results = client.search(
    collection_name="images",
    query_vector=text_features.tolist(),
    limit=10
)
```

## Distance Metrics

```python
# Cosine similarity (most common for embeddings)
"distance": "Cosine"  # Range: [0, 2]

# Euclidean distance
"distance": "Euclid"  # Range: [0, ∞)

# Dot product (for normalized vectors)
"distance": "Dot"  # Range: [-∞, ∞]

# Manhattan distance
"distance": "Manhattan"  # Range: [0, ∞]
```

## Performance Optimization

```yaml
# Optimize for indexing
- name: Configure Qdrant for bulk indexing
  shell: |
    curl -X PATCH http://localhost:6333/collections/my_collection \
      -H 'Content-Type: application/json' \
      -d '{
        "optimizers_config": {
          "indexing_threshold": 50000,
          "max_optimization_threads": 8
        }
      }'

# Create payload index for filtering
- name: Add payload index
  shell: |
    curl -X PUT http://localhost:6333/collections/my_collection/index \
      -H 'Content-Type: application/json' \
      -d '{
        "field_name": "category",
        "field_schema": "keyword"
      }'
```

## Agent Use

- Semantic search for knowledge bases and documentation
- RAG (Retrieval Augmented Generation) systems for LLMs
- Recommendation engines for products, content, users
- Image and video similarity search
- Anomaly detection in time-series or sensor data
- Question answering systems with context retrieval
- Document clustering and classification
- Deduplication of large datasets

## Troubleshooting

### Server won't start

Check logs and permissions:

```bash
# Check if port is in use
lsof -i :6333

# Run with debug logging
qdrant --log-level DEBUG

# Verify storage directory permissions
ls -la /var/lib/qdrant/storage
```

### Slow search performance

Optimize indexing:

```bash
# Check collection status
curl http://localhost:6333/collections/my_collection

# Force optimization
curl -X POST http://localhost:6333/collections/my_collection/points/optimize

# Adjust HNSW parameters
curl -X PATCH http://localhost:6333/collections/my_collection \
  -H 'Content-Type: application/json' \
  -d '{
    "hnsw_config": {
      "m": 16,
      "ef_construct": 100
    }
  }'
```

### Out of memory

Reduce memory footprint:

```bash
# Use on-disk storage
export QDRANT__STORAGE__STORAGE_TYPE=disk

# Limit index cache
export QDRANT__STORAGE__PERFORMANCE__MAX_INDEX_MEMORY_KB=2000000

# Monitor memory usage
curl http://localhost:6333/metrics | grep memory
```

### Connection errors

Verify network settings:

```bash
# Test HTTP API
curl http://localhost:6333/health

# Test gRPC
grpcurl -plaintext localhost:6334 list

# Check firewall
sudo ufw status | grep 6333
```

## Uninstall

```yaml
- preset: qdrant
  with:
    state: absent
```

**Note**: This removes Qdrant binary but preserves data in `/var/lib/qdrant/`.

## Resources

- Official docs: https://qdrant.tech/documentation/
- GitHub: https://github.com/qdrant/qdrant
- Python client: https://github.com/qdrant/qdrant-client
- Examples: https://github.com/qdrant/examples
- Search: "qdrant vector database tutorial", "qdrant semantic search", "qdrant RAG system"

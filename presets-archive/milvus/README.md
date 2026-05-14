# Milvus - Vector Database for AI Applications

Milvus is a cloud-native vector database purpose-built for AI similarity search and retrieval augmented generation (RAG). It provides high-performance semantic search at scale with advanced features like multi-modal embedding support, full-text search integration, and distributed clustering.

## Quick Start

```yaml
- preset: milvus
```

## Features

- **Vector-optimized**: Specialized for similarity search on embeddings from language models
- **Cloud-native**: Distributed architecture for horizontal scaling across clusters
- **Multiple access patterns**: REST API, Python SDK, Java SDK, Node.js SDK
- **Flexible indexing**: Supports FLAT, IVF, HNSW, Annoy, and specialized algorithms
- **Production ready**: ACID transactions, backup/restore, metrics collection
- **AI integration**: Native support for RAG pipelines and semantic search workflows

## Basic Usage

```bash
# Check version and system info
milvus --version

# View help
milvus --help

# Connect using SDK (Python example)
from pymilvus import Collection
collection = Collection("my_collection")
results = collection.search(embeddings, "embedding_field", limit=10)
```

## Advanced Configuration

```yaml
- preset: milvus
```

**Note**: Milvus is primarily used as a service via SDKs and APIs rather than direct CLI interaction. Configuration typically happens through client SDKs or Docker/Kubernetes deployment.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Milvus |

## Platform Support

- ✅ Linux (via Docker/package managers)
- ✅ macOS (via Docker/Homebrew)
- ✅ Cloud platforms (Kubernetes, Docker Compose)
- ⚠️ Windows (primarily Docker-based)

## Configuration

- **Config file**: `/etc/milvus/milvus.yaml` (Linux), configuration via SDK or Docker env vars
- **Data directory**: `/var/lib/milvus/` (persistent storage)
- **Default port**: 19530 (gRPC), 9091 (HTTP)
- **Logs**: `/var/log/milvus/`

## Real-World Examples

### RAG System with Vector Search

```python
# Store document embeddings in Milvus
from pymilvus import Collection
collection = Collection("documents")
embeddings = [get_embedding(doc) for doc in documents]
collection.insert([embeddings])

# Retrieve similar documents
query_embedding = get_embedding(user_query)
results = collection.search(query_embedding, "embedding", limit=5)
```

### Semantic Search Pipeline

```bash
# Pre-process documents and create embeddings
python embed_documents.py > embeddings.jsonl

# Store in Milvus for fast retrieval
python load_to_milvus.py embeddings.jsonl

# Query for similar items
python semantic_search.py "find similar items"
```

## Agent Use

- Build RAG (retrieval-augmented generation) systems for LLM augmentation
- Implement semantic search in AI applications and chatbots
- Create recommendation systems using embeddings
- Index multi-modal content (text, images) for AI-driven search
- Scale vector similarity search across millions of embeddings
- Integrate with LLM frameworks for context retrieval

## Troubleshooting

### Connection refused on default port

Check if Milvus service is running and listening:

```bash
# Check port availability
netstat -tuln | grep 19530

# View logs for errors
tail -f /var/log/milvus/milvus.log
```

### High memory usage

Milvus indexes consume memory based on data size. Adjust index parameters or use disk-based indexes (IVF) instead of memory-intensive options (HNSW).

### Embedding dimension mismatch

Ensure all embeddings match the schema dimension. Model embeddings must be the same size (e.g., 768 for sentence-transformers).

## Uninstall

```yaml
- preset: milvus
  with:
    state: absent
```

## Resources

- Official docs: https://milvus.io/docs/
- GitHub: https://github.com/milvus-io/milvus
- Python SDK: https://pymilvus.readthedocs.io/
- Search: "milvus vector database tutorial", "milvus RAG implementation", "milvus semantic search"

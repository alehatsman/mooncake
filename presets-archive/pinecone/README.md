# pinecone - Vector Database Client

Python client for Pinecone, a managed vector database for machine learning applications.

## Quick Start
```yaml
- preset: pinecone
```

## Features
- **Vector search**: Fast similarity search at scale
- **Managed service**: No infrastructure to manage
- **Real-time**: Low-latency queries
- **Metadata filtering**: Combine vector and metadata search
- **Cross-platform**: Linux and macOS support

## Basic Usage
```python
import pinecone

# Initialize
pinecone.init(api_key="YOUR_API_KEY", environment="us-west1-gcp")

# Create index
pinecone.create_index("example-index", dimension=128)

# Connect to index
index = pinecone.Index("example-index")

# Upsert vectors
index.upsert(vectors=[
    ("vec1", [0.1] * 128, {"genre": "action"}),
    ("vec2", [0.2] * 128, {"genre": "comedy"})
])

# Query
results = index.query(vector=[0.15] * 128, top_k=10)
```

## Advanced Configuration
```yaml
- preset: pinecone
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove pinecone client |

## Platform Support
- ✅ Linux (pip3)
- ✅ macOS (pip3)
- ❌ Windows (not supported)

## Configuration
- **API Key**: Set via `PINECONE_API_KEY` environment variable
- **Environment**: Set via `PINECONE_ENVIRONMENT` environment variable
- **Client library**: `pinecone-client` Python package

## Real-World Examples

### Semantic Search Setup
```python
import pinecone
from sentence_transformers import SentenceTransformer

# Initialize
pinecone.init(api_key="YOUR_API_KEY", environment="us-west1-gcp")

# Create index for semantic search
pinecone.create_index(
    "semantic-search",
    dimension=384,
    metric="cosine"
)

# Load embedding model
model = SentenceTransformer('all-MiniLM-L6-v2')

# Embed and index documents
documents = ["Hello world", "Machine learning is great"]
embeddings = model.encode(documents)

index = pinecone.Index("semantic-search")
index.upsert(vectors=list(zip(
    [f"doc{i}" for i in range(len(documents))],
    embeddings.tolist(),
    [{"text": doc} for doc in documents]
)))

# Query
query = "AI and ML"
query_embedding = model.encode([query])
results = index.query(
    vector=query_embedding[0].tolist(),
    top_k=5,
    include_metadata=True
)
```

### RAG (Retrieval Augmented Generation)
```yaml
- preset: pinecone

- name: Set Pinecone credentials
  vars:
    set:
      pinecone_api_key: "{{ lookup('env', 'PINECONE_API_KEY') }}"
      pinecone_env: "us-west1-gcp"

- name: Index knowledge base
  shell: python index_documents.py
  env:
    PINECONE_API_KEY: "{{ pinecone_api_key }}"
    PINECONE_ENVIRONMENT: "{{ pinecone_env }}"
```

### CI/CD Integration
```yaml
- preset: pinecone

- name: Run similarity search tests
  shell: pytest tests/test_vector_search.py
  env:
    PINECONE_API_KEY: "{{ pinecone_api_key }}"
```

## Agent Use
- Build semantic search systems for documents
- Create recommendation engines based on embeddings
- Implement RAG systems for LLM applications
- Store and query image/audio embeddings
- Build similarity-based deduplication systems

## Common Operations
```python
# List indexes
pinecone.list_indexes()

# Describe index
pinecone.describe_index("example-index")

# Delete index
pinecone.delete_index("example-index")

# Fetch vectors by ID
index.fetch(ids=["vec1", "vec2"])

# Update vector metadata
index.update(id="vec1", set_metadata={"genre": "thriller"})

# Delete vectors
index.delete(ids=["vec1"])

# Get index stats
index.describe_index_stats()
```

## Troubleshooting

### API key not set
```bash
export PINECONE_API_KEY="your-api-key"
export PINECONE_ENVIRONMENT="us-west1-gcp"
```

### Dimension mismatch
Ensure vectors match index dimension:
```python
# Check index dimension
index_info = pinecone.describe_index("example-index")
print(f"Expected dimension: {index_info.dimension}")
```

### Rate limits
Implement retry logic or upgrade plan:
```python
import time
from pinecone import PineconeException

try:
    index.upsert(vectors=data)
except PineconeException as e:
    if "rate limit" in str(e).lower():
        time.sleep(1)
        index.upsert(vectors=data)
```

## Uninstall
```yaml
- preset: pinecone
  with:
    state: absent
```

## Resources
- Official docs: https://docs.pinecone.io/
- Python client: https://github.com/pinecone-io/pinecone-python-client
- Search: "pinecone vector database", "pinecone python tutorial"

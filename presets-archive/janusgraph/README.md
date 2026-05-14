# JanusGraph - Distributed Graph Database

Scalable graph database optimized for storing and querying large graphs with billions of vertices and edges.

## Quick Start
```yaml
- preset: janusgraph
```

## Features
- **Distributed**: Horizontal scaling across multiple machines
- **Storage backends**: Cassandra, HBase, Berkeley DB support
- **Index backends**: Elasticsearch, Solr, Lucene integration
- **ACID transactions**: Full transactional semantics
- **Gremlin**: Apache TinkerPop graph traversal language
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Start JanusGraph server
janusgraph-server.sh start

# Connect with Gremlin console
gremlin.sh

# Basic graph operations in Gremlin
gremlin> graph = JanusGraphFactory.open('conf/janusgraph-inmemory.properties')
gremlin> g = graph.traversal()

# Add vertices
gremlin> v1 = g.addV('person').property('name', 'Alice').next()
gremlin> v2 = g.addV('person').property('name', 'Bob').next()

# Add edges
gremlin> g.V(v1).addE('knows').to(v2).iterate()

# Query graph
gremlin> g.V().has('name', 'Alice').out('knows').values('name')

# Stop server
janusgraph-server.sh stop
```

## Advanced Configuration
```yaml
- preset: janusgraph
  with:
    version: "1.0.0"           # Specific version
    storage_backend: cassandra  # Storage: berkeleyje, cassandra, hbase
    index_backend: elasticsearch # Index: elasticsearch, solr, lucene
    service: true              # Run as system service
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove JanusGraph |
| version | string | latest | JanusGraph version to install |
| storage_backend | string | berkeleyje | Storage backend (berkeleyje, cassandra, hbase) |
| index_backend | string | lucene | Index backend (elasticsearch, solr, lucene) |
| service | bool | false | Configure as system service |

## Configuration

### Storage Backends

**Berkeley DB (default - embedded):**
```properties
# conf/janusgraph-berkeleyje.properties
storage.backend=berkeleyje
storage.directory=/var/lib/janusgraph/data
```

**Cassandra (distributed):**
```properties
# conf/janusgraph-cassandra.properties
storage.backend=cql
storage.hostname=localhost
storage.port=9042
```

**HBase (distributed):**
```properties
# conf/janusgraph-hbase.properties
storage.backend=hbase
storage.hostname=localhost
storage.port=2181
```

### Index Backends

**Elasticsearch:**
```properties
index.search.backend=elasticsearch
index.search.hostname=localhost
index.search.port=9200
```

**Configuration files:**
- **Server config**: `/opt/janusgraph/conf/gremlin-server/gremlin-server.yaml`
- **Graph config**: `/opt/janusgraph/conf/janusgraph-*.properties`
- **Data directory**: `/var/lib/janusgraph/data`
- **Log files**: `/var/log/janusgraph/`
- **Default port**: 8182 (Gremlin Server WebSocket)

## Real-World Examples

### Social Network Graph
```groovy
// Define schema
mgmt = graph.openManagement()
person = mgmt.makeVertexLabel('person').make()
name = mgmt.makePropertyKey('name').dataType(String.class).make()
age = mgmt.makePropertyKey('age').dataType(Integer.class).make()
knows = mgmt.makeEdgeLabel('knows').make()
mgmt.commit()

// Add data
g.addV('person').property('name', 'Alice').property('age', 30).as('a')
 .addV('person').property('name', 'Bob').property('age', 25).as('b')
 .addE('knows').from('a').to('b').iterate()

// Complex queries
g.V().has('person', 'name', 'Alice')
 .out('knows')
 .out('knows')  // Friends of friends
 .values('name')
```

### Knowledge Graph with Full-Text Search
```groovy
// Configure composite index
mgmt = graph.openManagement()
name = mgmt.getPropertyKey('name')
mgmt.buildIndex('nameIndex', Vertex.class).addKey(name).buildCompositeIndex()
mgmt.commit()

// Mixed index for full-text search
mgmt = graph.openManagement()
content = mgmt.makePropertyKey('content').dataType(String.class).make()
mgmt.buildIndex('contentIndex', Vertex.class)
    .addKey(content, Mapping.TEXT.asParameter())
    .buildMixedIndex("search")
mgmt.commit()

// Query with text search
g.V().has('content', Text.textContains('graph database'))
```

### Time-Series Graph
```groovy
// Add temporal data
g.addV('event')
 .property('type', 'login')
 .property('timestamp', System.currentTimeMillis())
 .property('user', 'alice')
 .iterate()

// Query time range
now = System.currentTimeMillis()
yesterday = now - 86400000
g.V().has('event', 'timestamp', P.between(yesterday, now))
```

## Gremlin Query Examples

```groovy
# Find all vertices
g.V()

# Find vertices by property
g.V().has('name', 'Alice')

# Traversals
g.V().has('name', 'Alice').out('knows')        # Direct connections
g.V().has('name', 'Alice').out().out()         # Two hops
g.V().has('name', 'Alice').in('knows')         # Incoming edges

# Filtering
g.V().has('age', P.gt(25))                     # Age > 25
g.V().has('name', P.within('Alice', 'Bob'))    # Name in list

# Aggregation
g.V().count()                                   # Count vertices
g.V().values('age').mean()                     # Average age
g.V().groupCount().by('type')                  # Group by type

# Path queries
g.V().has('name', 'Alice').repeat(out()).times(2).path()
```

## Performance Tuning

```properties
# Batch loading
storage.batch-loading=true

# Cache configuration
cache.db-cache=true
cache.db-cache-size=0.5

# Transaction settings
graph.set-vertex-id=true
ids.block-size=100000
```

## Agent Use
- Build knowledge graphs from structured data sources
- Social network analysis and recommendation systems
- Fraud detection by analyzing transaction patterns
- Dependency graph analysis for software systems
- Real-time relationship queries at scale
- Time-series event correlation

## Troubleshooting

### Server won't start
Check logs:
```bash
tail -f /var/log/janusgraph/janusgraph.log
```

### Connection refused
Verify server is listening:
```bash
netstat -an | grep 8182
curl http://localhost:8182
```

### Performance issues
Enable metrics:
```properties
metrics.enabled=true
metrics.prefix=janusgraph
```

## Uninstall
```yaml
- preset: janusgraph
  with:
    state: absent
```

**Note:** This removes JanusGraph but preserves data in configured storage backends (Cassandra, HBase, etc.). To remove all data, manually clean storage backends.

## Resources
- Official docs: https://docs.janusgraph.org/
- Gremlin docs: https://tinkerpop.apache.org/docs/current/reference/
- GitHub: https://github.com/JanusGraph/janusgraph
- Search: "janusgraph tutorial", "gremlin graph queries", "janusgraph schema design"

## Platform Support
- ✅ Linux (systemd, script install)
- ✅ macOS (script install)
- ❌ Windows (not yet supported)

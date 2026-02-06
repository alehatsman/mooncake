# couchbase - NoSQL Document Database

Couchbase is a distributed NoSQL cloud database offering memory-first architecture, with built-in caching, full-text search, and mobile sync.

## Quick Start
```yaml
- preset: couchbase
```

## Features
- **Memory-first**: Sub-millisecond data access
- **SQL for JSON**: N1QL query language (SQL for JSON)
- **Full-text search**: Integrated FTS engine
- **Mobile sync**: Couchbase Lite and Sync Gateway
- **Multi-dimensional scaling**: Independent scaling of services
- **ACID transactions**: Cross-document ACID guarantees

## Basic Usage
```bash
# Check cluster status
couchbase-cli server-list -c localhost:8091 -u admin -p password

# Create bucket
couchbase-cli bucket-create -c localhost:8091 \
  -u admin -p password \
  --bucket mybucket \
  --bucket-type couchbase \
  --bucket-ramsize 512

# Query with cbq (N1QL shell)
cbq -u admin -p password -engine=http://localhost:8093
> SELECT * FROM mybucket LIMIT 10;

# Insert document
curl -X POST http://localhost:8093/query/service \
  -u admin:password \
  -d 'statement=INSERT INTO mybucket (KEY, VALUE) VALUES ("doc1", {"name": "test"})'

# Get document via KV
curl http://localhost:8092/mybucket/doc1 \
  -u admin:password
```

## Advanced Configuration
```yaml
- preset: couchbase
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove couchbase |

## Platform Support
- ✅ Linux (deb, rpm packages)
- ✅ macOS (dmg package)
- ❌ Windows (not yet supported)

## Configuration
- **Web UI**: http://localhost:8091 (admin console)
- **Data service**: Port 8092 (KV operations)
- **Query service**: Port 8093 (N1QL)
- **Search service**: Port 8094 (FTS)
- **Data directory**: `/opt/couchbase/var/lib/couchbase/data`
- **Config**: `/opt/couchbase/var/lib/couchbase/config`

## Real-World Examples

### Initialize Cluster
```bash
# Initialize cluster
couchbase-cli cluster-init -c localhost:8091 \
  --cluster-username admin \
  --cluster-password password \
  --services data,index,query,fts \
  --cluster-ramsize 2048 \
  --cluster-index-ramsize 512 \
  --cluster-fts-ramsize 512

# Check initialization
couchbase-cli server-info -c localhost:8091 \
  -u admin -p password
```

### Create Bucket with Replicas
```bash
couchbase-cli bucket-create -c localhost:8091 \
  -u admin -p password \
  --bucket production-data \
  --bucket-type couchbase \
  --bucket-ramsize 2048 \
  --bucket-replica 2 \
  --enable-flush 0 \
  --enable-index-replica 1 \
  --priority high \
  --compression-mode active
```

### N1QL Query Examples
```sql
-- Create primary index
CREATE PRIMARY INDEX ON mybucket;

-- Create secondary index
CREATE INDEX idx_type ON mybucket(type) WHERE type IS NOT NULL;

-- Query with JOIN
SELECT u.name, o.total
FROM users u
JOIN orders o ON KEYS u.order_ids
WHERE u.status = "active";

-- Aggregation
SELECT type, COUNT(*) as count, AVG(price) as avg_price
FROM mybucket
WHERE type IS NOT NULL
GROUP BY type
HAVING COUNT(*) > 10
ORDER BY count DESC;

-- Full-text search
SELECT META().id, name, description
FROM mybucket
WHERE SEARCH(mybucket, "keywords:database AND performance")
LIMIT 20;
```

### Python SDK Usage
```python
from couchbase.cluster import Cluster
from couchbase.auth import PasswordAuthenticator
from couchbase.options import ClusterOptions

# Connect
auth = PasswordAuthenticator("admin", "password")
cluster = Cluster("couchbase://localhost", ClusterOptions(auth))
bucket = cluster.bucket("mybucket")
collection = bucket.default_collection()

# Insert document
collection.upsert("user:1", {
    "name": "John Doe",
    "email": "john@example.com",
    "age": 30,
    "status": "active"
})

# Get document
result = collection.get("user:1")
print(result.content_as[dict])

# Query with N1QL
from couchbase.n1ql import N1QLQuery
query = N1QLQuery("SELECT * FROM mybucket WHERE status = 'active' LIMIT 10")
for row in bucket.n1ql_query(query):
    print(row)

# Subdocument operations
collection.mutate_in("user:1", [
    SD.upsert("status", "inactive"),
    SD.array_append("tags", "premium")
])
```

### Node.js SDK Usage
```javascript
const couchbase = require('couchbase');

async function main() {
  // Connect
  const cluster = await couchbase.connect('couchbase://localhost', {
    username: 'admin',
    password: 'password',
  });

  const bucket = cluster.bucket('mybucket');
  const collection = bucket.defaultCollection();

  // Insert
  await collection.upsert('user:1', {
    name: 'John Doe',
    email: 'john@example.com',
    age: 30
  });

  // Get
  const result = await collection.get('user:1');
  console.log(result.content);

  // Query
  const query = `SELECT * FROM mybucket WHERE age > $age`;
  const result = await cluster.query(query, { parameters: { age: 25 } });
  for await (const row of result.rows) {
    console.log(row);
  }
}
```

### Full-Text Search Index
```bash
# Create FTS index via REST API
curl -X PUT http://localhost:8094/api/index/product-search \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "type": "fulltext-index",
    "name": "product-search",
    "sourceType": "couchbase",
    "sourceName": "mybucket",
    "planParams": {
      "maxPartitionsPerPIndex": 1024
    },
    "params": {
      "mapping": {
        "default_mapping": {
          "enabled": true,
          "dynamic": true
        },
        "types": {
          "product": {
            "enabled": true,
            "properties": {
              "name": {
                "enabled": true,
                "fields": [{"name": "name", "type": "text"}]
              },
              "description": {
                "enabled": true,
                "fields": [{"name": "description", "type": "text"}]
              }
            }
          }
        }
      }
    }
  }'

# Search query
curl -X POST http://localhost:8094/api/index/product-search/query \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "match": "laptop computer",
      "field": "description"
    },
    "size": 10,
    "from": 0
  }'
```

### Backup and Restore
```bash
# Backup cluster
cbbackupmgr config --archive /backup --repo myrepo
cbbackupmgr backup --archive /backup --repo myrepo \
  --cluster http://localhost:8091 \
  --username admin --password password

# Restore
cbbackupmgr restore --archive /backup --repo myrepo \
  --cluster http://localhost:8091 \
  --username admin --password password \
  --bucket-source mybucket --bucket-target mybucket
```

### Rebalance Cluster
```bash
# Add node to cluster
couchbase-cli server-add -c localhost:8091 \
  -u admin -p password \
  --server-add 192.168.1.101:8091 \
  --server-add-username admin \
  --server-add-password password \
  --services data,query

# Rebalance
couchbase-cli rebalance -c localhost:8091 \
  -u admin -p password

# Remove node
couchbase-cli rebalance -c localhost:8091 \
  -u admin -p password \
  --server-remove 192.168.1.101:8091
```

## Agent Use
- Document store for microservices
- Caching layer with persistence
- Real-time analytics with N1QL
- Mobile application backend
- Session store and user profiles
- Full-text search engine

## Troubleshooting

### Node won't join cluster
Check network connectivity:
```bash
# Test connectivity
telnet <node-ip> 8091

# Check firewall rules
# Required ports: 8091-8096, 11210, 11207

# View cluster logs
tail -f /opt/couchbase/var/lib/couchbase/logs/couchbase.log
```

### Query performance issues
Create appropriate indexes:
```sql
-- Check query plan
EXPLAIN SELECT * FROM mybucket WHERE type = "user";

-- Create covering index
CREATE INDEX idx_user_covering ON mybucket(type, name, email)
WHERE type = "user";

-- Monitor slow queries
SELECT * FROM system:completed_requests
WHERE elapsedTime > 1000
ORDER BY elapsedTime DESC;
```

### Memory issues
Adjust bucket quotas:
```bash
# Check memory usage
couchbase-cli server-info -c localhost:8091 \
  -u admin -p password

# Adjust bucket RAM
couchbase-cli bucket-edit -c localhost:8091 \
  -u admin -p password \
  --bucket mybucket \
  --bucket-ramsize 4096
```

## Uninstall
```yaml
- preset: couchbase
  with:
    state: absent
```

## Resources
- Official docs: https://docs.couchbase.com/
- N1QL reference: https://docs.couchbase.com/server/current/n1ql/n1ql-language-reference/
- SDK documentation: https://docs.couchbase.com/home/sdk.html
- Search: "couchbase tutorial", "n1ql query examples"

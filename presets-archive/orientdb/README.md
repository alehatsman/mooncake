# OrientDB - Multi-Model Graph Database

Multi-model database supporting graph, document, key-value, and object models. Combines the flexibility of NoSQL with SQL queries and ACID transactions.

## Quick Start
```yaml
- preset: orientdb
```

## Features
- **Multi-model**: Graph, document, key-value, and object data models
- **SQL compatible**: Familiar SQL syntax with graph extensions
- **ACID transactions**: Full transactional support
- **Distributed**: Multi-master replication and sharding
- **Fast graph traversal**: Optimized for relationship queries
- **Schema-full or schema-less**: Flexible data modeling
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Start OrientDB server
orientdb/bin/server.sh

# Connect with console
orientdb/bin/console.sh

# Create database
CREATE DATABASE plocal:../databases/mydb

# Create vertex class
CREATE CLASS Person EXTENDS V

# Create edge class
CREATE CLASS Friend EXTENDS E

# Insert vertices
INSERT INTO Person SET name = 'Alice'
INSERT INTO Person SET name = 'Bob'

# Create edges
CREATE EDGE Friend FROM (SELECT FROM Person WHERE name = 'Alice')
  TO (SELECT FROM Person WHERE name = 'Bob')

# Traverse graph
SELECT FROM Person WHERE name = 'Alice'
  OUTGOING('Friend')
```

## Advanced Configuration
```yaml
# Install OrientDB (default)
- preset: orientdb

# Uninstall OrientDB
- preset: orientdb
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (tar.gz)
- ✅ macOS (tar.gz)
- ✅ Windows (zip)
- ✅ Docker (official images available)

## Configuration
- **Config file**: `config/orientdb-server-config.xml`
- **Database directory**: `databases/`
- **Backup directory**: `backup/`
- **Default ports**: 2424 (binary), 2480 (HTTP)
- **Log files**: `log/`

## Graph Operations
```sql
-- Create vertex
CREATE VERTEX Person SET name = 'John', age = 30

-- Create edge between vertices
CREATE EDGE WorksWith FROM #12:0 TO #12:1

-- Find vertex by property
SELECT FROM Person WHERE name = 'John'

-- Traverse outgoing edges
SELECT expand(out('WorksWith')) FROM Person WHERE name = 'John'

-- Traverse incoming edges
SELECT expand(in('WorksWith')) FROM Person WHERE name = 'John'

-- Bidirectional traversal
SELECT expand(both('WorksWith')) FROM Person WHERE name = 'John'

-- Multi-level traversal
SELECT FROM Person WHERE name = 'John'
  OUTGOING('WorksWith').OUTGOING('WorksWith')

-- Shortest path
SELECT shortestPath(#12:0, #12:5, 'BOTH')
```

## Document Operations
```sql
-- Create document
INSERT INTO Person CONTENT {
  "name": "Alice",
  "email": "alice@example.com",
  "tags": ["developer", "team-lead"]
}

-- Query documents
SELECT * FROM Person WHERE email LIKE '%@example.com'

-- Update document
UPDATE Person SET role = 'manager' WHERE name = 'Alice'

-- Delete document
DELETE FROM Person WHERE name = 'Alice'

-- Embedded documents
INSERT INTO Company CONTENT {
  "name": "ACME Corp",
  "address": {
    "street": "123 Main St",
    "city": "New York"
  }
}
```

## Indexes
```sql
-- Create index
CREATE INDEX Person.name UNIQUE

-- Create composite index
CREATE INDEX Person.name_email ON Person(name, email) UNIQUE

-- Full-text index
CREATE INDEX Person.description FULLTEXT

-- Spatial index
CREATE INDEX Location.coordinates SPATIAL

-- Show indexes
SELECT FROM (SELECT expand(indexes) FROM metadata:indexmanager)
```

## Classes and Schema
```sql
-- Create class
CREATE CLASS Person EXTENDS V

-- Add property
CREATE PROPERTY Person.name STRING
CREATE PROPERTY Person.age INTEGER

-- Set mandatory property
ALTER PROPERTY Person.name MANDATORY TRUE

-- Set default value
ALTER PROPERTY Person.active DEFAULT TRUE

-- Create link to another class
CREATE PROPERTY Person.company LINK Company

-- Show schema
INFO CLASS Person
```

## Backup and Restore
```bash
# Backup database
orientdb/bin/console.sh "BACKUP DATABASE plocal:../databases/mydb ../backup/mydb.zip"

# Restore database
orientdb/bin/console.sh "RESTORE DATABASE plocal:../databases/mydb ../backup/mydb.zip"

# Export to JSON
orientdb/bin/console.sh "EXPORT DATABASE mydb.json"

# Import from JSON
orientdb/bin/console.sh "IMPORT DATABASE mydb.json"
```

## Real-World Examples

### Social Network
```sql
-- Create schema
CREATE CLASS User EXTENDS V
CREATE CLASS Post EXTENDS V
CREATE CLASS Follows EXTENDS E
CREATE CLASS Likes EXTENDS E

-- Add users
INSERT INTO User SET username = 'alice', email = 'alice@example.com'
INSERT INTO User SET username = 'bob', email = 'bob@example.com'

-- Create friendship
CREATE EDGE Follows FROM (SELECT FROM User WHERE username = 'alice')
  TO (SELECT FROM User WHERE username = 'bob')

-- Create post
INSERT INTO Post SET content = 'Hello world!',
  author = (SELECT FROM User WHERE username = 'alice')

-- Like a post
CREATE EDGE Likes FROM (SELECT FROM User WHERE username = 'bob')
  TO (SELECT FROM Post WHERE content = 'Hello world!')

-- Find friends of friends
SELECT expand(out('Follows').out('Follows'))
FROM User WHERE username = 'alice'

-- Get user's feed
SELECT FROM Post
WHERE author IN (SELECT expand(out('Follows')) FROM User WHERE username = 'alice')
ORDER BY @rid DESC LIMIT 20
```

### Recommendation Engine
```sql
-- Find users who like similar products
SELECT expand(in('Likes').out('Likes')) AS recommended
FROM Product WHERE name = 'Laptop'
AND recommended.@rid NOT IN (
  SELECT expand(out('Likes')) FROM User WHERE username = 'alice'
)
```

### Fraud Detection
```sql
-- Find suspicious transaction patterns
SELECT FROM Transaction
WHERE amount > 10000
AND out('From').out('Transaction').out('To') IN (
  SELECT FROM Account WHERE flagged = true
)
```

## Performance Tuning
```xml
<!-- config/orientdb-server-config.xml -->
<entry name="cache.level1.enabled" value="true"/>
<entry name="cache.level2.enabled" value="true"/>
<entry name="db.mvcc" value="true"/>
<entry name="storage.useWAL" value="true"/>
<entry name="storage.wal.syncOnPageFlush" value="false"/>
```

## Agent Use
- Graph-based recommendation systems
- Social network analysis
- Fraud detection
- Knowledge graphs
- Network topology mapping
- Relationship analytics
- Master data management

## Troubleshooting

### Server won't start
Check port availability:
```bash
netstat -an | grep 2424
netstat -an | grep 2480
```

### Out of memory
Increase heap size in `bin/server.sh`:
```bash
export ORIENTDB_OPTS_MEMORY="-Xms2G -Xmx4G"
```

### Database corruption
Repair database:
```bash
orientdb/bin/console.sh "REPAIR DATABASE plocal:../databases/mydb"
```

### Slow queries
Add indexes and use EXPLAIN:
```sql
EXPLAIN SELECT FROM Person WHERE name = 'Alice'
```

## Uninstall
```yaml
- preset: orientdb
  with:
    state: absent
```

## Resources
- Official docs: https://orientdb.org/docs/
- GitHub: https://github.com/orientechnologies/orientdb
- Community: https://orientdb.org/community/
- Search: "orientdb tutorial", "orientdb graph database", "orientdb getting started"

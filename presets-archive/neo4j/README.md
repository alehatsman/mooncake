# neo4j - Graph Database Management System

neo4j is a native graph database that stores data as nodes and relationships rather than tables. It uses Cypher, an intuitive query language for graphs, enabling powerful queries on highly connected data with lightning-fast performance. Perfect for knowledge graphs, recommendation engines, and relationship analysis.

## Quick Start

```yaml
- preset: neo4j
```

## Features

- **Native Graph Storage**: Store and retrieve connected data at scale
- **Cypher Query Language**: Intuitive syntax designed specifically for graph queries
- **Real-Time Analytics**: Execute complex traversals in milliseconds
- **ACID Transactions**: Guaranteed data consistency and reliability
- **Cross-Platform**: Runs on Linux, macOS, and Windows
- **Browser UI**: Interactive Neo4j Browser for querying and visualization
- **REST API**: HTTP endpoints for programmatic access
- **Property Graph Model**: Store attributes on nodes and relationships

## Basic Usage

```bash
# Check neo4j version
neo4j --version

# Start neo4j service
sudo systemctl start neo4j

# Access Neo4j Browser
# Open http://localhost:7474/ in your web browser

# Query via CLI (if available)
neo4j-shell -u neo4j -p password

# View logs
sudo journalctl -u neo4j -f
```

## Advanced Configuration

```yaml
# Install neo4j with defaults
- preset: neo4j

# Install specific version
- preset: neo4j
  with:
    state: present

# Uninstall neo4j
- preset: neo4j
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove neo4j |

## Platform Support

- ✅ Linux (via package managers: apt, dnf, yum)
- ✅ macOS (Homebrew)
- ✅ Windows (binary installer)

## Configuration

- **Installation directory**: `/usr/bin/neo4j` (Linux), `/usr/local/bin/neo4j` (macOS)
- **Config directory**: `/etc/neo4j/` (Linux), `/usr/local/etc/neo4j/` (macOS)
- **Data directory**: `/var/lib/neo4j/` (Linux)
- **Browser UI**: http://localhost:7474/
- **Bolt protocol**: localhost:7687 (for driver connections)
- **Default credentials**: neo4j / neo4j (must change on first login)

## Real-World Examples

### Check Installation and Version

```bash
# Display neo4j version
neo4j --version

# Check if service is running
sudo systemctl status neo4j

# Verify database is accessible
curl http://localhost:7474/db/
```

### Query Graph Data

```bash
# Example Cypher query (via Neo4j Browser at http://localhost:7474)
# Find all people who work at the same company:
MATCH (p1:Person)-[:WORKS_AT]->(c:Company)<-[:WORKS_AT]-(p2:Person)
WHERE p1 <> p2
RETURN p1.name, c.name, p2.name

# Find shortest path between two nodes:
MATCH path = shortestPath((a:Person {name: 'Alice'})-[*]-(b:Person {name: 'Bob'}))
RETURN path
```

### Programmatic Access with Python

```python
# Install python driver: pip install neo4j
from neo4j import GraphDatabase

driver = GraphDatabase.driver("bolt://localhost:7687", auth=("neo4j", "password"))

with driver.session() as session:
    result = session.run(
        "MATCH (p:Person) WHERE p.age > $age RETURN p.name, p.age",
        age=30
    )
    for record in result:
        print(f"{record['p.name']}: {record['p.age']}")

driver.close()
```

### Bulk Data Import

```bash
# Create CSV file with node data
cat > persons.csv << EOF
name,age,city
Alice,30,NYC
Bob,35,LA
Carol,28,NYC
EOF

# Import into neo4j using Cypher (via Neo4j Browser)
# LOAD CSV WITH HEADERS FROM 'file:///persons.csv' AS row
# CREATE (p:Person {name: row.name, age: toInteger(row.age), city: row.city})

# Or use import tool if available
neo4j-admin import --nodes persons.csv --database neo4j
```

## Agent Use

- Build knowledge graphs from structured data
- Recommendation engine development (e.g., social networks, product suggestions)
- Entity relationship analysis and disambiguation
- Network analysis and bottleneck detection
- Compliance and risk analysis across connected entities
- Master data management and deduplication
- Query validation and performance testing

## Troubleshooting

### Service Won't Start

Check logs for errors:

```bash
# View recent logs
sudo journalctl -u neo4j -n 50

# Check for disk space issues
df -h /var/lib/neo4j/

# Check port availability
sudo netstat -tlnp | grep 7474
```

### Browser Inaccessible

Ensure neo4j is running and listening:

```bash
# Verify service is running
sudo systemctl status neo4j

# Check if port 7474 is listening
sudo ss -tlnp | grep 7474

# Try direct connection
curl -v http://localhost:7474/
```

### Forgot Admin Password

```bash
# Stop neo4j service
sudo systemctl stop neo4j

# Reset password (requires direct database access)
# Contact Neo4j documentation for detailed recovery steps
sudo neo4j-admin set-initial-password newpassword

# Restart service
sudo systemctl start neo4j
```

### Out of Memory

For large graphs, adjust JVM settings:

```bash
# Edit configuration file
sudo nano /etc/neo4j/neo4j.conf

# Find or add memory settings
dbms.memory.heap.initial_size=2G
dbms.memory.heap.max_size=4G
dbms.memory.pagecache.size=2G

# Restart service
sudo systemctl restart neo4j
```

## Uninstall

```yaml
- preset: neo4j
  with:
    state: absent
```

## Resources

- Official documentation: https://neo4j.com/docs/
- Neo4j Browser guide: https://neo4j.com/developer/neo4j-browser/
- Cypher query language: https://neo4j.com/docs/cypher-manual/current/
- Python driver: https://neo4j.com/developer/python/
- Graph academy (tutorials): https://neo4j.com/graphacademy/
- Search: "neo4j tutorial", "cypher query examples", "graph database design"

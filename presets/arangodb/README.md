# ArangoDB - Multi-Model Database

Native multi-model database supporting graphs, documents, key-value, and full-text search in a single engine.

## Quick Start
```yaml
- preset: arangodb
```

## Features
- **Multi-model**: Graphs, documents, key-value in one database
- **AQL query language**: Powerful SQL-like query language
- **Graph algorithms**: Shortest path, traversal, pattern matching
- **Full-text search**: Built-in search capabilities
- **Transactions**: ACID transactions across models
- **Horizontal scaling**: Sharding and replication
- **Foxx microservices**: JavaScript microservices in database

## Basic Usage
```bash
# Connect to database
arangosh --server.endpoint http+tcp://127.0.0.1:8529

# Create database
arangosh --server.database _system --javascript.execute-string "db._createDatabase('mydb')"

# Create collection
db._create('users')

# Insert document
db.users.save({name: 'Alice', age: 30})

# AQL query
db._query('FOR u IN users FILTER u.age > 25 RETURN u')

# Web interface: http://localhost:8529
```

## Advanced Configuration
```yaml
- preset: arangodb
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: arangodb
  with:
    state: absent
```

## Resources
- Search: "arangodb documentation", "arangodb tutorial"

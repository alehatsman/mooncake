# Mongosh - Modern MongoDB Shell

Official MongoDB shell with REPL, autocomplete, syntax highlighting, and MongoDB-specific helpers for interactive queries and database administration.

## Quick Start

```yaml
- preset: mongosh
```

## Features

- **Modern REPL**: Fully interactive shell with multi-line editing and command history
- **MongoDB Helpers**: Built-in functions for common operations (find, insert, update, aggregate)
- **Autocomplete**: Context-aware completion for collections, databases, and methods
- **Syntax Highlighting**: Color-coded output for improved readability
- **Cross-platform**: Works on Linux, macOS, and Windows
- **Native Replacement**: Modern successor to legacy `mongo` shell with improved features

## Basic Usage

```bash
# Interactive mode - connects to local MongoDB
mongosh

# Connect to specific host
mongosh --host mongodb.example.com --port 27017

# Connect with authentication
mongosh --host mongodb.example.com --username admin --password

# Connect using connection string
mongosh "mongodb+srv://user:password@cluster.mongodb.net/database"

# Execute commands and exit
mongosh --eval 'db.admin.ping()'

# Show database list
mongosh --eval 'show databases'

# Pretty-print output
mongosh --eval 'db.collection.find().pretty()'
```

## Advanced Configuration

```yaml
- preset: mongosh
  with:
    state: present
```

Note: Mongosh installation is simple and has no complex parameters. Additional configuration is done within the shell itself or via `.mongorc.js` file.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) mongosh |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ⚠️ Windows (requires MongoDB distribution)

## Configuration

- **Config file**: `~/.mongorc.js` (user startup script)
- **History file**: `~/.mongodb/mongosh/mongosh_history`
- **Default connection**: Connects to `localhost:27017` if no options specified

## Real-World Examples

### Database Administration

```bash
# List all databases
show databases

# Switch to database and check collections
use myapp_db
show collections

# Check database statistics
db.stats()

# View collection structure
db.users.findOne()
```

### Query Operations

```bash
# Find documents
db.users.find({ age: { $gt: 25 } })

# Insert documents
db.users.insertOne({ name: "Alice", age: 28, email: "alice@example.com" })

# Update documents
db.users.updateMany({ status: "inactive" }, { $set: { active: false } })

# Aggregation pipeline
db.orders.aggregate([
  { $match: { status: "shipped" } },
  { $group: { _id: "$customer_id", total: { $sum: "$amount" } } },
  { $sort: { total: -1 } }
])
```

### Backup and Restore

```bash
# Export collection to JSON (use mongosh helpers with shell)
db.collection.find().forEach(doc => print(JSON.stringify(doc)))

# Count documents by status
db.orders.countDocuments({ status: "pending" })

# Create index
db.users.createIndex({ email: 1 }, { unique: true })
```

## Agent Use

- Extract and transform MongoDB data in CI/CD pipelines
- Automated database validation and health checks
- Batch operations on collections (updates, deletions, migrations)
- Schema inspection and validation scripts
- Performance monitoring and slow query analysis
- Data migration between environments
- Automated backup and restore procedures

## Troubleshooting

### Connection refused

Check MongoDB is running and accepting connections:

```bash
# Test connection
mongosh --eval 'db.admin.ping()'

# Check MongoDB service
systemctl status mongod  # Linux
brew services list | grep mongodb  # macOS
```

### Authentication failed

Verify credentials and MongoDB user exists:

```bash
# Connect to admin database first
mongosh --authenticationDatabase admin -u root -p

# Inside mongosh, create user
use myapp_db
db.createUser({ user: "appuser", pwd: "password", roles: ["readWrite"] })
```

### Command not found after installation

Verify installation was successful:

```bash
# Check if in PATH
which mongosh

# Try full path if installed elsewhere
/opt/mongosh/bin/mongosh --version
```

## Uninstall

```yaml
- preset: mongosh
  with:
    state: absent
```

## Resources

- **Official Docs**: https://docs.mongodb.com/mongodb-shell/
- **GitHub**: https://github.com/mongodb-js/mongosh
- **Connection String**: https://docs.mongodb.com/manual/reference/connection-string/
- **Search**: "mongosh tutorial", "mongosh queries", "mongosh connection guide"

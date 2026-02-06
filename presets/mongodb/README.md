# MongoDB - Document-Oriented NoSQL Database with Native Replication

MongoDB is a popular NoSQL database that stores data in flexible JSON-like documents. It supports dynamic schemas, horizontal scaling through sharding, and built-in replication for high availability.

## Quick Start

```yaml
- preset: mongodb
  become: true
```

## Features

- **Flexible Schema**: JSON-like documents allow evolving data structures without migrations
- **Native Replication**: Built-in replica sets for high availability and disaster recovery
- **Horizontal Scaling**: Sharding distributes data across multiple servers
- **Aggregation Pipeline**: Powerful data processing and transformation framework
- **Indexes**: B-tree indexes for fast queries on any field
- **Transactions**: ACID transactions across multiple documents (MongoDB 4.0+)
- **Shell & Tools**: mongosh interactive shell, mongodump/restore, monitoring tools

## Basic Usage

```bash
# Start interactive shell
mongosh

# Inside mongosh:
> use mydb                          # Switch to database
> db.collection.find()              # Query documents
> db.collection.insertOne({name: "Alice", age: 30})  # Insert
> db.collection.updateOne({_id: 1}, {$set: {age: 31}})
> db.collection.deleteOne({_id: 1})

# Command line queries
mongosh --eval "db.version()"
mongosh --db mydb --eval "db.collection.countDocuments()"

# Check service status
systemctl status mongod              # Linux
brew services list | grep mongodb   # macOS

# View server status
mongosh --eval "db.serverStatus()"

# List databases and collections
mongosh --eval "show dbs"
mongosh --eval "show collections"
```

## Advanced Configuration

```yaml
# Default installation
- preset: mongodb
  become: true

# Custom version and port
- preset: mongodb
  with:
    version: "6.0"
    port: "27017"
    bind_ip: "0.0.0.0"
    data_dir: /mnt/data/mongodb
    service: true
  become: true

# Uninstall MongoDB
- preset: mongodb
  with:
    state: absent
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove MongoDB |
| version | string | 7.0 | MongoDB version (7.0, 6.0, 5.0) |
| service | bool | true | Enable and start as system service |
| port | string | 27017 | Server port for connections |
| bind_ip | string | 127.0.0.1 | Bind address (localhost vs network-accessible) |
| data_dir | string | /var/lib/mongodb | Database file storage location |
| log_dir | string | /var/log/mongodb | Log file directory |

## Platform Support

- ✅ Linux (apt, dnf, yum - official MongoDB repositories)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

**Data and Log Locations:**
- **Data Directory**: `/var/lib/mongodb` (created during install)
- **Config File**: `/etc/mongod.conf` (Linux), `/usr/local/etc/mongod.conf` (macOS)
- **Log File**: `/var/log/mongodb/mongod.log`
- **Default Port**: 27017
- **Local Socket**: `/tmp/mongodb-27017.sock`

**User and Permissions:**
- Service runs as `mongodb` user (auto-created)
- Data directory: `mongodb:mongodb` with 0755 permissions
- Log directory: `mongodb:mongodb` with 0755 permissions

**Network Security:**
- Default: Binds to localhost (127.0.0.1) - only local connections
- For remote access: Set `bind_ip: 0.0.0.0` but add firewall rules
- Always use authentication in production (`--auth` flag)

## Real-World Examples

### Development Setup with Local Database

```yaml
- name: Install MongoDB for development
  preset: mongodb
  with:
    version: "7.0"
    bind_ip: "127.0.0.1"
    port: "27017"
    service: true
  become: true

- name: Verify MongoDB is running
  assert:
    http:
      url: http://localhost:27017/serverStatus
      status: 200
```

### Production Cluster Setup

```yaml
# First node in replica set
- name: Install MongoDB server
  preset: mongodb
  with:
    version: "7.0"
    bind_ip: "0.0.0.0"
    data_dir: /mnt/mongodb-data
    service: true
  become: true

# Initialize replica set (run once on first node)
- shell: |
    mongosh --eval "rs.initiate({
      _id: 'rs0',
      members: [
        {_id: 0, host: 'mongo1:27017'},
        {_id: 1, host: 'mongo2:27017'},
        {_id: 2, host: 'mongo3:27017'}
      ]
    })"
```

### Backup and Restore

```bash
# Full database backup
mongodump --out=/backup/mongodb-$(date +%Y%m%d)

# Backup specific database
mongodump --db=mydb --out=/backup/mydb-backup

# Backup with compression
mongodump --out - | gzip > /backup/mongodb-backup.gz

# Restore from backup
mongorestore /backup/mongodb-2024-02-06/

# Restore specific database
mongorestore --nsInclude="mydb.*" /backup/mydb-backup/
```

### Connection Strings

```bash
# Local connection
mongosh mongodb://localhost:27017

# With database
mongosh mongodb://localhost:27017/mydb

# Remote connection
mongosh mongodb://user:password@db.example.com:27017/admin

# Replica set
mongosh "mongodb://mongo1,mongo2,mongo3/mydb?replicaSet=rs0"

# Environment variable
export MONGODB_URI="mongodb://localhost:27017/mydb"
mongosh $MONGODB_URI
```

## Agent Use

- Programmatically create/drop databases and collections
- Insert/query documents via shell or Python MongoClient
- Monitor replica set health and replication lag
- Perform automated backups and restore testing
- Run aggregation pipelines for ETL workflows
- Generate reports from MongoDB collections
- Verify data integrity and indexes after schema changes

## Troubleshooting

### Service won't start

Check logs for errors:

```bash
# Linux - view recent errors
journalctl -u mongod -n 50 -f

# macOS - view service logs
log stream --predicate 'process == "mongod"'

# Check if port is already in use
lsof -i :27017

# Try manual start for debug output
mongod --config /etc/mongod.conf --verbose
```

### Connection refused

Verify MongoDB is listening:

```bash
# Check if mongod process is running
ps aux | grep mongod

# Check if port is listening
netstat -tlnp | grep 27017  # Linux
lsof -i :27017              # macOS

# Verify bind address matches
mongosh --host 127.0.0.1:27017  # Local connection
mongosh --host 0.0.0.0:27017    # Network address
```

### Disk space issues

Monitor data directory size:

```bash
# Check data directory size
du -sh /var/lib/mongodb

# Find largest collections
db.stats()           # In mongosh
db.collection.stats()
```

### Replica set initialization failures

```bash
# Check replica set status
mongosh --eval "rs.status()"

# If replica set not yet initialized
mongosh --eval "rs.initiate()"

# Add member to existing set
mongosh --eval "rs.add('mongo2:27017')"
```

### Authentication issues

```bash
# Create admin user (before enabling auth)
mongosh --eval "
  db.createUser({
    user: 'admin',
    pwd: 'password',
    roles: ['root']
  })
"

# Connect with authentication
mongosh --username admin --password --authenticationDatabase admin
```

## Uninstall

```yaml
- preset: mongodb
  with:
    state: absent
  become: true
```

**Important**: Uninstallation removes MongoDB binaries and systemd service files, but preserves:
- Data directory (`/var/lib/mongodb`)
- Configuration (`/etc/mongod.conf`)
- Log files (`/var/log/mongodb`)

Manually remove these if needed:

```bash
sudo rm -rf /var/lib/mongodb
sudo rm /etc/mongod.conf
sudo rm -rf /var/log/mongodb
```

## Resources

- Official docs: https://docs.mongodb.com/manual/
- GitHub: https://github.com/mongodb/mongo
- mongosh Shell: https://www.mongodb.com/docs/mongodb-shell/
- Replication Guide: https://docs.mongodb.com/manual/replication/
- Sharding Guide: https://docs.mongodb.com/manual/sharding/
- Aggregation Pipeline: https://docs.mongodb.com/manual/aggregation/
- Search: "mongodb tutorial", "mongodb best practices", "mongodb replica set"

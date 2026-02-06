# MongoDB Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Connect to MongoDB
mongosh

# Check status
sudo systemctl status mongod  # Linux
brew services list | grep mongodb  # macOS

# Test connection
mongosh --eval "db.version()"
```

## Configuration

- **Config file:** `/etc/mongod.conf` (Linux), `/usr/local/etc/mongod.conf` (macOS)
- **Data directory:** `/var/lib/mongodb` (default)
- **Log file:** `/var/log/mongodb/mongod.log`
- **Default port:** 27017

## Common Operations

```bash
# Restart MongoDB
sudo systemctl restart mongod  # Linux
brew services restart mongodb-community  # macOS

# Create database and collection
mongosh
> use mydb
> db.createCollection("mycollection")

# Show databases
mongosh --eval "show dbs"

# Backup database
mongodump --db=mydb --out=/backup/

# Restore database
mongorestore --db=mydb /backup/mydb/

# Check server status
mongosh --eval "db.serverStatus()"
```

## Connection String

```
mongodb://localhost:27017/mydb
```

## Uninstall

```yaml
- preset: mongodb
  with:
    state: absent
```

**Note:** Data directory is preserved after uninstall.

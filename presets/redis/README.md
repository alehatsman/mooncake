# Redis Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Connect to Redis
redis-cli

# Ping server
redis-cli ping

# Check status
sudo systemctl status redis  # Linux
brew services list | grep redis  # macOS
```

## Configuration

- **Config file:** `/etc/redis/redis.conf` (Linux), `/usr/local/etc/redis.conf` (macOS)
- **Data directory:** `/var/lib/redis` (Linux), `/usr/local/var/db/redis` (macOS)
- **Default port:** 6379 (as specified during install)
- **Log file:** `/var/log/redis/redis-server.log`

## Common Operations

```bash
# Restart Redis
sudo systemctl restart redis  # Linux
brew services restart redis  # macOS

# Connect to specific host/port
redis-cli -h 127.0.0.1 -p 6379

# Set key
redis-cli SET mykey "Hello World"

# Get key
redis-cli GET mykey

# Check all keys
redis-cli KEYS "*"

# Monitor commands
redis-cli MONITOR

# Get server info
redis-cli INFO

# Flush all data (DANGEROUS!)
redis-cli FLUSHALL
```

## Redis CLI Commands

```
# String operations
SET key value
GET key
DEL key
EXISTS key
EXPIRE key 60

# List operations
LPUSH mylist "item1"
RPUSH mylist "item2"
LRANGE mylist 0 -1

# Hash operations
HSET user:1 name "John"
HGET user:1 name
HGETALL user:1

# Set operations
SADD myset "member1"
SMEMBERS myset

# Get all keys
KEYS *

# Database info
INFO
DBSIZE
```

## Connection String

```
redis://localhost:6379
redis://:password@localhost:6379  # with password
```

## Python Usage

```python
import redis

r = redis.Redis(host='localhost', port=6379, db=0)
r.set('key', 'value')
print(r.get('key'))
```

## Persistence

Redis supports two persistence modes:
- **RDB**: Point-in-time snapshots
- **AOF**: Append-only file logging

Configure in `/etc/redis/redis.conf`

## Uninstall

```yaml
- preset: redis
  with:
    state: absent
```

**Note:** Data files preserved after uninstall.

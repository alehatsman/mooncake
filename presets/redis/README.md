# Redis - In-Memory Data Store

Redis is an in-memory data structure store used as a database, cache, message broker, and streaming engine.

## Quick Start

```yaml
- preset: redis
  become: true
```

## Features

- **In-memory storage**: Extremely fast read/write operations
- **Data structures**: Strings, lists, sets, sorted sets, hashes, streams
- **Persistence**: RDB snapshots and AOF append-only file
- **Replication**: Master-slave replication for high availability
- **Pub/Sub**: Message broker capabilities
- **Lua scripting**: Server-side scripting support

## Basic Usage

```bash
# Connect to Redis
redis-cli

# Ping server
redis-cli ping

# Check status
sudo systemctl status redis  # Linux
brew services list | grep redis  # macOS

# Set and get values
redis-cli SET mykey "Hello"
redis-cli GET mykey

# Monitor commands
redis-cli MONITOR

# Get server info
redis-cli INFO
```

## Advanced Configuration

```yaml
# Basic installation with default settings
- preset: redis
  become: true

# Custom port and memory limit
- preset: redis
  with:
    port: "6380"
    max_memory: "512mb"
    bind_address: "127.0.0.1"
  become: true

# Production setup
- preset: redis
  with:
    port: "6379"
    bind_address: "0.0.0.0"
    max_memory: "2gb"
    start_service: true
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| start_service | bool | true | Start Redis service after installation |
| port | string | 6379 | Redis server port |
| bind_address | string | 127.0.0.1 | Bind address (127.0.0.1 for local, 0.0.0.0 for all) |
| max_memory | string | 256mb | Maximum memory usage |

## Platform Support

- ✅ Linux (systemd, apt, dnf, yum, zypper)
- ✅ macOS (launchd, Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Config file**: `/etc/redis/redis.conf` (Linux), `/usr/local/etc/redis.conf` (macOS)
- **Data directory**: `/var/lib/redis` (Linux), `/usr/local/var/db/redis` (macOS)
- **Log file**: `/var/log/redis/redis-server.log`
- **Default port**: 6379

## Real-World Examples

### Caching Layer

```python
import redis

# Connect to Redis
r = redis.Redis(host='localhost', port=6379, db=0)

# Cache user data
r.setex('user:1000', 3600, '{"name": "John", "email": "john@example.com"}')

# Get cached data
user_data = r.get('user:1000')

# Delete cache
r.delete('user:1000')
```

### Session Store

```yaml
# Deploy Redis for session storage
- preset: redis
  with:
    port: "6379"
    max_memory: "1gb"
    bind_address: "127.0.0.1"
  become: true

# Configure application
- name: Set session config
  shell: |
    export SESSION_STORE=redis
    export REDIS_URL=redis://localhost:6379/0
```

### Message Queue

```python
import redis

r = redis.Redis()

# Producer: Add tasks to queue
r.lpush('tasks', 'process_image_1')
r.lpush('tasks', 'send_email_2')

# Consumer: Process tasks
while True:
    task = r.brpop('tasks', timeout=5)
    if task:
        process_task(task[1])
```

### Pub/Sub Messaging

```python
import redis

# Publisher
r = redis.Redis()
r.publish('notifications', 'New user registered')

# Subscriber
pubsub = r.pubsub()
pubsub.subscribe('notifications')

for message in pubsub.listen():
    if message['type'] == 'message':
        print(f"Received: {message['data']}")
```

### Rate Limiting

```python
import redis
import time

r = redis.Redis()

def rate_limit(user_id, max_requests=10, window=60):
    key = f'rate_limit:{user_id}'
    current = r.incr(key)

    if current == 1:
        r.expire(key, window)

    return current <= max_requests

# Check rate limit
if rate_limit('user123'):
    # Process request
    pass
else:
    # Reject request
    pass
```

## Common Operations

```bash
# String operations
redis-cli SET key value
redis-cli GET key
redis-cli DEL key
redis-cli EXISTS key
redis-cli EXPIRE key 60

# List operations
redis-cli LPUSH mylist "item1"
redis-cli RPUSH mylist "item2"
redis-cli LRANGE mylist 0 -1
redis-cli LPOP mylist

# Hash operations
redis-cli HSET user:1 name "John"
redis-cli HGET user:1 name
redis-cli HGETALL user:1
redis-cli HDEL user:1 name

# Set operations
redis-cli SADD myset "member1"
redis-cli SMEMBERS myset
redis-cli SISMEMBER myset "member1"

# Sorted set operations
redis-cli ZADD leaderboard 100 "player1"
redis-cli ZRANGE leaderboard 0 -1 WITHSCORES
redis-cli ZINCRBY leaderboard 50 "player1"

# Database management
redis-cli SELECT 1
redis-cli KEYS "*"
redis-cli FLUSHDB  # Flush current database
redis-cli FLUSHALL # Flush all databases (DANGEROUS!)
redis-cli DBSIZE   # Number of keys
redis-cli INFO
```

## Agent Use

- Cache frequently accessed data for performance
- Store session data for web applications
- Implement task queues for background job processing
- Build pub/sub systems for real-time notifications
- Rate limiting and throttling for APIs

## Troubleshooting

### Service won't start

```bash
# Check logs
journalctl -u redis -f  # Linux
tail -f /usr/local/var/log/redis.log  # macOS

# Check port conflicts
lsof -i :6379

# Verify configuration
redis-server --test-config
```

### Connection refused

```bash
# Check if Redis is running
sudo systemctl status redis  # Linux
brew services list | grep redis  # macOS

# Start Redis
sudo systemctl start redis  # Linux
brew services start redis  # macOS

# Test connection
redis-cli ping
```

### Out of memory

```bash
# Check memory usage
redis-cli INFO memory

# Set maxmemory policy in redis.conf
maxmemory 512mb
maxmemory-policy allkeys-lru

# Restart Redis
sudo systemctl restart redis
```

### Slow queries

```bash
# Check slow log
redis-cli SLOWLOG GET 10

# Monitor commands
redis-cli MONITOR

# Check latency
redis-cli --latency
redis-cli --latency-history
```

## Uninstall

```yaml
- preset: redis
  with:
    state: absent
  become: true
```

**Note**: Data directory is preserved after uninstall. Remove manually if needed:

```bash
sudo rm -rf /var/lib/redis/  # Linux
rm -rf /usr/local/var/db/redis/  # macOS
```

## Resources

- Official docs: https://redis.io/docs/
- Commands: https://redis.io/commands/
- GitHub: https://github.com/redis/redis
- Search: "redis tutorial", "redis data structures", "redis best practices"

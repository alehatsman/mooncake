# valkey - Redis-Compatible Cache and Storage

Open-source fork of Redis. High-performance in-memory data store with Redis wire protocol compatibility.

## Quick Start
```yaml
- preset: valkey
```

## Features
- **Redis Compatible**: Drop-in replacement for Redis
- **High Performance**: In-memory data structure store
- **Multiple Data Types**: Strings, lists, sets, hashes, sorted sets, streams
- **Persistence Options**: RDB snapshots and AOF logging
- **Replication**: Master-replica replication
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Start server
valkey-server

# Start with config
valkey-server /etc/valkey/valkey.conf

# Start on custom port
valkey-server --port 6380

# Connect with CLI
valkey-cli

# Connect to specific host/port
valkey-cli -h localhost -p 6380

# Run command directly
valkey-cli SET mykey "Hello"
valkey-cli GET mykey
```

## Data Types and Commands

### Strings
```bash
# Set and get
valkey-cli SET key "value"
valkey-cli GET key

# Set with expiration (seconds)
valkey-cli SETEX key 3600 "expires in 1 hour"

# Set if not exists
valkey-cli SETNX key "value"

# Increment/decrement
valkey-cli SET counter 10
valkey-cli INCR counter
valkey-cli DECR counter
```

### Hashes
```bash
# Set hash fields
valkey-cli HSET user:1 name "Alice"
valkey-cli HSET user:1 email "alice@example.com"

# Get hash field
valkey-cli HGET user:1 name

# Get all fields
valkey-cli HGETALL user:1

# Multiple fields
valkey-cli HMSET user:2 name "Bob" email "bob@example.com" age 30
```

### Lists
```bash
# Push to list
valkey-cli LPUSH mylist "item1"
valkey-cli RPUSH mylist "item2"

# Get range
valkey-cli LRANGE mylist 0 -1

# Pop from list
valkey-cli LPOP mylist
valkey-cli RPOP mylist

# List length
valkey-cli LLEN mylist
```

### Sets
```bash
# Add to set
valkey-cli SADD myset "member1" "member2"

# Check membership
valkey-cli SISMEMBER myset "member1"

# Get all members
valkey-cli SMEMBERS myset

# Set operations
valkey-cli SADD set1 "a" "b" "c"
valkey-cli SADD set2 "b" "c" "d"
valkey-cli SINTER set1 set2  # Intersection
valkey-cli SUNION set1 set2  # Union
```

### Sorted Sets
```bash
# Add with score
valkey-cli ZADD leaderboard 100 "player1"
valkey-cli ZADD leaderboard 200 "player2"
valkey-cli ZADD leaderboard 150 "player3"

# Get by rank
valkey-cli ZRANGE leaderboard 0 -1 WITHSCORES

# Get by score
valkey-cli ZRANGEBYSCORE leaderboard 100 200

# Get rank
valkey-cli ZRANK leaderboard "player1"
```

## Real-World Examples

### Session Storage
```bash
# Store session
valkey-cli SETEX session:abc123 3600 '{"user_id": 42, "login_time": "2024-02-06"}'

# Get session
valkey-cli GET session:abc123

# Delete session (logout)
valkey-cli DEL session:abc123
```

### Rate Limiting
```bash
# Track requests per IP
IP="192.168.1.100"
valkey-cli INCR "ratelimit:$IP"
valkey-cli EXPIRE "ratelimit:$IP" 60

# Check limit
COUNT=$(valkey-cli GET "ratelimit:$IP")
if [ "$COUNT" -gt 100 ]; then
  echo "Rate limit exceeded"
fi
```

### Caching
```bash
# Cache API response
valkey-cli SETEX "cache:users:list" 300 '{"users": [...]}'

# Read from cache
CACHED=$(valkey-cli GET "cache:users:list")
if [ -z "$CACHED" ]; then
  echo "Cache miss - fetch from API"
else
  echo "Cache hit - return cached data"
fi
```

### Message Queue
```bash
# Producer: push tasks
valkey-cli LPUSH jobs '{"type": "email", "to": "user@example.com"}'

# Consumer: pop and process
while true; do
  JOB=$(valkey-cli BRPOP jobs 1)
  if [ -n "$JOB" ]; then
    echo "Processing: $JOB"
    # Process job...
  fi
done
```

### Leaderboard
```bash
# Add scores
valkey-cli ZADD game:scores 1000 "player1"
valkey-cli ZADD game:scores 1500 "player2"
valkey-cli ZADD game:scores 800 "player3"

# Top 10
valkey-cli ZREVRANGE game:scores 0 9 WITHSCORES

# Player rank
valkey-cli ZREVRANK game:scores "player1"
```

## Configuration
```conf
# /etc/valkey/valkey.conf

# Bind to all interfaces
bind 0.0.0.0

# Set port
port 6379

# Max memory
maxmemory 256mb
maxmemory-policy allkeys-lru

# Persistence
save 900 1
save 300 10
save 60 10000

# AOF
appendonly yes
appendfsync everysec

# Require password
requirepass yourpassword
```

## Persistence Options
```bash
# RDB Snapshot
valkey-cli SAVE  # Blocking save
valkey-cli BGSAVE  # Background save

# AOF (Append-Only File)
valkey-cli BGREWRITEAOF  # Rewrite AOF

# Check last save time
valkey-cli LASTSAVE

# Disable persistence (for cache-only)
valkey-server --save "" --appendonly no
```

## Monitoring
```bash
# Server info
valkey-cli INFO

# Memory stats
valkey-cli INFO memory

# Replication status
valkey-cli INFO replication

# Connected clients
valkey-cli CLIENT LIST

# Monitor commands in real-time
valkey-cli MONITOR

# Slow log
valkey-cli SLOWLOG GET 10
```

## Advanced Configuration
```yaml
- preset: valkey
  with:
    state: present
    version: latest
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove valkey |
| version | string | latest | Version to install |

## Platform Support
- ✅ Linux (package managers, source)
- ✅ macOS (Homebrew, source)
- ❌ Windows (WSL recommended)

## Performance Tips
- Use pipelining for multiple commands
- Enable AOF only if persistence is critical
- Set maxmemory to prevent OOM
- Use lazy freeing (lazyfree-lazy-eviction yes)
- Monitor slow log regularly

## Security Best Practices
```bash
# Set password
valkey-cli CONFIG SET requirepass "strongpassword"

# Bind to localhost only
valkey-server --bind 127.0.0.1

# Disable dangerous commands
valkey-server --rename-command FLUSHDB "" --rename-command FLUSHALL ""

# Use TLS
valkey-server --tls-port 6380 --tls-cert-file cert.pem --tls-key-file key.pem
```

## Troubleshooting

### Connection refused
```bash
# Check if server is running
valkey-cli PING

# Check port
netstat -tlnp | grep 6379

# Start server
valkey-server
```

### Out of memory
```bash
# Check memory usage
valkey-cli INFO memory

# Set max memory
valkey-cli CONFIG SET maxmemory 256mb

# Set eviction policy
valkey-cli CONFIG SET maxmemory-policy allkeys-lru
```

## Agent Use
- Session management in web applications
- Caching API responses and computed results
- Real-time leaderboards and rankings
- Rate limiting and throttling
- Job queues and background processing
- Pub/sub messaging between services

## Uninstall
```yaml
- preset: valkey
  with:
    state: absent
```

## Resources
- Official site: https://valkey.io/
- GitHub: https://github.com/valkey-io/valkey
- Search: "valkey redis", "valkey configuration", "valkey vs redis"

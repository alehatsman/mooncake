# Dragonfly - Modern Redis Alternative

High-performance in-memory datastore that's a drop-in replacement for Redis. Built from scratch with multi-threading, efficient memory usage, and vertical scaling. Up to 25x faster than Redis on modern hardware while maintaining full API compatibility.

## Quick Start
```yaml
- preset: dragonfly
```

Start server: `dragonfly --port 6379`
Connect: `redis-cli -p 6379` (use standard Redis clients)

Default port: 6379 (Redis compatible)
Default data directory: `/var/lib/dragonfly/`

## Features
- **Redis compatible**: Drop-in replacement for Redis 6.2+ API
- **Multi-threaded**: Utilizes all CPU cores efficiently
- **High performance**: 25x faster throughput than Redis on same hardware
- **Memory efficient**: 30% lower memory footprint with optimized data structures
- **Vertical scaling**: Linear performance scaling with CPU cores
- **Memcached support**: Dual protocol compatibility (Redis + Memcached)
- **Snapshot persistence**: RDB-compatible snapshots
- **Replication**: Master-replica setup with automatic failover
- **TLS support**: Encrypted client connections
- **RESP2/RESP3**: Both Redis protocols supported

## Basic Usage
```bash
# Start Dragonfly server
dragonfly --port 6379

# Start with custom configuration
dragonfly --port 6379 --maxmemory 4gb --dir /var/lib/dragonfly

# Connect with redis-cli
redis-cli -p 6379

# Basic commands (Redis compatible)
redis-cli SET mykey "value"
redis-cli GET mykey
redis-cli INCR counter
redis-cli LPUSH mylist "item1" "item2"
redis-cli HSET user:1 name "John" age 30

# Check server info
redis-cli INFO

# Monitor commands in real-time
redis-cli MONITOR
```

## Architecture

### Design Principles
```
┌────────────────────────────────────────────┐
│         Dragonfly Architecture             │
│                                            │
│  ┌──────────────────────────────────────┐ │
│  │      Multi-threaded Engine           │ │
│  │  ┌────────┐ ┌────────┐ ┌────────┐   │ │
│  │  │Thread 1│ │Thread 2│ │Thread N│   │ │
│  │  └───┬────┘ └───┬────┘ └───┬────┘   │ │
│  │      │          │          │         │ │
│  │  ┌───▼──────────▼──────────▼─────┐  │ │
│  │  │   Shared-Nothing Sharding     │  │ │
│  │  └───────────────────────────────┘  │ │
│  └──────────────────────────────────────┘ │
│                                            │
│  ┌──────────────────────────────────────┐ │
│  │     Protocol Handler (RESP2/3)       │ │
│  └──────────────────────────────────────┘ │
│                                            │
│  ┌──────────────────────────────────────┐ │
│  │    Memory-Efficient Data Store       │ │
│  │  - Compressed strings                │ │
│  │  - Packed lists/sets                 │ │
│  │  - Optimized hash tables             │ │
│  └──────────────────────────────────────┘ │
└────────────────────────────────────────────┘
```

### Key Differences from Redis
- **Multi-threaded**: Dragonfly uses N threads for N cores (Redis is single-threaded)
- **Shared-nothing sharding**: Data automatically partitioned across threads
- **No GIL**: True parallelism for concurrent operations
- **Fiber-based**: Cooperative multitasking within threads
- **Vertical scaling**: Performance scales linearly with CPU cores

## Advanced Configuration

### Production server setup
```yaml
- name: Install Dragonfly
  preset: dragonfly

- name: Create data directory
  file:
    path: /var/lib/dragonfly
    state: directory
    owner: dragonfly
    group: dragonfly
    mode: '0755'
  become: true

- name: Create Dragonfly configuration
  template:
    src: dragonfly.conf.j2
    dest: /etc/dragonfly/dragonfly.conf
    owner: dragonfly
    group: dragonfly
    mode: '0644'
  become: true

- name: Start Dragonfly service
  service:
    name: dragonfly
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=Dragonfly In-Memory Datastore
        After=network.target

        [Service]
        Type=simple
        User=dragonfly
        Group=dragonfly
        ExecStart=/usr/local/bin/dragonfly --flagfile=/etc/dragonfly/dragonfly.conf
        Restart=always
        LimitNOFILE=65535

        [Install]
        WantedBy=multi-user.target
  become: true
```

### Configuration file (dragonfly.conf)
```conf
# Network
port 6379
bind 0.0.0.0
requirepass {{ redis_password }}
maxclients 10000

# Memory
maxmemory 8gb
maxmemory-policy allkeys-lru

# Persistence
dir /var/lib/dragonfly
dbfilename dump.rdb
save 900 1
save 300 10
save 60 10000

# Performance
threads 8
cache-mode true

# Logging
loglevel info
logfile /var/log/dragonfly/dragonfly.log

# TLS (optional)
tls-port 6380
tls-cert-file /etc/dragonfly/cert.pem
tls-key-file /etc/dragonfly/key.pem
tls-ca-cert-file /etc/dragonfly/ca.pem
```

### Memory optimization
```yaml
- name: Configure memory-efficient Dragonfly
  shell: |
    dragonfly \
      --port 6379 \
      --maxmemory 4gb \
      --maxmemory-policy allkeys-lru \
      --cache-mode true \
      --threads $(nproc)
  async: true
```

### Replication setup
```yaml
# Master server
- name: Start Dragonfly master
  shell: |
    dragonfly \
      --port 6379 \
      --dir /var/lib/dragonfly/master \
      --requirepass master_password
  async: true

# Replica server
- name: Start Dragonfly replica
  shell: |
    dragonfly \
      --port 6380 \
      --dir /var/lib/dragonfly/replica \
      --replicaof master.example.com 6379 \
      --masterauth master_password
  async: true
```

## Performance

### Benchmark comparisons
```bash
# Install redis-benchmark (comes with Redis)
apt install redis-tools -y

# Benchmark Dragonfly
redis-benchmark -h localhost -p 6379 -q -t set,get,incr,lpush,rpush,lpop,rpop,sadd,hset,spop,zadd,zpopmin,lrange,mset

# Results (example on 8-core machine):
# SET: 1,200,000 requests/sec
# GET: 1,500,000 requests/sec
# INCR: 1,100,000 requests/sec
# LPUSH: 1,000,000 requests/sec

# Compare to Redis on same hardware:
# SET: 80,000 requests/sec
# GET: 100,000 requests/sec
```

### Performance tuning
```bash
# Maximum performance (use all cores)
dragonfly --threads $(nproc) --cache-mode true

# Balanced (reserve cores for OS)
dragonfly --threads $(($(nproc) - 2))

# Memory-constrained
dragonfly --maxmemory 2gb --maxmemory-policy allkeys-lru

# High-concurrency
ulimit -n 65535
dragonfly --maxclients 10000
```

## Redis Command Compatibility

### Fully supported commands
```bash
# Strings
SET, GET, INCR, DECR, INCRBY, DECRBY, APPEND, STRLEN, SETEX, SETNX, MGET, MSET

# Lists
LPUSH, RPUSH, LPOP, RPOP, LLEN, LRANGE, LINDEX, LSET, LTRIM

# Sets
SADD, SREM, SMEMBERS, SISMEMBER, SCARD, SUNION, SINTER, SDIFF

# Sorted Sets
ZADD, ZREM, ZRANGE, ZREVRANGE, ZRANGEBYSCORE, ZRANK, ZCARD, ZINCRBY

# Hashes
HSET, HGET, HMSET, HMGET, HGETALL, HDEL, HLEN, HINCRBY

# Keys
DEL, EXISTS, EXPIRE, TTL, KEYS, SCAN, TYPE, RENAME

# Transactions
MULTI, EXEC, DISCARD, WATCH, UNWATCH

# Pub/Sub
PUBLISH, SUBSCRIBE, PSUBSCRIBE, UNSUBSCRIBE

# Server
PING, INFO, DBSIZE, FLUSHDB, FLUSHALL, SAVE, BGSAVE, SHUTDOWN
```

### Limited/unsupported features
- Lua scripting (EVAL/EVALSHA) - not yet supported
- Cluster mode - in development
- Streams (XADD, XREAD) - partial support
- Modules - not supported (Redis modules incompatible)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Dragonfly |

## Platform Support
- ✅ Linux (all distributions) - native binary
- ✅ macOS (Homebrew)
- ✅ Docker (official images)
- ❌ Windows (use WSL2 or Docker)

## Configuration Options

### Command-line flags
```bash
# Network
--port 6379                    # Port to listen on
--bind 0.0.0.0                 # Bind address
--requirepass password         # Authentication password
--tls-port 6380               # TLS port

# Memory
--maxmemory 8gb               # Maximum memory limit
--maxmemory-policy allkeys-lru # Eviction policy
--cache-mode true             # Enable cache mode (more aggressive eviction)

# Persistence
--dir /var/lib/dragonfly      # Data directory
--dbfilename dump.rdb         # Snapshot filename
--save "900 1 300 10 60 10000" # Save snapshots

# Performance
--threads 8                   # Number of threads (default: auto)
--hz 10                       # Server cron frequency

# Logging
--loglevel info               # Log level (debug, info, warning, error)
--logfile /var/log/dragonfly.log
```

### Eviction policies
- `noeviction` - Return error when memory limit reached
- `allkeys-lru` - Evict least recently used keys (recommended)
- `allkeys-lfu` - Evict least frequently used keys
- `allkeys-random` - Evict random keys
- `volatile-lru` - Evict LRU keys with TTL set
- `volatile-lfu` - Evict LFU keys with TTL set
- `volatile-random` - Evict random keys with TTL set
- `volatile-ttl` - Evict keys with shortest TTL

## Use Cases

### Drop-in Redis replacement
```yaml
- name: Install Dragonfly
  preset: dragonfly

- name: Stop Redis
  service:
    name: redis
    state: stopped

- name: Migrate Redis data
  shell: |
    redis-cli --rdb /var/lib/redis/dump.rdb
    dragonfly --port 6379 --dir /var/lib/redis

- name: Update application config
  template:
    src: app-config.yml.j2
    dest: /etc/myapp/config.yml
  vars:
    redis_host: localhost
    redis_port: 6379

# No application code changes needed!
```

### High-performance cache layer
```yaml
- name: Install Dragonfly
  preset: dragonfly

- name: Configure cache-mode
  shell: |
    dragonfly \
      --port 6379 \
      --maxmemory 16gb \
      --maxmemory-policy allkeys-lru \
      --cache-mode true \
      --threads $(nproc)
  async: true

- name: Configure application
  template:
    src: cache-config.j2
    dest: /etc/myapp/cache.yml
  vars:
    cache_backend: dragonfly
    cache_host: localhost:6379
    cache_ttl: 3600
```

### Session storage
```yaml
- name: Deploy Dragonfly for sessions
  preset: dragonfly

- name: Start with persistence
  shell: |
    dragonfly \
      --port 6379 \
      --dir /var/lib/dragonfly/sessions \
      --save "60 1" \
      --requirepass {{ session_password }}
  async: true

- name: Configure web application
  template:
    src: session-config.j2
    dest: /etc/webapp/sessions.yml
  vars:
    session_store: dragonfly
    session_host: localhost:6379
    session_password: "{{ session_password }}"
```

## Client Integration

### Python (redis-py)
```python
import redis

# Standard connection
r = redis.Redis(host='localhost', port=6379, decode_responses=True)

# With authentication
r = redis.Redis(host='localhost', port=6379, password='secret', decode_responses=True)

# Connection pool
pool = redis.ConnectionPool(host='localhost', port=6379, max_connections=50)
r = redis.Redis(connection_pool=pool)

# Usage (no code changes from Redis)
r.set('key', 'value')
print(r.get('key'))
r.incr('counter')
r.lpush('queue', 'task1', 'task2')
```

### Node.js (ioredis)
```javascript
const Redis = require('ioredis');

// Standard connection
const redis = new Redis({
  host: 'localhost',
  port: 6379,
});

// With authentication
const redis = new Redis({
  host: 'localhost',
  port: 6379,
  password: 'secret',
});

// Usage (identical to Redis)
await redis.set('key', 'value');
const value = await redis.get('key');
await redis.incr('counter');
await redis.lpush('queue', 'task1', 'task2');
```

### Go (go-redis)
```go
package main

import (
    "context"
    "github.com/redis/go-redis/v9"
)

func main() {
    ctx := context.Background()

    // Standard connection
    rdb := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    })

    // Usage (identical to Redis)
    err := rdb.Set(ctx, "key", "value", 0).Err()
    val, err := rdb.Get(ctx, "key").Result()
    err = rdb.Incr(ctx, "counter").Err()
    err = rdb.LPush(ctx, "queue", "task1", "task2").Err()
}
```

## Migration from Redis

### Data migration
```yaml
- name: Backup Redis data
  shell: redis-cli --rdb /tmp/redis-backup.rdb

- name: Stop Redis
  service:
    name: redis
    state: stopped

- name: Install Dragonfly
  preset: dragonfly

- name: Start Dragonfly with Redis data
  shell: |
    # Copy Redis RDB file
    cp /var/lib/redis/dump.rdb /var/lib/dragonfly/

    # Start Dragonfly (loads RDB automatically)
    dragonfly --port 6379 --dir /var/lib/dragonfly
  async: true

- name: Verify data migration
  shell: |
    # Check key count
    redis-cli DBSIZE

    # Sample random keys
    redis-cli RANDOMKEY
```

### Zero-downtime migration
```yaml
# Phase 1: Dual-write setup
- name: Configure application for dual-write
  template:
    src: dual-write-config.j2
    dest: /etc/myapp/config.yml
  vars:
    primary_cache: redis
    secondary_cache: dragonfly

# Phase 2: Data sync
- name: Sync Redis to Dragonfly
  shell: |
    redis-cli --rdb /tmp/dump.rdb
    dragonfly --port 6380 --dir /var/lib/dragonfly

# Phase 3: Cutover
- name: Switch primary to Dragonfly
  template:
    src: app-config.j2
    dest: /etc/myapp/config.yml
  vars:
    primary_cache: dragonfly
```

## Monitoring

### Health checks
```bash
# Check if server is responsive
redis-cli PING
# Expected: PONG

# Server information
redis-cli INFO

# Memory usage
redis-cli INFO memory

# Stats
redis-cli INFO stats

# Clients
redis-cli CLIENT LIST
```

### Metrics endpoints
```yaml
- name: Configure Prometheus metrics
  shell: dragonfly --port 6379 --metrics-port 6378

- name: Scrape metrics
  shell: curl http://localhost:6378/metrics

# Key metrics:
# - dragonfly_uptime_seconds
# - dragonfly_used_memory_bytes
# - dragonfly_connected_clients
# - dragonfly_commands_processed_total
# - dragonfly_keyspace_hits_total
# - dragonfly_keyspace_misses_total
```

### Performance monitoring
```bash
# Real-time command monitoring
redis-cli MONITOR

# Slow log
redis-cli SLOWLOG GET 10

# Latency histogram
redis-cli --latency-hist

# Throughput test
redis-benchmark -h localhost -p 6379 -q
```

## CLI Commands

### Server management
```bash
# Server info
redis-cli INFO
redis-cli INFO server
redis-cli INFO memory
redis-cli INFO stats

# Configuration
redis-cli CONFIG GET maxmemory
redis-cli CONFIG SET maxmemory 4gb

# Persistence
redis-cli SAVE        # Synchronous save
redis-cli BGSAVE      # Background save
redis-cli LASTSAVE    # Last save timestamp

# Client management
redis-cli CLIENT LIST
redis-cli CLIENT KILL <ip:port>
```

### Data operations
```bash
# Database
redis-cli DBSIZE      # Number of keys
redis-cli KEYS "*"    # List all keys (use SCAN in production)
redis-cli SCAN 0      # Iterate keys safely
redis-cli FLUSHDB     # Clear current database
redis-cli FLUSHALL    # Clear all databases

# Key inspection
redis-cli TYPE mykey
redis-cli TTL mykey
redis-cli PTTL mykey  # Milliseconds
redis-cli MEMORY USAGE mykey
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Dragonfly
  preset: dragonfly
```

### Development setup
```yaml
- name: Install Dragonfly
  preset: dragonfly

- name: Start development server
  shell: dragonfly --port 6379 --loglevel debug
  async: true

- name: Wait for server
  shell: |
    for i in {1..30}; do
      redis-cli PING && break
      sleep 1
    done
```

### Production deployment
```yaml
- name: Install Dragonfly
  preset: dragonfly

- name: Create dragonfly user
  shell: useradd -r -s /bin/false dragonfly
  become: true

- name: Create directories
  file:
    path: "{{ item }}"
    state: directory
    owner: dragonfly
    group: dragonfly
    mode: '0755'
  loop:
    - /var/lib/dragonfly
    - /var/log/dragonfly
    - /etc/dragonfly
  become: true

- name: Deploy configuration
  template:
    src: dragonfly.conf.j2
    dest: /etc/dragonfly/dragonfly.conf
  become: true

- name: Start Dragonfly service
  service:
    name: dragonfly
    state: started
    enabled: true
  become: true
```

## Agent Use
- **Redis replacement**: Drop-in replacement for Redis with better performance
- **High-performance cache**: Web application caching with multi-core scaling
- **Session storage**: User session persistence with replication
- **Real-time analytics**: Fast counters, leaderboards, time-series data
- **Message queues**: List-based job queues with higher throughput
- **Rate limiting**: Token bucket implementation with atomic operations
- **Pub/Sub**: Real-time messaging between services

## Troubleshooting

### Connection refused
```bash
# Check if Dragonfly is running
ps aux | grep dragonfly
pgrep dragonfly

# Check port binding
netstat -tuln | grep 6379
lsof -i :6379

# Test connection
redis-cli -h localhost -p 6379 PING

# Check logs
tail -f /var/log/dragonfly/dragonfly.log
journalctl -u dragonfly -f
```

### Out of memory
```bash
# Check memory usage
redis-cli INFO memory

# Check maxmemory setting
redis-cli CONFIG GET maxmemory

# Increase memory limit
dragonfly --maxmemory 8gb

# Enable eviction
dragonfly --maxmemory 8gb --maxmemory-policy allkeys-lru

# Clear database if needed
redis-cli FLUSHALL
```

### Performance issues
```bash
# Check CPU usage
top -p $(pgrep dragonfly)

# Increase threads (should match CPU cores)
dragonfly --threads $(nproc)

# Check slow commands
redis-cli SLOWLOG GET 10

# Monitor commands
redis-cli MONITOR

# Run benchmark
redis-benchmark -h localhost -p 6379 -q -t set,get
```

### Persistence failures
```bash
# Check disk space
df -h /var/lib/dragonfly

# Check permissions
ls -la /var/lib/dragonfly

# Verify save configuration
redis-cli CONFIG GET save

# Manual snapshot
redis-cli BGSAVE

# Check last save time
redis-cli LASTSAVE
```

## Best Practices

1. **Use all CPU cores**: Set `--threads $(nproc)` for maximum performance
2. **Configure memory limits**: Set `--maxmemory` to prevent OOM issues
3. **Enable persistence**: Use `--save` flags for data durability
4. **Set password**: Always use `--requirepass` in production
5. **Monitor memory usage**: Track with `INFO memory` and set up alerts
6. **Use connection pooling**: Reuse connections in client applications
7. **Enable TLS**: Use `--tls-port` and certificates for encrypted connections
8. **Set file descriptor limits**: `ulimit -n 65535` for high-concurrency workloads
9. **Use cache mode**: Enable `--cache-mode` for pure cache workloads
10. **Regular snapshots**: Configure automatic snapshots for data safety

## Uninstall
```yaml
- name: Stop Dragonfly
  service:
    name: dragonfly
    state: stopped

- name: Remove Dragonfly
  preset: dragonfly
  with:
    state: absent

- name: Remove data
  file:
    path: /var/lib/dragonfly
    state: absent
  become: true
```

**Note**: Uninstalling does not remove data directory. Delete `/var/lib/dragonfly` manually if needed.

## Resources
- Official: https://www.dragonflydb.io/
- GitHub: https://github.com/dragonflydb/dragonfly
- Documentation: https://www.dragonflydb.io/docs
- Discord: https://discord.gg/HsPjXGVH85
- Blog: https://www.dragonflydb.io/blog
- Benchmarks: https://www.dragonflydb.io/blog/dragonflydb-vs-redis-performance
- Docker: https://hub.docker.com/r/docker/dragonflydb
- Search: "dragonfly vs redis", "dragonfly performance benchmarks", "dragonfly migration"

# Garnet - High-Performance Cache Store

Distributed cache server built on .NET with Redis protocol compatibility. Microsoft's fast, scalable alternative to Redis with better performance.

## Quick Start
```yaml
- preset: garnet
```

## Features
- **Redis compatible**: Drop-in replacement with Redis protocol support
- **High performance**: 10x throughput improvement over Redis in many scenarios
- **.NET native**: Built on .NET for modern cloud workloads
- **Memory efficient**: Advanced memory management and compression
- **Cluster mode**: Built-in clustering and replication
- **Persistence**: RDB and AOF persistence options

## Basic Usage
```bash
# Start Garnet server
garnet

# Start with custom port
garnet --port 6380

# Start with persistence
garnet --checkpoint-dir /data/checkpoints --log-dir /data/logs

# Connect with redis-cli
redis-cli -p 6379

# Basic operations
redis-cli SET mykey "Hello Garnet"
redis-cli GET mykey
redis-cli DEL mykey
```

## Advanced Configuration
```yaml
- preset: garnet
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Garnet |

## Platform Support
- ✅ Linux (binary download, .NET runtime)
- ✅ macOS (binary download, .NET runtime)
- ✅ Windows (binary download, .NET runtime)

## Configuration
- **Default port**: 6379 (Redis compatible)
- **Config file**: `garnet.conf` (optional)
- **Data directory**: `./data` (default)
- **Memory**: Auto-configured based on system RAM

## Real-World Examples

### Caching Layer
```bash
# Start Garnet as cache
garnet --port 6379 --memory 4gb --eviction-policy lru

# Use from application (Python example)
import redis

client = redis.Redis(host='localhost', port=6379)
client.set('user:1000', '{"name":"Alice","email":"alice@example.com"}')
user = client.get('user:1000')
```

### Session Store
```csharp
// ASP.NET Core session configuration
services.AddStackExchangeRedisCache(options =>
{
    options.Configuration = "localhost:6379";
    options.InstanceName = "SessionStore_";
});

// Store session data
HttpContext.Session.SetString("UserId", "12345");
var userId = HttpContext.Session.GetString("UserId");
```

### Distributed Lock
```python
from redis import Redis
from redis.lock import Lock

client = Redis(host='localhost', port=6379)

# Acquire distributed lock
lock = Lock(client, "resource_lock", timeout=10)
if lock.acquire(blocking=True):
    try:
        # Critical section
        print("Lock acquired, performing operation")
    finally:
        lock.release()
```

### Pub/Sub Messaging
```bash
# Terminal 1: Subscribe to channel
redis-cli SUBSCRIBE notifications

# Terminal 2: Publish messages
redis-cli PUBLISH notifications "New user registered"
redis-cli PUBLISH notifications "Order placed"
```

### Rate Limiting
```python
import redis
import time

client = redis.Redis(host='localhost', port=6379)

def rate_limit(user_id, max_requests=10, window=60):
    key = f"rate_limit:{user_id}"
    pipe = client.pipeline()

    # Increment counter
    pipe.incr(key)
    pipe.expire(key, window)
    count, _ = pipe.execute()

    return count[0] <= max_requests

# Check rate limit
if rate_limit("user123"):
    print("Request allowed")
else:
    print("Rate limit exceeded")
```

## Agent Use
- Implement distributed caching for microservices
- Build session stores for web applications
- Create message queues with pub/sub
- Implement distributed locks for coordination
- Build rate limiting systems
- Cache API responses and database queries

## Troubleshooting

### Server won't start
```bash
# Check if port is already in use
lsof -i :6379
netstat -an | grep 6379

# Start on different port
garnet --port 6380

# Check .NET runtime
dotnet --version

# View logs
garnet --log-level Debug
```

### Connection refused
```bash
# Check if server is running
ps aux | grep garnet

# Test connection
redis-cli ping
# Expected: PONG

# Check firewall
sudo ufw allow 6379/tcp  # Linux
```

### Memory issues
```bash
# Monitor memory usage
redis-cli INFO memory

# Set memory limit
garnet --memory 2gb --eviction-policy allkeys-lru

# View current config
redis-cli CONFIG GET maxmemory
redis-cli CONFIG GET maxmemory-policy
```

### Persistence errors
```bash
# Check checkpoint directory permissions
ls -la /data/checkpoints

# Manual checkpoint
redis-cli SAVE

# Check last save time
redis-cli LASTSAVE

# View persistence info
redis-cli INFO persistence
```

## Uninstall
```yaml
- preset: garnet
  with:
    state: absent
```

## Resources
- Official docs: https://microsoft.github.io/garnet/
- GitHub: https://github.com/microsoft/garnet
- Benchmarks: https://microsoft.github.io/garnet/docs/benchmarking
- Search: "garnet cache", "garnet vs redis", "microsoft garnet"

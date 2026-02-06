# dragonfly - Modern Redis Alternative

High-performance in-memory datastore compatible with Redis and Memcached APIs.

## Quick Start
```yaml
- preset: dragonfly
```

## Features
- **Redis compatible**: Drop-in replacement for Redis
- **High performance**: Up to 25x faster than Redis
- **Multi-threaded**: Efficient use of modern CPUs
- **Memory efficient**: Lower memory footprint
- **Vertical scaling**: Scales with CPU cores
- **Memcached support**: Dual API compatibility

## Basic Usage
```bash
# Start dragonfly
dragonfly --port 6379

# Connect with redis-cli
redis-cli -p 6379

# Set and get values
redis-cli -p 6379 SET mykey "value"
redis-cli -p 6379 GET mykey

# Use with existing Redis clients
# Python: redis-py, Node.js: ioredis, Go: go-redis
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### As Redis Replacement
```python
# Python - no code changes needed
import redis
r = redis.Redis(host='localhost', port=6379)
r.set('key', 'value')
print(r.get('key'))
```

### High-Performance Cache
```bash
# Start with custom memory limit
dragonfly --port 6379 --maxmemory 4gb --maxmemory-policy allkeys-lru
```

## Agent Use
- Drop-in Redis replacement for higher performance
- Cache layer for applications
- Session storage
- Real-time analytics
- Message queues
- Rate limiting

## Uninstall
```yaml
- preset: dragonfly
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/dragonflydb/dragonfly
- Search: "dragonfly vs redis", "dragonfly performance"

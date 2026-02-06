# Memcached Preset

Install Memcached - a high-performance distributed memory object caching system.

## Quick Start

```yaml
- preset: memcached
  with:
    memory_limit: "128"
    max_connections: "2048"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `start_service` | bool | `true` | Start service after install |
| `port` | string | `11211` | Memcached port |
| `memory_limit` | string | `64` | Memory limit (MB) |
| `max_connections` | string | `1024` | Max connections |

## Usage

### Basic Installation
```yaml
- preset: memcached
```

### Production Setup
```yaml
- preset: memcached
  with:
    memory_limit: "512"
    max_connections: "4096"
```

## Verify Installation

```bash
# Check status
sudo systemctl status memcached  # Linux
brew services list | grep memcached  # macOS

# Test connection
echo "stats" | nc localhost 11211

# Or with telnet
telnet localhost 11211
stats
quit
```

## Common Operations

```bash
# Stats
echo "stats" | nc localhost 11211

# Get value
echo "get mykey" | nc localhost 11211

# Set value
echo "set mykey 0 0 5\r\nhello" | nc localhost 11211

# Delete value
echo "delete mykey" | nc localhost 11211

# Flush all
echo "flush_all" | nc localhost 11211
```

## Python Client

```python
import memcache

mc = memcache.Client(['127.0.0.1:11211'])

# Set value
mc.set("key", "value")

# Get value
value = mc.get("key")

# Delete
mc.delete("key")

# Increment
mc.incr("counter")

# Decrement
mc.decr("counter")
```

## Node.js Client

```javascript
const Memcached = require('memcached');
const memcached = new Memcached('localhost:11211');

// Set value
memcached.set('key', 'value', 10, (err) => {
  if (err) console.error(err);
});

// Get value
memcached.get('key', (err, data) => {
  console.log(data);
});

// Delete
memcached.del('key', (err) => {
  if (err) console.error(err);
});
```

## Configuration

- **Linux**: `/etc/memcached.conf`
- **macOS**: No config file by default, use command-line args

## Monitoring

```bash
# Memory stats
echo "stats slabs" | nc localhost 11211

# Item stats
echo "stats items" | nc localhost 11211

# Connection stats
echo "stats conns" | nc localhost 11211

# Settings
echo "stats settings" | nc localhost 11211
```

## Best Practices

1. **Memory sizing**: Set to available RAM minus OS/app needs
2. **Connection limit**: Based on expected concurrent clients
3. **Key naming**: Use namespacing (e.g., `app:user:123`)
4. **TTL**: Always set expiration times
5. **Monitoring**: Track hit/miss ratio

## Uninstall

```yaml
- preset: memcached
  with:
    state: absent
```

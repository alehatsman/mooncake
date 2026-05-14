# Memcached - High-Performance Distributed Memory Caching

Memcached is a free, open-source, high-performance distributed memory caching system that speeds up dynamic applications by reducing database load.

## Quick Start

```yaml
- preset: memcached
```

## Features

- **Distributed caching**: Cache across multiple servers transparently
- **High-performance**: Microsecond-level operation latency, scales to petabyte-sized datasets
- **Simple protocol**: Human-readable ASCII protocol over TCP/IP (memcache text protocol)
- **LRU eviction**: Automatic memory management with least-recently-used eviction policy
- **Cross-platform**: Linux, macOS (via Homebrew)
- **Multi-language support**: Python, Node.js, Ruby, Go, Java, C/C++, and more

## Basic Usage

```bash
# Check status
sudo systemctl status memcached  # Linux
brew services list | grep memcached  # macOS

# Connect with telnet
telnet localhost 11211

# Connect with netcat
echo "stats" | nc localhost 11211

# Get server statistics
echo "stats" | nc localhost 11211

# Get memory stats
echo "stats slabs" | nc localhost 11211

# Get cache statistics
echo "stats items" | nc localhost 11211

# Flush all data
echo "flush_all" | nc localhost 11211
```

## Advanced Configuration

```yaml
# Custom memory and connection limits
- preset: memcached
  with:
    state: present
    port: "11211"
    memory_limit: "512"
    max_connections: "4096"
    start_service: true
  become: true

# Production configuration with increased resources
- preset: memcached
  with:
    memory_limit: "2048"
    max_connections: "8192"
    port: "11211"
    start_service: true
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | Whether Memcached should be installed (`present`) or removed (`absent`) |
| `start_service` | bool | `true` | Start Memcached service after installation |
| `port` | string | `11211` | TCP port for Memcached to listen on |
| `memory_limit` | string | `64` | Maximum memory to use for object storage in MB |
| `max_connections` | string | `1024` | Maximum number of simultaneous connections |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration

- **Linux config file**: `/etc/memcached.conf`
- **macOS config file**: No default config file (use command-line arguments)
- **Default port**: 11211
- **Data storage**: In-memory only (no persistence)

## Real-World Examples

### Session Caching

Cache user sessions to reduce database queries in a web application:

```bash
# In your application
echo "set session:user:123 0 3600 50
{\"user_id\": 123, \"name\": \"John\"}" | nc localhost 11211

# Retrieve session
echo "get session:user:123" | nc localhost 11211
```

### Database Query Result Caching

Cache frequently-accessed database results with expiration:

```bash
# Cache product data for 1 hour (3600 seconds)
echo "set products:all 0 3600 100
[product data here]" | nc localhost 11211

# Invalidate cache
echo "delete products:all" | nc localhost 11211
```

### Rate Limiting Counter

Use Memcached to track API call rates:

```python
import memcache

mc = memcache.Client(['127.0.0.1:11211'])

# Increment request counter
counter = mc.incr("api:requests:user:123")

# Set expiration to reset daily
if counter == 1:
    mc.expire("api:requests:user:123", 86400)
```

### Node.js Configuration Service

Cache configuration objects for Node.js microservices:

```javascript
const Memcached = require('memcached');
const mc = new Memcached('localhost:11211');

// Cache service configuration
mc.set('service:config:api', JSON.stringify({
  timeout: 5000,
  retries: 3,
  debug: false
}), 3600, (err) => {
  if (err) console.error(err);
});

// Retrieve configuration
mc.get('service:config:api', (err, data) => {
  if (data) console.log(JSON.parse(data));
});
```

## Agent Use

- Cache API responses to reduce downstream service load
- Store intermediate computation results in distributed workflows
- Implement request deduplication and rate limiting
- Cache ML model inference results and embeddings
- Distribute temporary data between parallel processing workers
- Optimize repeated queries in data pipelines

## Troubleshooting

### Service won't start

Check logs for permission or port binding issues:

```bash
# Linux - check systemd logs
journalctl -u memcached -f

# macOS - check Homebrew service logs
tail -f ~/Library/Logs/memcached.log
```

### Port already in use

If port 11211 is already in use, verify another service isn't running:

```bash
# Check what's using the port
lsof -i :11211

# Use a different port
- preset: memcached
  with:
    port: "11212"
  become: true
```

### Connection refused

Ensure Memcached is running and listening on the correct port:

```bash
# Check if service is active
sudo systemctl is-active memcached  # Linux
brew services list | grep memcached  # macOS

# Test connectivity
echo "stats" | nc -w 1 localhost 11211
```

### Memory limits reached

Monitor memory usage and adjust `memory_limit` parameter:

```bash
# Check memory statistics
echo "stats" | nc localhost 11211 | grep bytes_

# Increase memory
- preset: memcached
  with:
    memory_limit: "1024"
  become: true
```

### Eviction rate too high

If items are being evicted frequently, increase memory:

```bash
# Check eviction stats
echo "stats" | nc localhost 11211 | grep evictions
```

## Uninstall

```yaml
- preset: memcached
  with:
    state: absent
  become: true
```

## Resources

- Official docs: https://memcached.org/
- Protocol documentation: https://github.com/memcached/memcached/blob/master/doc/protocol.txt
- Client libraries: https://memcached.org/clients
- Search: "memcached tutorial", "memcached best practices", "memcached performance tuning"

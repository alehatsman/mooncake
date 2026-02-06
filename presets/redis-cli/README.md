# redis-cli - Redis Command-Line Interface

redis-cli is the Redis command-line interface for interacting with Redis servers.

## Quick Start

```yaml
- preset: redis-cli
```

## Features

- **Interactive mode**: REPL for exploring Redis commands
- **Command execution**: Run single commands or scripts
- **Pipeline support**: Execute multiple commands efficiently
- **Monitoring**: Real-time command monitoring
- **Cluster support**: Connect to Redis Cluster nodes
- **Scripting**: Lua script execution and debugging

## Basic Usage

```bash
# Connect to local Redis
redis-cli

# Connect to remote Redis
redis-cli -h redis.example.com -p 6379

# Authenticate
redis-cli -a password

# Execute single command
redis-cli SET mykey "Hello"

# Get key value
redis-cli GET mykey

# Monitor all commands
redis-cli MONITOR

# Get server info
redis-cli INFO

# Ping server
redis-cli PING
```

## Advanced Configuration

```yaml
# Simple installation
- preset: redis-cli

# Remove installation
- preset: redis-cli
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt installs redis-tools, dnf/yum installs redis)
- ✅ macOS (Homebrew installs redis package)
- ❌ Windows (not yet supported)

## Configuration

- **No config file**: redis-cli uses command-line options
- **History file**: `~/.rediscli_history` (interactive mode)
- **Default connection**: localhost:6379

## Real-World Examples

### Data Operations

```bash
# String operations
redis-cli SET user:1:name "John Doe"
redis-cli GET user:1:name
redis-cli APPEND user:1:name " Jr."
redis-cli STRLEN user:1:name

# List operations
redis-cli LPUSH tasks "task1" "task2" "task3"
redis-cli LRANGE tasks 0 -1
redis-cli LPOP tasks

# Hash operations
redis-cli HSET user:1 name "John" age 30 city "NYC"
redis-cli HGETALL user:1
redis-cli HGET user:1 name

# Set operations
redis-cli SADD tags "redis" "database" "nosql"
redis-cli SMEMBERS tags
redis-cli SISMEMBER tags "redis"

# Sorted set operations
redis-cli ZADD leaderboard 100 "player1" 200 "player2"
redis-cli ZRANGE leaderboard 0 -1 WITHSCORES
redis-cli ZINCRBY leaderboard 50 "player1"
```

### Database Management

```bash
# Select database (0-15 by default)
redis-cli SELECT 1

# List all keys
redis-cli KEYS "*"

# Get key pattern
redis-cli KEYS "user:*"

# Check key type
redis-cli TYPE mykey

# Set key expiration
redis-cli EXPIRE mykey 3600

# Check TTL
redis-cli TTL mykey

# Persist key (remove expiration)
redis-cli PERSIST mykey

# Delete key
redis-cli DEL mykey

# Flush database (DANGEROUS!)
redis-cli FLUSHDB

# Flush all databases (VERY DANGEROUS!)
redis-cli FLUSHALL
```

### Monitoring and Debugging

```bash
# Monitor all commands in real-time
redis-cli MONITOR

# Get server statistics
redis-cli INFO stats
redis-cli INFO memory
redis-cli INFO replication

# Get slow log
redis-cli SLOWLOG GET 10

# Check latency
redis-cli --latency
redis-cli --latency-history

# Benchmark performance
redis-cli --intrinsic-latency 60
```

### Bulk Operations

```bash
# Mass insertion from file
cat data.txt | redis-cli --pipe

# Export all keys
redis-cli --scan | while read key; do
  echo "SET $key $(redis-cli GET $key)"
done > dump.redis

# Import keys
cat dump.redis | redis-cli --pipe

# Scan with pattern
redis-cli --scan --pattern "user:*"
```

### Lua Scripting

```bash
# Execute Lua script
redis-cli EVAL "return redis.call('SET', KEYS[1], ARGV[1])" 1 mykey myvalue

# Load script
SCRIPT_SHA=$(redis-cli SCRIPT LOAD "return redis.call('GET', KEYS[1])")

# Execute loaded script
redis-cli EVALSHA $SCRIPT_SHA 1 mykey
```

### Cluster Operations

```bash
# Connect to cluster
redis-cli -c -h cluster-node.example.com

# Get cluster info
redis-cli CLUSTER INFO
redis-cli CLUSTER NODES

# Check key slot
redis-cli CLUSTER KEYSLOT mykey

# Resharding (admin operation)
redis-cli --cluster reshard cluster-node.example.com:6379
```

## Agent Use

- Query Redis for application state and caching data
- Monitor Redis performance and memory usage
- Automate data migrations and backups
- Test Redis availability in health checks
- Execute bulk operations for data initialization

## Troubleshooting

### Connection refused

```bash
# Check if Redis is running
redis-cli PING
# Error: Could not connect to Redis at 127.0.0.1:6379: Connection refused

# Start Redis server
sudo systemctl start redis  # Linux
brew services start redis  # macOS

# Check Redis is listening
netstat -an | grep 6379
```

### Authentication required

```bash
# Connect with password
redis-cli -a mypassword

# Or authenticate after connecting
redis-cli
AUTH mypassword

# Set password in redis.conf
requirepass mypassword
```

### Command not allowed

```bash
# Some commands may be renamed or disabled in redis.conf
# Check configuration
redis-cli CONFIG GET rename-command
redis-cli CONFIG GET disable-command
```

### Slow responses

```bash
# Check slow log
redis-cli SLOWLOG GET 10

# Check memory usage
redis-cli INFO memory

# Check connected clients
redis-cli CLIENT LIST

# Check latency
redis-cli --latency
```

## Uninstall

```yaml
- preset: redis-cli
  with:
    state: absent
```

**Note**: On macOS and some Linux distributions, redis-cli is part of the redis package. Uninstalling removes the entire Redis package including the server.

## Resources

- Official docs: https://redis.io/docs/ui/cli/
- Command reference: https://redis.io/commands/
- GitHub: https://github.com/redis/redis
- Search: "redis-cli tutorial", "redis commands cheatsheet", "redis-cli examples"

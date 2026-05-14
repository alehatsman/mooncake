# nats-server - NATS Cloud-Native Messaging System

NATS server is a cloud-native messaging system that provides fast, secure, and reliable communication for distributed systems. It's lightweight, written in Go, and designed for high performance with minimal memory footprint.

## Quick Start

```yaml
- preset: nats-server
```

## Features

- **High Performance**: Written in Go, optimized for low latency and high throughput
- **Cloud Native**: Runs in containers and Kubernetes with minimal resource usage
- **Multiple Messaging Patterns**: Publish-Subscribe, Request-Reply, and Queueing
- **Built-in Clustering**: Scale horizontally with automatic cluster management
- **Security**: TLS/SSL support, authentication, and authorization out of the box
- **Monitoring**: Real-time metrics and observability for operations
- **Reliability**: Message streaming and persistence options available

## Basic Usage

```bash
# Check version
nats-server --version

# Start server with defaults
nats-server

# Start with custom configuration
nats-server -c /etc/nats-server/nats.conf

# Display help
nats-server --help

# Start in debug mode
nats-server -D

# Monitor running server
nats-server --signal reopen

# Verify server is listening
lsof -i :4222  # macOS
netstat -tuln | grep 4222  # Linux
```

## Advanced Configuration

```yaml
# Basic installation with defaults
- preset: nats-server

# Prepare for uninstallation
- preset: nats-server
  with:
    state: absent
```

## Parameters

| Parameter | Type   | Default | Description              |
|-----------|--------|---------|--------------------------|
| state     | string | present | Install or remove server |

## Platform Support

- ✅ Linux (package managers: apt, dnf, yum, etc.)
- ✅ macOS (via package managers where available)
- ✅ Windows (via direct download or package managers)

## Configuration

- **Config file**: `/etc/nats-server/nats.conf` (typical Linux path)
- **Data directory**: `/var/lib/nats/` (default data storage)
- **Port**: 4222 (default client port)
- **Management port**: 8222 (monitoring/stats)
- **Log file**: `/var/log/nats-server/nats.log` (typical Linux path)
- **User/Group**: `_nats:_nats` or `nats:nats` (system account)

## Real-World Examples

### Local Development Server

```bash
# Start a simple local NATS server
nats-server

# In another terminal, test with nats-cli
nats -s nats://localhost:4222 pub test.subject "Hello"
nats -s nats://localhost:4222 sub test.subject
```

### Production Configuration with Clustering

```yaml
# Deploy clustered NATS for high availability
- preset: nats-server
  become: true

# Then create a configuration file:
# /etc/nats-server/nats.conf
# port: 4222
# http: 8222
#
# cluster {
#   name: "my-cluster"
#   host: 0.0.0.0
#   port: 6222
#   routes: [
#     nats://server1:6222
#     nats://server2:6222
#   ]
# }
```

### Health Check in Deployment

```bash
# Verify NATS server is ready for connections
#!/bin/bash
max_attempts=30
attempt=1

while [ $attempt -le $max_attempts ]; do
  if nats -s nats://localhost:4222 server ping 2>/dev/null; then
    echo "NATS server is ready"
    exit 0
  fi

  echo "Waiting for NATS server... (attempt $attempt/$max_attempts)"
  sleep 1
  ((attempt++))
done

echo "ERROR: NATS server failed to start"
exit 1
```

### Docker Compose Setup Example

```bash
# Create and run NATS in container
docker run -d \
  --name nats-server \
  -p 4222:4222 \
  -p 8222:8222 \
  nats:latest \
  -js  # Enable JetStream

# Verify it's running
nats -s nats://localhost:4222 server info
```

## Agent Use

- Automated deployment and health verification in infrastructure-as-code workflows
- Real-time monitoring of message broker availability
- Validation of messaging system configuration before application startup
- Integration testing with message publishing and subscription verification
- Cluster state monitoring and failover detection
- Provisioning messaging infrastructure for microservices

## Troubleshooting

### Port Already in Use

**Problem**: `listen: address already in use :4222`

**Solution**: Either stop the existing process or use a different port:
```bash
# Find what's using port 4222
lsof -i :4222  # macOS
netstat -tuln | grep 4222  # Linux

# Kill the process
kill -9 <PID>

# Or start on a different port
nats-server -p 4223
```

### Server Won't Start

**Problem**: `fatal error: server failed to start` or similar error

**Solution**: Check configuration file and logs:
```bash
# Verify configuration syntax
nats-server -c /etc/nats-server/nats.conf --parse-only

# Run in debug mode to see detailed output
nats-server -D -c /etc/nats-server/nats.conf

# Check system logs
journalctl -u nats-server -f  # Linux systemd
tail -f /var/log/nats-server/nats.log  # Direct log file
```

### Clustering Issues

**Problem**: Cluster routes not connecting

**Solution**: Verify cluster configuration:
```bash
# Check server info including cluster status
nats -s nats://localhost:4222 server info

# Verify firewall allows cluster port (default 6222)
nc -zv server1 6222
nc -zv server2 6222
```

### High Memory Usage

**Problem**: NATS server consuming more memory than expected

**Solution**: Check active connections and messages:
```bash
# View detailed statistics
nats -s nats://localhost:4222 server stats --all

# Check for large queued messages
nats -s nats://localhost:4222 server info
```

## Uninstall

```yaml
- preset: nats-server
  with:
    state: absent
```

## Resources

- Official docs: https://docs.nats.io/nats-server/introduction
- GitHub: https://github.com/nats-io/nats-server
- Configuration guide: https://docs.nats.io/running-a-nats-service/configuration
- Clustering: https://docs.nats.io/running-a-nats-service/clustering
- Search: "nats-server setup", "nats clustering guide", "nats-server configuration"

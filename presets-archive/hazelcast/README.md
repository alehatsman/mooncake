# Hazelcast - Distributed In-Memory Data Grid

Open-source distributed caching and computing platform for fast data processing and storage.

## Quick Start
```yaml
- preset: hazelcast
```

## Features
- **In-memory data grid**: Distributed caching with automatic data partitioning
- **High availability**: Automatic failover and data replication
- **Low latency**: Microsecond read/write performance
- **Scalability**: Linear scale-out by adding nodes
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Start standalone Hazelcast member
hazelcast start

# Start with custom config
hazelcast -c /path/to/config.xml start

# Check cluster status
hazelcast cluster-state

# View member list
hazelcast list-members

# Submit distributed compute job
hazelcast submit -c com.example.MyJob

# Stop member
hazelcast stop
```

## Configuration Examples

### Basic Cluster Setup
```xml
<!-- hazelcast.xml -->
<hazelcast>
    <cluster-name>production</cluster-name>
    <network>
        <port auto-increment="true">5701</port>
        <join>
            <multicast enabled="false"/>
            <tcp-ip enabled="true">
                <member>192.168.1.10</member>
                <member>192.168.1.11</member>
            </tcp-ip>
        </join>
    </network>
</hazelcast>
```

### Cache Configuration
```xml
<hazelcast>
    <map name="users">
        <max-size policy="PER_NODE">10000</max-size>
        <eviction eviction-policy="LRU"/>
        <time-to-live-seconds>3600</time-to-live-seconds>
    </map>
</hazelcast>
```

## Advanced Configuration
```yaml
- preset: hazelcast
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Hazelcast CLI |

## Real-World Examples

### Distributed Cache
```java
// Java client example
HazelcastInstance hz = Hazelcast.newHazelcastInstance();
IMap<String, String> cache = hz.getMap("my-cache");

// Put/Get operations (distributed)
cache.put("user:123", "John Doe");
String user = cache.get("user:123");

// Automatic replication across cluster
```

### Session Storage
```yaml
# Deploy Hazelcast cluster for web session storage
- name: Start Hazelcast cluster
  shell: hazelcast -c /etc/hazelcast/session-store.xml start
  become: true

# Configure web app to use Hazelcast for sessions
# Sessions automatically replicated across nodes
```

### Compute Grid
```bash
# Submit distributed computation
hazelcast submit \
  -c com.example.DataProcessor \
  -n production-cluster \
  input-file.csv
```

## Command Line Operations

### Cluster Management
```bash
# View cluster state
hazelcast cluster-state

# Change cluster state
hazelcast change-cluster-state --state ACTIVE

# Force cluster startup
hazelcast force-start

# Shutdown cluster gracefully
hazelcast shutdown
```

### Data Operations
```bash
# Execute SQL query
hazelcast sql "SELECT * FROM users WHERE age > 30"

# Import data
hazelcast import --format json --file data.json --map users

# Export data
hazelcast export --format json --map users > backup.json
```

### Monitoring
```bash
# Show member statistics
hazelcast stats

# Show map statistics
hazelcast map-stats --name users

# Enable metrics
hazelcast metrics --enable
```

## Configuration
- **Config file**: `/etc/hazelcast/hazelcast.xml` (system), `~/.hazelcast/hazelcast.xml` (user)
- **Logs**: `/var/log/hazelcast/` (system), `~/.hazelcast/logs/` (user)
- **Default ports**: 5701-5703 (member communication), 8080 (Management Center)
- **Data directory**: Configured per map/cache

## Use Cases
- **Session replication**: Share web sessions across application servers
- **Distributed cache**: Cache database queries, API responses
- **Real-time analytics**: Process streaming data with low latency
- **Microservices coordination**: Shared state between services
- **Event-driven architecture**: Distributed event bus

## Platform Support
- ✅ Linux (binary installation)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported in preset)

## Agent Use
- Deploy distributed caching layer for applications
- Set up high-availability session storage
- Configure compute grids for data processing
- Implement real-time analytics pipelines
- Coordinate state across microservices

## Troubleshooting

### Cluster formation issues
```bash
# Check network connectivity between nodes
telnet <member-ip> 5701

# Verify multicast is disabled (TCP-IP preferred)
grep -A 5 "<join>" hazelcast.xml

# Check firewall rules
sudo iptables -L | grep 5701
```

### High memory usage
```xml
<!-- Limit cache size -->
<map name="my-cache">
    <max-size policy="PER_NODE">5000</max-size>
    <eviction eviction-policy="LRU"/>
</map>
```

### Split-brain scenarios
```xml
<!-- Configure split-brain protection -->
<split-brain-protection enabled="true" name="quorum">
    <minimum-cluster-size>3</minimum-cluster-size>
</split-brain-protection>
```

## Uninstall
```yaml
- preset: hazelcast
  with:
    state: absent
```

## Resources
- Official docs: https://docs.hazelcast.com/
- CLI Guide: https://docs.hazelcast.com/hazelcast/latest/getting-started/cli
- GitHub: https://github.com/hazelcast/hazelcast-command-line
- Search: "hazelcast tutorial", "hazelcast distributed cache"

# Apache ActiveMQ Artemis - High-Performance Message Broker

Multi-protocol, embeddable message broker with support for AMQP, MQTT, STOMP, and JMS. High-throughput messaging for distributed systems.

## Quick Start
```yaml
- preset: artemis
```

## Features
- **Multi-protocol**: AMQP 1.0, MQTT 3.1.1/5, STOMP, JMS 2.0/3.1
- **High performance**: Millions of messages per second throughput
- **Clustering**: High availability with shared storage or network replication
- **Embeddable**: Can be embedded in Java applications
- **Protocol agnostic**: Simple, powerful addressing model
- **Cross-platform**: Java-based, runs on Linux, macOS, Windows

## Basic Usage
```bash
# Start broker
artemis run

# Create broker instance
artemis create /var/lib/artemis-instance

# Start broker instance
/var/lib/artemis-instance/bin/artemis run

# Check status
artemis producer --message-count 100 --url tcp://localhost:61616
artemis consumer --message-count 100 --url tcp://localhost:61616

# View queue stats
artemis queue stat --url tcp://localhost:61616
```

## Creating Broker Instance
```bash
# Create with defaults
artemis create mybroker

# Create with custom settings
artemis create mybroker \
  --user admin \
  --password secret \
  --host 0.0.0.0 \
  --port 61616 \
  --http-host 0.0.0.0 \
  --http-port 8161

# Create clustered instance
artemis create mybroker \
  --clustered \
  --cluster-user cluster-admin \
  --cluster-password cluster-secret
```

## Managing Broker
```bash
# Start broker
cd mybroker
bin/artemis run

# Start as background service
bin/artemis-service start

# Stop broker
bin/artemis-service stop

# Check if running
bin/artemis-service status

# Kill broker
bin/artemis-service kill
```

## Message Operations
```bash
# Send messages
artemis producer \
  --url tcp://localhost:61616 \
  --destination queue://myQueue \
  --message-count 1000 \
  --message-size 1024

# Consume messages
artemis consumer \
  --url tcp://localhost:61616 \
  --destination queue://myQueue \
  --message-count 1000

# Browse queue (non-destructive)
artemis browser \
  --url tcp://localhost:61616 \
  --destination queue://myQueue
```

## Queue Management
```bash
# Create queue
artemis queue create \
  --url tcp://localhost:61616 \
  --name myQueue \
  --address myAddress \
  --anycast

# Create topic
artemis queue create \
  --url tcp://localhost:61616 \
  --name myTopic \
  --address myAddress \
  --multicast

# Delete queue
artemis queue delete \
  --url tcp://localhost:61616 \
  --name myQueue

# List queues
artemis queue stat --url tcp://localhost:61616

# Purge queue
artemis queue purge \
  --url tcp://localhost:61616 \
  --name myQueue
```

## Address Management
```bash
# Create address
artemis address create \
  --url tcp://localhost:61616 \
  --name myAddress \
  --anycast

# Delete address
artemis address delete \
  --url tcp://localhost:61616 \
  --name myAddress

# Show address
artemis address show \
  --url tcp://localhost:61616 \
  --name myAddress
```

## User Management
```bash
# Add user
artemis user add \
  --user newuser \
  --password secret \
  --role admin

# Remove user
artemis user rm --user olduser

# List users
artemis user list

# Reset user password
artemis user reset \
  --user admin \
  --password newsecret
```

## Advanced Configuration
```yaml
# Deploy with custom configuration
- preset: artemis
  with:
    state: present

# Typical usage: manual broker instance creation
- name: Create Artemis broker instance
  shell: |
    artemis create /opt/artemis-broker \
      --user admin \
      --password {{ admin_password }} \
      --host 0.0.0.0 \
      --require-login
  args:
    creates: /opt/artemis-broker
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (requires Java/JDK)
- ✅ macOS (requires Java/JDK)
- ✅ Windows (requires Java/JDK)

## Configuration
- **Broker instance**: Custom location (e.g., `/var/lib/artemis-instance`)
- **Configuration**: `etc/broker.xml` (in broker instance directory)
- **Data directory**: `data/` (in broker instance)
- **Default ports**:
  - AMQP: 5672
  - MQTT: 1883
  - STOMP: 61613
  - OpenWire: 61616
  - Web console: 8161
- **Web console**: http://localhost:8161/console (default credentials: admin/admin)

## Real-World Examples

### CI/CD Testing
```bash
# Start broker for integration tests
artemis create test-broker --silent --user test --password test
test-broker/bin/artemis-service start

# Run tests
pytest tests/integration/

# Cleanup
test-broker/bin/artemis-service stop
rm -rf test-broker
```

### Clustered Deployment
```bash
# Create cluster node 1
artemis create node1 \
  --clustered \
  --cluster-user cluster \
  --cluster-password secret \
  --host node1.example.com \
  --static-cluster tcp://node2.example.com:61616

# Create cluster node 2
artemis create node2 \
  --clustered \
  --cluster-user cluster \
  --cluster-password secret \
  --host node2.example.com \
  --static-cluster tcp://node1.example.com:61616
```

### Performance Testing
```bash
# High-throughput producer test
artemis producer \
  --url tcp://localhost:61616 \
  --destination queue://perfTest \
  --message-count 1000000 \
  --message-size 1024 \
  --threads 10

# High-throughput consumer test
artemis consumer \
  --url tcp://localhost:61616 \
  --destination queue://perfTest \
  --message-count 1000000 \
  --threads 10
```

## Agent Use
- Message broker infrastructure for microservices
- Event-driven architecture backbone
- Task queue management
- IoT message routing (MQTT)
- Integration testing with message queues
- Performance benchmarking

## Troubleshooting

### Broker won't start
Check Java version (requires Java 11+):
```bash
java -version
```

Check logs:
```bash
tail -f /path/to/broker/log/artemis.log
```

### Port already in use
Change ports in broker configuration:
```bash
vi /path/to/broker/etc/broker.xml
# Edit acceptor port bindings
```

### Connection refused
Verify broker is running:
```bash
netstat -ln | grep 61616
# or
ss -ln | grep 61616
```

## Uninstall
```yaml
- preset: artemis
  with:
    state: absent
```

**Note:** This removes the CLI tool only. Broker instances must be removed manually.

## Resources
- Official site: https://artemis.apache.org/
- Documentation: https://activemq.apache.org/components/artemis/documentation/
- GitHub: https://github.com/apache/activemq-artemis
- Search: "artemis message broker", "activemq artemis tutorial"

Sources:
- [Apache ActiveMQ Artemis](https://artemis.apache.org/components/artemis/)
- [Apache ActiveMQ Artemis GitHub](https://github.com/apache/activemq-artemis)

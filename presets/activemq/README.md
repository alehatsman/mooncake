# ActiveMQ - Message Broker

Apache ActiveMQ is an open-source message broker that implements JMS (Java Message Service) and supports multiple protocols including AMQP, STOMP, MQTT, and OpenWire.

## Quick Start
```yaml
- preset: activemq
```

## Features
- **Multi-protocol support**: JMS, AMQP, STOMP, MQTT, OpenWire, REST
- **High availability**: Master-slave and network of brokers configurations
- **Message persistence**: File-based and JDBC storage options
- **Virtual destinations**: Queue and topic composites
- **Message filtering**: SQL92 selectors for routing
- **Web console**: Built-in administration interface on port 8161
- **Spring integration**: Native support for Spring Framework

## Basic Usage
```bash
# Start ActiveMQ
activemq start

# Check status
activemq status

# Stop broker
activemq stop

# View logs
tail -f /var/log/activemq/activemq.log

# Access web console
# http://localhost:8161/admin
# Default credentials: admin/admin
```

## Advanced Configuration
```yaml
- preset: activemq
  with:
    state: present
  become: true
```

## Configuration

- **Install directory**: `/opt/activemq/` (typical)
- **Config file**: `/opt/activemq/conf/activemq.xml`
- **Data directory**: `/opt/activemq/data/`
- **Web console**: http://localhost:8161/admin
- **Broker port**: 61616 (OpenWire), 5672 (AMQP), 1883 (MQTT), 61613 (STOMP)
- **Default credentials**: admin/admin (change in production)

## Real-World Examples

### Queue Producer/Consumer
```bash
# Send message to queue (using activemq command)
activemq producer --destination queue://test.queue --messageCount 10 --message "Hello World"

# Consume messages
activemq consumer --destination queue://test.queue
```

### Spring Boot Integration
```yaml
# application.yml
spring:
  activemq:
    broker-url: tcp://localhost:61616
    user: admin
    password: admin
```

### Docker Deployment
```yaml
- name: Deploy ActiveMQ in container
  shell: |
    docker run -d \
      --name activemq \
      -p 61616:61616 \
      -p 8161:8161 \
      -p 5672:5672 \
      -p 61613:61613 \
      -p 1883:1883 \
      rmohr/activemq:latest
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (manual installation required)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Message queue setup for microservices
- Event-driven architecture deployment
- Async communication infrastructure
- Enterprise integration patterns
- IoT message routing with MQTT
- Legacy system integration via JMS

## Troubleshooting

### Broker won't start
Check Java version and port availability.
```bash
# Verify Java installed
java -version

# Check if ports are in use
sudo netstat -tuln | grep -E '61616|8161'

# View startup logs
cat /opt/activemq/data/activemq.log
```

### Out of memory errors
Increase JVM heap size in wrapper configuration.
```bash
# Edit /opt/activemq/bin/env
export ACTIVEMQ_OPTS="-Xms512M -Xmx2G"

# Restart broker
activemq restart
```

### Web console not accessible
Firewall or Jetty configuration issue.
```bash
# Check if Jetty is listening
curl http://localhost:8161/admin

# Verify firewall rules
sudo ufw allow 8161/tcp  # Ubuntu
sudo firewall-cmd --add-port=8161/tcp --permanent  # RHEL/Fedora
```

## Uninstall
```yaml
- preset: activemq
  with:
    state: absent
```

## Resources
- Official docs: https://activemq.apache.org/
- Getting started: https://activemq.apache.org/getting-started
- GitHub: https://github.com/apache/activemq
- Search: "activemq tutorial", "activemq spring boot", "activemq clustering"

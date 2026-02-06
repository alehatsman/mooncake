# RabbitMQ - Message Broker

RabbitMQ is a robust, production-ready message broker supporting multiple messaging protocols.

## Quick Start

```yaml
- preset: rabbitmq
```

## Features

- **Multi-protocol**: AMQP 0-9-1, AMQP 1.0, STOMP, MQTT
- **Reliable**: Persistent messages with acknowledgements
- **Clustering**: High availability and federation support
- **Management UI**: Web-based monitoring and administration
- **Flexible routing**: Exchanges, queues, and bindings
- **Plugin system**: Extensible architecture

## Basic Usage

```bash
# Check status
rabbitmqctl status

# List users
rabbitmqctl list_users

# List queues
rabbitmqctl list_queues

# List exchanges
rabbitmqctl list_exchanges

# List bindings
rabbitmqctl list_bindings

# Open Management UI
# Navigate to: http://localhost:15672
# Default credentials: guest/guest (localhost only)
```

## Advanced Configuration

```yaml
# Basic installation with management UI
- preset: rabbitmq
  with:
    enable_management: true
  become: true

# Production setup with custom credentials
- preset: rabbitmq
  with:
    enable_management: true
    admin_user: "{{ vault_rabbitmq_user }}"
    admin_password: "{{ vault_rabbitmq_password }}"
    port: "5672"
    management_port: "15672"
  become: true

# Minimal installation (no service start)
- preset: rabbitmq
  with:
    start_service: false
    enable_management: false
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| start_service | bool | true | Start RabbitMQ service after installation |
| enable_management | bool | true | Enable management plugin (web UI) |
| admin_user | string | admin | Admin username |
| admin_password | string | admin | Admin password (change in production) |
| port | string | 5672 | AMQP protocol port |
| management_port | string | 15672 | Management UI port |

## Platform Support

- ✅ Linux (systemd, apt, dnf, yum, zypper)
- ✅ macOS (launchd, Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Config file**: `/etc/rabbitmq/rabbitmq.conf` (Linux), `/opt/homebrew/etc/rabbitmq/rabbitmq.conf` (macOS)
- **Data directory**: `/var/lib/rabbitmq/` (Linux), `/opt/homebrew/var/lib/rabbitmq/` (macOS)
- **Log directory**: `/var/log/rabbitmq/` (Linux), `/opt/homebrew/var/log/rabbitmq/` (macOS)
- **AMQP port**: 5672
- **Management UI**: http://localhost:15672

## Real-World Examples

### Microservices Communication

```yaml
# Install RabbitMQ for service mesh
- preset: rabbitmq
  with:
    enable_management: true
    admin_password: "{{ vault_password }}"
  become: true

# Verify installation
- assert:
    http:
      url: "http://localhost:15672"
      status: 200
```

### Task Queue System

```python
import pika

# Connect to RabbitMQ
connection = pika.BlockingConnection(
    pika.ConnectionParameters('localhost')
)
channel = connection.channel()

# Declare durable queue
channel.queue_declare(queue='tasks', durable=True)

# Send task
channel.basic_publish(
    exchange='',
    routing_key='tasks',
    body='Process this task',
    properties=pika.BasicProperties(
        delivery_mode=2,  # make message persistent
    )
)

# Consume tasks
def callback(ch, method, properties, body):
    print(f"Processing: {body}")
    ch.basic_ack(delivery_tag=method.delivery_tag)

channel.basic_qos(prefetch_count=1)
channel.basic_consume(queue='tasks', on_message_callback=callback)
channel.start_consuming()
```

### Pub/Sub Pattern

```bash
# Create fanout exchange
rabbitmqctl eval 'rabbit_exchange:declare({resource, <<"/">>, exchange, <<"logs">>}, fanout, true, false, false, []).'

# Bind queues to exchange
rabbitmqctl eval 'rabbit_binding:add({binding, {resource, <<"/">>, exchange, <<"logs">>}, <<"">>, {resource, <<"/">>, queue, <<"queue1">>}, []}).'

# Publish message to all subscribers
rabbitmqadmin publish exchange=logs routing_key="" payload="Log message"
```

### User Management

```bash
# Create new user
rabbitmqctl add_user myuser mypassword

# Set user tags (administrator, monitoring, management)
rabbitmqctl set_user_tags myuser administrator

# Set permissions (configure, write, read)
rabbitmqctl set_permissions -p / myuser ".*" ".*" ".*"

# List permissions
rabbitmqctl list_permissions -p /

# Delete user
rabbitmqctl delete_user myuser
```

## Agent Use

- Build distributed task queues for background processing
- Implement event-driven microservices architectures
- Create pub/sub systems for real-time notifications
- Manage request/reply patterns for RPC-style communication
- Buffer messages between services with different processing speeds

## Troubleshooting

### Service won't start

```bash
# Check logs
journalctl -u rabbitmq-server -f  # Linux
tail -f /opt/homebrew/var/log/rabbitmq/rabbit@*.log  # macOS

# Check port conflicts
lsof -i :5672
lsof -i :15672

# Verify Erlang installation
erl -version
```

### Management UI not accessible

```bash
# Enable management plugin
rabbitmq-plugins enable rabbitmq_management

# Restart service
sudo systemctl restart rabbitmq-server  # Linux
brew services restart rabbitmq  # macOS

# Check if plugin is enabled
rabbitmq-plugins list
```

### Connection refused

```bash
# Check if service is running
sudo systemctl status rabbitmq-server  # Linux
brew services list | grep rabbitmq  # macOS

# Test connection
telnet localhost 5672

# Check firewall
sudo ufw status  # Linux
```

### Out of memory

```bash
# Check memory usage
rabbitmqctl status | grep memory

# Set memory limit in rabbitmq.conf
vm_memory_high_watermark.relative = 0.4
```

## Uninstall

```yaml
- preset: rabbitmq
  with:
    state: absent
  become: true
```

**Note**: Data directory is preserved after uninstall. Remove manually if needed:

```bash
sudo rm -rf /var/lib/rabbitmq/  # Linux
rm -rf /opt/homebrew/var/lib/rabbitmq/  # macOS
```

## Resources

- Official docs: https://www.rabbitmq.com/documentation.html
- GitHub: https://github.com/rabbitmq/rabbitmq-server
- Tutorials: https://www.rabbitmq.com/getstarted.html
- Search: "rabbitmq tutorial", "rabbitmq clustering", "rabbitmq best practices"

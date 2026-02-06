# RabbitMQ Preset

Install and configure RabbitMQ - a robust message broker for distributed systems.

## Quick Start

```yaml
- preset: rabbitmq
  with:
    enable_management: true
    admin_password: "secure_password"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `start_service` | bool | `true` | Start service after install |
| `enable_management` | bool | `true` | Enable web UI |
| `admin_user` | string | `admin` | Admin username |
| `admin_password` | string | `admin` | Admin password |
| `port` | string | `5672` | AMQP port |
| `management_port` | string | `15672` | Management UI port |

## Usage

### Basic Installation
```yaml
- preset: rabbitmq
```

### Production Setup
```yaml
- preset: rabbitmq
  with:
    admin_user: "{{ vault_rabbitmq_user }}"
    admin_password: "{{ vault_rabbitmq_password }}"
    enable_management: true
```

## Verify Installation

```bash
# Check status
rabbitmqctl status

# List users
rabbitmqctl list_users

# List queues
rabbitmqctl list_queues

# Open Management UI
open http://localhost:15672  # macOS
xdg-open http://localhost:15672  # Linux
```

## Common Operations

```bash
# Create user
rabbitmqctl add_user myuser mypassword
rabbitmqctl set_user_tags myuser administrator
rabbitmqctl set_permissions -p / myuser ".*" ".*" ".*"

# Create queue
rabbitmqctl add_queue myqueue

# List exchanges
rabbitmqctl list_exchanges

# Enable plugin
rabbitmq-plugins enable rabbitmq_shovel

# Restart service
sudo systemctl restart rabbitmq-server  # Linux
brew services restart rabbitmq          # macOS
```

## Python Client Example

```python
import pika

connection = pika.BlockingConnection(
    pika.ConnectionParameters('localhost')
)
channel = connection.channel()

# Declare queue
channel.queue_declare(queue='hello')

# Send message
channel.basic_publish(exchange='', routing_key='hello', body='Hello World!')

# Receive message
def callback(ch, method, properties, body):
    print(f"Received {body}")

channel.basic_consume(queue='hello', on_message_callback=callback, auto_ack=True)
channel.start_consuming()
```

## Configuration Files

- **Linux**: `/etc/rabbitmq/rabbitmq.conf`
- **macOS**: `/opt/homebrew/etc/rabbitmq/rabbitmq.conf`
- **Data**: `/var/lib/rabbitmq/`

## Uninstall

```yaml
- preset: rabbitmq
  with:
    state: absent
```

**Note:** Data directory preserved after uninstall.

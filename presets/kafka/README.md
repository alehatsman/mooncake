# Kafka Preset

Install and configure Apache Kafka - a distributed streaming platform for building real-time data pipelines.

## Quick Start

```yaml
- preset: kafka
  with:
    kraft_mode: true
    start_service: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `start_service` | bool | `true` | Start service after install |
| `kraft_mode` | bool | `true` | Use KRaft (no Zookeeper) |
| `broker_id` | string | `1` | Broker ID |
| `port` | string | `9092` | Kafka port |
| `data_dir` | string | `/tmp/kafka-logs` | Data directory |

## Usage

### Modern Setup (KRaft)
```yaml
- preset: kafka
  with:
    kraft_mode: true
```

### Legacy Setup (with Zookeeper)
```yaml
- preset: kafka
  with:
    kraft_mode: false
```

## Verify Installation

```bash
# List topics
kafka-topics --bootstrap-server localhost:9092 --list

# Create topic
kafka-topics --bootstrap-server localhost:9092 --create --topic test --partitions 1 --replication-factor 1

# Describe topic
kafka-topics --bootstrap-server localhost:9092 --describe --topic test
```

## Common Operations

```bash
# Create topic
kafka-topics --bootstrap-server localhost:9092 \
  --create --topic mytopic --partitions 3 --replication-factor 1

# List topics
kafka-topics --bootstrap-server localhost:9092 --list

# Produce messages
kafka-console-producer --bootstrap-server localhost:9092 --topic mytopic

# Consume messages
kafka-console-consumer --bootstrap-server localhost:9092 --topic mytopic --from-beginning

# Delete topic
kafka-topics --bootstrap-server localhost:9092 --delete --topic mytopic

# Get consumer groups
kafka-consumer-groups --bootstrap-server localhost:9092 --list

# Describe consumer group
kafka-consumer-groups --bootstrap-server localhost:9092 --describe --group mygroup
```

## Python Client Example

```python
from kafka import KafkaProducer, KafkaConsumer

# Producer
producer = KafkaProducer(bootstrap_servers=['localhost:9092'])
producer.send('mytopic', b'Hello Kafka')
producer.flush()

# Consumer
consumer = KafkaConsumer(
    'mytopic',
    bootstrap_servers=['localhost:9092'],
    auto_offset_reset='earliest'
)

for message in consumer:
    print(f"Received: {message.value.decode('utf-8')}")
```

## Configuration Files

- **macOS**: `/opt/homebrew/etc/kafka/`
- **Linux**: `/opt/kafka/config/`
- **Logs**: `/tmp/kafka.log`

## KRaft vs Zookeeper

**KRaft Mode (Recommended):**
- No Zookeeper dependency
- Simpler architecture
- Better scalability
- Kafka 3.0+ feature

**Zookeeper Mode (Legacy):**
- Traditional setup
- Requires Zookeeper
- Will be deprecated

## Performance Tuning

```properties
# Increase throughput
num.network.threads=8
num.io.threads=8

# Retention
log.retention.hours=168
log.segment.bytes=1073741824

# Replication
default.replication.factor=3
min.insync.replicas=2
```

## Uninstall

```yaml
- preset: kafka
  with:
    state: absent
```

**Note:** Data directory preserved after uninstall.

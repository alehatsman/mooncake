# Apache Pulsar - Distributed Messaging

Cloud-native, distributed messaging and streaming platform. Unified publish-subscribe and queue model with multi-tenancy and geo-replication.

## Quick Start
```yaml
- preset: pulsar
```

## Features
- **Multi-tenancy**: Isolated namespaces per tenant
- **Geo-replication**: Cross-datacenter message replication
- **Persistent storage**: Apache BookKeeper for durability
- **Flexible messaging**: Pub/sub, queuing, streaming
- **Schema registry**: Enforce message structure
- **Tiered storage**: Offload old data to S3/GCS/Azure
- **Functions**: Serverless compute on streams
- **SQL queries**: Query topics with Presto/Trino

## Basic Usage
```bash
# Start Pulsar (standalone mode)
pulsar standalone

# Create topic
pulsar-admin topics create persistent://public/default/my-topic

# Produce messages
pulsar-client produce persistent://public/default/my-topic --messages "Hello Pulsar"

# Consume messages
pulsar-client consume persistent://public/default/my-topic --subscription-name my-sub --num-messages 0

# View topics
pulsar-admin topics list public/default

# Get topic stats
pulsar-admin topics stats persistent://public/default/my-topic

# Delete topic
pulsar-admin topics delete persistent://public/default/my-topic
```

## Advanced Configuration

### Standalone deployment
```yaml
- name: Install Pulsar
  preset: pulsar
  become: true

- name: Start Pulsar standalone
  shell: pulsar standalone > /var/log/pulsar/standalone.log 2>&1 &
  creates: /var/log/pulsar/standalone.log

- name: Wait for Pulsar to start
  assert:
    http:
      url: http://localhost:8080/admin/v2/clusters
      status: 200
```

### Production cluster
```yaml
- name: Install Pulsar
  preset: pulsar
  become: true

- name: Configure ZooKeeper
  template:
    dest: /etc/pulsar/zookeeper.conf
    content: |
      tickTime=2000
      dataDir=/var/lib/pulsar/zookeeper
      clientPort=2181
      server.1={{ zk1_host }}:2888:3888
      server.2={{ zk2_host }}:2888:3888
      server.3={{ zk3_host }}:2888:3888

- name: Start BookKeeper
  shell: pulsar bookie
  become: true

- name: Start broker
  shell: pulsar broker
  become: true
```

### Geo-replication setup
```yaml
- name: Configure cluster replication
  shell: |
    # Register clusters
    pulsar-admin clusters create us-east --url http://broker-us-east:8080
    pulsar-admin clusters create us-west --url http://broker-us-west:8080

    # Create tenant
    pulsar-admin tenants create my-tenant --allowed-clusters us-east,us-west

    # Create namespace with replication
    pulsar-admin namespaces create my-tenant/my-namespace
    pulsar-admin namespaces set-clusters my-tenant/my-namespace --clusters us-east,us-west
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Pulsar |

## Platform Support
- ✅ Linux (systemd, apt, dnf, yum)
- ✅ macOS (Homebrew, binary)
- ✅ Docker (official images)
- ✅ Kubernetes (Helm charts, operator)

## Configuration
- **Config directory**: `/etc/pulsar/`
- **Data directory**: `/var/lib/pulsar/`
- **Logs**: `/var/log/pulsar/`
- **Broker API**: `http://localhost:8080` (HTTP), `pulsar://localhost:6650` (binary)
- **Web UI**: `http://localhost:8080` (admin interface)

## Real-World Examples

### Event streaming application
```java
// Producer
PulsarClient client = PulsarClient.builder()
    .serviceUrl("pulsar://localhost:6650")
    .build();

Producer<String> producer = client.newProducer(Schema.STRING)
    .topic("persistent://public/default/events")
    .create();

producer.send("Event data");
producer.close();
client.close();
```

```java
// Consumer
PulsarClient client = PulsarClient.builder()
    .serviceUrl("pulsar://localhost:6650")
    .build();

Consumer<String> consumer = client.newConsumer(Schema.STRING)
    .topic("persistent://public/default/events")
    .subscriptionName("my-subscription")
    .subscribe();

Message<String> msg = consumer.receive();
System.out.println("Received: " + msg.getValue());
consumer.acknowledge(msg);
```

### Python event processing
```python
import pulsar

# Producer
client = pulsar.Client('pulsar://localhost:6650')
producer = client.create_producer('persistent://public/default/events')

producer.send('Hello Pulsar'.encode('utf-8'))
client.close()

# Consumer
client = pulsar.Client('pulsar://localhost:6650')
consumer = client.subscribe(
    'persistent://public/default/events',
    'my-subscription'
)

msg = consumer.receive()
print(f"Received: {msg.data().decode('utf-8')}")
consumer.acknowledge(msg)
client.close()
```

### Microservices messaging
```yaml
- name: Setup Pulsar for microservices
  preset: pulsar
  become: true

- name: Create tenants and namespaces
  shell: |
    pulsar-admin tenants create myapp --allowed-clusters standalone
    pulsar-admin namespaces create myapp/production
    pulsar-admin namespaces create myapp/staging

- name: Set retention policy
  shell: |
    pulsar-admin namespaces set-retention myapp/production --size 50G --time 7d
```

## Topic Management

### Create topics
```bash
# Persistent topic
pulsar-admin topics create persistent://public/default/my-topic

# Non-persistent (in-memory only)
pulsar-admin topics create non-persistent://public/default/my-topic

# Partitioned topic
pulsar-admin topics create-partitioned-topic persistent://public/default/my-partitioned-topic --partitions 3
```

### Topic stats
```bash
# Get statistics
pulsar-admin topics stats persistent://public/default/my-topic

# List subscriptions
pulsar-admin topics subscriptions persistent://public/default/my-topic

# Peek messages
pulsar-admin topics peek-messages persistent://public/default/my-topic --count 10 --subscription my-sub
```

### Topic cleanup
```bash
# Unload topic
pulsar-admin topics unload persistent://public/default/my-topic

# Delete topic
pulsar-admin topics delete persistent://public/default/my-topic

# Clear backlog
pulsar-admin topics clear-backlog persistent://public/default/my-topic --subscription my-sub
```

## Multi-Tenancy

### Tenant management
```bash
# Create tenant
pulsar-admin tenants create acme --allowed-clusters us-east

# List tenants
pulsar-admin tenants list

# Update tenant
pulsar-admin tenants update acme --allowed-clusters us-east,us-west

# Delete tenant
pulsar-admin tenants delete acme
```

### Namespace management
```bash
# Create namespace
pulsar-admin namespaces create acme/production

# Set policies
pulsar-admin namespaces set-retention acme/production --size 100G --time 30d
pulsar-admin namespaces set-message-ttl acme/production --messageTTL 86400

# List namespaces
pulsar-admin namespaces list acme
```

## Schema Registry

### Define schema
```java
import org.apache.pulsar.client.api.Schema;
import org.apache.pulsar.client.api.schema.SchemaDefinition;

// JSON schema
Schema<User> schema = Schema.JSON(User.class);

// Avro schema
Schema<User> avroSchema = Schema.AVRO(
    SchemaDefinition.<User>builder()
        .withPojo(User.class)
        .build()
);

// Producer with schema
Producer<User> producer = client.newProducer(schema)
    .topic("persistent://public/default/users")
    .create();

User user = new User("john", "john@example.com");
producer.send(user);
```

### Schema operations
```bash
# Get schema
pulsar-admin schemas get persistent://public/default/my-topic

# Upload schema
pulsar-admin schemas upload persistent://public/default/my-topic --filename schema.json

# Delete schema
pulsar-admin schemas delete persistent://public/default/my-topic
```

## Functions (Serverless)

### Create function
```java
// Java function
public class ExclamationFunction implements Function<String, String> {
    @Override
    public String process(String input, Context context) {
        return input + "!";
    }
}
```

```bash
# Deploy function
pulsar-admin functions create \
  --jar my-function.jar \
  --classname com.example.ExclamationFunction \
  --inputs persistent://public/default/input \
  --output persistent://public/default/output

# List functions
pulsar-admin functions list --tenant public --namespace default

# Get function status
pulsar-admin functions status --tenant public --namespace default --name ExclamationFunction

# Delete function
pulsar-admin functions delete --tenant public --namespace default --name ExclamationFunction
```

## Geo-Replication

### Setup replication
```bash
# Register clusters
pulsar-admin clusters create us-east --url http://pulsar-us-east:8080 --broker-url pulsar://pulsar-us-east:6650
pulsar-admin clusters create eu-west --url http://pulsar-eu-west:8080 --broker-url pulsar://pulsar-eu-west:6650

# Create replicated namespace
pulsar-admin namespaces create my-tenant/global
pulsar-admin namespaces set-clusters my-tenant/global --clusters us-east,eu-west

# Check replication
pulsar-admin topics stats-internal persistent://my-tenant/global/my-topic
```

## Tiered Storage

### Configure offloading
```bash
# S3 offloading
pulsar-admin namespaces set-offload-policies my-tenant/my-namespace \
  --driver aws-s3 \
  --bucket my-bucket \
  --region us-west-2 \
  --offloadAfterElapsed 1h

# GCS offloading
pulsar-admin namespaces set-offload-policies my-tenant/my-namespace \
  --driver google-cloud-storage \
  --bucket my-bucket \
  --offloadAfterElapsed 2h

# Trigger offload
pulsar-admin topics offload persistent://my-tenant/my-namespace/my-topic --size-threshold 10G
```

## Monitoring

### Metrics
```bash
# Broker metrics
curl http://localhost:8080/metrics/

# Topic metrics
pulsar-admin topics stats persistent://public/default/my-topic

# Subscription metrics
pulsar-admin topics stats-internal persistent://public/default/my-topic
```

### Prometheus integration
```yaml
# Add to Prometheus config
scrape_configs:
  - job_name: 'pulsar'
    static_configs:
      - targets: ['pulsar-broker:8080']
    metrics_path: /metrics/
```

## Client Libraries
- **Java**: Native support
- **Python**: `pulsar-client` PyPI package
- **Go**: `pulsar-client-go` module
- **C++**: Native library
- **Node.js**: `pulsar-client` npm package
- **C#**: NuGet package
- **WebSocket**: Browser-compatible

## Agent Use
- Event-driven microservices communication
- Real-time data pipelines and ETL
- IoT message ingestion at scale
- Log aggregation and streaming analytics
- Multi-region disaster recovery
- Decoupling application components
- Queue-based task processing

## Troubleshooting

### Broker not starting
```bash
# Check logs
tail -f /var/log/pulsar/broker.log

# Verify ZooKeeper
echo stat | nc localhost 2181

# Check BookKeeper
pulsar-admin bookies list
```

### Topic creation fails
```bash
# Check namespace exists
pulsar-admin namespaces list public

# Verify cluster
pulsar-admin clusters list

# Check permissions
pulsar-admin namespaces permissions get public/default
```

### High latency
```bash
# Check broker stats
pulsar-admin broker-stats monitoring-metrics

# View slow consumer
pulsar-admin topics stats persistent://public/default/my-topic

# Increase partitions
pulsar-admin topics update-partitioned-topic persistent://public/default/my-topic --partitions 10
```

### Message backlog
```bash
# View backlog
pulsar-admin topics stats persistent://public/default/my-topic

# Skip messages
pulsar-admin topics skip persistent://public/default/my-topic --count 1000 --subscription my-sub

# Reset cursor
pulsar-admin topics reset-cursor persistent://public/default/my-topic --subscription my-sub --time 1h
```

## Best Practices
- **Use partitioned topics**: Scale throughput horizontally
- **Set retention policies**: Balance storage vs durability
- **Enable schema registry**: Enforce data contracts
- **Multi-tenancy**: Isolate applications with tenants/namespaces
- **Geo-replication**: Disaster recovery and low latency
- **Tiered storage**: Cost-effective long-term retention
- **Monitor metrics**: Track throughput, latency, backlog
- **Tune BookKeeper**: Optimize write performance

## Comparison

| Feature | Pulsar | Kafka | RabbitMQ | NATS |
|---------|--------|-------|----------|------|
| Multi-tenancy | ✅ | ❌ | ✅ | ✅ |
| Geo-replication | ✅ | ✅ | ❌ | ✅ |
| Queuing | ✅ | ❌ | ✅ | ✅ |
| Streaming | ✅ | ✅ | ❌ | ✅ |
| Serverless functions | ✅ | ❌ | ❌ | ❌ |
| Tiered storage | ✅ | ✅ | ❌ | ✅ |

## Uninstall
```yaml
- preset: pulsar
  with:
    state: absent
```

**Note**: This removes Pulsar but keeps data in `/var/lib/pulsar/`.

## Resources
- Official docs: https://pulsar.apache.org/docs/
- GitHub: https://github.com/apache/pulsar
- Client docs: https://pulsar.apache.org/docs/client-libraries/
- Functions: https://pulsar.apache.org/docs/functions-overview/
- Search: "apache pulsar tutorial", "pulsar vs kafka", "pulsar functions"

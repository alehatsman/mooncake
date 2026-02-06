# Apache Flink - Stream Processing Framework

Stateful computations over unbounded and bounded data streams. Real-time data processing with exactly-once semantics and event time processing.

## Quick Start
```yaml
- preset: flink
```

## Features
- **Stream processing**: Real-time data processing with low latency
- **Exactly-once semantics**: Consistent state even during failures
- **Event time processing**: Handle out-of-order events correctly
- **Batch and streaming**: Unified engine for batch and stream processing
- **High throughput**: Process millions of events per second
- **Stateful operations**: Built-in state management with checkpointing

## Basic Usage
```bash
# Start Flink cluster
$FLINK_HOME/bin/start-cluster.sh

# Submit job
flink run examples/streaming/WordCount.jar

# List running jobs
flink list

# Stop job
flink stop <job-id>

# Cancel job
flink cancel <job-id>

# Check cluster status
curl http://localhost:8081/overview
```

## Advanced Configuration
```yaml
- preset: flink
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Flink |

## Platform Support
- ✅ Linux (binary download, package managers)
- ✅ macOS (Homebrew, binary download)
- ❌ Windows (not yet supported in this preset)

## Configuration
- **Install directory**: `/opt/flink` (Linux), `/usr/local/opt/flink` (macOS)
- **Config file**: `$FLINK_HOME/conf/flink-conf.yaml`
- **JobManager**: Web UI on `http://localhost:8081`
- **TaskManager**: Handles task execution
- **Checkpoints**: `file://$FLINK_HOME/checkpoints` (local filesystem)

## Real-World Examples

### Stream Processing Job
```java
// WordCount.java
DataStream<String> text = env.socketTextStream("localhost", 9999);

DataStream<Tuple2<String, Integer>> counts = text
    .flatMap(new Tokenizer())
    .keyBy(value -> value.f0)
    .sum(1);

counts.print();
env.execute("Streaming WordCount");
```

### Kafka to Kafka ETL
```java
// Read from Kafka
FlinkKafkaConsumer<String> consumer = new FlinkKafkaConsumer<>(
    "input-topic",
    new SimpleStringSchema(),
    properties
);

DataStream<String> stream = env.addSource(consumer);

// Transform and write back
stream
    .map(s -> s.toUpperCase())
    .addSink(new FlinkKafkaProducer<>(
        "output-topic",
        new SimpleStringSchema(),
        properties
    ));

env.execute("Kafka ETL");
```

### Event Time Processing
```java
DataStream<Event> events = env
    .addSource(new EventSource())
    .assignTimestampsAndWatermarks(
        WatermarkStrategy
            .<Event>forBoundedOutOfOrderness(Duration.ofSeconds(5))
            .withTimestampAssigner((event, timestamp) -> event.getTimestamp())
    );

// Window by event time
events
    .keyBy(Event::getUserId)
    .window(TumblingEventTimeWindows.of(Time.minutes(5)))
    .reduce((e1, e2) -> e1.merge(e2))
    .print();
```

### Stateful Processing
```java
// Count events per user with state
events
    .keyBy(Event::getUserId)
    .flatMap(new RichFlatMapFunction<Event, Tuple2<String, Long>>() {
        private transient ValueState<Long> count;

        @Override
        public void open(Configuration config) {
            count = getRuntimeContext().getState(
                new ValueStateDescriptor<>("count", Long.class)
            );
        }

        @Override
        public void flatMap(Event event, Collector<Tuple2<String, Long>> out) {
            Long current = count.value();
            current = (current == null) ? 1L : current + 1;
            count.update(current);
            out.collect(new Tuple2<>(event.getUserId(), current));
        }
    });
```

## Agent Use
- Build real-time analytics pipelines for streaming data
- Process event streams from Kafka, Kinesis, or message queues
- Implement complex event processing (CEP) for pattern detection
- Run ETL jobs with exactly-once guarantees
- Perform windowed aggregations on time-series data
- Join multiple data streams in real-time

## Troubleshooting

### JobManager not starting
```bash
# Check logs
tail -f $FLINK_HOME/log/flink-*-jobmanager-*.log

# Verify Java version (requires Java 8 or 11)
java -version

# Check port availability
netstat -an | grep 8081

# Increase memory in conf/flink-conf.yaml
jobmanager.memory.process.size: 2048m
```

### Task execution failures
```bash
# Check TaskManager logs
tail -f $FLINK_HOME/log/flink-*-taskmanager-*.log

# Increase parallelism
taskmanager.numberOfTaskSlots: 4

# Monitor via Web UI
open http://localhost:8081
```

### Checkpoint failures
```bash
# Verify checkpoint directory exists and is writable
ls -la $FLINK_HOME/checkpoints

# Enable checkpointing in job
env.enableCheckpointing(60000); // checkpoint every 60s

# Check checkpoint statistics in Web UI
# Navigate to: Job → Checkpoints
```

### Out of memory errors
```bash
# Increase TaskManager memory
taskmanager.memory.process.size: 4096m

# Tune memory configuration
taskmanager.memory.managed.fraction: 0.4
taskmanager.memory.network.fraction: 0.2

# Monitor memory usage
open http://localhost:8081/#/task-manager
```

## Uninstall
```yaml
- preset: flink
  with:
    state: absent
```

## Resources
- Official docs: https://flink.apache.org/docs/
- GitHub: https://github.com/apache/flink
- Training: https://flink.apache.org/training
- Examples: https://github.com/apache/flink/tree/master/flink-examples
- Search: "flink stream processing", "flink kafka", "flink event time"

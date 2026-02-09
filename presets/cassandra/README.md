# Apache Cassandra - Distributed NoSQL Database

Massively scalable, masterless NoSQL database designed for high availability and fault tolerance. Linear scalability across thousands of nodes with no single point of failure. Perfect for time-series, IoT, and high-throughput workloads.

## Quick Start
```yaml
- preset: cassandra
```

Default ports: CQL 9042, JMX 7199, Gossip 7000

## Features
- **Masterless architecture**: No single point of failure, all nodes equal
- **Linear scalability**: Add nodes to increase throughput proportionally
- **High availability**: Data replicated across multiple nodes and datacenters
- **Tunable consistency**: Choose between consistency and availability per query
- **Wide-column store**: Flexible schema, billions of columns per row
- **CQL**: SQL-like query language familiar to developers
- **Multi-datacenter**: Active-active replication across regions
- **Fast writes**: Log-structured storage optimized for write-heavy workloads
- **No downtime**: Rolling upgrades and repairs without service interruption

## Architecture

### Masterless Design
```
┌─────────────────────────────────────────┐
│           Cassandra Ring                │
│                                         │
│    Node 1          Node 2          Node 3│
│   (Token: 0)   (Token: 85)   (Token: 170)│
│      │              │              │    │
│      └──────────────┴──────────────┘    │
│           Gossip Protocol                │
│    (every node knows about every node)  │
└─────────────────────────────────────────┘

Client ──> Any Node (Coordinator)
              │
              ├──> Node 1 (Replica 1)
              ├──> Node 2 (Replica 2)
              └──> Node 3 (Replica 3)
```

### Key Concepts
- **Partition Key**: Determines which nodes store the data (hash distribution)
- **Clustering Key**: Determines sort order within a partition
- **Replication Factor**: Number of copies of data (typically 3)
- **Consistency Level**: How many replicas must respond (ONE, QUORUM, ALL)
- **Gossip Protocol**: Peer-to-peer communication for cluster state
- **Compaction**: Background merging of SSTables for performance

## Basic Usage

### CQL Shell (cqlsh)
```bash
# Connect to local instance
cqlsh

# Connect to remote node
cqlsh 192.168.1.10 9042

# Connect with authentication
cqlsh -u cassandra -p cassandra

# Execute CQL file
cqlsh -f schema.cql

# Execute inline command
cqlsh -e "DESCRIBE KEYSPACES;"
```

### Create Keyspace (Database)
```sql
-- Simple strategy (single datacenter)
CREATE KEYSPACE myapp
WITH replication = {
    'class': 'SimpleStrategy',
    'replication_factor': 3
};

-- Network topology (multi-datacenter)
CREATE KEYSPACE myapp
WITH replication = {
    'class': 'NetworkTopologyStrategy',
    'dc1': 3,
    'dc2': 2
};

USE myapp;
```

### Create Table
```sql
-- Time-series table
CREATE TABLE sensor_data (
    sensor_id UUID,
    timestamp TIMESTAMP,
    temperature DOUBLE,
    humidity DOUBLE,
    PRIMARY KEY (sensor_id, timestamp)
) WITH CLUSTERING ORDER BY (timestamp DESC);

-- User table
CREATE TABLE users (
    user_id UUID PRIMARY KEY,
    email TEXT,
    username TEXT,
    created_at TIMESTAMP,
    profile MAP<TEXT, TEXT>
);

-- Wide-row table (many columns per row)
CREATE TABLE metrics (
    metric_name TEXT,
    timestamp TIMESTAMP,
    value DOUBLE,
    tags MAP<TEXT, TEXT>,
    PRIMARY KEY (metric_name, timestamp)
) WITH CLUSTERING ORDER BY (timestamp DESC)
  AND compaction = {'class': 'TimeWindowCompactionStrategy'};
```

### CRUD Operations
```sql
-- Insert
INSERT INTO users (user_id, email, username, created_at)
VALUES (uuid(), 'user@example.com', 'john', toTimestamp(now()));

-- Insert with TTL (time-to-live, auto-delete)
INSERT INTO sessions (session_id, user_id, data)
VALUES (uuid(), uuid(), 'data')
USING TTL 3600;  -- Expire after 1 hour

-- Update
UPDATE users
SET email = 'newemail@example.com'
WHERE user_id = ?;

-- Delete
DELETE FROM users WHERE user_id = ?;

-- Select
SELECT * FROM sensor_data
WHERE sensor_id = ? AND timestamp > '2024-01-01';

-- Select with LIMIT
SELECT * FROM sensor_data
WHERE sensor_id = ?
ORDER BY timestamp DESC
LIMIT 100;
```

## Advanced Configuration

### Single-node development setup
```yaml
- name: Install Cassandra
  preset: cassandra

- name: Start Cassandra
  service:
    name: cassandra
    state: started
  become: true

- name: Wait for Cassandra to be ready
  shell: |
    for i in {1..30}; do
      cqlsh -e "DESCRIBE KEYSPACES" && break
      sleep 2
    done
```

### Multi-node cluster setup
```yaml
- name: Install Cassandra on all nodes
  hosts: cassandra-cluster
  tasks:
    - preset: cassandra
      become: true

    - name: Configure Cassandra
      template:
        src: cassandra.yaml.j2
        dest: /etc/cassandra/cassandra.yaml
      become: true

    - name: Set seeds for cluster
      lineinfile:
        path: /etc/cassandra/cassandra.yaml
        regexp: '^          - seeds:'
        line: '          - seeds: "{{ seed_nodes }}"'
      become: true

    - name: Set cluster name
      lineinfile:
        path: /etc/cassandra/cassandra.yaml
        regexp: '^cluster_name:'
        line: "cluster_name: '{{ cluster_name }}'"
      become: true

    - name: Start Cassandra
      service:
        name: cassandra
        state: restarted
      become: true

    - name: Wait for node to join cluster
      shell: nodetool status | grep UN
      register: result
      until: result.rc == 0
      retries: 30
      delay: 10
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Cassandra |

## Platform Support
- ✅ Linux (apt, yum, dnf) - Ubuntu, Debian, RHEL, CentOS
- ✅ Docker (official cassandra images)
- ❌ macOS (use Docker)
- ❌ Windows (use Docker)

## Configuration

### cassandra.yaml key settings
```yaml
# Cluster
cluster_name: 'MyCluster'
seeds: "10.0.1.10,10.0.1.11,10.0.1.12"
listen_address: 10.0.1.10
rpc_address: 0.0.0.0
broadcast_rpc_address: 10.0.1.10

# Directories
data_file_directories:
  - /var/lib/cassandra/data
commitlog_directory: /var/lib/cassandra/commitlog
saved_caches_directory: /var/lib/cassandra/saved_caches

# Performance
concurrent_reads: 32
concurrent_writes: 32
concurrent_counter_writes: 32

# Memory
memtable_heap_space_in_mb: 2048
memtable_offheap_space_in_mb: 2048

# Networking
native_transport_port: 9042
storage_port: 7000
ssl_storage_port: 7001

# Authentication
authenticator: PasswordAuthenticator
authorizer: CassandraAuthorizer

# Compaction
compaction_throughput_mb_per_sec: 64
```

## Data Modeling

### Partition key design (critical!)
```sql
-- ❌ Bad: Single partition (hot spot)
CREATE TABLE logs (
    log_level TEXT,
    timestamp TIMESTAMP,
    message TEXT,
    PRIMARY KEY (log_level, timestamp)
);  -- All ERROR logs in one partition!

-- ✅ Good: Distributed partitions
CREATE TABLE logs (
    log_level TEXT,
    date DATE,
    timestamp TIMESTAMP,
    message TEXT,
    PRIMARY KEY ((log_level, date), timestamp)
);  -- Partitions by level AND date
```

### Denormalization (required in Cassandra)
```sql
-- Query: Get user's posts
CREATE TABLE posts_by_user (
    user_id UUID,
    post_id UUID,
    title TEXT,
    content TEXT,
    created_at TIMESTAMP,
    PRIMARY KEY (user_id, created_at, post_id)
) WITH CLUSTERING ORDER BY (created_at DESC);

-- Query: Get recent posts globally
CREATE TABLE posts_by_time (
    bucket TEXT,  -- e.g., "2024-01"
    created_at TIMESTAMP,
    post_id UUID,
    user_id UUID,
    title TEXT,
    content TEXT,
    PRIMARY KEY (bucket, created_at, post_id)
) WITH CLUSTERING ORDER BY (created_at DESC);

-- Same data, multiple tables for different queries!
```

## Consistency Levels

### Read consistency
```sql
-- ONE: Fastest, least consistent (any 1 replica)
SELECT * FROM users WHERE user_id = ? USING CONSISTENCY ONE;

-- QUORUM: Balanced (majority of replicas, RF/2 + 1)
SELECT * FROM users WHERE user_id = ? USING CONSISTENCY QUORUM;

-- ALL: Slowest, most consistent (all replicas)
SELECT * FROM users WHERE user_id = ? USING CONSISTENCY ALL;

-- LOCAL_QUORUM: Quorum within local datacenter
SELECT * FROM users WHERE user_id = ? USING CONSISTENCY LOCAL_QUORUM;
```

### Write consistency
```sql
-- Fast writes
INSERT INTO users (...) VALUES (...) USING CONSISTENCY ONE;

-- Durable writes
INSERT INTO users (...) VALUES (...) USING CONSISTENCY QUORUM;
```

### Strong consistency formula
```
Write CL + Read CL > Replication Factor = Strong Consistency

Example with RF=3:
  QUORUM (2) + QUORUM (2) > 3 ✓ Strongly consistent
  ONE (1) + ONE (1) > 3 ✗ Eventually consistent
```

## Cluster Management

### nodetool commands
```bash
# Cluster status
nodetool status

# Ring information
nodetool ring

# Node information
nodetool info

# Repair (anti-entropy)
nodetool repair
nodetool repair -pr  # Primary range only

# Cleanup (after adding nodes)
nodetool cleanup

# Compact SSTables
nodetool compact

# Flush memtables to disk
nodetool flush

# Decommission node (graceful removal)
nodetool decommission

# Remove dead node
nodetool removenode <host-id>

# Drain (before shutdown)
nodetool drain

# JMX metrics
nodetool tpstats      # Thread pool stats
nodetool cfstats      # Table stats
nodetool proxyhistograms  # Latency histograms
```

### Adding nodes
```bash
# 1. Install Cassandra on new node
# 2. Configure cassandra.yaml (same cluster name, seed nodes)
# 3. Start Cassandra
systemctl start cassandra

# 4. Verify node joined
nodetool status

# 5. Run cleanup on OLD nodes (remove data that migrated)
nodetool cleanup
```

## Backup and Restore

### Snapshot backup
```bash
# Create snapshot
nodetool snapshot -t backup-20240208

# Snapshot location
ls /var/lib/cassandra/data/keyspace/table-uuid/snapshots/backup-20240208/

# Copy snapshots to backup location
rsync -av /var/lib/cassandra/data/ backup-server:/backups/

# Clear old snapshots
nodetool clearsnapshot
```

### Restore from snapshot
```bash
# 1. Stop Cassandra
systemctl stop cassandra

# 2. Clear current data
rm -rf /var/lib/cassandra/data/keyspace/table-uuid/*.db

# 3. Copy snapshot files
cp /backup/snapshots/*.db /var/lib/cassandra/data/keyspace/table-uuid/

# 4. Start Cassandra
systemctl start cassandra

# 5. Repair to ensure consistency
nodetool repair
```

## Performance Tuning

### JVM heap size
```bash
# /etc/cassandra/jvm.options
-Xms8G  # Min heap (set to same as max)
-Xmx8G  # Max heap (25% of RAM, max 32GB)
```

### OS tuning
```bash
# Disable swap
sudo swapoff -a

# Increase file descriptors
ulimit -n 100000

# TCP tuning
sysctl -w net.ipv4.tcp_keepalive_time=60
sysctl -w net.ipv4.tcp_keepalive_intvl=10
```

### Compaction strategies
```sql
-- Size-tiered (default, write-heavy)
ALTER TABLE users WITH compaction = {
    'class': 'SizeTieredCompactionStrategy'
};

-- Leveled (read-heavy, more consistent performance)
ALTER TABLE users WITH compaction = {
    'class': 'LeveledCompactionStrategy'
};

-- Time-window (time-series data)
ALTER TABLE metrics WITH compaction = {
    'class': 'TimeWindowCompactionStrategy',
    'compaction_window_unit': 'DAYS',
    'compaction_window_size': 1
};
```

## Use Cases

### Time-Series Data
```yaml
- name: Setup Cassandra for IoT
  shell: |
    cqlsh << 'EOF'
    CREATE KEYSPACE iot
    WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 3};

    USE iot;

    CREATE TABLE sensor_readings (
      sensor_id UUID,
      day DATE,
      timestamp TIMESTAMP,
      temperature DOUBLE,
      humidity DOUBLE,
      battery DOUBLE,
      PRIMARY KEY ((sensor_id, day), timestamp)
    ) WITH CLUSTERING ORDER BY (timestamp DESC)
      AND compaction = {'class': 'TimeWindowCompactionStrategy'};

    CREATE INDEX ON sensor_readings (sensor_id);
    EOF
```

### User Sessions
```yaml
- name: Create sessions table
  shell: |
    cqlsh -e "
    CREATE TABLE sessions (
      session_id UUID PRIMARY KEY,
      user_id UUID,
      created_at TIMESTAMP,
      last_activity TIMESTAMP,
      data MAP<TEXT, TEXT>
    ) WITH default_time_to_live = 86400;  -- Auto-expire after 24 hours
    "
```

### Event Logging
```yaml
- name: Create event log
  shell: |
    cqlsh -e "
    CREATE TABLE events (
      application TEXT,
      date DATE,
      timestamp TIMESTAMP,
      event_type TEXT,
      user_id UUID,
      data TEXT,
      PRIMARY KEY ((application, date), timestamp)
    ) WITH CLUSTERING ORDER BY (timestamp DESC);
    "
```

## Monitoring

### Key metrics
```bash
# Read/Write latency
nodetool proxyhistograms

# Pending tasks
nodetool tpstats | grep -E 'Read|Write|Mutation'

# Disk usage
nodetool tablestats keyspace.table

# Compaction stats
nodetool compactionstats

# GC stats
nodetool gcstats
```

### Health checks
```bash
# Node status (UN = Up Normal)
nodetool status | grep UN

# Check if cluster is healthy
nodetool describecluster
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Cassandra
  preset: cassandra
```

### Production cluster
```yaml
- name: Deploy Cassandra cluster
  hosts: cassandra-nodes
  vars:
    cluster_name: "production"
    seed_nodes: "10.0.1.10,10.0.1.11,10.0.1.12"
  tasks:
    - preset: cassandra
      become: true

    - name: Configure cluster
      template:
        src: cassandra.yaml.j2
        dest: /etc/cassandra/cassandra.yaml
      become: true

    - name: Set JVM heap
      lineinfile:
        path: /etc/cassandra/jvm.options
        regexp: '^-Xms'
        line: '-Xms{{ heap_size }}'
      become: true

    - name: Start Cassandra
      service:
        name: cassandra
        state: started
        enabled: true
      become: true
```

## Agent Use
- **Time-series storage**: IoT sensor data, metrics, logs at scale
- **User sessions**: High-throughput session storage with TTL
- **Event sourcing**: Append-only event logs with time-based queries
- **Recommendation systems**: User activity tracking and analysis
- **Messaging systems**: Message queues with guaranteed delivery
- **Financial transactions**: Audit logs and transaction history
- **Content management**: High-availability content delivery

## Troubleshooting

### Node won't start
```bash
# Check logs
tail -f /var/log/cassandra/system.log

# Common issues:
# 1. Port already in use
netstat -tulpn | grep -E '9042|7000|7199'

# 2. Insufficient memory
free -h

# 3. Disk full
df -h /var/lib/cassandra
```

### Slow queries
```bash
# Enable query logging
nodetool settraceprobability 0.001  # Log 0.1% of queries

# Check slow queries
tail -f /var/log/cassandra/system.log | grep SlowQuery

# Analyze query plan
cqlsh -e "TRACING ON; SELECT * FROM table WHERE ...;"
```

### High disk usage
```bash
# Check table sizes
nodetool tablestats | grep -E 'Table:|Space'

# Run compaction
nodetool compact

# Clear old snapshots
nodetool clearsnapshot
```

### Node out of sync
```bash
# Repair specific table
nodetool repair keyspace table

# Full repair (expensive!)
nodetool repair -full
```

### Connection refused
```bash
# Check if Cassandra is running
systemctl status cassandra

# Check network binding
netstat -tulpn | grep 9042

# Test connection
cqlsh localhost 9042
```

## Best Practices
- **Partition size**: Keep partitions under 100MB (ideally 10-50MB)
- **Replication factor**: Use RF=3 for production
- **Consistency**: Use QUORUM for balanced consistency/availability
- **Data modeling**: Query-first design, denormalize for reads
- **Avoid ALLOW FILTERING**: Indicates missing index or bad data model
- **Use prepared statements**: Better performance, prevents injection
- **Monitor compaction**: Ensure compactions keep up with writes
- **Regular repairs**: Run weekly to maintain consistency
- **Backup strategy**: Daily snapshots + incremental backups

## Anti-Patterns
- ❌ Using secondary indexes on high-cardinality columns
- ❌ Reading before writing (read-modify-write)
- ❌ Using SELECT * without WHERE clause
- ❌ Large partitions (>100MB)
- ❌ Counter columns in high-throughput scenarios
- ❌ Relational modeling (joins, normalization)

## Uninstall
```yaml
- preset: cassandra
  with:
    state: absent
```

**Warning**: This removes Cassandra but preserves data in `/var/lib/cassandra`. Delete manually if needed.

## Resources
- Official: https://cassandra.apache.org/
- Documentation: https://cassandra.apache.org/doc/latest/
- DataStax Academy: https://academy.datastax.com/
- GitHub: https://github.com/apache/cassandra
- Mailing List: https://cassandra.apache.org/community/
- Search: "cassandra data modeling", "cassandra performance tuning", "cassandra cluster setup"

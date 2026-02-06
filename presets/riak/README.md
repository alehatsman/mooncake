# Riak - Distributed Key-Value Store

Distributed NoSQL database designed for high availability, fault tolerance, and operational simplicity.

## Quick Start

```yaml
- preset: riak
```

## Features

- **Always available**: Multi-datacenter replication with no single point of failure
- **Fault tolerant**: Continues operating during hardware failures
- **Eventually consistent**: Tunable consistency with conflict resolution
- **Masterless**: All nodes are equal, no primary/replica distinction
- **Scalable**: Linear scale-out by adding nodes
- **Flexible data model**: Key-value store with rich secondary indexes
- **Cross-platform**: Linux, Docker support

## Basic Usage

```bash
# Check version
riak version

# Start Riak node
riak start

# Check node status
riak-admin status

# Create bucket type
riak-admin bucket-type create my_type '{"props":{"n_val":3}}'
riak-admin bucket-type activate my_type

# HTTP API - store value
curl -X PUT http://localhost:8098/types/my_type/buckets/test/keys/key1 \
  -H 'Content-Type: text/plain' \
  -d 'Hello Riak'

# HTTP API - retrieve value
curl http://localhost:8098/types/my_type/buckets/test/keys/key1

# Protocol Buffers API (faster)
riak-shell
```

## Advanced Configuration

```yaml
# Install Riak
- preset: riak
  register: riak_result
  become: true

# Configure node
- name: Set node configuration
  template:
    src: riak.conf.j2
    dest: /etc/riak/riak.conf
  become: true

# Start Riak service
- name: Enable and start Riak
  service:
    name: riak
    state: started
    enabled: true
  become: true

# Verify node is running
- name: Check Riak status
  shell: riak-admin status
  register: status_check
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (`present` or `absent`) |

## Platform Support

- ✅ Linux (apt, yum, package install)
- ✅ Docker (official images available)
- ❌ macOS (limited support, use Docker)
- ❌ Windows (use Docker or WSL)

## Configuration

- **Config file**: `/etc/riak/riak.conf`
- **Data directory**: `/var/lib/riak/`
- **Log directory**: `/var/log/riak/`
- **HTTP API port**: 8098
- **Protocol Buffers port**: 8087
- **Cluster port**: 8099
- **Binary location**: `/usr/sbin/riak`

## Real-World Examples

### Distributed Session Store

```yaml
# Deploy Riak cluster for session storage
- preset: riak
  hosts: riak-cluster
  become: true

# Configure each node
- name: Setup Riak node
  template:
    src: riak.conf.j2
    dest: /etc/riak/riak.conf
  vars:
    node_name: "riak@{{ ansible_host }}"
    ring_size: 64
  become: true

# Join cluster
- name: Join nodes to cluster
  shell: riak-admin cluster join riak@{{ groups['riak-cluster'][0] }}
  when: inventory_hostname != groups['riak-cluster'][0]
  become: true

# Commit cluster plan
- name: Commit cluster changes
  shell: riak-admin cluster plan && riak-admin cluster commit
  when: inventory_hostname == groups['riak-cluster'][0]
  become: true

# Create session bucket type
- name: Create sessions bucket type
  shell: |
    riak-admin bucket-type create sessions '{"props":{"n_val":3,"allow_mult":false}}'
    riak-admin bucket-type activate sessions
  become: true
```

Python client example:

```python
import riak

# Connect to cluster
client = riak.RiakClient(
    nodes=[
        {'host': '10.0.1.1', 'pb_port': 8087},
        {'host': '10.0.1.2', 'pb_port': 8087},
        {'host': '10.0.1.3', 'pb_port': 8087}
    ]
)

# Store session
bucket = client.bucket_type('sessions').bucket('user_sessions')
session = bucket.new('user123', data={
    'user_id': 123,
    'login_time': '2024-01-01T12:00:00Z',
    'ip_address': '192.168.1.100'
})
session.store()

# Retrieve session
session = bucket.get('user123')
print(session.data)

# Delete session
session.delete()
```

### Content Delivery System

```yaml
# Deploy Riak for CDN metadata
- preset: riak
  hosts: cdn-backend
  become: true

# Create content bucket type with strong consistency
- name: Setup content bucket
  shell: |
    riak-admin bucket-type create content '{
      "props": {
        "n_val": 3,
        "consistent": true,
        "search_index": "content_index"
      }
    }'
    riak-admin bucket-type activate content
  become: true

# Enable Riak Search
- name: Create search index
  shell: |
    curl -X PUT http://localhost:8098/search/index/content_index \
      -H 'Content-Type: application/json' \
      -d '{"schema":"_yz_default"}'
  become: true
```

Store content metadata:

```python
import riak

client = riak.RiakClient(pb_port=8087)
bucket = client.bucket_type('content').bucket('videos')

# Store video metadata
video = bucket.new('video123', data={
    'title': 'Demo Video',
    'duration': 180,
    'format': 'mp4',
    'cdn_urls': [
        'https://cdn1.example.com/video123.mp4',
        'https://cdn2.example.com/video123.mp4'
    ],
    'tags': ['tutorial', 'demo']
})
video.store()

# Query by tag using secondary index
bucket.set_property('search_index', 'content_index')
results = client.fulltext_search('content_index', 'tags:tutorial')
```

### Multi-Datacenter Replication

```yaml
# Setup MDC replication
- name: Configure datacenter replication
  hosts: riak-leader-dc1
  tasks:
    - name: Setup replication to DC2
      shell: |
        riak-repl clustername DC1
        riak-repl connect 10.2.0.1:9080 DC2
        riak-repl realtime enable all
        riak-repl fullsync enable all
      become: true

# Monitor replication status
- name: Check replication status
  shell: riak-repl status
  register: repl_status
  become: true
```

### High-Availability Cache

```yaml
# Deploy Riak as distributed cache
- preset: riak
  become: true

# Create cache bucket with TTL
- name: Setup cache bucket
  shell: |
    riak-admin bucket-type create cache '{
      "props": {
        "n_val": 2,
        "allow_mult": false,
        "last_write_wins": true,
        "dvv_enabled": false
      }
    }'
    riak-admin bucket-type activate cache
  become: true
```

Ruby client example:

```ruby
require 'riak'

# Connect
client = Riak::Client.new(nodes: [
  {host: 'localhost', pb_port: 8087}
])

# Store with expiry
bucket = client.bucket_type('cache').bucket('user_cache')
obj = bucket.new('user123')
obj.data = {name: 'John', email: 'john@example.com'}
obj.content_type = 'application/json'
obj.store

# Retrieve
obj = bucket.get('user123')
puts obj.data

# Delete
obj.delete
```

## Cluster Management

```bash
# View cluster status
riak-admin cluster status
riak-admin ring-status
riak-admin member-status

# Add node to cluster
riak-admin cluster join riak@new-node.example.com

# Review and commit plan
riak-admin cluster plan
riak-admin cluster commit

# Remove node
riak-admin cluster leave riak@old-node.example.com
riak-admin cluster plan
riak-admin cluster commit

# Transfer node
riak-admin transfer-limit 4
riak-admin transfers
```

## Bucket Types

```bash
# Create bucket type with custom properties
riak-admin bucket-type create my_type '{
  "props": {
    "n_val": 3,
    "allow_mult": false,
    "last_write_wins": true,
    "r": 2,
    "w": 2,
    "dw": 1,
    "pr": 0,
    "pw": 0
  }
}'

# Activate bucket type
riak-admin bucket-type activate my_type

# List bucket types
riak-admin bucket-type list

# Get bucket type properties
riak-admin bucket-type status my_type

# Update bucket type
riak-admin bucket-type update my_type '{"props":{"n_val":5}}'
```

## Secondary Indexes

```bash
# Store with secondary indexes
curl -X PUT http://localhost:8098/types/users/buckets/accounts/keys/user1 \
  -H 'Content-Type: application/json' \
  -H 'x-riak-index-email_bin: john@example.com' \
  -H 'x-riak-index-age_int: 30' \
  -d '{"name":"John","email":"john@example.com"}'

# Query by exact match
curl http://localhost:8098/types/users/buckets/accounts/index/email_bin/john@example.com

# Range query
curl http://localhost:8098/types/users/buckets/accounts/index/age_int/25/35
```

## Agent Use

- Session storage for web applications
- Distributed cache with high availability
- Content delivery network metadata storage
- Shopping cart and user preference storage
- Multi-datacenter data synchronization
- Time-series data collection and aggregation
- IoT device state management
- Real-time analytics data store

## Troubleshooting

### Node won't start

Check logs and configuration:

```bash
# View logs
tail -f /var/log/riak/console.log
tail -f /var/log/riak/error.log

# Check configuration
riak config effective

# Verify data directory permissions
ls -la /var/lib/riak

# Check port availability
lsof -i :8087
lsof -i :8098
```

### Nodes can't join cluster

Verify network connectivity:

```bash
# Test connectivity
nc -zv riak-node1.example.com 8099

# Check node name resolution
riak-admin member-status

# Verify Erlang distribution
epmd -names

# Check firewall
sudo iptables -L | grep 8087
```

### Data inconsistency

Resolve conflicts:

```bash
# Enable automatic sibling resolution
riak-admin bucket-type update my_type '{"props":{"allow_mult":true,"dvv_enabled":true}}'

# Manually resolve siblings via API
curl http://localhost:8098/types/my_type/buckets/test/keys/key1

# Use last-write-wins
riak-admin bucket-type update my_type '{"props":{"last_write_wins":true}}'
```

### Performance issues

Optimize configuration:

```bash
# Increase ring size (before cluster creation)
# Edit riak.conf: ring_size = 128

# Tune I/O settings
echo 'leveldb.maximum_memory.percent = 70' >> /etc/riak/riak.conf

# Monitor performance
riak-admin stat | grep latency
riak-admin diag
```

## Uninstall

```yaml
- preset: riak
  with:
    state: absent
```

**Note**: This removes Riak but preserves data in `/var/lib/riak/`.

## Resources

- Official docs: https://riak.com/docs/
- GitHub: https://github.com/basho/riak
- Python client: https://github.com/basho/riak-python-client
- Best practices: https://docs.riak.com/riak/kv/latest/using/performance/
- Search: "riak cluster setup", "riak replication", "riak performance tuning"

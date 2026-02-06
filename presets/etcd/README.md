# etcd Preset

Distributed reliable key-value store. Foundation for Kubernetes and distributed systems.

## Quick Start

```yaml
# Basic installation
- preset: etcd

# Production setup
- preset: etcd
  with:
    data_dir: /var/lib/etcd
    start_service: true
  become: true
```

## Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `version` | `latest` | etcd version |
| `data_dir` | `/var/lib/etcd` | Data directory |
| `client_port` | `2379` | Client port |
| `peer_port` | `2380` | Peer port |
| `start_service` | `false` | Auto-start service |

## Usage

```bash
# Set environment
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=http://localhost:2379

# Put key-value
etcdctl put mykey "Hello World"

# Get value
etcdctl get mykey

# Delete key
etcdctl del mykey

# List keys
etcdctl get "" --prefix --keys-only

# Watch key
etcdctl watch mykey

# Cluster health
etcdctl endpoint health

# Member list
etcdctl member list
```

## Examples

```bash
# Service discovery
etcdctl put /services/web "192.168.1.10:8080"
etcdctl get /services/ --prefix

# Configuration management
etcdctl put /config/app/db_host "localhost"
etcdctl put /config/app/db_port "5432"

# Distributed locking
etcdctl lock mylock command

# Lease management
etcdctl lease grant 60  # 60 second TTL
etcdctl put --lease=<lease-id> key value

# Transactions
etcdctl txn < transaction.txt

# Snapshot backup
etcdctl snapshot save backup.db
etcdctl snapshot restore backup.db
```

## Clustering

```yaml
# Node 1
- preset: etcd
  with:
    initial_cluster: "node1=http://10.0.1.10:2380,node2=http://10.0.1.11:2380,node3=http://10.0.1.12:2380"

# Join existing cluster
etcdctl member add node2 --peer-urls=http://10.0.1.11:2380
```

## Resources
- Docs: https://etcd.io/docs/
- Operations: https://etcd.io/docs/v3.5/op-guide/

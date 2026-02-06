# etcd - Distributed Key-Value Store

Distributed, reliable key-value store for the most critical data of distributed systems. Foundation for Kubernetes control plane and service discovery.

## Quick Start
```yaml
- preset: etcd
```

## Features
- **Distributed consensus**: Raft protocol for strong consistency
- **Reliable storage**: ACID guarantees for critical configuration data
- **Watch API**: Real-time notification of key changes
- **Kubernetes foundation**: Powers K8s cluster state storage
- **Cross-platform**: Linux and macOS support via package managers

## Basic Usage
```bash
# Set a key-value pair
etcdctl put mykey "hello world"

# Get a value
etcdctl get mykey

# Watch for changes
etcdctl watch mykey

# List all keys
etcdctl get "" --prefix

# Delete a key
etcdctl del mykey
```

## Advanced Configuration
```yaml
# Production cluster setup
- preset: etcd
  with:
    version: "3.5.12"
    data_dir: /var/lib/etcd
    client_port: "2379"
    peer_port: "2380"
    start_service: true
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove etcd |
| version | string | latest | Specific etcd version to install |
| data_dir | string | /var/lib/etcd | Data directory for etcd storage |
| client_port | string | 2379 | Port for client communication |
| peer_port | string | 2380 | Port for peer communication |
| start_service | bool | false | Automatically start etcd service after installation |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Data directory**: `/var/lib/etcd` (configurable)
- **Client endpoint**: `http://localhost:2379`
- **Peer endpoint**: `http://localhost:2380`
- **Config file**: `/etc/etcd/etcd.conf.yml` (if using service)
- **Environment**: Set `ETCDCTL_API=3` for v3 API

## Real-World Examples

### Kubernetes-compatible Single Node
```yaml
- preset: etcd
  with:
    data_dir: /var/lib/etcd
    client_port: "2379"
    peer_port: "2380"
    start_service: true
  become: true
```

### Service Discovery Backend
```bash
# Register a service
etcdctl put /services/web/node1 '{"ip":"192.168.1.10","port":8080}'

# Discover services
etcdctl get /services/web --prefix

# Watch for new services
etcdctl watch /services/ --prefix
```

### Configuration Store
```bash
# Store application config
etcdctl put /config/app/db_host "postgres.example.com"
etcdctl put /config/app/cache_ttl "3600"

# Read config
etcdctl get /config/app --prefix
```

## Agent Use
- Store and retrieve distributed configuration for microservices
- Service discovery and registration for container orchestration
- Leader election for distributed systems coordination
- Kubernetes cluster state storage (control plane dependency)
- Feature flag management with real-time updates via watch API
- Distributed lock coordination for job scheduling

## Troubleshooting

### Check if etcd is running
```bash
# Linux
systemctl status etcd
journalctl -u etcd -f

# macOS
brew services list | grep etcd

# Health check
etcdctl endpoint health
```

### Connection refused
Ensure etcd is listening on the correct interface:
```bash
# Check endpoints
etcdctl member list

# Set correct endpoint
export ETCDCTL_ENDPOINTS=http://127.0.0.1:2379
```

### Data directory permissions
```bash
# Fix permissions
sudo chown -R etcd:etcd /var/lib/etcd
sudo chmod 0700 /var/lib/etcd
```

## Uninstall
```yaml
- preset: etcd
  with:
    state: absent
  become: true
```

## Resources
- Official docs: https://etcd.io/docs/
- GitHub: https://github.com/etcd-io/etcd
- Kubernetes integration: https://kubernetes.io/docs/tasks/administer-cluster/configure-upgrade-etcd/
- Search: "etcd tutorial", "etcd kubernetes", "etcd service discovery"

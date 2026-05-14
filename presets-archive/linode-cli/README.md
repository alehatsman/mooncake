# linode-cli - Linode Cloud Platform CLI

Command-line interface for managing Linode cloud infrastructure including compute instances, volumes, networking, and DNS.

## Quick Start
```yaml
- preset: linode-cli
```

## Features
- **Full API coverage**: Manage all Linode resources
- **Interactive shell**: Context-aware command completion
- **JSON output**: Machine-readable formats for automation
- **Configuration profiles**: Multiple account support
- **Object Storage**: S3-compatible storage management
- **Kubernetes**: LKE cluster management

## Basic Usage
```bash
# Configure CLI (interactive)
linode-cli configure

# List linodes
linode-cli linodes list

# Create linode
linode-cli linodes create \
  --label my-server \
  --region us-east \
  --type g6-nanode-1 \
  --image linode/ubuntu22.04

# Reboot linode
linode-cli linodes reboot 12345678

# Delete linode
linode-cli linodes delete 12345678

# List volumes
linode-cli volumes list

# Create volume
linode-cli volumes create --label my-volume --size 20 --region us-east

# DNS management
linode-cli domains list
linode-cli domains records-list example.com

# Object storage
linode-cli object-storage buckets-list
```

## Advanced Configuration
```yaml
- preset: linode-cli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove linode-cli |

## Platform Support
- ✅ Linux (pip install)
- ✅ macOS (pip install, Homebrew)
- ✅ Windows (pip install)

## Configuration
- **Config file**: `~/.config/linode-cli` (Linux/macOS), `%USERPROFILE%\.config\linode-cli` (Windows)
- **API token**: Required, obtainable from Linode Cloud Manager
- **Multiple profiles**: Support for multiple accounts

## Real-World Examples

### Infrastructure Provisioning
```yaml
- name: Configure Linode CLI
  shell: |
    echo "{{ linode_token }}" | linode-cli configure --token
  environment:
    LINODE_CLI_TOKEN: "{{ linode_api_token }}"

- name: Create compute instance
  shell: |
    linode-cli linodes create \
      --label web-server-{{ inventory_hostname }} \
      --region us-east \
      --type g6-standard-2 \
      --image linode/ubuntu22.04 \
      --root_pass "{{ root_password }}" \
      --json
  register: linode_create

- name: Get instance IP
  shell: |
    echo "{{ linode_create.stdout }}" | jq -r '.[0].ipv4[0]'
  register: instance_ip
```

### DNS Management
```bash
# Add A record
linode-cli domains records-create example.com \
  --type A \
  --name www \
  --target 192.0.2.1 \
  --ttl_sec 300

# Update record
linode-cli domains records-update example.com 98765 \
  --target 192.0.2.2

# List all records
linode-cli domains records-list example.com --json
```

### LKE Cluster Management
```bash
# Create Kubernetes cluster
linode-cli lke cluster-create \
  --label my-cluster \
  --region us-east \
  --k8s_version 1.28 \
  --node_pools.type g6-standard-2 \
  --node_pools.count 3

# Get kubeconfig
linode-cli lke kubeconfig-view 12345 --json | jq -r '.[0].kubeconfig' | base64 -d > ~/.kube/config

# List clusters
linode-cli lke clusters-list
```

### Backup and Snapshot
```bash
# Enable backups
linode-cli linodes backups-enable 12345678

# Create snapshot
linode-cli linodes snapshot 12345678 --label backup-$(date +%Y%m%d)

# List backups
linode-cli linodes backups-list 12345678
```

## Agent Use
- Infrastructure provisioning automation
- DNS record management
- Kubernetes cluster deployment
- Resource monitoring and alerting
- Disaster recovery automation

## Troubleshooting

### Authentication failed
Configure with valid token:
```bash
linode-cli configure
# Or set environment variable
export LINODE_CLI_TOKEN=your_token_here
```

### Command not found
Verify installation:
```bash
pip show linode-cli
which linode-cli
```

### Region/type not available
List available options:
```bash
linode-cli regions list
linode-cli linodes types
```

## Uninstall
```yaml
- preset: linode-cli
  with:
    state: absent
```

## Resources
- Official docs: https://www.linode.com/docs/products/tools/cli/
- API reference: https://www.linode.com/docs/api/
- GitHub: https://github.com/linode/linode-cli
- Search: "linode cli tutorial", "linode api automation"

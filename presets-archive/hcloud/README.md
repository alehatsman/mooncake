# hcloud - Hetzner Cloud CLI

Command-line interface for managing Hetzner Cloud infrastructure and resources.

## Quick Start
```yaml
- preset: hcloud
```

## Features
- **Complete infrastructure control**: Servers, networks, volumes, load balancers
- **Fast operations**: Quick server creation and management
- **Cost-effective**: Manage affordable European cloud infrastructure
- **Powerful automation**: Full API access via CLI
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Authentication
export HCLOUD_TOKEN="your-api-token"

# Server management
hcloud server list
hcloud server create --name web01 --type cx11 --image ubuntu-22.04
hcloud server describe web01
hcloud server ssh web01
hcloud server delete web01

# Network operations
hcloud network list
hcloud network create --name private-net --ip-range 10.0.0.0/16
hcloud network delete private-net

# Volume management
hcloud volume list
hcloud volume create --name data-vol --size 10 --server web01
hcloud volume attach data-vol web01
hcloud volume detach data-vol

# Floating IP
hcloud floating-ip list
hcloud floating-ip create --type ipv4 --name web-ip
hcloud floating-ip assign web-ip web01
```

## Authentication Setup
```bash
# Set API token (get from Hetzner Cloud Console)
export HCLOUD_TOKEN="your-api-token-here"

# Or use context for multiple projects
hcloud context create production
hcloud context use production

# List contexts
hcloud context list

# Active context
hcloud context active
```

## Server Management

### Create and Configure Server
```bash
# Create server with all options
hcloud server create \
  --name prod-web-01 \
  --type cx21 \
  --image ubuntu-22.04 \
  --location nbg1 \
  --ssh-key my-key \
  --network private-net

# Available server types (pricing tiers)
hcloud server-type list

# Available images
hcloud image list --type system

# Available locations (datacenters)
hcloud location list
```

### Server Operations
```bash
# Power operations
hcloud server poweroff web01
hcloud server poweron web01
hcloud server reboot web01
hcloud server reset web01

# Resize server
hcloud server change-type web01 --upgrade-disk --server-type cx31

# Change image (rebuild)
hcloud server rebuild web01 --image ubuntu-22.04

# Enable rescue mode
hcloud server enable-rescue web01
hcloud server reboot web01  # Boot into rescue

# Disable rescue
hcloud server disable-rescue web01
```

## Advanced Configuration
```yaml
- preset: hcloud
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove hcloud CLI |

## Real-World Examples

### Deploy Web Application
```yaml
- name: Create Hetzner Cloud infrastructure
  shell: |
    # Create network
    hcloud network create --name app-net --ip-range 10.0.0.0/16
    hcloud network add-subnet app-net --type cloud --network-zone eu-central --ip-range 10.0.1.0/24

    # Create load balancer
    hcloud load-balancer create \
      --name lb-web \
      --type lb11 \
      --location nbg1 \
      --network-zone eu-central

    # Create web servers
    for i in {1..3}; do
      hcloud server create \
        --name "web0$i" \
        --type cx21 \
        --image ubuntu-22.04 \
        --network app-net \
        --ssh-key production
    done
  register: infra
```

### Automated Backup and Snapshot
```bash
# Enable automatic backups
hcloud server enable-backup web01 --window "22-02"

# Create manual snapshot
hcloud server create-image web01 --description "pre-deployment-snapshot"

# List images/snapshots
hcloud image list --type snapshot

# Restore from snapshot
hcloud server rebuild web01 --image <snapshot-id>
```

### CI/CD Server Provisioning
```bash
# Create ephemeral test server
hcloud server create \
  --name "ci-${CI_JOB_ID}" \
  --type cx11 \
  --image ubuntu-22.04 \
  --ssh-key ci-key

# Run tests (SSH or commands)
hcloud server ssh "ci-${CI_JOB_ID}" "make test"

# Cleanup
hcloud server delete "ci-${CI_JOB_ID}"
```

## Network Configuration

### Private Networks
```bash
# Create private network
hcloud network create --name k8s-net --ip-range 10.0.0.0/16

# Add subnet
hcloud network add-subnet k8s-net \
  --type cloud \
  --network-zone eu-central \
  --ip-range 10.0.1.0/24

# Attach server to network
hcloud server attach-to-network web01 --network k8s-net --ip 10.0.1.5

# Detach from network
hcloud server detach-from-network web01 --network k8s-net
```

### Load Balancers
```bash
# Create load balancer
hcloud load-balancer create \
  --name web-lb \
  --type lb11 \
  --location nbg1

# Add service
hcloud load-balancer add-service web-lb \
  --protocol http \
  --listen-port 80 \
  --destination-port 80

# Add target servers
hcloud load-balancer add-target web-lb --server web01
hcloud load-balancer add-target web-lb --server web02

# Health check
hcloud load-balancer update-service web-lb \
  --http-path /health \
  --http-interval 15s \
  --http-timeout 10s
```

## Volumes and Storage
```bash
# Create volume
hcloud volume create --name db-data --size 50 --format ext4

# Attach to server
hcloud volume attach db-data db01

# Mount (on server)
mkdir /mnt/data
mount /dev/disk/by-id/scsi-0HC_Volume_* /mnt/data

# Resize volume
hcloud volume resize db-data --size 100

# Detach and delete
hcloud volume detach db-data
hcloud volume delete db-data
```

## Configuration
- **Config file**: `~/.config/hcloud/cli.toml`
- **Token storage**: Stored in config file or environment variable
- **Context**: Multiple projects via context switching
- **API endpoint**: https://api.hetzner.cloud/v1

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported in preset)

## Agent Use
- Provision infrastructure for deployments
- Scale server fleets dynamically
- Manage development/staging environments
- Automate backup and snapshot workflows
- Implement disaster recovery procedures
- Cost optimization via automated cleanup

## Troubleshooting

### Authentication errors
```bash
# Verify token is set
echo $HCLOUD_TOKEN

# Test API access
hcloud server list

# Check context
hcloud context active
```

### Rate limiting
```bash
# Hetzner has rate limits - add delays in loops
for server in $(hcloud server list -o noheader -o columns=name); do
  hcloud server delete "$server"
  sleep 1
done
```

### Network connectivity
```bash
# Verify server network setup
hcloud server describe web01

# Check private IP assignment
hcloud server describe web01 | grep -A 10 "Private Networks"

# Test connectivity from server
hcloud server ssh web01 "ping -c 3 10.0.1.5"
```

## Uninstall
```yaml
- preset: hcloud
  with:
    state: absent
```

## Resources
- Official docs: https://docs.hetzner.com/cloud/cli/
- API reference: https://docs.hetzner.cloud/
- GitHub: https://github.com/hetznercloud/cli
- Search: "hetzner cloud cli tutorial", "hcloud examples"

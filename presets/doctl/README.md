# doctl - DigitalOcean CLI

Official command-line tool for managing DigitalOcean resources.

## Quick Start
```yaml
- preset: doctl
```

## Features
- **Droplets**: Create and manage virtual machines
- **Kubernetes**: Manage DigitalOcean Kubernetes clusters
- **Databases**: Manage managed databases
- **Load Balancers**: Configure load balancing
- **Networking**: VPCs, firewalls, floating IPs
- **Object Storage**: Manage Spaces (S3-compatible storage)

## Basic Usage
```bash
# Authenticate
doctl auth init

# List droplets
doctl compute droplet list

# Create droplet
doctl compute droplet create my-droplet \
  --image ubuntu-22-04-x64 \
  --size s-1vcpu-1gb \
  --region nyc3

# SSH into droplet
doctl compute ssh my-droplet

# Delete droplet
doctl compute droplet delete my-droplet

# List Kubernetes clusters
doctl kubernetes cluster list

# Get kubeconfig
doctl kubernetes cluster kubeconfig save my-cluster

# List databases
doctl databases list

# Create database
doctl databases create my-db --engine pg --region nyc3

# List Spaces
doctl compute cdn list
```

## Advanced Configuration
```yaml
# Basic install
- preset: doctl

# Uninstall
- preset: doctl
  with:
    state: absent
```

## Authentication Setup
```bash
# Initialize authentication
doctl auth init
# Enter your DigitalOcean API token when prompted

# List available authentication contexts
doctl auth list

# Switch between contexts
doctl auth switch --context my-context

# Use specific context
doctl --context production compute droplet list
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Real-World Examples

### Deploy Web Application
```bash
# Create droplet
doctl compute droplet create web-app \
  --image ubuntu-22-04-x64 \
  --size s-2vcpu-4gb \
  --region nyc3 \
  --ssh-keys $(doctl compute ssh-key list --format ID --no-header)

# Wait for droplet to be ready
doctl compute droplet list --format Name,Status,PublicIPv4

# Deploy via SSH
doctl compute ssh web-app --ssh-command "bash -s" < deploy.sh
```

### Kubernetes Cluster Management
```bash
# Create cluster
doctl kubernetes cluster create prod-cluster \
  --region nyc3 \
  --version 1.28.2-do.0 \
  --node-pool "name=workers;size=s-2vcpu-4gb;count=3"

# Get credentials
doctl kubernetes cluster kubeconfig save prod-cluster

# Scale node pool
doctl kubernetes cluster node-pool update prod-cluster \
  --name workers --count 5

# Upgrade cluster
doctl kubernetes cluster upgrade prod-cluster --version 1.29.0-do.0
```

### Database Provisioning
```bash
# Create PostgreSQL database
doctl databases create prod-db \
  --engine pg \
  --version 15 \
  --region nyc3 \
  --size db-s-2vcpu-4gb

# Get connection details
doctl databases get prod-db --format ID,Name,Engine,Host,Port

# Create database user
doctl databases user create prod-db app-user

# Get credentials
doctl databases user get prod-db app-user
```

### Load Balancer Setup
```bash
# Create load balancer
doctl compute load-balancer create \
  --name web-lb \
  --region nyc3 \
  --forwarding-rules "entry_protocol:https,entry_port:443,target_protocol:http,target_port:80,certificate_id:$CERT_ID" \
  --droplet-ids $(doctl compute droplet list --format ID --no-header | tr '\n' ',')

# Update health check
doctl compute load-balancer update web-lb \
  --health-check "protocol:http,port:80,path:/health"
```

## CI/CD Integration
```bash
# Set API token in environment
export DIGITALOCEAN_ACCESS_TOKEN="your-token"

# Deploy infrastructure
doctl compute droplet create staging-$CI_COMMIT_SHA \
  --image ubuntu-22-04-x64 \
  --size s-1vcpu-1gb \
  --region nyc3 \
  --user-data-file cloud-init.yml

# Wait for ready
until doctl compute droplet get staging-$CI_COMMIT_SHA --format Status --no-header | grep -q active; do
  sleep 5
done

# Get IP and deploy
IP=$(doctl compute droplet get staging-$CI_COMMIT_SHA --format PublicIPv4 --no-header)
ssh root@$IP "bash -s" < deploy.sh
```

## Configuration
```bash
# Config file location
~/.config/doctl/config.yaml

# Example config
access-token: your-token-here
output: text  # or json, table
```

## Common Commands Reference

### Droplets
```bash
doctl compute droplet list                    # List all droplets
doctl compute droplet get <id>                # Get droplet details
doctl compute droplet create <name>           # Create droplet
doctl compute droplet delete <id>             # Delete droplet
doctl compute droplet-action reboot <id>      # Reboot droplet
doctl compute droplet-action snapshot <id>    # Create snapshot
```

### Kubernetes
```bash
doctl kubernetes cluster list                 # List clusters
doctl kubernetes cluster get <id>             # Get cluster details
doctl kubernetes cluster create <name>        # Create cluster
doctl kubernetes cluster delete <id>          # Delete cluster
doctl kubernetes cluster kubeconfig save <id> # Download kubeconfig
```

### Databases
```bash
doctl databases list                          # List databases
doctl databases get <id>                      # Get database details
doctl databases create <name>                 # Create database
doctl databases delete <id>                   # Delete database
doctl databases backups list <id>             # List backups
```

### Networking
```bash
doctl compute firewall list                   # List firewalls
doctl compute floating-ip list                # List floating IPs
doctl vpcs list                               # List VPCs
doctl compute load-balancer list              # List load balancers
```

## Agent Use
- Automate DigitalOcean infrastructure provisioning
- Deploy applications to droplets
- Manage Kubernetes clusters
- Configure load balancers and networking
- Database lifecycle management
- CI/CD deployment pipelines

## Troubleshooting

### Authentication failed
```bash
# Verify token
doctl auth list

# Re-initialize
doctl auth init

# Test connection
doctl account get
```

### Rate limiting
```bash
# Check rate limit status
doctl compute action list --format ID,Type,Status | head -1

# Wait between API calls in scripts
sleep 1
```

## Uninstall
```yaml
- preset: doctl
  with:
    state: absent
```

## Resources
- Official docs: https://docs.digitalocean.com/reference/doctl/
- API reference: https://docs.digitalocean.com/reference/api/
- Search: "doctl tutorial", "digitalocean cli examples"

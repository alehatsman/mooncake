# Vultr-CLI - Vultr Cloud Management Tool

Official command-line interface for managing Vultr cloud resources including instances, DNS, snapshots, and more.

## Quick Start
```yaml
- preset: vultr-cli
```

## Features
- **Instance Management**: Create, list, delete, and manage cloud instances
- **DNS Management**: Manage DNS records and domains
- **Snapshot Management**: Create and restore server snapshots
- **Load Balancers**: Configure and manage load balancers
- **Block Storage**: Manage block storage volumes
- **Cross-platform**: Works on Linux, macOS, and Windows

## Basic Usage
```bash
# Configure API key
export VULTR_API_KEY="your-api-key-here"

# List all instances
vultr-cli instance list

# Create instance
vultr-cli instance create \
  --region ewr \
  --plan vc2-1c-1gb \
  --os 387 \
  --label my-server

# Get instance details
vultr-cli instance get <instance-id>

# Delete instance
vultr-cli instance delete <instance-id>

# List regions
vultr-cli regions list

# List available plans
vultr-cli plans list

# List OS images
vultr-cli os list
```

## Advanced Configuration
```yaml
- preset: vultr-cli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Vultr CLI |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (binary download, Homebrew)
- ✅ Windows (binary download)

## Configuration
- **API Key**: Set via `VULTR_API_KEY` environment variable
- **Config file**: `~/.vultr-cli.yaml` (optional)
- **API endpoint**: https://api.vultr.com/v2

## Authentication

Get API key from Vultr dashboard:
```bash
# Set API key (add to shell profile for persistence)
export VULTR_API_KEY="your-api-key-here"

# Or use config file
cat > ~/.vultr-cli.yaml <<EOF
api-key: your-api-key-here
EOF

# Verify authentication
vultr-cli account
```

## Real-World Examples

### Deploy Server Instance
```bash
# Create Ubuntu server
vultr-cli instance create \
  --region ewr \
  --plan vc2-2c-4gb \
  --os 387 \
  --label web-server \
  --hostname web01.example.com \
  --enable-ipv6 \
  --ssh-keys "ssh-key-id"

# Wait for instance to be ready
vultr-cli instance get <instance-id>

# Get instance IP
vultr-cli instance get <instance-id> | grep main_ip
```

### DNS Management
```bash
# Create DNS domain
vultr-cli dns domain create --domain example.com

# Add A record
vultr-cli dns record create \
  --domain example.com \
  --type A \
  --name www \
  --data 203.0.113.1 \
  --ttl 300

# List records
vultr-cli dns record list --domain example.com

# Update record
vultr-cli dns record update \
  --domain example.com \
  --record-id <id> \
  --data 203.0.113.2
```

### Snapshot Management
```bash
# Create snapshot
vultr-cli snapshot create \
  --instance-id <instance-id> \
  --description "Backup before upgrade"

# List snapshots
vultr-cli snapshot list

# Restore from snapshot
vultr-cli instance restore --instance-id <instance-id> --snapshot-id <snapshot-id>

# Delete snapshot
vultr-cli snapshot delete <snapshot-id>
```

### CI/CD Integration
```yaml
- name: Install Vultr CLI
  preset: vultr-cli

- name: Deploy application server
  shell: |
    vultr-cli instance create \
      --region {{ region }} \
      --plan {{ plan }} \
      --os 387 \
      --label {{ app_name }}-{{ env }} \
      --ssh-keys {{ ssh_key_id }} \
      --script-id {{ startup_script_id }}
  env:
    VULTR_API_KEY: "{{ vultr_api_key }}"
  register: instance

- name: Wait for instance
  shell: |
    while ! vultr-cli instance get {{ instance.stdout }} | grep -q "active"; do
      sleep 5
    done
  env:
    VULTR_API_KEY: "{{ vultr_api_key }}"

- name: Configure DNS
  shell: |
    vultr-cli dns record create \
      --domain {{ domain }} \
      --type A \
      --name {{ app_name }} \
      --data $(vultr-cli instance get {{ instance.stdout }} | grep main_ip | awk '{print $2}')
  env:
    VULTR_API_KEY: "{{ vultr_api_key }}"
```

### Load Balancer Setup
```bash
# Create load balancer
vultr-cli load-balancer create \
  --region ewr \
  --label production-lb \
  --balancing-algorithm roundrobin \
  --forwarding-rules "protocol:http,port:80,target_port:8080"

# Attach instances
vultr-cli load-balancer instance attach \
  --load-balancer-id <lb-id> \
  --instance-id <instance-id>

# List load balancers
vultr-cli load-balancer list
```

## Agent Use
- Automated cloud infrastructure provisioning
- Server deployment and scaling
- DNS record management and updates
- Backup and snapshot automation
- CI/CD pipeline infrastructure creation
- Multi-region deployment orchestration

## Troubleshooting

### API authentication errors
```bash
# Verify API key is set
echo $VULTR_API_KEY

# Test authentication
vultr-cli account

# Check API key permissions in Vultr dashboard
# Ensure key has necessary scopes
```

### Instance creation failures
```bash
# Verify region is correct
vultr-cli regions list

# Check available plans in region
vultr-cli plans list --region ewr

# Verify OS ID
vultr-cli os list

# Check account limits
vultr-cli account
```

### Rate limiting
```bash
# Vultr API has rate limits
# Add delays between requests
sleep 2
vultr-cli instance list
```

## Uninstall
```yaml
- preset: vultr-cli
  with:
    state: absent
```

## Resources
- Official docs: https://www.vultr.com/docs/vultr-cli
- GitHub: https://github.com/vultr/vultr-cli
- API docs: https://www.vultr.com/api/
- Search: "vultr cli tutorial", "vultr api examples"

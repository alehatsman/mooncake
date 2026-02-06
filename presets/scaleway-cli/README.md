# Scaleway CLI - Cloud Provider Command-Line Interface

Manage Scaleway cloud resources from the terminal. Create instances, configure networks, manage object storage, deploy containers.

## Quick Start
```yaml
- preset: scaleway-cli
```

## Features
- **Complete cloud management**: Instances, storage, networking, databases
- **Multiple regions**: Par1, Ams1, War1
- **Interactive commands**: Guided setup and configuration
- **Output formats**: JSON, YAML, table, human-readable
- **Script-friendly**: Exit codes and structured output
- **Auto-completion**: Bash, Zsh, Fish support
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Initialize CLI
scw init

# List instances
scw instance server list

# Create instance
scw instance server create type=DEV1-S name=myserver image=ubuntu-focal

# List images
scw instance image list

# Get account info
scw account project list
```

## Advanced Configuration
```yaml
# Install Scaleway CLI (default)
- preset: scaleway-cli

# Uninstall Scaleway CLI
- preset: scaleway-cli
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, manual install)
- ✅ Windows (Chocolatey, manual install)

## Configuration
- **Config file**: `~/.config/scw/config.yaml`
- **Default region**: Configurable per project
- **Authentication**: API access key and secret key
- **Profiles**: Multiple account support

## Authentication
```bash
# Interactive setup
scw init

# Manual config
scw config set access-key=SCWXXXXXXXXXXXXXXXXX
scw config set secret-key=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
scw config set default-organization-id=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
scw config set default-region=fr-par
scw config set default-zone=fr-par-1

# Use environment variables
export SCW_ACCESS_KEY="SCWXXXXXXXXXXXXXXXXX"
export SCW_SECRET_KEY="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export SCW_DEFAULT_ORGANIZATION_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

## Instance Management
```bash
# List instances
scw instance server list

# Create instance
scw instance server create \
  type=DEV1-S \
  name=myserver \
  image=ubuntu-focal \
  zone=fr-par-1

# Start/stop instance
scw instance server start <server-id>
scw instance server stop <server-id>

# Reboot instance
scw instance server reboot <server-id>

# Delete instance
scw instance server delete <server-id>

# Get instance details
scw instance server get <server-id>

# List available types
scw instance server-type list
```

## Object Storage
```bash
# Create bucket
scw object bucket create name=mybucket region=fr-par

# List buckets
scw object bucket list

# Upload file
scw object put mybucket/file.txt < file.txt

# Download file
scw object get mybucket/file.txt > file.txt

# List objects
scw object ls mybucket/

# Delete object
scw object rm mybucket/file.txt

# Delete bucket
scw object bucket delete name=mybucket
```

## Networking
```bash
# List IPs
scw instance ip list

# Reserve IP
scw instance ip create zone=fr-par-1

# Attach IP to server
scw instance ip attach <ip-id> server-id=<server-id>

# Security groups
scw instance security-group list
scw instance security-group create name=mygroup

# Add rule
scw instance security-group-rule create \
  security-group-id=<group-id> \
  action=accept \
  direction=inbound \
  ip-range=0.0.0.0/0 \
  protocol=TCP \
  dest-port-from=22 \
  dest-port-to=22
```

## Databases
```bash
# List databases
scw rdb instance list

# Create PostgreSQL instance
scw rdb instance create \
  engine=PostgreSQL-13 \
  node-type=db-dev-s \
  name=mydb \
  user-name=admin \
  password=MySecurePass123

# Get connection info
scw rdb instance get <instance-id>

# Create backup
scw rdb backup create instance-id=<instance-id>

# List backups
scw rdb backup list
```

## Container Registry
```bash
# Create namespace
scw registry namespace create name=myregistry

# List namespaces
scw registry namespace list

# Get Docker credentials
scw registry login

# Tag and push image
docker tag myapp:latest rg.fr-par.scw.cloud/myregistry/myapp:latest
docker push rg.fr-par.scw.cloud/myregistry/myapp:latest

# List images
scw registry image list namespace-id=<namespace-id>
```

## Kubernetes
```bash
# List clusters
scw k8s cluster list

# Create cluster
scw k8s cluster create \
  name=mycluster \
  version=1.28 \
  cni=cilium \
  zone=fr-par-1

# Get kubeconfig
scw k8s kubeconfig get <cluster-id> > kubeconfig.yaml
export KUBECONFIG=./kubeconfig.yaml

# List nodes
scw k8s node list cluster-id=<cluster-id>

# Delete cluster
scw k8s cluster delete <cluster-id>
```

## Real-World Examples

### Deploy Web Application
```bash
# Create instance
scw instance server create \
  type=DEV1-M \
  name=webapp \
  image=ubuntu-focal \
  zone=fr-par-1

# Reserve and attach IP
IP_ID=$(scw instance ip create zone=fr-par-1 -o json | jq -r '.id')
SERVER_ID=$(scw instance server list name=webapp -o json | jq -r '.[0].id')
scw instance ip attach $IP_ID server-id=$SERVER_ID

# Configure security group
SG_ID=$(scw instance security-group create name=webapp-sg -o json | jq -r '.id')

# Allow HTTP
scw instance security-group-rule create \
  security-group-id=$SG_ID \
  action=accept direction=inbound ip-range=0.0.0.0/0 \
  protocol=TCP dest-port-from=80 dest-port-to=80

# Allow HTTPS
scw instance security-group-rule create \
  security-group-id=$SG_ID \
  action=accept direction=inbound ip-range=0.0.0.0/0 \
  protocol=TCP dest-port-from=443 dest-port-to=443
```

### CI/CD Integration
```yaml
# .github/workflows/deploy.yml
deploy:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4

    - name: Install Scaleway CLI
      preset: scaleway-cli

    - name: Configure Scaleway
      shell: |
        scw config set access-key=${{ secrets.SCW_ACCESS_KEY }}
        scw config set secret-key=${{ secrets.SCW_SECRET_KEY }}

    - name: Build and push image
      shell: |
        scw registry login
        docker build -t rg.fr-par.scw.cloud/myregistry/myapp:${{ github.sha }} .
        docker push rg.fr-par.scw.cloud/myregistry/myapp:${{ github.sha }}

    - name: Update Kubernetes
      shell: |
        scw k8s kubeconfig get $CLUSTER_ID > kubeconfig.yaml
        kubectl set image deployment/myapp myapp=rg.fr-par.scw.cloud/myregistry/myapp:${{ github.sha }}
```

### Infrastructure as Code
```bash
#!/bin/bash
# deploy-infra.sh

# Create VPC
VPC_ID=$(scw vpc private-network create name=prod-vpc -o json | jq -r '.id')

# Create database
DB_ID=$(scw rdb instance create \
  engine=PostgreSQL-14 \
  node-type=db-dev-s \
  name=prod-db \
  user-name=app \
  password=$DB_PASSWORD \
  -o json | jq -r '.id')

# Create app servers
for i in {1..3}; do
  scw instance server create \
    type=PRO2-S \
    name=app-$i \
    image=ubuntu-focal \
    zone=fr-par-1
done

# Create load balancer
scw lb lb create \
  name=prod-lb \
  type=LB-S \
  zone=fr-par-1
```

## Output Formats
```bash
# Human-readable (default)
scw instance server list

# JSON
scw instance server list -o json

# JSON with jq
scw instance server list -o json | jq '.[] | {name, id, state}'

# YAML
scw instance server list -o yaml

# Table
scw instance server list -o table
```

## Profiles
```bash
# Create profile
scw config profile create prod
scw config set access-key=<prod-key> profile=prod

# List profiles
scw config profile list

# Switch profile
scw config profile activate prod

# Use profile for single command
scw instance server list --profile prod
```

## Troubleshooting

### Authentication errors
Check credentials:
```bash
scw config get access-key
scw config get secret-key

# Re-initialize
scw init
```

### Rate limiting
```bash
# Add delays between commands
sleep 1
scw instance server list
```

### Connection timeouts
Increase timeout:
```bash
scw --timeout 300s instance server create ...
```

## Agent Use
- Automate cloud resource provisioning
- Deploy applications to Scaleway infrastructure
- Manage multi-region deployments
- Monitor resource usage and costs
- Implement backup and disaster recovery
- Scale infrastructure based on metrics
- Orchestrate container deployments

## Uninstall
```yaml
- preset: scaleway-cli
  with:
    state: absent
```

## Resources
- Official docs: https://www.scaleway.com/en/docs/
- CLI reference: https://github.com/scaleway/scaleway-cli
- API docs: https://www.scaleway.com/en/developers/api/
- Community: https://www.scaleway.com/en/community/
- Search: "scaleway cli tutorial", "scaleway cli examples"

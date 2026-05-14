# Rancher CLI - Kubernetes Management Tool

Rancher CLI is a command-line interface for managing Kubernetes clusters through Rancher.

## Quick Start

```yaml
- preset: rancher
```

## Features

- **Multi-cluster**: Manage multiple Kubernetes clusters from one CLI
- **Rancher integration**: Full access to Rancher API features
- **kubectl alternative**: Familiar Kubernetes commands with Rancher context
- **Project management**: Switch between Rancher projects and namespaces
- **App deployment**: Deploy Helm charts and Rancher catalog apps
- **Context switching**: Easy cluster and project selection

## Basic Usage

```bash
# Login to Rancher server
rancher login https://rancher.example.com --token <api-token>

# List clusters
rancher clusters ls

# Switch context to cluster
rancher context switch

# List projects
rancher projects ls

# Deploy a workload
rancher kubectl run nginx --image=nginx

# List pods
rancher kubectl get pods

# Access cluster
rancher kubectl --cluster my-cluster get nodes
```

## Advanced Configuration

```yaml
# Simple installation
- preset: rancher

# Remove installation
- preset: rancher
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (manual binary installation)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Config file**: `~/.rancher/cli2.json`
- **Context data**: Stored in config file
- **Default server**: Last logged-in Rancher instance

## Real-World Examples

### Multi-Cluster Management

```bash
# Login to Rancher
rancher login https://rancher.example.com --token $RANCHER_TOKEN --skip-verify

# Switch to production cluster
rancher context switch prod-cluster

# Deploy application
rancher kubectl apply -f deployment.yaml

# Check status across clusters
for cluster in prod staging dev; do
  echo "=== $cluster ==="
  rancher kubectl --cluster $cluster get pods -A
done
```

### CI/CD Integration

```yaml
# Deploy using Rancher CLI in pipeline
- name: Login to Rancher
  shell: rancher login {{ rancher_url }} --token {{ rancher_token }}
  register: rancher_login

- name: Deploy application
  shell: |
    rancher context switch {{ cluster_name }}
    rancher kubectl apply -f k8s/
  when: rancher_login.rc == 0

- name: Wait for rollout
  shell: |
    rancher kubectl rollout status deployment/myapp
```

### Automated Cluster Operations

```bash
# Create namespace across all clusters
rancher clusters ls --format json | jq -r '.[].name' | while read cluster; do
  rancher kubectl --cluster "$cluster" create namespace myapp
done

# Get resource usage
rancher kubectl top nodes
rancher kubectl top pods -A
```

## Agent Use

- Automate Kubernetes cluster management operations
- Deploy applications across multiple clusters
- Monitor cluster health and resource usage
- Manage Rancher projects and namespaces programmatically
- Integrate cluster operations into CI/CD pipelines

## Troubleshooting

### Login fails

```bash
# Verify Rancher URL is accessible
curl -k https://rancher.example.com/ping

# Check token validity
rancher login https://rancher.example.com --token <token> --debug

# Skip TLS verification (development only)
rancher login https://rancher.example.com --token <token> --skip-verify
```

### Context not found

```bash
# List available contexts
rancher context ls

# Reset context
rm ~/.rancher/cli2.json
rancher login https://rancher.example.com --token <token>
```

### kubectl commands not working

```bash
# Verify cluster access
rancher clusters ls

# Switch to correct cluster
rancher context switch <cluster-name>

# Use explicit cluster flag
rancher kubectl --cluster my-cluster get nodes
```

## Uninstall

```yaml
- preset: rancher
  with:
    state: absent
```

## Resources

- Official docs: https://ranchermanager.docs.rancher.com/reference-guides/cli-with-rancher
- GitHub: https://github.com/rancher/cli
- Rancher docs: https://rancher.com/docs/
- Search: "rancher cli tutorial", "rancher cli examples", "rancher multi-cluster management"

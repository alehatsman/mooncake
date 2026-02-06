# Rancher Fleet - GitOps at Scale

Manage large fleets of Kubernetes clusters using GitOps. Deploy to thousands of clusters from a single Git repository with per-cluster customization.

## Quick Start
```yaml
- preset: fleet
```

## Features
- **Multi-cluster GitOps**: Deploy to thousands of clusters simultaneously
- **Helm integration**: Native Helm chart deployment at scale
- **Customization**: Per-cluster configuration overlays and patches
- **Drift detection**: Continuous monitoring and automatic reconciliation
- **Resource-efficient**: Lightweight agent architecture for edge deployments
- **Bundle system**: Group resources for coordinated deployment

## Basic Usage
```bash
# Deploy from Git repository
kubectl apply -f - <<EOF
apiVersion: fleet.cattle.io/v1alpha1
kind: GitRepo
metadata:
  name: my-app
  namespace: fleet-default
spec:
  repo: https://github.com/org/fleet-examples
  paths:
  - simple
  targets:
  - name: dev
    clusterSelector:
      matchLabels:
        env: dev
EOF

# Check deployment status
kubectl get gitrepos -n fleet-default
kubectl get bundles -n fleet-default
kubectl get bundledeployments -A

# View cluster targets
kubectl get clusters -n fleet-default
```

## Advanced Configuration
```yaml
- preset: fleet
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Fleet CLI |

## Platform Support
- ✅ Linux (Kubernetes controller + CLI)
- ✅ macOS (kubectl plugin, CLI binary)
- ✅ Windows (CLI binary)

## Configuration
- **Namespace**: `fleet-system` (controllers), `fleet-default` (GitRepos)
- **Agent**: Deployed to target clusters via `fleet-agent`
- **Bundle**: Unit of deployment containing Kubernetes resources

## Real-World Examples

### Multi-Environment Deployment
```yaml
# Deploy to dev, staging, prod with overlays
apiVersion: fleet.cattle.io/v1alpha1
kind: GitRepo
metadata:
  name: microservices
  namespace: fleet-default
spec:
  repo: https://github.com/org/microservices
  paths:
  - base
  targets:
  - name: dev
    clusterSelector:
      matchLabels:
        env: dev
  - name: prod
    clusterSelector:
      matchLabels:
        env: production
```

### Helm Chart Deployment
```yaml
# fleet.yaml in Git repo
helm:
  chart: ./charts/myapp
  releaseName: myapp
  values:
    replicaCount: 3
    image:
      repository: myapp
      tag: v1.0.0

# Per-cluster customization
targetCustomizations:
- name: production
  helm:
    values:
      replicaCount: 10
      resources:
        limits:
          memory: 2Gi
```

### Edge Cluster Management
```yaml
# Deploy to edge locations
apiVersion: fleet.cattle.io/v1alpha1
kind: GitRepo
metadata:
  name: edge-apps
spec:
  repo: https://github.com/org/edge
  paths:
  - apps
  targets:
  - clusterSelector:
      matchLabels:
        location: edge
        region: us-west
```

## Agent Use
- Manage Kubernetes deployments across multiple cloud providers
- Deploy to geographically distributed edge clusters
- Implement multi-tenant cluster management
- Automate application rollouts to dev/staging/prod environments
- Manage IoT and edge computing Kubernetes fleets

## Troubleshooting

### GitRepo not syncing
```bash
# Check GitRepo status
kubectl describe gitrepo my-app -n fleet-default

# View controller logs
kubectl logs -n fleet-system -l app=fleet-controller

# Force reconciliation
kubectl annotate gitrepo my-app -n fleet-default \
  fleet.cattle.io/force-sync=true --overwrite
```

### Bundle deployment stuck
```bash
# Check bundle status
kubectl get bundles -n fleet-default
kubectl describe bundle my-app -n fleet-default

# Check deployment on target cluster
kubectl get bundledeployments -A
```

### Cluster not registered
```bash
# Check cluster registration
kubectl get clusters -n fleet-default

# Re-register cluster
kubectl apply -f cluster-registration.yaml
```

## Uninstall
```yaml
- preset: fleet
  with:
    state: absent
```

## Resources
- Official docs: https://fleet.rancher.io/
- GitHub: https://github.com/rancher/fleet
- Examples: https://github.com/rancher/fleet-examples
- Search: "rancher fleet gitops", "fleet kubernetes multi-cluster"

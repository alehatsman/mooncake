# Flux - GitOps for Kubernetes

Continuous delivery solution for Kubernetes. Automatically deploy code changes to production by syncing Git repositories with running clusters.

## Quick Start
```yaml
- preset: flux
```

## Features
- **GitOps workflow**: Git as single source of truth for infrastructure
- **Automated deployments**: Deploy on git push automatically
- **Drift detection**: Continuous reconciliation to match desired state
- **Multi-tenancy**: Isolate teams and applications
- **Progressive delivery**: Canary deployments with Flagger
- **Kubernetes native**: CRDs for declarative configuration

## Basic Usage
```bash
# Check prerequisites
flux check --pre

# Bootstrap Flux on cluster
flux bootstrap github \
  --owner=your-org \
  --repository=fleet-infra \
  --branch=main \
  --path=clusters/production \
  --personal

# Create GitRepository source
flux create source git webapp \
  --url=https://github.com/org/webapp \
  --branch=main \
  --interval=1m

# Create Kustomization
flux create kustomization webapp \
  --source=webapp \
  --path="./deploy" \
  --prune=true \
  --interval=5m

# Watch reconciliation
flux get all
flux logs --all-namespaces

# Suspend/resume
flux suspend kustomization webapp
flux resume kustomization webapp
```

## Advanced Configuration
```yaml
- preset: flux
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Flux |

## Platform Support
- ✅ Linux (Kubernetes CLI)
- ✅ macOS (Homebrew, binary download)
- ✅ Windows (binary download, Scoop)

## Configuration
- **Namespace**: `flux-system` (default)
- **Components**: source-controller, kustomize-controller, helm-controller, notification-controller
- **Git sync interval**: 1m (configurable)
- **Reconciliation**: Continuous, automatic

## Real-World Examples

### Basic GitOps Setup
```yaml
# clusters/production/infrastructure.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: infrastructure
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./infrastructure
  prune: true
  wait: true
```

### Helm Release with Flux
```yaml
# apps/nginx/helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: nginx
  namespace: default
spec:
  interval: 5m
  chart:
    spec:
      chart: nginx
      version: '13.x'
      sourceRef:
        kind: HelmRepository
        name: bitnami
        namespace: flux-system
  values:
    replicaCount: 3
    service:
      type: LoadBalancer
```

### Multi-Environment Deployment
```bash
# Deploy to dev, staging, production
flux bootstrap github \
  --owner=org \
  --repository=k8s-clusters \
  --path=clusters/dev

flux bootstrap github \
  --owner=org \
  --repository=k8s-clusters \
  --path=clusters/staging

flux bootstrap github \
  --owner=org \
  --repository=k8s-clusters \
  --path=clusters/production
```

### Image Automation
```yaml
# Image repository scanning
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageRepository
metadata:
  name: webapp
spec:
  image: docker.io/org/webapp
  interval: 1m

---
# Image policy (semantic versioning)
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImagePolicy
metadata:
  name: webapp
spec:
  imageRepositoryRef:
    name: webapp
  policy:
    semver:
      range: '>=1.0.0 <2.0.0'

---
# Auto-update deployment
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageUpdateAutomation
metadata:
  name: webapp
spec:
  interval: 1m
  sourceRef:
    kind: GitRepository
    name: flux-system
  git:
    commit:
      author:
        email: fluxcdbot@example.com
        name: fluxcdbot
  update:
    path: ./apps/production
    strategy: Setters
```

## Agent Use
- Automate Kubernetes deployments via GitOps
- Implement continuous delivery for microservices
- Manage multi-cluster configurations from single repository
- Automate Helm chart deployments and upgrades
- Synchronize infrastructure-as-code to clusters
- Implement progressive delivery with canaries

## Troubleshooting

### Bootstrap fails
```bash
# Check Kubernetes connection
kubectl cluster-info

# Verify GitHub token permissions
# Required: repo (all), admin:repo_hook

# Check prerequisites
flux check --pre

# Use verbose output
flux bootstrap github --verbose
```

### Reconciliation stuck
```bash
# Check controller status
kubectl get pods -n flux-system

# View controller logs
flux logs --level=error
flux logs --kind=Kustomization --name=webapp

# Force reconciliation
flux reconcile kustomization webapp --with-source

# Check Git source
flux get sources git
```

### Resource conflicts
```bash
# Check kustomization status
flux get kustomizations

# Describe resource
kubectl describe kustomization webapp -n flux-system

# View events
kubectl get events -n flux-system --sort-by='.lastTimestamp'

# Suspend and resume
flux suspend kustomization webapp
flux resume kustomization webapp
```

### Image automation not working
```bash
# Check image reflector controller
kubectl logs -n flux-system deploy/image-reflector-controller

# Verify image policy
flux get image policy webapp

# Check repository scanning
flux get image repository webapp

# Manual reconciliation
flux reconcile image repository webapp
```

## Uninstall
```yaml
- preset: flux
  with:
    state: absent
```

## Resources
- Official docs: https://fluxcd.io/docs/
- GitHub: https://github.com/fluxcd/flux2
- Get started: https://fluxcd.io/docs/get-started/
- Guides: https://fluxcd.io/docs/guides/
- Search: "flux gitops", "flux kubernetes", "flux helm"

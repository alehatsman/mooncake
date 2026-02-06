# FluxCD v2 - GitOps for Kubernetes

GitOps continuous delivery solution for Kubernetes. Automatically sync cluster state from Git repositories.

## Quick Start
```yaml
- preset: fluxcd
```

## Features
- **GitOps native**: Kubernetes controllers sync state from Git
- **Multi-tenancy**: Manage multiple teams and namespaces
- **Progressive delivery**: Canary deployments with Flagger
- **Policy enforcement**: OPA and Kyverno integration
- **CNCF graduated**: Production-ready, vendor-neutral standard

## Basic Usage
```bash
# Bootstrap Flux on cluster
flux bootstrap github \
  --owner=my-org \
  --repository=my-repo \
  --path=clusters/production

# Check system status
flux check

# Create source from Git repo
flux create source git my-app \
  --url=https://github.com/my-org/my-app \
  --branch=main

# Create kustomization
flux create kustomization my-app \
  --source=my-app \
  --path="./kustomize" \
  --prune=true

# Reconcile immediately
flux reconcile kustomization my-app
```

## Advanced Configuration
```yaml
- preset: fluxcd
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove FluxCD |

## Platform Support
- ✅ Linux (GitHub releases, install script)
- ✅ macOS (Homebrew, GitHub releases)
- ✅ Windows (Chocolatey, GitHub releases)

## Configuration
- **Namespace**: `flux-system` (default Kubernetes namespace)
- **Controllers**: source-controller, kustomize-controller, helm-controller, notification-controller
- **Config repo**: Git repository containing cluster manifests

## Real-World Examples

### Multi-Environment Setup
```bash
# Production cluster
flux bootstrap github \
  --owner=my-org \
  --repository=fleet-infra \
  --path=clusters/production \
  --personal=false

# Staging cluster
flux bootstrap github \
  --owner=my-org \
  --repository=fleet-infra \
  --path=clusters/staging
```

### Helm Release Management
```yaml
# HelmRelease manifest
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
```

### Image Automation
```yaml
# Auto-update image tags
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageUpdateAutomation
metadata:
  name: my-app
spec:
  interval: 1m
  sourceRef:
    kind: GitRepository
    name: my-app
  git:
    commit:
      author:
        name: fluxbot
        email: flux@example.com
  update:
    path: ./manifests
    strategy: Setters
```

## Agent Use
- Implement GitOps workflows for Kubernetes cluster management
- Automate application deployments from Git repositories
- Manage Helm releases declaratively
- Configure image update automation for CI/CD
- Enforce policy compliance across multiple clusters
- Multi-cluster fleet management with consistent configurations

## Troubleshooting

### Bootstrap fails
```bash
# Check prerequisites
flux check --pre

# Verify GitHub token permissions
echo $GITHUB_TOKEN

# Use verbose output
flux bootstrap github --verbose
```

### Reconciliation stuck
```bash
# Check controller logs
flux logs --all-namespaces

# Suspend/resume reconciliation
flux suspend kustomization my-app
flux resume kustomization my-app

# Force reconciliation
flux reconcile source git my-app --with-source
```

### Git authentication issues
```bash
# Create SSH key secret
flux create secret git my-app \
  --url=ssh://git@github.com/my-org/my-app \
  --private-key-file=./identity

# Use HTTPS with token
flux create secret git my-app \
  --url=https://github.com/my-org/my-app \
  --username=git \
  --password=$GITHUB_TOKEN
```

## Uninstall
```yaml
- preset: fluxcd
  with:
    state: absent
```

## Resources
- Official docs: https://fluxcd.io/docs/
- GitHub: https://github.com/fluxcd/flux2
- Getting started: https://fluxcd.io/docs/get-started/
- Search: "fluxcd gitops", "flux kubernetes", "flux helm"

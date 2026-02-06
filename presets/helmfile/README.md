# helmfile - Declarative Helm

Deploy Kubernetes Helm Charts declaratively with helmfile.yaml.

## Quick Start
```yaml
- preset: helmfile
```

## Usage
```bash
# Sync all releases
helmfile sync

# Diff
helmfile diff

# Apply
helmfile apply

# Destroy
helmfile destroy
```

**Agent Use**: GitOps workflows, declarative K8s management

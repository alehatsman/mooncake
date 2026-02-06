# helm - Kubernetes Packages

Package manager for Kubernetes. Deploy and manage applications.

## Quick Start
```yaml
- preset: helm
```

## Usage
```bash
# Add repo
helm repo add stable https://charts.helm.sh/stable

# Install
helm install myapp stable/nginx

# Upgrade
helm upgrade myapp stable/nginx

# List
helm list

# Uninstall
helm uninstall myapp
```

**Agent Use**: Automated K8s deployments, application management

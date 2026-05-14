# kubectl - Kubernetes CLI

Command-line tool for interacting with Kubernetes clusters.

## Quick Start
```yaml
- preset: kubectl
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Advanced Configuration
```yaml
- preset: kubectl
  with:
    version: "1.29.0"              # Specific version
    configure_completion: true     # Shell completion
    install_krew: true             # Plugin manager
    krew_plugins:
      - ctx                        # Switch contexts
      - ns                         # Switch namespaces
      - view-secret                # Decode secrets
      - tree                       # Resource hierarchy
```


## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kubectl |
## Basic Usage
```bash
# Version and cluster info
kubectl version --client
kubectl cluster-info
kubectl config view

# Resources
kubectl get nodes
kubectl get pods -A
kubectl get services
kubectl get deployments

# Create/Apply
kubectl create -f manifest.yml
kubectl apply -f manifest.yml

# Describe/Logs
kubectl describe pod <name>
kubectl logs <pod-name>
kubectl logs -f <pod-name>

# Execute commands
kubectl exec -it <pod-name> -- /bin/bash

# Port forwarding
kubectl port-forward svc/<service-name> 8080:80
```

## Context and Namespace
```bash
# View contexts
kubectl config get-contexts
kubectl config current-context

# Switch context
kubectl config use-context <context-name>

# Set namespace
kubectl config set-context --current --namespace=<namespace>

# With krew ctx/ns plugins
kubectl ctx                        # List contexts
kubectl ctx <context-name>         # Switch context
kubectl ns                         # List namespaces
kubectl ns <namespace>             # Switch namespace
```

## Krew Plugin Manager
```bash
# Search plugins
kubectl krew search

# Install plugin
kubectl krew install <plugin-name>

# List installed
kubectl krew list

# Update plugins
kubectl krew upgrade

# Popular plugins
kubectl krew install ctx           # Context switcher
kubectl krew install ns            # Namespace switcher
kubectl krew install view-secret   # Decode secrets
kubectl krew install tree          # Resource tree
kubectl krew install neat          # Clean output
```

## Configuration
- **Kubeconfig:** `~/.kube/config`
- **Contexts:** Cluster + user + namespace combinations
- **Plugins:** `~/.krew/bin/` (krew) or `~/.local/share/kubectl/plugins/`

## Agent Use
- Cluster management and deployment
- Resource inspection and debugging
- Configuration validation
- CI/CD pipeline integration
- Infrastructure automation

## Uninstall
```yaml
- preset: kubectl
  with:
    state: absent
```

**Note:** Uninstalling does not remove `~/.kube/config` or cluster configurations.

## Resources
Search: "kubectl cheat sheet", "kubernetes kubectl reference"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

# kubectx - Kubernetes Context Switcher

Fast way to switch between Kubernetes clusters and namespaces.

## Quick Start
```yaml
- preset: kubectx
```

## Advanced Configuration
```yaml
- preset: kubectx
  with:
    install_kubens: true        # Install namespace switcher
    configure_aliases: true     # Shell aliases (ctx, ns)
```

## Basic Usage
```bash
# Context switching
kubectx                        # List all contexts
kubectx <context-name>         # Switch to context
kubectx -                      # Switch to previous context
kubectx -c                     # Show current context
kubectx -d <context>           # Delete context
kubectx <new>=<old>            # Rename context

# With alias
ctx                            # List contexts
ctx production                 # Switch to production
ctx -                          # Switch back
```

## Namespace Switching (kubens)
```bash
# Namespace operations
kubens                         # List all namespaces
kubens <namespace>             # Switch to namespace
kubens -                       # Switch to previous namespace
kubens -c                      # Show current namespace

# With alias
ns                             # List namespaces
ns kube-system                 # Switch to kube-system
ns -                           # Switch back
```

## Common Workflows
```bash
# Quick context switching
kubectx staging && kubectl get pods
kubectx production && kubectl get pods

# Switch context and namespace together
kubectx production && kubens api && kubectl logs -f deployment/api

# Interactive mode (if fzf installed)
kubectx                        # Fuzzy search contexts
kubens                         # Fuzzy search namespaces
```

## Features
- **Fast switching:** No need to type full context names
- **Previous context:** Use `-` to switch back
- **Interactive:** Integrates with fzf for fuzzy searching
- **Context renaming:** Simplify long context names
- **Shell completion:** Tab completion support

## Configuration
- **Kubeconfig:** Uses `~/.kube/config` or `$KUBECONFIG`
- **Aliases:** `ctx` for kubectx, `ns` for kubens
- **History:** Remembers previous context/namespace

## Agent Use
- Rapid context switching in multi-cluster environments
- Namespace isolation during development
- CI/CD pipeline context management
- Development workflow optimization

## Uninstall
```yaml
- preset: kubectx
  with:
    state: absent
```

**Note:** Shell aliases in rc files are not automatically removed.

## Enhanced Experience
Install `fzf` for interactive selection:
```bash
# macOS
brew install fzf

# Linux
apt install fzf    # Debian/Ubuntu
dnf install fzf    # Fedora
```

## Resources
- Repository: https://github.com/ahmetb/kubectx
- Search: "kubectx tutorial", "kubectl context management"

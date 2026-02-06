# k9s Preset

Kubernetes TUI for managing clusters. Navigate, observe, and manage K8s resources with a beautiful terminal interface.

## Quick Start

```yaml
- preset: k9s

# With config
- preset: k9s
  with:
    create_config: true
```

## Launch

```bash
k9s                    # Default context
k9s -n namespace       # Specific namespace
k9s --context prod     # Specific context
k9s --readonly         # Read-only mode
```

## Navigation

```
:pod        Pods
:svc        Services
:deploy     Deployments
:ns         Namespaces
:no         Nodes
:pv         Persistent Volumes
:cm         ConfigMaps
:sec        Secrets
```

## Key Bindings

| Key | Action |
|-----|--------|
| `?` | Help |
| `:` | Command mode |
| `/` | Filter |
| `d` | Describe |
| `y` | YAML |
| `e` | Edit |
| `l` | Logs |
| `s` | Shell |
| `f` | Port-forward |
| `Ctrl-D` | Delete |
| `Ctrl-K` | Kill |

## Examples

```bash
# View pod logs
# 1. Type :pod
# 2. Select pod with j/k
# 3. Press 'l' for logs

# Shell into container
# 1. Navigate to pod
# 2. Press 's'

# Port forward
# 1. Navigate to pod or service
# 2. Press 'Shift-f'
# 3. Enter local:remote ports

# Edit resource
# 1. Navigate to resource
# 2. Press 'e'
# 3. Edit YAML, save to apply
```

## Resources
- Docs: https://k9scli.io/
- GitHub: https://github.com/derailed/k9s

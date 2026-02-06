# helm - Kubernetes Package Manager

Deploy and manage applications on Kubernetes using Helm charts.

## Quick Start
```yaml
- preset: helm
```

## Advanced Configuration
```yaml
- preset: helm
  with:
    version: "3.13.0"                      # Specific Helm version
    configure_completion: true             # Shell completion
    add_repos:
      - stable=https://charts.helm.sh/stable
      - bitnami=https://charts.bitnami.com/bitnami
    install_plugins:
      - databus23/helm-diff                # Compare releases
      - jkroepke/helm-secrets              # Manage secrets
      - aslafy-z/helm-git                  # Git-based charts
```

## Basic Usage
```bash
# Repository management
helm repo add stable https://charts.helm.sh/stable
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm repo list

# Search for charts
helm search repo nginx
helm search hub wordpress

# Install applications
helm install myapp stable/nginx
helm install mydb bitnami/postgresql --set auth.postgresPassword=secret

# Release management
helm list                              # List releases
helm status myapp                      # Check status
helm get values myapp                  # Show values
helm upgrade myapp stable/nginx        # Upgrade release
helm rollback myapp 1                  # Rollback to revision
helm uninstall myapp                   # Remove release

# Chart management
helm create mychart                    # Create new chart
helm package mychart                   # Package chart
helm lint mychart                      # Validate chart
helm template mychart                  # Render templates
```

## Repository Management
```bash
# Official repos
helm repo add stable https://charts.helm.sh/stable
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add prometheus https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts

# Update repos
helm repo update

# Search charts
helm search repo <keyword>
```

## Installation with Values
```bash
# Using --set flag
helm install myapp bitnami/nginx \
  --set replicaCount=3 \
  --set service.type=LoadBalancer

# Using values file
helm install myapp bitnami/nginx -f values.yaml

# Using multiple values files
helm install myapp bitnami/nginx \
  -f base-values.yaml \
  -f prod-values.yaml

# Generate values template
helm show values bitnami/nginx > values.yaml
```

## Useful Plugins
```bash
# helm-diff - Preview upgrades
helm diff upgrade myapp stable/nginx -f values.yaml

# helm-secrets - Manage encrypted secrets
helm secrets install myapp ./chart -f secrets.yaml

# helm-git - Use Git repos as chart sources
helm install myapp git+https://github.com/user/charts@path/to/chart

# Install plugins manually
helm plugin install https://github.com/databus23/helm-diff
helm plugin install https://github.com/jkroepke/helm-secrets
helm plugin list
```

## Release Management
```bash
# View release history
helm history myapp

# Rollback to previous version
helm rollback myapp

# Rollback to specific revision
helm rollback myapp 3

# Test before install
helm install myapp ./chart --dry-run --debug

# Atomic install (rollback on failure)
helm install myapp ./chart --atomic --timeout 5m
```

## Configuration
- **Helm home:** `~/.config/helm/` (Linux), `~/Library/Preferences/helm/` (macOS)
- **Repository cache:** `~/.cache/helm/repository/`
- **Plugins:** `~/.local/share/helm/plugins/`
- **Kubeconfig:** Uses kubectl's config (`~/.kube/config`)

## Agent Use
- Automated Kubernetes deployments
- Application lifecycle management
- Multi-environment configuration
- Release versioning and rollbacks
- Infrastructure as Code workflows

## Uninstall
```yaml
- preset: helm
  with:
    state: absent
```

**Note:** Uninstalling Helm does not remove deployed releases. Uninstall releases first with `helm uninstall <release>`.

## Resources
- Official docs: https://helm.sh/docs/
- Chart repository: https://artifacthub.io/
- Search: "helm chart tutorial", "helm best practices"

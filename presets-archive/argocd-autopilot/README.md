# Argo CD Autopilot - Automated GitOps Bootstrap

Bootstrap Argo CD with GitOps best practices. Automated installation, application management, and declarative GitOps repository structure.

## Quick Start
```yaml
- preset: argocd-autopilot
```

## Features
- **Automated bootstrap**: Install Argo CD and setup GitOps repository
- **Git-based structure**: Opinionated directory layout for applications
- **Application management**: Create apps with CLI, stored in Git
- **Multi-cluster support**: Manage multiple Kubernetes clusters
- **Declarative config**: Everything-as-code in Git repository
- **Application sets**: Bulk application management
- **Promotion workflows**: Move apps between environments

## Basic Usage
```bash
# Initialize GitOps repository
argocd-autopilot repo bootstrap \
  --repo https://github.com/org/gitops-repo

# Create application
argocd-autopilot app create myapp \
  --app github.com/org/app-repo/manifests \
  --project default

# List applications
argocd-autopilot app list

# Delete application
argocd-autopilot app delete myapp
```

## Bootstrap Command
```bash
# GitHub
export GIT_TOKEN=ghp_xxx
argocd-autopilot repo bootstrap \
  --repo https://github.com/org/gitops-repo \
  --installation-mode normal

# GitLab
export GIT_TOKEN=glpat-xxx
argocd-autopilot repo bootstrap \
  --repo https://gitlab.com/org/gitops-repo \
  --git-token $GIT_TOKEN

# With specific Argo CD version
argocd-autopilot repo bootstrap \
  --repo https://github.com/org/gitops-repo \
  --revision v2.8.0
```

## Repository Structure
```
gitops-repo/
├── apps/                    # Application definitions
│   └── myapp/
│       ├── base/
│       │   └── myapp.yaml
│       └── overlays/
│           ├── dev/
│           └── prod/
├── bootstrap/               # Argo CD installation
│   ├── argo-cd.yaml
│   └── cluster-resources.yaml
├── projects/                # Argo CD projects
│   └── default.yaml
└── config.json             # Autopilot configuration
```

## Application Management
```bash
# Create app from Helm chart
argocd-autopilot app create nginx \
  --app https://github.com/bitnami/charts/tree/main/bitnami/nginx \
  --type helm \
  --project web

# Create from kustomize
argocd-autopilot app create api \
  --app github.com/org/api-repo/k8s/overlays/prod \
  --type kustomize \
  --dest-namespace production

# Update application
argocd-autopilot app update myapp \
  --app github.com/org/myapp/k8s/v2

# Delete application
argocd-autopilot app delete myapp --project default
```

## Project Management
```bash
# Create project
argocd-autopilot project create frontend \
  --dest-namespace frontend-*

# List projects
argocd-autopilot project list

# Delete project
argocd-autopilot project delete frontend
```

## Advanced Configuration
```yaml
# config.json - Autopilot configuration
{
  "appName": "argo-cd",
  "repoURL": "https://github.com/org/gitops-repo",
  "destServer": "https://kubernetes.default.svc",
  "destNamespace": "argocd",
  "revision": "main",
  "pathPrefix": "",
  "projects": {
    "default": {
      "destServer": "https://kubernetes.default.svc",
      "destNamespace": "*"
    }
  }
}
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Configuration

### Installation Modes
```bash
# Normal mode (Argo CD in 'argocd' namespace)
argocd-autopilot repo bootstrap --installation-mode normal

# Flat mode (Argo CD in same namespace as apps)
argocd-autopilot repo bootstrap --installation-mode flat
```

### Multi-Cluster Setup
```bash
# Bootstrap on management cluster
argocd-autopilot repo bootstrap \
  --repo https://github.com/org/gitops-repo

# Add second cluster
kubectl config use-context prod-cluster
argocd-autopilot cluster add prod \
  --kubeconfig ~/.kube/config \
  --context prod-cluster

# Deploy app to specific cluster
argocd-autopilot app create myapp \
  --app github.com/org/myapp/k8s \
  --dest-server https://prod.k8s.local
```

## Real-World Examples

### Bootstrap New GitOps Repository
```bash
#!/bin/bash
# bootstrap-gitops.sh - Setup new GitOps repository

export GIT_TOKEN=ghp_xxx
REPO_URL="https://github.com/myorg/gitops-production"

# Create empty repository on GitHub first
gh repo create myorg/gitops-production --private

# Bootstrap Argo CD Autopilot
argocd-autopilot repo bootstrap \
  --repo $REPO_URL \
  --installation-mode normal

# Verify installation
kubectl get pods -n argocd
argocd-autopilot version

echo "GitOps repository ready: $REPO_URL"
```

### Multi-Environment Application
```bash
#!/bin/bash
# deploy-multi-env.sh - Deploy app to dev and prod

APP_NAME="myapi"
GIT_REPO="github.com/myorg/myapi"

# Create development version
argocd-autopilot app create $APP_NAME-dev \
  --app $GIT_REPO/k8s/overlays/dev \
  --dest-namespace development \
  --project default

# Create production version
argocd-autopilot app create $APP_NAME-prod \
  --app $GIT_REPO/k8s/overlays/prod \
  --dest-namespace production \
  --project default \
  --wait-timeout 10m

# List deployed applications
argocd-autopilot app list
```

### Microservices Platform
```bash
#!/bin/bash
# bootstrap-microservices.sh - Setup platform with multiple apps

# Bootstrap GitOps repo
argocd-autopilot repo bootstrap \
  --repo https://github.com/org/platform-gitops

# Create projects for each team
argocd-autopilot project create platform --dest-namespace platform-*
argocd-autopilot project create frontend --dest-namespace frontend-*
argocd-autopilot project create backend --dest-namespace backend-*

# Deploy platform services
argocd-autopilot app create ingress-nginx \
  --app https://github.com/kubernetes/ingress-nginx/tree/main/deploy/static/provider/cloud \
  --project platform

argocd-autopilot app create cert-manager \
  --app https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml \
  --project platform

# Deploy team applications
for service in auth users orders payments; do
  argocd-autopilot app create $service \
    --app github.com/org/$service-service/k8s \
    --project backend \
    --dest-namespace backend-$service
done
```

### CI/CD Integration
```yaml
# .github/workflows/deploy.yml
name: Deploy Application
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Install argocd-autopilot
        preset: argocd-autopilot

      - name: Deploy to staging
        env:
          GIT_TOKEN: ${{ secrets.GITOPS_TOKEN }}
        run: |
          # Update application with new image tag
          cd gitops-repo
          argocd-autopilot app update myapp-staging \
            --app github.com/org/myapp/k8s/overlays/staging
          git commit -am "Update staging to ${{ github.sha }}"
          git push
```

## Agent Use
- Automate GitOps repository creation and bootstrap
- Manage application deployments across multiple clusters
- Implement promotion workflows between environments
- Enforce GitOps best practices with standardized structure
- CI/CD integration for declarative deployments
- Multi-tenant platform management with projects

## Troubleshooting

### Bootstrap Fails
```bash
# Check prerequisites
kubectl version
git --version

# Verify token permissions
# Token needs: repo (full control)

# Debug mode
argocd-autopilot repo bootstrap \
  --repo https://github.com/org/gitops \
  --debug
```

### Application Not Syncing
```bash
# Check application status
kubectl get applications -n argocd

# View Argo CD logs
kubectl logs -n argocd deployment/argocd-server

# Re-sync application
argocd app sync myapp
```

### Permission Errors
```bash
# Verify kubeconfig
kubectl auth can-i create namespace

# Check Argo CD RBAC
kubectl get configmap argocd-rbac-cm -n argocd -o yaml
```

## Comparison

### Autopilot vs Manual Argo CD
| Feature | Autopilot | Manual |
|---------|-----------|--------|
| Bootstrap | One command | Multiple steps |
| Structure | Opinionated | Custom |
| App management | CLI-driven | YAML files |
| Learning curve | Lower | Higher |
| Flexibility | Standard patterns | Full control |

## Uninstall
```yaml
- preset: argocd-autopilot
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/argoproj-labs/argocd-autopilot
- Docs: https://argocd-autopilot.readthedocs.io/
- Argo CD: https://argo-cd.readthedocs.io/
- Search: "argocd autopilot bootstrap", "argocd autopilot app create"

# werf - GitOps CLI Tool

GitOps delivery tool that integrates with Git, CI/CD systems, and Kubernetes for consistent application delivery.

## Quick Start
```yaml
- preset: werf
```

## Features
- **GitOps Native**: Git as single source of truth
- **Build and Deploy**: Build images, deploy to Kubernetes
- **Helm Integration**: Native Helm chart support
- **Cleanup**: Automatic cleanup of old images and releases
- **Multi-environment**: Consistent deployment across environments
- **Reproducible Builds**: Content-based tagging and caching

## Basic Usage
```bash
# Initialize werf project
werf init

# Build images
werf build

# Deploy to Kubernetes
werf converge --repo registry.example.com/project

# Deploy to specific environment
werf converge --env production --repo registry.example.com/project

# Cleanup old images
werf cleanup

# Run in CI/CD
werf ci-env gitlab --tagging-strategy tag-or-branch
werf build-and-publish
werf converge
```

## Advanced Configuration
```yaml
- preset: werf
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove werf |

## Platform Support
- ✅ Linux (binary, packages)
- ✅ macOS (binary, Homebrew)
- ✅ Windows (binary)

## Configuration
- **Config file**: `werf.yaml`
- **Helm charts**: `.helm/` directory
- **Dockerfiles**: `Dockerfile` or custom paths
- **State**: Stored in Kubernetes ConfigMaps/Secrets

## Project Structure

```
my-app/
├── werf.yaml               # Main configuration
├── .helm/
│   ├── templates/          # Kubernetes manifests
│   │   ├── deployment.yaml
│   │   └── service.yaml
│   ├── values.yaml         # Default values
│   ├── secret-values.yaml  # Encrypted secrets
│   └── Chart.yaml
├── Dockerfile              # Application image
└── .github/
    └── workflows/
        └── deploy.yml      # CI/CD pipeline
```

## Real-World Examples

### Basic Deploy
```yaml
# werf.yaml
project: myapp
configVersion: 1

---
image: backend
dockerfile: Dockerfile
context: .
```

```bash
# Build and deploy
werf converge --repo registry.example.com/myapp
```

### Multi-environment Deployment
```bash
# Development
werf converge --env development --repo registry.example.com/myapp

# Staging
werf converge --env staging --repo registry.example.com/myapp

# Production
werf converge --env production --repo registry.example.com/myapp
```

### CI/CD Integration
```yaml
# GitLab CI
deploy:
  stage: deploy
  image: werf/werf
  script:
    - source $(werf ci-env gitlab --tagging-strategy tag-or-branch)
    - werf build
    - werf converge --repo $CI_REGISTRY_IMAGE
```

```yaml
# GitHub Actions
- name: Install werf
  preset: werf

- name: Deploy application
  shell: |
    source $(werf ci-env github --tagging-strategy tag-or-branch)
    werf build
    werf converge --repo ghcr.io/${{ github.repository }}
  env:
    WERF_KUBECONFIG_BASE64: ${{ secrets.KUBECONFIG_BASE64 }}
```

## Agent Use
- Automated GitOps deployments
- Multi-environment Kubernetes management
- CI/CD pipeline integration
- Image build and registry management
- Helm chart templating and deployment
- Cleanup and garbage collection

## Uninstall
```yaml
- preset: werf
  with:
    state: absent
```

## Resources
- Official docs: https://werf.io/
- GitHub: https://github.com/werf/werf
- Search: "werf gitops", "werf kubernetes deployment"

# Harness CLI - DevOps Platform Command Line Interface

Command-line interface for Harness, the modern software delivery platform with CI/CD, GitOps, and feature flags.

## Quick Start
```yaml
- preset: harness
```

## Features
- **Unified platform**: CI/CD, GitOps, feature flags, and cloud cost management
- **Pipeline management**: Create and manage deployment pipelines from CLI
- **GitOps workflows**: Sync and manage Kubernetes deployments
- **Service management**: Manage services, environments, and infrastructure
- **Feature flags**: Control feature rollouts programmatically
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Authenticate
harness login

# List pipelines
harness pipeline list

# Execute pipeline
harness pipeline run --pipeline-id <id>

# Get pipeline execution status
harness pipeline execution get --execution-id <id>

# List services
harness service list

# Deploy service
harness service deploy --service <name> --env <environment>

# List feature flags
harness feature-flag list

# Toggle feature flag
harness feature-flag toggle --flag <name> --state on
```

## Configuration
- **Config file**: `~/.harness/config.yml`
- **API endpoint**: Configurable (cloud or self-hosted)
- **Authentication**: API key or OAuth

## Real-World Examples

### CI/CD Pipeline Automation
```bash
# Trigger deployment pipeline
harness pipeline run \
  --pipeline-id prod-deploy \
  --input-set deployment.env=production \
  --input-set deployment.version=v1.2.3

# Wait for completion
harness pipeline execution get \
  --execution-id <exec-id> \
  --wait
```

### GitOps Deployment
```yaml
- name: Deploy application via GitOps
  shell: |
    harness gitops application sync \
      --app-name my-app \
      --revision main
  register: deploy

- name: Verify deployment
  assert:
    command:
      cmd: harness gitops application status --app-name my-app
      exit_code: 0
```

### Feature Flag Management
```bash
# Enable feature for testing
harness feature-flag toggle --flag new-ui --state on --target-group beta-users

# Check flag status
harness feature-flag get --flag new-ui

# Disable feature
harness feature-flag toggle --flag new-ui --state off
```

### Service Deployment with Rollback
```bash
#!/bin/bash
# Deploy with automatic rollback on failure

VERSION="v2.0.0"
SERVICE="api-service"
ENV="production"

# Deploy new version
DEPLOY_ID=$(harness service deploy \
  --service $SERVICE \
  --env $ENV \
  --version $VERSION \
  --json | jq -r '.deployment_id')

# Monitor deployment
if ! harness service deployment status --id $DEPLOY_ID --wait; then
  echo "Deployment failed, rolling back..."
  harness service rollback --deployment-id $DEPLOY_ID
  exit 1
fi

echo "Deployment successful!"
```

### Environment Management
```bash
# List environments
harness environment list --project my-project

# Create new environment
harness environment create \
  --name staging \
  --type PreProduction \
  --project my-project

# Update environment variables
harness environment update \
  --name staging \
  --variables DB_HOST=staging-db.example.com
```

## CI/CD Integration

### Pipeline Trigger in GitHub Actions
```yaml
name: Deploy to Production
on:
  push:
    tags:
      - 'v*'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Install Harness CLI
        run: |
          preset: harness

      - name: Authenticate
        run: harness login --api-key ${{ secrets.HARNESS_API_KEY }}

      - name: Trigger deployment
        run: |
          harness pipeline run \
            --pipeline-id prod-deploy \
            --input-set version=${{ github.ref_name }}
```

## Agent Use
- Automate CI/CD pipeline execution and monitoring
- Manage multi-environment deployments programmatically
- Implement progressive delivery with feature flags
- Integrate deployments into automated workflows
- Monitor and rollback failed deployments
- Synchronize GitOps applications across clusters

## Advanced Configuration
```yaml
- preset: harness
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Harness CLI |

## Troubleshooting

### Authentication Issues
```bash
# Verify credentials
harness whoami

# Re-authenticate
harness login --api-key <your-key>

# Check config
cat ~/.harness/config.yml
```

### API Connection Problems
```bash
# Test API connectivity
harness account get

# Use custom API endpoint
harness login --endpoint https://app.harness.io
```

### Pipeline Execution Errors
```bash
# Get detailed execution logs
harness pipeline execution logs \
  --execution-id <id> \
  --stage <stage-name>

# Check pipeline validation
harness pipeline validate --pipeline-id <id>
```

## Platform Support
- ✅ Linux (binary installation)
- ✅ macOS (Homebrew)
- ⚠️  Windows (binary download available)

## Uninstall
```yaml
- preset: harness
  with:
    state: absent
```

## Resources
- Official docs: https://docs.harness.io/
- CLI reference: https://docs.harness.io/article/u7vyy6e2zs-harness-cli
- GitHub: https://github.com/harness/harness-cli
- Community: https://community.harness.io/
- Search: "harness cli tutorial", "harness pipeline automation", "harness gitops"

# doppler - Secrets Management Platform

Universal secrets manager for applications and infrastructure.

## Quick Start
```yaml
- preset: doppler
```

## Features
- **Universal secrets**: Manage secrets across all environments
- **Version control**: Track secret changes with full history
- **Access control**: Role-based permissions and audit logs
- **Integrations**: 50+ platform integrations (AWS, GCP, Azure, etc.)
- **CLI & API**: Programmatic access to secrets
- **Dynamic secrets**: Auto-rotate credentials

## Basic Usage
```bash
# Login
doppler login

# Setup project
doppler setup

# Run command with secrets injected
doppler run -- node server.js

# Get secret value
doppler secrets get API_KEY

# Set secret
doppler secrets set API_KEY="secret_value"

# List all secrets
doppler secrets

# Download secrets as env file
doppler secrets download --no-file --format env > .env
```

## Advanced Configuration
```yaml
# Install doppler
- preset: doppler

# Uninstall
- preset: doppler
  with:
    state: absent
```

## Project Setup
```bash
# Initialize new project
doppler projects create myapp

# Create environments
doppler environments create dev
doppler environments create staging
doppler environments create production

# Switch environment
doppler setup --project myapp --config dev

# View current config
doppler setup
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Real-World Examples

### Node.js Application
```bash
# Replace dotenv
# Before: require('dotenv').config()
# After:  doppler run -- node server.js

# server.js
const apiKey = process.env.API_KEY;
const dbUrl = process.env.DATABASE_URL;
```

### Docker Integration
```dockerfile
FROM node:18
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .

# Install doppler
RUN apt-get update && apt-get install -y apt-transport-https ca-certificates curl gnupg && \
    curl -sLf --retry 3 --tlsv1.2 --proto "=https" 'https://packages.doppler.com/public/cli/gpg.DE2A7741A397C129.key' | apt-key add - && \
    echo "deb https://packages.doppler.com/public/cli/deb/debian any-version main" | tee /etc/apt/sources.list.d/doppler-cli.list && \
    apt-get update && apt-get install doppler

CMD ["doppler", "run", "--", "node", "server.js"]
```

### CI/CD Pipeline
```yaml
# GitHub Actions
- name: Deploy with secrets
  env:
    DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}
  run: |
    curl -Ls https://cli.doppler.com/install.sh | sh
    doppler run --token "$DOPPLER_TOKEN" -- ./deploy.sh
```

### Kubernetes Secrets
```bash
# Create Kubernetes secret from Doppler
doppler secrets download --no-file --format json | \
  kubectl create secret generic app-secrets --from-file=secrets=/dev/stdin
```

## Integrations

### AWS Secrets Manager Sync
```bash
# Configure AWS sync
doppler integrations create aws-secrets-manager \
  --region us-east-1 \
  --sync-interval 1h
```

### GitHub Actions
```yaml
# .github/workflows/deploy.yml
- uses: dopplerhq/cli-action@v1
- run: doppler run -- ./deploy.sh
  env:
    DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}
```

### Terraform
```hcl
# terraform.tf
data "doppler_secrets" "this" {}

resource "aws_lambda_function" "app" {
  function_name = "my-function"
  
  environment {
    variables = data.doppler_secrets.this.map
  }
}
```

## Agent Use
- Inject secrets into deployment pipelines
- Rotate credentials automatically
- Sync secrets across cloud providers
- Audit secret access
- Manage multi-environment configurations
- Replace hardcoded credentials

## Troubleshooting

### Authentication failed
```bash
# Re-login
doppler logout
doppler login

# Use service token
export DOPPLER_TOKEN="dp.st.xxx"
doppler secrets
```

### Project not configured
```bash
# Setup project and config
doppler setup --project myapp --config production
```

## Uninstall
```yaml
- preset: doppler
  with:
    state: absent
```

## Resources
- Official docs: https://docs.doppler.com/
- CLI reference: https://docs.doppler.com/docs/cli
- Search: "doppler secrets management", "doppler tutorial"

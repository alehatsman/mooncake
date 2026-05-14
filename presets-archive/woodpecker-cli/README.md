# Woodpecker CLI - CI/CD Command Line Tool

Command-line interface for Woodpecker CI, a lightweight CI/CD system focused on containers and simplicity.

## Quick Start
```yaml
- preset: woodpecker-cli
```

## Features
- **Pipeline Management**: Create, trigger, and manage CI/CD pipelines
- **Log Streaming**: Real-time build log viewing
- **Secret Management**: Secure secret storage and injection
- **Repository Control**: Configure repositories and webhooks
- **Lightweight**: Container-native with minimal configuration
- **Self-hosted**: Full control over CI/CD infrastructure

## Basic Usage
```bash
# Configure CLI
woodpecker-cli config set server https://ci.example.com
woodpecker-cli config set token your-api-token

# View pipelines
woodpecker-cli pipeline list --repo owner/repo

# Trigger pipeline
woodpecker-cli pipeline start --repo owner/repo

# View logs
woodpecker-cli log view --repo owner/repo --build 123

# Manage secrets
woodpecker-cli secret add --repository owner/repo --name SECRET_KEY --value secret123
woodpecker-cli secret list --repository owner/repo

# Repository info
woodpecker-cli repo info owner/repo
woodpecker-cli repo list
```

## Advanced Configuration
```yaml
- preset: woodpecker-cli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Woodpecker CLI |

## Platform Support
- ✅ Linux (binary, packages)
- ✅ macOS (binary, Homebrew)
- ✅ Windows (binary)

## Configuration
- **Config file**: `~/.woodpecker/config.yaml`
- **Environment**: `WOODPECKER_SERVER`, `WOODPECKER_TOKEN`

## Real-World Examples

### CI/CD Pipeline
```yaml
# .woodpecker.yml
pipeline:
  build:
    image: golang:1.20
    commands:
      - go build
      - go test

  docker:
    image: plugins/docker
    settings:
      repo: registry.example.com/myapp
      tags: latest
    when:
      branch: main
```

```bash
# Trigger pipeline
woodpecker-cli pipeline start --repo owner/myapp
```

### Secret Management
```bash
# Add secrets
woodpecker-cli secret add \
  --repository owner/repo \
  --name DOCKER_PASSWORD \
  --value secretpass

# List secrets
woodpecker-cli secret list --repository owner/repo

# Delete secret
woodpecker-cli secret rm --repository owner/repo --name OLD_SECRET
```

## Agent Use
- Automated CI/CD pipeline triggering
- Build status monitoring
- Secret management automation
- Repository configuration
- Log aggregation and analysis

## Uninstall
```yaml
- preset: woodpecker-cli
  with:
    state: absent
```

## Resources
- Official docs: https://woodpecker-ci.org/
- GitHub: https://github.com/woodpecker-ci/woodpecker
- Search: "woodpecker ci", "woodpecker pipeline"

# drone-cli - CI/CD Command-Line Tool

Command-line client for Drone CI/CD platform.

## Quick Start
```yaml
- preset: drone-cli
```

## Features
- **Pipeline management**: Create and manage CI/CD pipelines
- **Secret management**: Store and access build secrets
- **Repo activation**: Enable/disable repositories
- **Build control**: Trigger, restart, and cancel builds
- **Logs**: View build logs from command line
- **Plugin system**: Extend with custom plugins

## Basic Usage
```bash
# Configure server
export DRONE_SERVER=https://drone.example.com
export DRONE_TOKEN=your-token-here

# List repositories
drone repo ls

# Enable repository
drone repo enable myorg/myrepo

# List builds
drone build ls myorg/myrepo

# View build
drone build info myorg/myrepo 42

# View build logs
drone build logs myorg/myrepo 42

# Trigger build
drone build create myorg/myrepo --branch main

# Restart build
drone build restart myorg/myrepo 42

# Cancel build
drone build cancel myorg/myrepo 42
```

## Advanced Configuration
```yaml
# Install drone-cli
- preset: drone-cli

# Uninstall
- preset: drone-cli
  with:
    state: absent
```

## Pipeline Definition
```yaml
# .drone.yml
kind: pipeline
type: docker
name: default

steps:
- name: test
  image: golang:1.21
  commands:
  - go test ./...

- name: build
  image: golang:1.21
  commands:
  - go build -o app

- name: deploy
  image: plugins/ssh
  settings:
    host: server.example.com
    username: deploy
    password:
      from_secret: deploy_password
    script:
      - ./deploy.sh
  when:
    branch:
    - main
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

### Manage Secrets
```bash
# Add secret
drone secret add myorg/myrepo \
  --name docker_password \
  --data "secret-value"

# List secrets
drone secret ls myorg/myrepo

# Delete secret
drone secret rm myorg/myrepo docker_password
```

### Repository Management
```bash
# Enable repo
drone repo enable myorg/myrepo

# Update repo settings
drone repo update myorg/myrepo \
  --timeout 60m \
  --trusted

# View repo info
drone repo info myorg/myrepo
```

### Build Management
```bash
# Trigger build with parameters
drone build create myorg/myrepo \
  --branch develop \
  --param VERSION=1.2.3

# Follow build logs
drone build logs --follow myorg/myrepo 42
```

## Agent Use
- Automate CI/CD pipeline management
- Trigger builds programmatically
- Monitor build status
- Manage secrets across projects
- Configure repository settings
- Deploy from command line

## Troubleshooting

### Connection failed
```bash
# Verify server URL
echo $DRONE_SERVER

# Test connection
drone info
```

### Authentication failed
```bash
# Get new token from Drone UI
# Settings -> Account -> Show Token

# Set environment variable
export DRONE_TOKEN=your-new-token
```

## Uninstall
```yaml
- preset: drone-cli
  with:
    state: absent
```

## Resources
- Official docs: https://docs.drone.io/
- CLI docs: https://docs.drone.io/cli/
- Search: "drone ci tutorial", "drone pipeline examples"

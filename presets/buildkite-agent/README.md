# Buildkite Agent - CI/CD Build Agent

Self-hosted agent for running Buildkite CI/CD pipeline builds on your own infrastructure.

## Quick Start
```yaml
- preset: buildkite-agent
```

## Features
- **Self-hosted**: Run builds on your own servers with full control
- **Parallel execution**: Scale horizontally with multiple agents
- **Docker support**: Native container build capabilities
- **Artifact management**: Upload/download build artifacts automatically
- **Plugin system**: Extend functionality with community plugins
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Check agent version
buildkite-agent --version

# Start agent (requires token)
buildkite-agent start --token YOUR_TOKEN

# Run a single build
buildkite-agent start --token YOUR_TOKEN --spawn 1 --disconnect-after-job

# Check agent status
buildkite-agent meta-data exists "key"

# Upload artifacts
buildkite-agent artifact upload "build/**/*"

# Download artifacts
buildkite-agent artifact download "build/**/*" .

# Annotate build
buildkite-agent annotate "Build completed successfully" --style success
```

## Configuration

### Config File Locations
- **Linux**: `/etc/buildkite-agent/buildkite-agent.cfg`
- **macOS**: `~/Library/Preferences/buildkite-agent/buildkite-agent.cfg`
- **User install**: `~/.buildkite-agent/buildkite-agent.cfg`

### Key Configuration Options
```bash
# Required: Agent token from Buildkite
token="YOUR_TOKEN_HERE"

# Agent name (defaults to hostname)
name="my-agent-%hostname"

# Number of parallel builds
spawn=1

# Build directory
build-path="/tmp/buildkite-builds"

# Hooks directory
hooks-path="/etc/buildkite-agent/hooks"

# Tags for targeting builds
tags="queue=default,os=linux,docker=true"

# Git configuration
git-clean-flags="-fdqx"
git-clone-flags="-v"

# Priority (higher numbers = higher priority)
priority=0
```

## Advanced Configuration

```yaml
# Install with custom configuration
- name: Create agent config
  template:
    dest: /etc/buildkite-agent/buildkite-agent.cfg
    content: |
      token="{{ buildkite_token }}"
      name="agent-%hostname"
      spawn=3
      tags="queue=deploy,os=linux,docker=true"
      priority=5
  become: true

- preset: buildkite-agent
  become: true

- name: Start agent service
  service:
    name: buildkite-agent
    state: started
    enabled: true
  become: true
```

## Agent Tags

Tags allow targeting specific agents for specific builds:

```bash
# In buildkite-agent.cfg
tags="queue=deploy,os=linux,docker=true,region=us-east-1"
```

In your pipeline.yml:
```yaml
steps:
  - label: "Deploy"
    command: "./deploy.sh"
    agents:
      queue: "deploy"
      os: "linux"
```

## Hooks

Buildkite agents support hooks for customizing build behavior:

### Available Hooks
- `environment` - Set environment variables
- `pre-checkout` - Before git checkout
- `post-checkout` - After git checkout
- `pre-command` - Before running build command
- `post-command` - After build command
- `pre-artifact` - Before artifact upload
- `post-artifact` - After artifact upload
- `pre-exit` - Before agent exits

### Example Hook
```bash
# /etc/buildkite-agent/hooks/environment
#!/bin/bash
set -euo pipefail

# Load Docker credentials
export DOCKER_USERNAME="myuser"
export DOCKER_PASSWORD_FILE="/secrets/docker-password"

# Set build environment
export NODE_ENV="production"
export GO111MODULE="on"
```

## Real-World Examples

### Docker-enabled Build Agent
```yaml
- name: Install Docker
  preset: docker
  become: true

- name: Configure Buildkite agent for Docker
  template:
    dest: /etc/buildkite-agent/buildkite-agent.cfg
    content: |
      token="{{ buildkite_token }}"
      name="docker-agent-%hostname"
      spawn=2
      tags="docker=true,queue=docker-builds"
  become: true

- preset: buildkite-agent
  become: true

- name: Add buildkite-agent to docker group
  shell: usermod -aG docker buildkite-agent
  become: true

- name: Start agent
  service:
    name: buildkite-agent
    state: restarted
    enabled: true
  become: true
```

### Multi-queue Agent Setup
```yaml
# Deploy queue agent
- name: Configure deploy queue agent
  template:
    dest: /etc/buildkite-agent/buildkite-agent.cfg
    content: |
      token="{{ buildkite_token }}"
      name="deploy-agent-%hostname"
      spawn=1
      tags="queue=deploy,env=production"
      priority=10
  become: true

- preset: buildkite-agent
  become: true

- name: Start deploy agent
  service:
    name: buildkite-agent
    state: started
    enabled: true
  become: true
```

### Autoscaling Agent
```yaml
# Install agent with autoscaling config
- preset: buildkite-agent
  become: true

- name: Configure for autoscaling
  template:
    dest: /etc/buildkite-agent/buildkite-agent.cfg
    content: |
      token="{{ buildkite_token }}"
      name="autoscale-agent-%hostname"
      spawn=1
      tags="queue=autoscale,instance={{ instance_id }}"
      disconnect-after-job=true
      disconnect-after-idle-timeout=300
  become: true
```

## Service Management

### Linux (systemd)
```bash
# Start agent
sudo systemctl start buildkite-agent

# Enable on boot
sudo systemctl enable buildkite-agent

# Check status
sudo systemctl status buildkite-agent

# View logs
sudo journalctl -u buildkite-agent -f

# Restart agent
sudo systemctl restart buildkite-agent
```

### macOS (launchd)
```bash
# Start agent
launchctl load ~/Library/LaunchAgents/com.buildkite.buildkite-agent.plist

# Stop agent
launchctl unload ~/Library/LaunchAgents/com.buildkite.buildkite-agent.plist

# View logs
tail -f ~/Library/Logs/buildkite-agent.log
```

## Troubleshooting

### Agent not connecting
```bash
# Check agent token
cat /etc/buildkite-agent/buildkite-agent.cfg | grep token

# Test connectivity
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://agent.buildkite.com/v3/register

# Check logs
sudo journalctl -u buildkite-agent -n 50
```

### Permission issues
```bash
# Ensure agent has access to build directory
sudo chown -R buildkite-agent:buildkite-agent /tmp/buildkite-builds

# For Docker access
sudo usermod -aG docker buildkite-agent
sudo systemctl restart buildkite-agent
```

### Build artifacts not uploading
```bash
# Check artifact path exists
ls -la build/

# Test manual upload
buildkite-agent artifact upload "build/**/*" \
  --job $BUILDKITE_JOB_ID

# Check agent has network access
curl https://api.buildkite.com/v2/ping
```

### Agent disconnecting
- Check `disconnect-after-job` setting in config
- Verify `disconnect-after-idle-timeout` value
- Check network stability and firewall rules
- Review agent logs for connection errors

## Security Best Practices

### Token Management
```bash
# Store token in separate file
echo "token=\"YOUR_TOKEN\"" | sudo tee /etc/buildkite-agent/token.cfg
sudo chmod 600 /etc/buildkite-agent/token.cfg

# Reference in main config
# In buildkite-agent.cfg
# token="$(cat /etc/buildkite-agent/token.cfg)"
```

### Sandboxing
```bash
# Run agent in restricted environment
# In buildkite-agent.cfg
plugins-path="/etc/buildkite-agent/plugins"
hooks-path="/etc/buildkite-agent/hooks"

# Use Docker plugin for isolation
# In pipeline.yml
steps:
  - plugins:
      - docker#v5.0.0:
          image: "node:18"
          command: ["npm", "test"]
```

## Platform Support
- ✅ Linux (apt, dnf, yum, Debian/Ubuntu packages)
- ✅ macOS (Homebrew)
- ✅ Windows (MSI installer - manual)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Provision self-hosted CI/CD infrastructure
- Scale build capacity dynamically based on load
- Set up deployment-specific build agents
- Create isolated build environments with specific tools
- Manage multi-region build infrastructure
- Configure specialized agents (Docker, GPU, ARM)

## Uninstall
```yaml
- preset: buildkite-agent
  with:
    state: absent
```

## Resources
- Official docs: https://buildkite.com/docs/agent/v3
- Installation guide: https://buildkite.com/docs/agent/v3/installation
- Configuration: https://buildkite.com/docs/agent/v3/configuration
- Hooks: https://buildkite.com/docs/agent/v3/hooks
- GitHub: https://github.com/buildkite/agent
- Search: "buildkite agent setup", "buildkite agent configuration", "buildkite docker agent"

# GitLab Runner - CI/CD Job Executor

Execute GitLab CI/CD pipeline jobs on your infrastructure. Self-hosted runner for GitLab CI/CD with support for multiple executors.

## Quick Start
```yaml
- preset: gitlab-runner
  become: true
```

## Features
- **Multiple executors**: Docker, Shell, Kubernetes, SSH support
- **Autoscaling**: Dynamic runner scaling with cloud providers
- **Concurrent jobs**: Run multiple jobs simultaneously
- **Cache support**: Distributed caching for faster builds
- **Security**: Isolated job execution with Docker or VMs

## Basic Usage
```bash
# Register runner
sudo gitlab-runner register \
  --url https://gitlab.com/ \
  --registration-token YOUR_TOKEN \
  --executor docker \
  --docker-image alpine:latest

# Start runner
sudo gitlab-runner start

# View status
sudo gitlab-runner status

# List registered runners
sudo gitlab-runner list

# Unregister runner
sudo gitlab-runner unregister --name my-runner
```

## Advanced Configuration
```yaml
- preset: gitlab-runner
  with:
    state: present
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove GitLab Runner |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman - systemd service)
- ✅ macOS (Homebrew - launchd service)
- ✅ Windows (installer - Windows service)

## Configuration
- **Config file**: `/etc/gitlab-runner/config.toml` (Linux), `~/.gitlab-runner/config.toml` (user mode)
- **Service**: `gitlab-runner` (systemd/launchd/Windows service)
- **Logs**: `journalctl -u gitlab-runner` (Linux), `/var/log/gitlab-runner/` (macOS)
- **Working directory**: `/home/gitlab-runner` (default)

## Real-World Examples

### Docker Executor Setup
```bash
# Register with Docker executor
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image "alpine:latest" \
  --description "docker-runner" \
  --tag-list "docker,linux" \
  --run-untagged="true" \
  --locked="false"
```

### Kubernetes Executor
```yaml
# /etc/gitlab-runner/config.toml
[[runners]]
  name = "kubernetes-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "kubernetes"
  [runners.kubernetes]
    host = ""
    namespace = "gitlab-runner"
    privileged = true
    cpu_limit = "1"
    memory_limit = "1Gi"
    service_cpu_limit = "1"
    service_memory_limit = "1Gi"
    helper_cpu_limit = "500m"
    helper_memory_limit = "100Mi"
```

### CI/CD Pipeline with Cache
```yaml
# .gitlab-ci.yml
build:
  image: node:18
  cache:
    paths:
      - node_modules/
  script:
    - npm install
    - npm run build
  artifacts:
    paths:
      - dist/
```

### Concurrent Jobs Configuration
```toml
# /etc/gitlab-runner/config.toml
concurrent = 4  # Run 4 jobs simultaneously

[[runners]]
  name = "concurrent-runner"
  limit = 2  # This runner handles max 2 concurrent jobs
```

## Agent Use
- Provision self-hosted CI/CD runners for GitLab projects
- Configure runners with specific executors (Docker, Kubernetes, Shell)
- Set up autoscaling runners for dynamic workloads
- Manage runner registration and token rotation
- Deploy runners in air-gapped or private networks
- Monitor runner health and job execution metrics

## Troubleshooting

### Runner not picking up jobs
```bash
# Check runner status
sudo gitlab-runner status

# Verify registration
sudo gitlab-runner verify

# Check GitLab connection
curl -v https://gitlab.com/

# View runner logs
sudo journalctl -u gitlab-runner -f
```

### Docker executor permission denied
```bash
# Add gitlab-runner user to docker group
sudo usermod -aG docker gitlab-runner

# Restart runner
sudo gitlab-runner restart

# Verify docker access
sudo -u gitlab-runner docker ps
```

### Jobs stuck in pending
```bash
# Check runner is active
gitlab-runner list

# Verify tags match job requirements
# Edit /etc/gitlab-runner/config.toml
# Set run_untagged = true or add matching tags

# Restart runner
sudo gitlab-runner restart
```

### High disk usage
```bash
# Clean Docker images/containers
docker system prune -a

# Configure cache size limit in config.toml
[runners.cache]
  Type = "s3"
  Shared = true
```

## Uninstall
```yaml
- preset: gitlab-runner
  with:
    state: absent
  become: true
```

## Resources
- Official docs: https://docs.gitlab.com/runner/
- GitHub: https://gitlab.com/gitlab-org/gitlab-runner
- Executors: https://docs.gitlab.com/runner/executors/
- Autoscaling: https://docs.gitlab.com/runner/configuration/autoscale.html
- Search: "gitlab runner setup", "gitlab ci docker", "gitlab runner kubernetes"

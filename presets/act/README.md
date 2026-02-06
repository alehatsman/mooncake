# act - Run GitHub Actions Locally

Run GitHub Actions workflows on your local machine with Docker. Test workflows before pushing, debug failures faster, and save CI minutes.

## Quick Start
```yaml
- preset: act
```

## Features
- **Local workflow testing**: Run GitHub Actions without pushing commits
- **Fast iteration**: Test changes in seconds, not minutes
- **Cost savings**: Reduce cloud CI minutes by testing locally
- **Offline development**: Work on workflows without internet
- **Matrix testing**: Validate complex build matrices locally
- **Secret injection**: Test with production-like secrets safely
- **Debugging**: Verbose output and dry-run modes

## Basic Usage
```bash
# Run default workflow (push event)
act

# Run specific event
act pull_request
act push
act workflow_dispatch

# List workflows
act -l

# Dry run (show what would run)
act -n
```

## Events
```bash
# Push event (default)
act push

# Pull request
act pull_request

# Manual workflow
act workflow_dispatch

# Scheduled workflows
act schedule

# Custom event
act repository_dispatch -e event.json
```

## Running Specific Jobs
```bash
# Run specific job
act -j build

# Run specific workflow
act -W .github/workflows/test.yml

# Multiple jobs
act -j lint -j test

# Job from specific workflow
act -W .github/workflows/ci.yml -j build
```

## Environment Variables
```bash
# Set environment variable
act -s MY_SECRET=value

# From .env file
act --env-file .env

# Multiple secrets
act -s TOKEN=abc123 -s API_KEY=xyz789

# GitHub token
act -s GITHUB_TOKEN=$GITHUB_TOKEN
```

## Platform Selection
```bash
# Use specific Docker image
act -P ubuntu-latest=node:16

# Multiple platforms
act -P ubuntu-latest=node:16 -P ubuntu-20.04=node:14

# Use GitHub's images
act -P ubuntu-latest=catthehacker/ubuntu:act-latest
```

## Workflow Testing
```bash
# Test PR workflow
act pull_request

# Test with specific branch
act -e event.json
# event.json:
{
  "pull_request": {
    "head": {
      "ref": "feature-branch"
    }
  }
}

# Test workflow dispatch with inputs
cat > dispatch.json <<'EOF'
{
  "inputs": {
    "environment": "staging",
    "version": "v1.2.3"
  }
}
EOF
act workflow_dispatch -e dispatch.json
```

## Debugging
```bash
# Verbose output
act -v

# Very verbose
act -vv

# Show Docker commands
act --verbose

# Dry run (plan)
act -n

# Interactive mode (requires nectos/act fork)
act --container-architecture linux/amd64
```

## Matrix Strategies
```bash
# Run matrix jobs
act
# Automatically runs all matrix combinations

# Test specific matrix
cat > matrix-event.json <<'EOF'
{
  "inputs": {
    "matrix_os": "ubuntu-latest",
    "matrix_node": "16"
  }
}
EOF
act -e matrix-event.json
```

## Artifacts
```bash
# Enable artifact server
act --artifact-server-path /tmp/artifacts

# Upload artifacts work automatically
# Download artifacts saved to /tmp/artifacts
```

## Limitations
```bash
# These don't work in act:
# - actions/cache (no shared cache)
# - GitHub-hosted runners features
# - GITHUB_TOKEN permissions (limited)
# - Some GitHub API calls

# Workarounds:
# - Use local Docker cache
# - Mock GitHub API responses
# - Use personal access token
```

## CI/CD Integration
```bash
# Pre-commit hook
#!/bin/bash
# .git/hooks/pre-commit
act -n || {
  echo "Workflow validation failed"
  exit 1
}

# Pre-push validation
act pull_request --dry-run

# Local CI before push
act push && git push
```

## Common Workflows
```bash
# Test build workflow
act -j build

# Run tests locally
act -j test

# Lint before commit
act -j lint

# Deploy to staging
act workflow_dispatch \
  -e '{"inputs":{"environment":"staging"}}'

# Full CI pipeline
act push
```

## Docker Configuration
```bash
# Use custom Docker socket
act --bind

# Container options
act --container-architecture linux/amd64
act --container-daemon-socket /var/run/docker.sock

# Network mode
act --network host

# Remove containers after run
act --rm
```

## Reusable Workflows
```bash
# Test reusable workflow
# .github/workflows/reusable.yml
act workflow_call

# With inputs
act workflow_call -e inputs.json
```

## Event Files
```json
// push-event.json
{
  "ref": "refs/heads/main",
  "repository": {
    "name": "myrepo",
    "owner": {
      "name": "myorg"
    }
  }
}

// pr-event.json
{
  "pull_request": {
    "number": 123,
    "head": {
      "ref": "feature-branch"
    },
    "base": {
      "ref": "main"
    }
  }
}

// workflow_dispatch.json
{
  "inputs": {
    "environment": "production",
    "version": "v1.0.0",
    "dry_run": "false"
  }
}
```

## Configuration File
```yaml
# .actrc
-P ubuntu-latest=catthehacker/ubuntu:act-latest
-P ubuntu-20.04=catthehacker/ubuntu:act-20.04
--artifact-server-path /tmp/artifacts
--env-file .env
```

## Configuration
- **Config file**: `.actrc` in project root or `~/.actrc` (global)
- **Artifact directory**: Specify with `--artifact-server-path`
- **Docker images**: Default uses `node:16-buster-slim`, configure via `-P` flag
- **Cache**: Docker image cache in `~/.docker/`
- **Requires**: Docker daemon running locally

## Real-World Examples

### Pre-Push Validation
```bash
#!/bin/bash
# .git/hooks/pre-push
echo "Testing workflows locally..."
act push --dry-run
if [ $? -ne 0 ]; then
  echo "Workflow validation failed. Fix errors before pushing."
  exit 1
fi
```

### CI Pipeline Testing
```yaml
# Test full CI before merging PR
- name: Validate pull request workflows
  preset: act

- name: Run PR checks locally
  shell: |
    act pull_request --env-file .env.test
    act push -j lint -j test -j build
```

### Multi-Environment Deployment Testing
```bash
# Test deployments to different environments
for env in dev staging prod; do
  echo "Testing $env deployment..."
  act workflow_dispatch \
    -e deployment-events.json \
    --secret-file ".env.$env" \
    -j "deploy-$env"
done
```

## Troubleshooting

### Docker permission denied
User not in docker group or daemon not running.
```bash
# Check Docker daemon
docker ps

# Add user to docker group (Linux)
sudo usermod -aG docker $USER
newgrp docker

# Or use sudo
sudo act
```

### Workflow file not found
Running outside Git repository or workflows in wrong location.
```bash
# Verify workflow files exist
ls -la .github/workflows/

# Specify workflow explicitly
act -W .github/workflows/ci.yml
```

### Actions failing with "connection refused"
Service dependencies not available or network issues.
```bash
# Use host network for service access
act --network host

# Or start services with docker-compose first
docker-compose up -d postgres redis
act
```

### Platform image not found
Default image missing or wrong architecture.
```bash
# Pull recommended image
docker pull catthehacker/ubuntu:act-latest

# Use in act
act -P ubuntu-latest=catthehacker/ubuntu:act-latest

# Add to .actrc to make persistent
echo "-P ubuntu-latest=catthehacker/ubuntu:act-latest" >> .actrc
```

## Best Practices
- **Test before pushing** to catch errors early and save CI minutes
- **Use `.actrc`** for project-specific configuration
- **Mock external services** or use test instances
- **Store secrets** in `.env` files (add to .gitignore)
- **Use `--dry-run`** to validate workflow syntax
- **Match platform images** to GitHub-hosted runners
- **Cache Docker images** to speed up subsequent runs
- **Combine with actionlint** for comprehensive validation

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Pre-commit workflow validation
- Local CI/CD testing
- Workflow debugging
- Integration test automation
- Deployment dry-runs
- Cost reduction (fewer cloud runs)

## Advanced Configuration
```yaml
# Use act with custom event files and platform configuration
- name: Install act
  preset: act

- name: Test workflow with custom platform
  shell: |
    act -P ubuntu-latest=catthehacker/ubuntu:act-latest

- name: Test with secrets from file
  shell: |
    act --env-file .env.test pull_request
```

## Uninstall
```yaml
- preset: act
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/nektos/act
- Search: "act github actions local", "act examples"

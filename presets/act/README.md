# act - Local GitHub Actions

Run GitHub Actions locally with Docker. Test workflows before pushing, debug failures, iterate faster.

## Quick Start
```yaml
- preset: act
```

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

## Best Practices
- **Test before pushing** to save CI minutes
- **Use `.actrc`** for consistent configuration
- **Mock external services** in local tests
- **Set secrets** via environment file
- **Use `--dry-run`** to validate syntax
- **Match platform images** to GitHub runners
- **Cache Docker images** for faster runs

## Tips
- Saves GitHub Actions minutes
- Faster iteration (no push required)
- Debug workflows locally
- Test complex matrix strategies
- Validate syntax before commit
- Works offline (after image pull)
- Great for private repos

## Agent Use
- Pre-commit workflow validation
- Local CI/CD testing
- Workflow debugging
- Integration test automation
- Deployment dry-runs
- Cost reduction (fewer cloud runs)

## Uninstall
```yaml
- preset: act
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/nektos/act
- Search: "act github actions local", "act examples"

# CircleCI CLI - CI/CD Tool

Command-line tool for interacting with CircleCI from your terminal, validating configs and triggering builds.

## Quick Start
```yaml
- preset: circleci-cli
```

## Features
- **Config validation**: Validate .circleci/config.yml locally
- **Local execution**: Run jobs locally with Docker
- **Pipeline triggering**: Trigger builds from command line
- **Orb management**: Create and publish CircleCI orbs
- **API access**: Full CircleCI API access from CLI
- **Context management**: Manage environment variables

## Basic Usage
```bash
# Setup and authenticate
circleci setup

# Validate config
circleci config validate

# Process config (expand orbs)
circleci config process .circleci/config.yml

# Run job locally
circleci local execute --job build

# Trigger pipeline
circleci pipeline trigger

# List pipelines
circleci pipeline list
```

## Advanced Usage
```bash
# Run specific job with parameters
circleci local execute --job test \
  --env KEY=value

# Validate config from stdin
cat .circleci/config.yml | circleci config validate -

# Trigger with parameters
circleci pipeline trigger \
  --parameters environment=production \
  --branch main

# Get workflow status
circleci workflow list

# Follow build logs
circleci step halt
```

## Real-World Examples

### Pre-commit Validation
```yaml
- name: Install CircleCI CLI
  preset: circleci-cli

- name: Validate CI config
  shell: circleci config validate .circleci/config.yml
  cwd: /project
```

### Local Testing
```yaml
- name: Test job locally
  shell: circleci local execute --job build
  cwd: /app
```

### Trigger Deployment
```bash
# Trigger production deployment
circleci pipeline trigger \
  --parameters run_deploy=true \
  --parameters environment=production \
  --branch main
```

## Platform Support
- ✅ Linux (binary, Homebrew)
- ✅ macOS (Homebrew)
- ✅ Windows (chocolatey)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Validate CircleCI configs before pushing
- Test jobs locally before committing
- Trigger deployments from automation
- Manage orbs and contexts
- Integrate CircleCI with other tools


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install circleci-cli
  preset: circleci-cli

- name: Use circleci-cli in automation
  shell: |
    # Custom configuration here
    echo "circleci-cli configured"
```
## Uninstall
```yaml
- preset: circleci-cli
  with:
    state: absent
```

## Resources
- Official docs: https://circleci.com/docs/local-cli/
- GitHub: https://github.com/CircleCI-Public/circleci-cli
- Search: "circleci cli tutorial", "circleci local", "circleci cli examples"

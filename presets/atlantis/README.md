# Atlantis - Terraform Pull Request Automation

Automate Terraform workflows in pull requests. Run plan, apply, and unlock commands via PR comments for GitOps-style infrastructure management.

## Quick Start
```yaml
- preset: atlantis
```

## Features
- **PR-driven workflows**: Terraform plan/apply via pull request comments
- **Git provider support**: GitHub, GitLab, Bitbucket, Azure DevOps
- **Approval workflows**: Require approvals before apply
- **Lock management**: Prevent concurrent modifications
- **Policy enforcement**: OPA/Sentinel policy checks
- **Multi-repo support**: Manage multiple Terraform repositories
- **Webhook automation**: Automatic reactions to PR events

## Basic Usage
```bash
# Check version
atlantis version

# Run Atlantis server
atlantis server \
  --atlantis-url="https://atlantis.example.com" \
  --gh-user="atlantis-bot" \
  --gh-token="$GITHUB_TOKEN" \
  --repo-allowlist="github.com/myorg/*"
```

## Pull Request Commands
```bash
# Comment on PR to trigger Atlantis:

# Run terraform plan
atlantis plan

# Plan specific directory
atlantis plan -d terraform/staging

# Plan specific project
atlantis plan -p staging

# Apply changes
atlantis apply

# Apply specific directory
atlantis apply -d terraform/staging

# Unlock
atlantis unlock

# Show help
atlantis help
```

## Server Configuration
```yaml
# atlantis.yaml (repository root)
version: 3
automerge: true
delete_source_branch_on_merge: true

projects:
  - name: staging
    dir: terraform/staging
    workspace: default
    autoplan:
      when_modified: ["*.tf", "*.tfvars"]
      enabled: true
    apply_requirements: [approved, mergeable]

  - name: production
    dir: terraform/production
    workspace: default
    autoplan:
      when_modified: ["*.tf"]
      enabled: true
    apply_requirements: [approved, mergeable]
    workflow: production

workflows:
  production:
    plan:
      steps:
        - init
        - plan:
            extra_args: ["-lock-timeout=5m"]
    apply:
      steps:
        - apply:
            extra_args: ["-lock-timeout=10m"]
```

## Advanced Configuration
```yaml
# Server-side config (server.yaml)
repos:
  - id: github.com/myorg/infrastructure
    branch: /.*/
    apply_requirements: [approved]
    allowed_overrides: [apply_requirements, workflow]
    allow_custom_workflows: true
    delete_source_branch_on_merge: true
    pre_workflow_hooks:
      - run: terraform fmt -check
      - run: tflint
    post_workflow_hooks:
      - run: ./scripts/notify-slack.sh

workflows:
  custom:
    plan:
      steps:
        - env:
            name: TF_VAR_environment
            command: echo $HEAD_BRANCH_NAME | cut -d/ -f2
        - init
        - plan
    apply:
      steps:
        - apply
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Configuration

### GitHub Setup
```bash
# Create GitHub webhook
# URL: https://atlantis.example.com/events
# Content type: application/json
# Events: Pull requests, Issue comments, Push

# Create bot user token (need repo access)
ATLANTIS_GH_TOKEN="ghp_..."

# Run server
atlantis server \
  --atlantis-url="https://atlantis.example.com" \
  --gh-user="atlantis-bot" \
  --gh-token="$ATLANTIS_GH_TOKEN" \
  --gh-webhook-secret="$WEBHOOK_SECRET" \
  --repo-allowlist="github.com/myorg/*"
```

### GitLab Setup
```bash
# Create GitLab webhook
# URL: https://atlantis.example.com/events
# Trigger: Comments, Merge requests

# Create access token
ATLANTIS_GITLAB_TOKEN="glpat-..."

atlantis server \
  --atlantis-url="https://atlantis.example.com" \
  --gitlab-user="atlantis-bot" \
  --gitlab-token="$ATLANTIS_GITLAB_TOKEN" \
  --gitlab-webhook-secret="$WEBHOOK_SECRET" \
  --repo-allowlist="gitlab.com/myorg/*"
```

## Real-World Examples

### CI/CD Pipeline Setup
```yaml
# docker-compose.yml - Deploy Atlantis
version: '3'
services:
  atlantis:
    image: ghcr.io/runatlantis/atlantis:latest
    ports:
      - "4141:4141"
    environment:
      ATLANTIS_ATLANTIS_URL: https://atlantis.example.com
      ATLANTIS_GH_USER: atlantis-bot
      ATLANTIS_GH_TOKEN: ${GITHUB_TOKEN}
      ATLANTIS_GH_WEBHOOK_SECRET: ${WEBHOOK_SECRET}
      ATLANTIS_REPO_ALLOWLIST: github.com/myorg/*
      ATLANTIS_DATA_DIR: /atlantis-data
    volumes:
      - atlantis-data:/atlantis-data
    command: server

volumes:
  atlantis-data:
```

### Multi-Environment Terraform
```yaml
# atlantis.yaml - Separate staging and production workflows
version: 3

projects:
  - name: staging-us-west
    dir: terraform/staging
    workspace: us-west-2
    terraform_version: 1.5.0
    autoplan:
      when_modified: ["*.tf", "staging.tfvars"]
    apply_requirements: [approved]

  - name: staging-eu-west
    dir: terraform/staging
    workspace: eu-west-1
    terraform_version: 1.5.0
    autoplan:
      when_modified: ["*.tf", "staging.tfvars"]
    apply_requirements: [approved]

  - name: production
    dir: terraform/production
    terraform_version: 1.5.0
    autoplan:
      enabled: false  # Manual planning only
    apply_requirements: [approved, mergeable]
    workflow: production-safe

workflows:
  production-safe:
    plan:
      steps:
        - init
        - run: tfsec .
        - run: checkov -d .
        - plan
```

### Policy Enforcement
```yaml
# atlantis.yaml with OPA policies
version: 3

projects:
  - name: infrastructure
    dir: terraform
    workflow: policy-check

workflows:
  policy-check:
    plan:
      steps:
        - init
        - plan
        - run: |
            terraform show -json plan.tfplan > plan.json
            conftest test plan.json -p policies/
    apply:
      steps:
        - apply
```

### Automated Notifications
```bash
#!/bin/bash
# post_workflow_hook.sh - Notify Slack on apply

if [ "$COMMAND_NAME" = "apply" ] && [ "$SUCCESS" = "true" ]; then
  curl -X POST $SLACK_WEBHOOK \
    -H 'Content-Type: application/json' \
    -d "{
      \"text\": \"Terraform apply completed for $PROJECT_NAME\",
      \"blocks\": [{
        \"type\": \"section\",
        \"text\": {
          \"type\": \"mrkdwn\",
          \"text\": \"*Terraform Apply Success*\nProject: $PROJECT_NAME\nPR: $PULL_NUM\nUser: $USER_NAME\"
        }
      }]
    }"
fi
```

## Agent Use
- Automate Terraform infrastructure changes via pull requests
- Enforce approval workflows for production changes
- Implement policy-as-code with automated checks
- Coordinate multi-region deployments with workspace management
- Integrate infrastructure changes with CI/CD pipelines
- Audit infrastructure modifications through Git history

## Troubleshooting

### Webhook Not Working
```bash
# Check Atlantis logs
docker logs atlantis

# Test webhook delivery
curl -X POST https://atlantis.example.com/events \
  -H "X-Hub-Signature: sha256=..." \
  -d @webhook-payload.json
```

### Lock Issues
```bash
# Comment on PR to unlock
atlantis unlock

# Server-side unlock
atlantis server unlock --repo github.com/myorg/infra --pull 123
```

### Permission Errors
```bash
# Verify bot has repo access
# Verify webhook secret matches
# Check repo allowlist includes your repository
```

## Comparison

### Atlantis vs Terraform Cloud
| Feature | Atlantis | Terraform Cloud |
|---------|----------|-----------------|
| Hosting | Self-hosted | SaaS |
| Cost | Free | Paid tiers |
| Git providers | Multiple | Multiple |
| Customization | High | Limited |
| State management | Your backend | Built-in |

## Uninstall
```yaml
- preset: atlantis
  with:
    state: absent
```

## Resources
- Official docs: https://www.runatlantis.io/
- GitHub: https://github.com/runatlantis/atlantis
- Configuration: https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html
- Search: "atlantis terraform", "atlantis workflows", "atlantis pr automation"

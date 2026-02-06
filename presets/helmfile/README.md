# helmfile - Declarative Helm Deployments

Deploy and manage multiple Helm releases declaratively. GitOps-friendly Helm chart orchestration.

## Quick Start
```yaml
- preset: helmfile
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage
```bash
# Sync all releases (install/upgrade)
helmfile sync

# Dry-run (show what would change)
helmfile diff

# Apply changes (safer than sync)
helmfile apply

# List releases
helmfile list

# Destroy all releases
helmfile destroy
```


## Advanced Configuration
```yaml
- preset: helmfile
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove helmfile |
## Helmfile Structure
```yaml
# helmfile.yaml
repositories:
  - name: stable
    url: https://charts.helm.sh/stable
  - name: bitnami
    url: https://charts.bitnami.com/bitnami

releases:
  - name: nginx
    namespace: web
    chart: bitnami/nginx
    version: 13.2.0
    values:
      - values/nginx.yaml
    set:
      - name: replicaCount
        value: 3

  - name: postgres
    namespace: db
    chart: bitnami/postgresql
    values:
      - values/postgres-{{ .Environment.Name }}.yaml
```

## Environments
```yaml
# helmfile.yaml
environments:
  dev:
    values:
      - environments/dev/values.yaml
  staging:
    values:
      - environments/staging/values.yaml
  production:
    values:
      - environments/production/values.yaml

releases:
  - name: myapp
    chart: ./charts/myapp
    values:
      - values/common.yaml
      - values/{{ .Environment.Name }}.yaml
```

```bash
# Deploy to specific environment
helmfile -e dev sync
helmfile -e production sync
```

## Selectors
```bash
# Sync specific release
helmfile -l name=nginx sync

# Multiple selectors
helmfile -l tier=frontend,env=prod sync

# Exclude releases
helmfile -l name!=nginx sync
```

## Values Management
```yaml
releases:
  - name: myapp
    chart: stable/myapp
    values:
      # Inline values
      - replicaCount: 3
        image:
          tag: v1.2.3
      # External files
      - values/common.yaml
      - values/{{ .Environment.Name }}.yaml
      # Go templates
      - image:
          tag: {{ .Values.imageTag }}
```

## Secrets Management
```yaml
# With sops
releases:
  - name: myapp
    chart: ./myapp
    secrets:
      - secrets/{{ .Environment.Name }}.yaml  # Encrypted with sops

# With vals
releases:
  - name: myapp
    chart: ./myapp
    values:
      - database:
          password: ref+vault://secret/data/db#password
```

## Hooks
```yaml
releases:
  - name: myapp
    chart: ./myapp
    hooks:
      # Run before sync
      - events: ["presync"]
        command: "./scripts/backup-db.sh"
      # Run after sync
      - events: ["postsync"]
        command: "./scripts/smoke-test.sh"
      # Cleanup hook
      - events: ["preuninstall"]
        command: "./scripts/drain-traffic.sh"
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Deploy with helmfile
  run: |
    helmfile -e ${{ github.ref_name }} diff
    helmfile -e ${{ github.ref_name }} apply

# GitLab CI
deploy:
  script:
    - helmfile -e ${CI_ENVIRONMENT_NAME} diff
    - helmfile -e ${CI_ENVIRONMENT_NAME} sync
  only:
    - main

# Validation
helmfile lint
helmfile template | kubectl apply --dry-run=client -f -
```

## Advanced Patterns
```yaml
# Templating with environments
releases:
  - name: myapp-{{ .Environment.Name }}
    namespace: {{ .Environment.Values.namespace }}
    chart: ./charts/myapp
    version: {{ .Environment.Values.appVersion }}

# Conditional releases
releases:
  - name: redis
    chart: bitnami/redis
    condition: redis.enabled  # From values file

# Dependencies
releases:
  - name: postgres
    chart: bitnami/postgresql

  - name: backend
    chart: ./backend
    needs:
      - postgres  # Wait for postgres
```

## Directory Structure
```
.
├── helmfile.yaml
├── environments/
│   ├── dev/
│   │   └── values.yaml
│   ├── staging/
│   │   └── values.yaml
│   └── production/
│       └── values.yaml
├── values/
│   ├── common.yaml
│   ├── nginx.yaml
│   └── postgres.yaml
├── secrets/
│   ├── dev.yaml.enc
│   └── prod.yaml.enc
└── charts/
    └── myapp/
```

## Common Workflows
```bash
# Preview changes
helmfile diff

# Apply changes with confirmation
helmfile apply

# Sync specific namespace
helmfile -l namespace=web sync

# Update all charts
helmfile deps

# Template and review
helmfile template

# Destroy and recreate
helmfile destroy && helmfile sync
```

## Multi-Environment Deploy
```bash
#!/bin/bash
# deploy.sh
ENV=$1

case $ENV in
  dev|staging|production)
    helmfile -e $ENV diff
    read -p "Apply changes? (y/n) " -n 1 -r
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      helmfile -e $ENV apply
    fi
    ;;
  *)
    echo "Usage: $0 {dev|staging|production}"
    exit 1
    ;;
esac
```

## GitOps Pattern
```yaml
# helmfile.yaml (in Git repo)
repositories:
  - name: bitnami
    url: https://charts.bitnami.com/bitnami

releases:
  - name: app
    chart: bitnami/nginx
    values:
      - git::https://github.com/org/config@config/{{ .Environment.Name }}/nginx.yaml?ref=main
```

```bash
# CI pipeline watches repo
helmfile -e production diff > diff.txt
if [ -s diff.txt ]; then
  helmfile -e production apply
fi
```

## Debugging
```bash
# Show rendered values
helmfile -e dev template

# Verbose output
helmfile --debug sync

# Test without applying
helmfile --dry-run sync

# Skip dependencies
helmfile --skip-deps sync
```

## Best Practices
- **Version control**: Commit helmfile.yaml
- **Secrets**: Encrypt with sops/vals
- **Diff first**: Always run `helmfile diff`
- **Environments**: Separate configs per environment
- **Dependencies**: Use `needs` for ordering
- **Validation**: Run `helmfile lint`
- **Atomic operations**: Use `helmfile apply`

## Tips
- Use selectors (`-l`) for targeted updates
- Leverage templates for DRY configs
- Store secrets encrypted in Git
- Run diffs in CI/CD before applying
- Use environments for multi-env deployments
- Test with `--dry-run` flag

## Agent Use
- Declarative infrastructure deployment
- Multi-environment orchestration
- GitOps workflows
- Release coordination
- Configuration management

## Uninstall
```yaml
- preset: helmfile
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/helmfile/helmfile
- Docs: https://helmfile.readthedocs.io/
- Search: "helmfile gitops", "helmfile examples"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

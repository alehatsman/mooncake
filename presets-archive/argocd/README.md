# argocd - GitOps Continuous Delivery

Declarative GitOps CD for Kubernetes. Sync apps from Git, automate deployments, manage multi-cluster environments.

## Quick Start
```yaml
- preset: argocd
```

## Features
- **GitOps workflow**: Declarative app deployment from Git repositories
- **Multi-cluster support**: Manage applications across multiple Kubernetes clusters
- **Automated sync**: Continuous reconciliation of desired vs actual state
- **Web UI + CLI**: Full-featured GUI and command-line interface
- **SSO integration**: OIDC, SAML, LDAP authentication
- **RBAC**: Fine-grained access control for projects and applications
- **Rollback**: Easy rollback to previous versions
- **Health assessment**: Automatic application health checking

## Basic Usage
```bash
# Login
argocd login argocd.example.com

# List applications
argocd app list

# Get app details
argocd app get myapp

# Sync app
argocd app sync myapp

# View sync status
argocd app wait myapp
```

## Authentication
```bash
# Login with username/password
argocd login argocd.example.com --username admin

# Login with token
argocd login argocd.example.com --auth-token $ARGOCD_AUTH_TOKEN

# Login via SSO
argocd login argocd.example.com --sso

# Change password
argocd account update-password

# Logout
argocd logout argocd.example.com
```

## Application Management
```bash
# Create app from Git
argocd app create myapp \
  --repo https://github.com/org/repo \
  --path k8s/overlays/prod \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace production

# Create with Helm
argocd app create myapp \
  --repo https://github.com/org/charts \
  --path mychart \
  --helm-set image.tag=v1.2.3 \
  --dest-server https://kubernetes.default.svc

# Create from Kustomize
argocd app create myapp \
  --repo https://github.com/org/repo \
  --path overlays/production \
  --kustomize-image myapp:v1.2.3

# Delete app
argocd app delete myapp

# Delete with cascade (remove K8s resources)
argocd app delete myapp --cascade
```

## Syncing
```bash
# Manual sync
argocd app sync myapp

# Sync specific resource
argocd app sync myapp --resource Deployment:myapp

# Dry run
argocd app sync myapp --dry-run

# Force sync (replace resources)
argocd app sync myapp --force

# Prune (remove extra resources)
argocd app sync myapp --prune

# Sync and wait
argocd app sync myapp --timeout 300
argocd app wait myapp --health
```

## Auto-Sync
```bash
# Enable auto-sync
argocd app set myapp --sync-policy automated

# With self-heal (auto-fix drift)
argocd app set myapp \
  --sync-policy automated \
  --self-heal

# With prune (auto-remove extra resources)
argocd app set myapp \
  --sync-policy automated \
  --auto-prune

# Disable auto-sync
argocd app unset myapp --sync-policy
```

## Viewing Resources
```bash
# List resources
argocd app resources myapp

# Get resource details
argocd app get myapp --show-operation

# View live manifest
argocd app manifests myapp

# Diff with Git
argocd app diff myapp

# View sync history
argocd app history myapp

# View specific revision
argocd app manifests myapp --revision HEAD~1
```

## Rollback
```bash
# List history
argocd app history myapp

# Rollback to specific revision
argocd app rollback myapp 5

# Rollback to previous
argocd app rollback myapp
```

## Health & Status
```bash
# Check health
argocd app get myapp

# Wait for healthy
argocd app wait myapp --health

# Wait for sync
argocd app wait myapp --sync

# Watch status
watch argocd app get myapp
```

## Multi-Cluster
```bash
# List clusters
argocd cluster list

# Add cluster
argocd cluster add my-cluster-context

# Remove cluster
argocd cluster rm https://my-cluster.example.com

# Get cluster info
argocd cluster get https://my-cluster.example.com
```

## Repository Management
```bash
# List repos
argocd repo list

# Add repo (public)
argocd repo add https://github.com/org/repo

# Add repo (private SSH)
argocd repo add git@github.com:org/repo.git \
  --ssh-private-key-path ~/.ssh/id_rsa

# Add repo (private HTTPS)
argocd repo add https://github.com/org/repo \
  --username user \
  --password $GITHUB_TOKEN

# Add Helm repo
argocd repo add https://charts.helm.sh/stable --type helm

# Remove repo
argocd repo rm https://github.com/org/repo
```

## Projects
```bash
# List projects
argocd proj list

# Create project
argocd proj create myproject \
  --description "My Project" \
  --dest https://kubernetes.default.svc,production

# Add source repo
argocd proj add-source myproject https://github.com/org/repo

# Add destination
argocd proj add-destination myproject \
  https://kubernetes.default.svc production

# Delete project
argocd proj delete myproject
```

## CI/CD Integration
```bash
# Trigger sync in CI
argocd app sync myapp --grpc-web

# Wait for deployment
argocd app wait myapp --health --timeout 300

# Check sync status
if argocd app get myapp | grep -q "Synced"; then
  echo "App synced successfully"
else
  echo "Sync failed"
  exit 1
fi

# Update image tag
argocd app set myapp --kustomize-image myapp:$CI_COMMIT_SHA
argocd app sync myapp
```

## GitHub Actions Example
```yaml
- name: Deploy to ArgoCD
  env:
    ARGOCD_AUTH_TOKEN: ${{ secrets.ARGOCD_TOKEN }}
  run: |
    argocd app set myapp \
      --kustomize-image myapp:${{ github.sha }} \
      --grpc-web
    argocd app sync myapp --grpc-web
    argocd app wait myapp --health --timeout 600
```

## Application Sets
```bash
# List appsets
argocd appset list

# Get appset details
argocd appset get my-appset

# Create from file
argocd appset create -f appset.yaml

# Delete appset
argocd appset delete my-appset
```

## Notifications
```bash
# List notification config
argocd admin notifications list

# Test notification
argocd admin notifications test myapp slack

# Template notifications
argocd admin notifications trigger get on-deployed
```

## Admin Operations
```bash
# Get version
argocd version

# Update admin password
argocd account update-password

# Generate bcrypt password
argocd account bcrypt --password mysecret

# Export applications
argocd app list -o yaml > apps-backup.yaml

# Import applications
kubectl apply -f apps-backup.yaml

# Cluster settings
argocd settings get
```

## Debugging
```bash
# Verbose output
argocd app get myapp --output yaml

# Show parameters
argocd app parameters myapp

# Show diffs
argocd app diff myapp --local .

# Validate manifests locally
argocd app manifests myapp --local-repo-root . | kubectl apply --dry-run=client -f -

# View logs
argocd app logs myapp

# View events
kubectl get events -n myapp
```

## Best Practices
- **Use projects** to organize applications
- **Enable auto-sync** with prune and self-heal for GitOps
- **Use --grpc-web** in CI/CD behind load balancers
- **Store secrets** in Sealed Secrets or External Secrets
- **Use ApplicationSets** for multi-environment deployments
- **Set resource hooks** for migrations and jobs
- **Use sync waves** for ordered deployments
- **Enable notifications** for sync events

## Tips
- Apps sync from Git every 3 minutes by default
- Use `--refresh` to force immediate check
- Health checks customizable per resource
- Supports Helm, Kustomize, Jsonnet, plain YAML
- Multi-tenancy via projects
- SSO integration (OIDC, SAML, LDAP)
- Webhook support for instant sync

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### CI/CD Pipeline Integration
```bash
#!/bin/bash
# deploy.sh - Automated deployment via ArgoCD

# Login with token from CI secret
argocd login argocd.example.com --auth-token $ARGOCD_TOKEN --grpc-web

# Update image tag in Git (GitOps approach)
git clone https://github.com/org/k8s-manifests
cd k8s-manifests
yq eval ".spec.template.spec.containers[0].image = \"myapp:$CI_COMMIT_SHA\"" \
  -i overlays/production/deployment.yaml
git commit -am "Update production to $CI_COMMIT_SHA"
git push

# Sync and wait for deployment
argocd app sync myapp-prod --prune --timeout 600
argocd app wait myapp-prod --health --timeout 600

# Verify deployment
argocd app get myapp-prod --show-params
```

### Multi-Environment Promotion
```bash
#!/bin/bash
# promote.sh - Promote image from staging to production

STAGING_IMAGE=$(argocd app get myapp-staging -o json | \
  jq -r '.spec.source.helm.parameters[] | select(.name=="image.tag") | .value')

echo "Promoting $STAGING_IMAGE from staging to production"

# Update production app
argocd app set myapp-prod \
  --helm-set image.tag=$STAGING_IMAGE \
  --helm-set environment=production

# Sync with confirmation
argocd app sync myapp-prod --dry-run
read -p "Proceed with production deployment? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  argocd app sync myapp-prod --timeout 600
  argocd app wait myapp-prod --health
  echo "Production deployment complete: $STAGING_IMAGE"
fi
```

### Disaster Recovery
```bash
#!/bin/bash
# rollback.sh - Quick rollback to previous revision

APP_NAME="myapp-prod"

# Get current and previous revisions
CURRENT=$(argocd app get $APP_NAME -o json | jq -r '.status.sync.revision')
HISTORY=$(argocd app history $APP_NAME -o json)
PREVIOUS=$(echo $HISTORY | jq -r '.[1].revision')

echo "Current: $CURRENT"
echo "Rolling back to: $PREVIOUS"

# Rollback
argocd app rollback $APP_NAME $(echo $HISTORY | jq -r '.[1].id')
argocd app wait $APP_NAME --sync --health --timeout 300

echo "Rollback complete"
```

### Cluster Migration
```bash
#!/bin/bash
# migrate-cluster.sh - Move app to new cluster

SOURCE_APP="myapp-old-cluster"
DEST_CLUSTER="https://new-cluster.k8s.local"

# Export app definition
argocd app get $SOURCE_APP -o yaml > myapp-export.yaml

# Create on new cluster
sed "s|destination:.*|destination:\n  server: $DEST_CLUSTER|g" \
  myapp-export.yaml | argocd app create -f -

# Sync to new cluster
argocd app sync myapp-new-cluster --timeout 600

# Verify before deletion
argocd app get myapp-new-cluster
read -p "Delete old app? (y/n) " -n 1 -r
echo
[[ $REPLY =~ ^[Yy]$ ]] && argocd app delete $SOURCE_APP --cascade
```

## Agent Use
- Automated GitOps deployment pipelines with sync and health checks
- Multi-environment promotion workflows (dev → staging → production)
- Disaster recovery and rollback automation
- Cluster migration and application portability
- Compliance enforcement through policy-as-code
- Application lifecycle management across Kubernetes clusters

## Uninstall

## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install argocd
  preset: argocd

- name: Use argocd in automation
  shell: |
    # Custom configuration here
    echo "argocd configured"
```

```yaml
- preset: argocd
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/argoproj/argo-cd
- Docs: https://argo-cd.readthedocs.io/
- Search: "argocd gitops", "argocd cli examples"

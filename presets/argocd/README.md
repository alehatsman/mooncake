# argocd - GitOps Continuous Delivery

Declarative GitOps CD for Kubernetes. Sync apps from Git, automate deployments, manage multi-cluster environments.

## Quick Start
```yaml
- preset: argocd
```

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

## Agent Use
- Automated deployment pipelines
- Multi-environment management
- GitOps workflow automation
- Cluster bootstrapping
- Application lifecycle management
- Compliance enforcement

## Uninstall
```yaml
- preset: argocd
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/argoproj/argo-cd
- Docs: https://argo-cd.readthedocs.io/
- Search: "argocd gitops", "argocd cli examples"

# Gitkube - Git Push to Kubernetes

Build and deploy Docker images to Kubernetes using git push. GitOps deployment tool for developers who want Heroku-like workflow on Kubernetes.

## Quick Start
```yaml
- preset: gitkube
```

## Features
- **Git push workflow**: Deploy with `git push gitkube master`
- **Automatic builds**: Build Docker images from source in-cluster
- **Zero configuration**: Works with existing Dockerfiles
- **Multi-environment**: Deploy to different namespaces
- **Service exposure**: Automatic service creation and ingress
- **Resource management**: Control CPU, memory, replicas via git

## Basic Usage
```bash
# Install Gitkube on cluster
gitkube install

# Create remote
gitkube remote create myapp

# Add git remote
git remote add gitkube ssh://default-myapp@gitkube-server/~/git/default-myapp

# Deploy application
git push gitkube master

# View deployments
kubectl get remotes
kubectl get deployments

# Check build logs
kubectl logs -f gitkubed-<pod-name>
```

## Advanced Configuration
```yaml
- preset: gitkube
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Gitkube |

## Platform Support
- ✅ Linux (Kubernetes cluster required)
- ✅ macOS (Kubernetes cluster required)
- ✅ Windows (Kubernetes cluster required)

## Configuration
- **Remote file**: `.gitkube.yaml` in repository root
- **Namespace**: Any Kubernetes namespace
- **SSH**: Git over SSH protocol
- **Builder**: In-cluster Docker builds

## Real-World Examples

### Basic Application Deploy
```yaml
# .gitkube.yaml
remotes:
  - name: myapp
    manifests:
      path: k8s/
    deployments:
      - name: myapp
        containers:
          - name: app
            path: .
            dockerfile: Dockerfile
```

```bash
# Deploy
git add .
git commit -m "Deploy v1.0"
git push gitkube master
```

### Multi-Service Application
```yaml
# .gitkube.yaml
remotes:
  - name: fullstack
    manifests:
      path: k8s/
    deployments:
      - name: frontend
        containers:
          - name: web
            path: frontend/
            dockerfile: frontend/Dockerfile
      - name: backend
        containers:
          - name: api
            path: backend/
            dockerfile: backend/Dockerfile
```

### Environment-Specific Deploys
```yaml
# Production remote
remotes:
  - name: production
    namespace: prod
    deployments:
      - name: myapp
        containers:
          - name: app
            path: .
            dockerfile: Dockerfile
        replicas: 5

# Staging remote
  - name: staging
    namespace: staging
    deployments:
      - name: myapp
        containers:
          - name: app
            path: .
            dockerfile: Dockerfile
        replicas: 2
```

```bash
# Deploy to different environments
git push gitkube-staging develop
git push gitkube-production master
```

### Service with Ingress
```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  ports:
    - port: 80
      targetPort: 8080
  selector:
    app: myapp
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
spec:
  rules:
    - host: myapp.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: myapp
                port:
                  number: 80
```

## Agent Use
- Enable git-based deployment workflows for Kubernetes
- Build and deploy microservices with single command
- Implement continuous deployment from git repositories
- Manage multi-environment deployments (dev/staging/prod)
- Automate Docker image builds in Kubernetes
- Simplify developer deployment experience

## Troubleshooting

### Remote creation fails
```bash
# Check Gitkube installation
kubectl get pods -n kube-system | grep gitkube

# Verify RBAC permissions
kubectl auth can-i create remotes --as=system:serviceaccount:default:gitkube

# Recreate remote
gitkube remote delete myapp
gitkube remote create myapp
```

### Git push rejected
```bash
# Check SSH keys
ssh-add -l

# Verify remote URL
git remote -v

# Re-add SSH key to Gitkube
cat ~/.ssh/id_rsa.pub | kubectl create secret generic gitkube-ssh \
  --from-file=authorized_keys=/dev/stdin

# Force push (use with caution)
git push gitkube master --force
```

### Build failures
```bash
# Check build logs
kubectl logs -f gitkubed-<pod>

# Verify Dockerfile
docker build -t test .

# Check resource limits
kubectl describe remote myapp

# Increase builder resources
# Edit remote spec
kubectl edit remote myapp
```

### Image not updating
```bash
# Clear image cache
kubectl delete pod gitkubed-<pod>

# Use image pull policy Always
# In .gitkube.yaml:
deployments:
  - name: myapp
    imagePullPolicy: Always
```

## Uninstall
```yaml
- preset: gitkube
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/hasura/gitkube
- Docs: https://gitkube.sh/
- Examples: https://github.com/hasura/gitkube/tree/master/examples
- Search: "gitkube kubernetes", "gitkube git push deploy"

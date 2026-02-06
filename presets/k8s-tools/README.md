# Kubernetes Tools Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check kubectl version
kubectl version --client

# Check helm version
helm version

# Check k9s version
k9s version

# View cluster info (requires cluster)
kubectl cluster-info
```

## Installed Tools

- **kubectl** - Kubernetes command-line tool
- **helm** - Kubernetes package manager
- **k9s** - Terminal UI for Kubernetes

## kubectl Common Operations

```bash
# Get cluster info
kubectl cluster-info
kubectl get nodes

# Get resources
kubectl get pods
kubectl get deployments
kubectl get services
kubectl get all

# Describe resource
kubectl describe pod <pod-name>

# View logs
kubectl logs <pod-name>
kubectl logs -f <pod-name>  # Follow

# Execute command
kubectl exec -it <pod-name> -- bash

# Apply configuration
kubectl apply -f deployment.yaml

# Delete resource
kubectl delete pod <pod-name>

# Scale deployment
kubectl scale deployment myapp --replicas=3

# Port forward
kubectl port-forward pod/myapp 8080:80

# Get resource YAML
kubectl get pod myapp -o yaml
```

## Helm Operations

```bash
# Add repository
helm repo add stable https://charts.helm.sh/stable
helm repo update

# Search charts
helm search repo nginx

# Install chart
helm install myrelease stable/nginx

# List releases
helm list

# Upgrade release
helm upgrade myrelease stable/nginx

# Uninstall release
helm uninstall myrelease

# Create chart
helm create mychart

# Package chart
helm package mychart
```

## k9s Usage

```bash
# Start k9s
k9s

# View specific namespace
k9s -n mynamespace

# Common shortcuts in k9s:
# :pods - View pods
# :svc - View services
# :deploy - View deployments
# d - Describe resource
# l - View logs
# s - Shell into pod
# / - Filter
# ? - Help
```

## kubectl Contexts

```bash
# List contexts
kubectl config get-contexts

# Switch context
kubectl config use-context minikube

# Set namespace
kubectl config set-context --current --namespace=myns
```

## Connecting to Cluster

For local development:
```bash
# Minikube
minikube start
kubectl config use-context minikube

# kind (Kubernetes in Docker)
kind create cluster
kubectl config use-context kind-kind

# Docker Desktop
# Enable Kubernetes in Docker Desktop settings
kubectl config use-context docker-desktop
```

## Uninstall

```yaml
- preset: k8s-tools
  with:
    state: absent
```

**Note:** kubeconfig files in `~/.kube/` preserved after uninstall.

# linkerd - Service Mesh for Kubernetes

Ultra-light service mesh for Kubernetes providing observability, reliability, and security without code changes.

## Quick Start
```yaml
- preset: linkerd
```

## Features
- **Zero-config**: Automatic protocol detection and mTLS
- **Ultra-light**: Minimal resource overhead with Rust-based proxy
- **Observability**: Golden metrics (success rate, latency, throughput)
- **Reliability**: Automatic retries, timeouts, load balancing
- **Security**: mTLS by default, policy enforcement
- **Progressive deployment**: Traffic splitting, blue-green deployments

## Basic Usage
```bash
# Check cluster compatibility
linkerd check --pre

# Install Linkerd
linkerd install | kubectl apply -f -

# Verify installation
linkerd check

# Inject sidecar into deployment
linkerd inject deployment.yaml | kubectl apply -f -

# Get service mesh metrics
linkerd stat deployment/myapp

# View service dependencies
linkerd viz tap deploy/myapp

# Check mTLS status
linkerd viz edges deployment

# Dashboard
linkerd viz dashboard
```

## Advanced Configuration
```yaml
- preset: linkerd
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove linkerd CLI |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (Homebrew)
- ✅ Windows (binary download)
- ✅ Kubernetes cluster required

## Configuration
- **Kubeconfig**: Uses `~/.kube/config` or `$KUBECONFIG`
- **Control plane**: Installed in `linkerd` namespace
- **Viz extension**: Optional observability components

## Real-World Examples

### Install Linkerd on Cluster
```bash
# Pre-flight checks
linkerd check --pre

# Install CRDs and control plane
linkerd install --crds | kubectl apply -f -
linkerd install | kubectl apply -f -

# Install viz extension for dashboard
linkerd viz install | kubectl apply -f -

# Verify
linkerd check
```

### Inject Existing Deployment
```yaml
# Add Linkerd annotation to trigger injection
- name: Annotate deployment
  shell: |
    kubectl annotate deployment myapp \
      linkerd.io/inject=enabled \
      --overwrite

- name: Restart pods
  shell: kubectl rollout restart deployment/myapp
```

### Traffic Splitting (Canary)
```yaml
apiVersion: split.smc.linkerd.io/v1alpha2
kind: TrafficSplit
metadata:
  name: myapp-canary
spec:
  service: myapp
  backends:
  - service: myapp-stable
    weight: 90
  - service: myapp-canary
    weight: 10
```

### Monitor Service Health
```bash
# Real-time metrics
linkerd viz stat deploy/myapp

# Success rate over time
linkerd viz routes deploy/myapp

# Live request tracing
linkerd viz tap deploy/myapp

# Top routes by traffic
linkerd viz top deploy/myapp
```

## Agent Use
- Service mesh deployment automation
- Canary deployment orchestration
- Microservice observability
- Zero-trust security implementation
- Service-to-service encryption

## Troubleshooting

### Pods not injected
Check namespace annotation:
```bash
kubectl get namespace -o yaml | grep linkerd.io/inject
```

Add annotation:
```bash
kubectl annotate namespace default linkerd.io/inject=enabled
```

### Control plane issues
Check control plane health:
```bash
linkerd check
kubectl -n linkerd get pods
```

### mTLS not working
Verify certificates:
```bash
linkerd viz edges deployment
```

Rotate certificates:
```bash
linkerd upgrade | kubectl apply -f -
```

## Uninstall
```yaml
- preset: linkerd
  with:
    state: absent
```

**Note**: This only removes the CLI. To uninstall from cluster:
```bash
linkerd viz uninstall | kubectl delete -f -
linkerd uninstall | kubectl delete -f -
```

## Resources
- Official docs: https://linkerd.io/
- Getting started: https://linkerd.io/getting-started/
- Architecture: https://linkerd.io/2/reference/architecture/
- Search: "linkerd service mesh", "linkerd kubernetes tutorial"

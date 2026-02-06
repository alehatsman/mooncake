# Ambassador - Kubernetes API Gateway

Kubernetes-native API gateway built on Envoy Proxy. Manage ingress traffic with advanced routing, authentication, and rate limiting.

## Quick Start
```yaml
- preset: ambassador
```

## Features
- **Envoy-based**: Built on Envoy Proxy for performance and reliability
- **Kubernetes-native**: Configuration via CRDs and annotations
- **Advanced routing**: Path, header, method-based routing
- **Authentication**: OAuth, JWT, API key authentication
- **Rate limiting**: Per-service and global rate limits
- **Observability**: Prometheus metrics, distributed tracing
- **Service mesh integration**: Works with Linkerd, Consul

## Basic Usage
```bash
# Install Ambassador
kubectl apply -f https://app.getambassador.io/yaml/ambassador/latest/ambassador.yaml

# Check status
kubectl get svc ambassador -n ambassador

# View mappings
kubectl get mappings

# Create mapping
kubectl apply -f mapping.yaml
```

## Advanced Configuration
```yaml
- preset: ambassador
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: ambassador
  with:
    state: absent
```

## Resources
- Search: "ambassador documentation", "ambassador tutorial"

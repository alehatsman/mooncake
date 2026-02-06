# envoy-gateway - Kubernetes Gateway API

Kubernetes-native Gateway API implementation using Envoy Proxy.

## Quick Start
```yaml
- preset: envoy-gateway
```

## Features
- **Gateway API**: Kubernetes Gateway API standard
- **Envoy-powered**: Built on Envoy Proxy
- **Cloud-native**: Kubernetes-first design
- **Extensible**: Plugin architecture
- **Production-ready**: Battle-tested in enterprise
- **Multi-tenancy**: Namespace isolation

## Basic Usage
```bash
# Install Envoy Gateway
kubectl apply -f https://github.com/envoyproxy/gateway/releases/latest/download/install.yaml

# Create Gateway
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: example-gateway
spec:
  gatewayClassName: envoy
  listeners:
  - name: http
    port: 80
    protocol: HTTP
EOF

# Create HTTPRoute
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: example-route
spec:
  parentRefs:
  - name: example-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: my-service
      port: 8080
EOF
```

## Platform Support
- ✅ Linux (Kubernetes)
- ✅ macOS (Kubernetes)
- ✅ Cloud (EKS, GKE, AKS)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### Traffic Splitting
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: canary-route
spec:
  parentRefs:
  - name: example-gateway
  rules:
  - backendRefs:
    - name: app-v1
      port: 8080
      weight: 90
    - name: app-v2
      port: 8080
      weight: 10
```

### TLS Termination
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: tls-gateway
spec:
  gatewayClassName: envoy
  listeners:
  - name: https
    port: 443
    protocol: HTTPS
    tls:
      mode: Terminate
      certificateRefs:
      - name: my-cert
```

## Agent Use
- Kubernetes ingress management
- API gateway for microservices
- Traffic routing and load balancing
- TLS termination
- Multi-tenant environments

## Uninstall
```yaml
- preset: envoy-gateway
  with:
    state: absent
```

## Resources
- Official docs: https://gateway.envoyproxy.io/
- GitHub: https://github.com/envoyproxy/gateway
- Search: "envoy gateway tutorial"

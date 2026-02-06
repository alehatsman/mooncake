# Gloo - Kubernetes-Native API Gateway

Gloo Edge is a feature-rich, Kubernetes-native API gateway and ingress controller built on Envoy Proxy, providing advanced traffic management and security for microservices.

## Quick Start

```yaml
- preset: gloo
```

```bash
# Check Gloo status
glooctl check

# Get proxy configuration
glooctl get proxy

# View routes
glooctl get virtualservice
```

## Features

- **Envoy-based**: Built on battle-tested Envoy Proxy
- **Function routing**: Route to individual functions, not just services
- **Protocol support**: HTTP/1.1, HTTP/2, gRPC, WebSockets
- **Transformation**: Request/response modification without code changes
- **Security**: OAuth, JWT validation, CORS, rate limiting
- **Kubernetes-native**: CRD-based configuration

## Basic Usage

```bash
# Check installation
glooctl check

# Get proxy URL
glooctl proxy url

# Create virtual service
glooctl create virtualservice --name myapp \
  --domains myapp.example.com \
  --upstream default-myapp-8080

# View configuration
glooctl get upstream
glooctl get virtualservice
glooctl get proxy

# Debug proxy configuration
glooctl proxy served-config
```

## Advanced Configuration

```yaml
- preset: gloo
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove gloo |

## Platform Support

- ✅ Linux (CLI tool)
- ✅ macOS (CLI tool)
- ❌ Windows (not yet supported)

**Note**: Gloo itself runs on Kubernetes clusters. This preset installs the `glooctl` CLI tool for managing Gloo.

## Configuration

- **CLI tool**: `glooctl` for managing Gloo Edge
- **Kubernetes namespace**: `gloo-system` (default)
- **Admin console**: Port forwarding required

## Real-World Examples

### Basic Route Configuration

```yaml
# Create VirtualService with routing
apiVersion: gateway.solo.io/v1
kind: VirtualService
metadata:
  name: myapp
  namespace: gloo-system
spec:
  virtualHost:
    domains:
      - 'myapp.example.com'
    routes:
      - matchers:
          - prefix: /api
        routeAction:
          single:
            upstream:
              name: default-myapp-8080
              namespace: gloo-system
```

### Request Transformation

```bash
# Add header transformation
glooctl create virtualservice myapp \
  --domains myapp.example.com \
  --upstream default-myapp-8080 \
  --add-header "X-Custom-Header:value"
```

### Rate Limiting

```yaml
# Enable rate limiting (requires CRD configuration)
apiVersion: ratelimit.solo.io/v1alpha1
kind: RateLimitConfig
metadata:
  name: global-limit
  namespace: gloo-system
spec:
  raw:
    descriptors:
      - key: generic_key
        value: per-minute
        rateLimit:
          requestsPerUnit: 100
          unit: MINUTE
```

### CI/CD Deployment Check

```bash
# Verify Gloo is healthy before deployment
if ! glooctl check; then
  echo "Gloo Gateway not ready"
  exit 1
fi

# Deploy application and create route
kubectl apply -f app.yaml
glooctl create vs --name myapp --domains app.example.com \
  --upstream default-myapp-8080
```

## Agent Use

- Automate API gateway configuration in CI/CD
- Create routes during application deployment
- Validate gateway health before releases
- Configure rate limiting and security policies
- Generate API documentation from routes
- Monitor gateway metrics and upstreams

## Troubleshooting

### glooctl check fails

Check Kubernetes connection:
```bash
kubectl get pods -n gloo-system
kubectl logs -n gloo-system deployment/gloo
```

### Routes not working

Verify upstream discovery:
```bash
glooctl get upstream
kubectl get upstreams -n gloo-system
```

### Debug proxy configuration

```bash
# Get Envoy configuration
glooctl proxy served-config

# Check logs
glooctl proxy logs
```

## Uninstall

```yaml
- preset: gloo
  with:
    state: absent
```

## Resources

- Official docs: https://docs.solo.io/gloo-edge/
- GitHub: https://github.com/solo-io/gloo
- Search: "gloo edge tutorial", "gloo kubernetes gateway", "glooctl examples"

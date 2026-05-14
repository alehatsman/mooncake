# emissary - API Gateway and Service Mesh

Kubernetes-native API gateway built on Envoy Proxy.

## Quick Start
```yaml
- preset: emissary
```

## Features
- **API Gateway**: Route traffic to Kubernetes services
- **Service Mesh**: Service-to-service communication
- **Load Balancing**: Automatic traffic distribution
- **Rate Limiting**: Protect APIs from overload
- **Authentication**: Integrate with OAuth, JWT
- **TLS Termination**: SSL/TLS handling
- **Observability**: Metrics and tracing

## Basic Usage
```bash
# Install in Kubernetes
kubectl apply -f https://app.getambassador.io/yaml/emissary/3.9.1/emissary-crds.yaml
kubectl apply -f https://app.getambassador.io/yaml/emissary/3.9.1/emissary-emissaryns.yaml

# Wait for deployment
kubectl wait --timeout=90s --for=condition=available deployment emissary-ingress -n emissary-system

# Create mapping
kubectl apply -f - <<EOF
apiVersion: getambassador.io/v3alpha1
kind: Mapping
metadata:
  name: my-service
spec:
  prefix: /api/
  service: my-service:8080
EOF

# Check status
kubectl get mappings
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

### API Routing
```yaml
apiVersion: getambassador.io/v3alpha1
kind: Mapping
metadata:
  name: api-gateway
spec:
  prefix: /api/v1/
  service: backend-api:8080
  timeout_ms: 5000
```

### Rate Limiting
```yaml
apiVersion: getambassador.io/v3alpha1
kind: RateLimit
metadata:
  name: basic-rate-limit
spec:
  domain: ambassador
  limits:
  - pattern: [{generic_key: "global"}]
    rate: 100
    unit: minute
```

### Authentication
```yaml
apiVersion: getambassador.io/v3alpha1
kind: Filter
metadata:
  name: jwt-filter
spec:
  JWT:
    jwksURI: https://example.com/.well-known/jwks.json
    audience: my-api
```

## Agent Use
- Route API traffic in Kubernetes
- Implement API gateway patterns
- Secure service-to-service communication
- Rate limit API endpoints
- Manage ingress traffic

## Uninstall
```yaml
- preset: emissary
  with:
    state: absent
```

## Resources
- Official docs: https://www.getambassador.io/docs/emissary/
- GitHub: https://github.com/emissary-ingress/emissary
- Search: "emissary ingress tutorial"

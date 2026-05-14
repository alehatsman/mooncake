# kusk - OpenAPI-Driven API Gateway for Kubernetes

Kubernetes-native API gateway that automatically configures routing, rate limiting, authentication, and more from OpenAPI specifications.

## Quick Start
```yaml
- preset: kusk
```

## Features
- **OpenAPI-driven**: Configure gateway directly from OpenAPI specs
- **Automatic routing**: Generate K8s Ingress/Gateway from API definitions
- **Built-in validation**: Request/response validation from schema
- **Rate limiting**: Per-operation or global rate limits
- **Authentication**: OAuth2, JWT, API key support
- **Multiple backends**: Envoy Gateway, Ambassador, NGINX Ingress

## Basic Usage
```bash
# Deploy API from OpenAPI spec
kusk deploy -i openapi.yaml

# Generate Kubernetes manifests
kusk generate -i openapi.yaml

# Validate OpenAPI spec
kusk validate -i openapi.yaml

# List deployed APIs
kusk api list

# Get API details
kusk api get my-api

# Update API configuration
kusk api update my-api -i openapi-v2.yaml
```

## Advanced Configuration
```yaml
- preset: kusk
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kusk |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (Homebrew)
- ✅ Windows (binary download)
- ✅ Kubernetes cluster required

## Configuration
- **Kubeconfig**: Uses `~/.kube/config` or `$KUBECONFIG`
- **Gateway**: Configurable backend (Envoy, Ambassador, NGINX)
- **CRDs**: Installs Kusk Gateway custom resources

## Real-World Examples

### Deploy API with Rate Limiting
```yaml
# openapi.yaml with Kusk extensions
openapi: 3.0.0
info:
  title: My API
  version: 1.0.0
x-kusk:
  cors:
    origins:
      - "*"
  rate_limit:
    requests_per_unit: 100
    unit: minute
paths:
  /users:
    get:
      operationId: getUsers
      x-kusk:
        upstream:
          service:
            name: user-service
            namespace: default
            port: 8080
```

Deploy:
```bash
kusk deploy -i openapi.yaml --namespace production
```

### CI/CD API Deployment
```yaml
# Deploy API on every commit
- name: Validate OpenAPI spec
  shell: kusk validate -i openapi.yaml

- name: Generate manifests
  shell: kusk generate -i openapi.yaml > manifests.yaml

- name: Deploy to cluster
  shell: kubectl apply -f manifests.yaml
```

### Multi-Environment Deployment
```bash
# Development
kusk deploy -i openapi.yaml --namespace dev --envoy-fleet dev-gateway

# Staging
kusk deploy -i openapi.yaml --namespace staging --envoy-fleet staging-gateway

# Production
kusk deploy -i openapi.yaml --namespace prod --envoy-fleet prod-gateway
```

## Agent Use
- Automated API gateway configuration from OpenAPI specs
- API deployment in CI/CD pipelines
- Multi-environment API management
- API versioning and rollout strategies
- Configuration drift prevention via GitOps

## Troubleshooting

### Gateway CRDs not found
Install Kusk Gateway:
```bash
kubectl apply -f https://github.com/kubeshop/kusk-gateway/releases/latest/download/kusk-gateway.yaml
```

### Service not reachable
Verify service exists:
```bash
kubectl get svc -n <namespace>
```

Check Envoy proxy logs:
```bash
kubectl logs -n kusk-system -l app=kusk-gateway-envoy-fleet
```

### Validation errors
Check OpenAPI spec format:
```bash
kusk validate -i openapi.yaml --verbose
```

## Uninstall
```yaml
- preset: kusk
  with:
    state: absent
```

**Note**: Does not remove Kusk Gateway from cluster. Uninstall manually:
```bash
kubectl delete -f https://github.com/kubeshop/kusk-gateway/releases/latest/download/kusk-gateway.yaml
```

## Resources
- Official docs: https://kusk.io/
- GitHub: https://github.com/kubeshop/kusk-gateway
- OpenAPI extensions: https://kusk.io/openapi-extension
- Search: "kusk kubernetes api gateway", "kusk openapi"

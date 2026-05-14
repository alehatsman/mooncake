# contour - Kubernetes Ingress Controller

Contour is a high-performance ingress controller for Kubernetes using Envoy proxy to provide modern HTTP(S) load balancing and routing.

## Quick Start
```yaml
- preset: contour
```

## Features
- **Envoy-based**: Built on Envoy proxy for performance
- **HTTPProxy CRD**: Advanced routing beyond Ingress
- **TLS support**: Automatic certificate management
- **Blue/green deployments**: Traffic splitting and weighted routing
- **Kubernetes-native**: Integrates seamlessly with K8s
- **Dynamic configuration**: Updates without proxy restarts

## Basic Usage
```bash
# Check version
contour version

# Verify deployment
kubectl get pods -n projectcontour

# View HTTPProxy resources
kubectl get httpproxy -A

# Check Envoy configuration
kubectl exec -n projectcontour <envoy-pod> -- curl localhost:19000/config_dump

# View ingress status
kubectl get ingress -A
```

## Advanced Configuration
```yaml
- preset: contour
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove contour |

## Platform Support
- ✅ Linux (binary download for kubectl plugin)
- ✅ macOS (Homebrew, binary download)
- ❌ Windows (not yet supported)

## Configuration
- **Contour config**: Managed via ConfigMap in `projectcontour` namespace
- **Envoy config**: Generated dynamically from K8s resources
- **Install manifest**: `kubectl apply -f contour.yaml`
- **Namespace**: `projectcontour` (default)

## Real-World Examples

### Basic HTTPProxy
```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: basic-app
  namespace: default
spec:
  virtualhost:
    fqdn: app.example.com
  routes:
  - services:
    - name: app-service
      port: 80
```

### TLS with Let's Encrypt
```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: secure-app
spec:
  virtualhost:
    fqdn: app.example.com
    tls:
      secretName: app-tls
  routes:
  - services:
    - name: app-service
      port: 443
```

### Blue/Green Deployment
```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: canary-app
spec:
  virtualhost:
    fqdn: app.example.com
  routes:
  - services:
    - name: app-v1
      port: 80
      weight: 90  # 90% to v1
    - name: app-v2
      port: 80
      weight: 10  # 10% to v2 (canary)
```

### Path-Based Routing
```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: multi-path
spec:
  virtualhost:
    fqdn: api.example.com
  routes:
  - conditions:
    - prefix: /v1
    services:
    - name: api-v1
      port: 8080
  - conditions:
    - prefix: /v2
    services:
    - name: api-v2
      port: 8080
  - conditions:
    - prefix: /admin
    services:
    - name: admin-api
      port: 9000
```

### Rate Limiting
```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: rate-limited
spec:
  virtualhost:
    fqdn: api.example.com
    rateLimitPolicy:
      global:
        descriptors:
        - entries:
          - genericKey:
              value: api-ratelimit
      local:
        requests: 100
        unit: minute
  routes:
  - services:
    - name: api-service
      port: 80
```

### WebSocket Support
```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: websocket-app
spec:
  virtualhost:
    fqdn: ws.example.com
  routes:
  - services:
    - name: websocket-service
      port: 8080
    enableWebsockets: true
```

## Installation on Kubernetes
```bash
# Install Contour
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Verify installation
kubectl get pods -n projectcontour
kubectl get svc -n projectcontour envoy

# Get LoadBalancer IP
kubectl get svc -n projectcontour envoy
```

## Cert-Manager Integration
```yaml
# Install cert-manager first
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: contour

# Use in HTTPProxy
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: app-with-tls
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  virtualhost:
    fqdn: app.example.com
    tls:
      secretName: app-tls
  routes:
  - services:
    - name: app
      port: 80
```

## Monitoring
```bash
# Prometheus metrics
kubectl port-forward -n projectcontour <contour-pod> 8000:8000
curl localhost:8000/metrics

# Envoy stats
kubectl port-forward -n projectcontour <envoy-pod> 9001:9001
curl localhost:9001/stats

# Envoy admin interface
kubectl port-forward -n projectcontour <envoy-pod> 19000:19000
# Visit http://localhost:19000
```

## Agent Use
- Automated ingress configuration in K8s clusters
- Dynamic traffic management for microservices
- Canary deployments and A/B testing
- API gateway implementation
- TLS certificate automation
- Load balancer configuration as code

## Troubleshooting

### HTTPProxy not working
Check status and events:
```bash
# Check HTTPProxy status
kubectl describe httpproxy <name>

# Check Contour logs
kubectl logs -n projectcontour -l app=contour

# Check Envoy logs
kubectl logs -n projectcontour -l app=envoy

# Verify service endpoints
kubectl get endpoints <service-name>
```

### TLS certificate issues
Verify certificate setup:
```bash
# Check secret exists
kubectl get secret <tls-secret-name>

# Describe secret
kubectl describe secret <tls-secret-name>

# Check cert-manager (if used)
kubectl get certificate
kubectl describe certificate <cert-name>
```

### Service not reachable
Check connectivity:
```bash
# Get LoadBalancer IP
kubectl get svc -n projectcontour envoy

# Test with curl
curl -H "Host: app.example.com" http://<EXTERNAL-IP>

# Check pod logs
kubectl logs -l app=<your-app>
```

## Uninstall
```yaml
- preset: contour
  with:
    state: absent
```

To remove from Kubernetes:
```bash
kubectl delete namespace projectcontour
```

## Resources
- Official docs: https://projectcontour.io/docs/
- GitHub: https://github.com/projectcontour/contour
- HTTPProxy API: https://projectcontour.io/docs/main/config/api/
- Search: "contour kubernetes ingress", "envoy ingress controller"

# Istio - Service Mesh Control Plane

Service mesh platform providing traffic management, security, and observability for microservices. Transparently adds load balancing, mutual TLS, circuit breaking, and distributed tracing without changing application code. Built on Envoy proxy.

## Quick Start
```yaml
- preset: istio
```

Install Istio CLI (`istioctl`). Deploy to Kubernetes cluster separately.

Default profile: `default` (production-ready)
Dashboard: Kiali, Grafana, Jaeger (optional add-ons)

## Features
- **Traffic management**: Intelligent routing, load balancing, traffic splitting
- **Security**: Mutual TLS (mTLS) encryption, authentication, authorization policies
- **Observability**: Metrics, distributed tracing, access logs
- **Policy enforcement**: Rate limiting, quotas, access control
- **Multi-cluster support**: Service discovery across Kubernetes clusters
- **Fault injection**: Test resilience with delays and errors
- **Circuit breaking**: Automatic failure detection and recovery
- **Canary deployments**: Gradual rollouts with traffic shifting
- **Ingress/Egress gateways**: Secure cluster entry/exit points
- **Service discovery**: Automatic endpoint detection

## Basic Usage
```bash
# Install Istio into Kubernetes cluster
istioctl install --set profile=demo

# Verify installation
istioctl verify-install

# Check version
istioctl version

# Analyze configuration
istioctl analyze

# Enable sidecar injection for namespace
kubectl label namespace default istio-injection=enabled

# Deploy sample application
kubectl apply -f samples/bookinfo/platform/kube/bookinfo.yaml

# Check proxy status
istioctl proxy-status

# Get proxy configuration
istioctl proxy-config routes <pod-name>
```

## Architecture

### Components
```
┌─────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                    │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │              Istio Control Plane                   │ │
│  │  ┌─────────┐  ┌────────┐  ┌──────────┐           │ │
│  │  │  Pilot  │  │ Citadel│  │  Galley  │           │ │
│  │  │(istiod) │  │  (CA)  │  │ (Config) │           │ │
│  │  └────┬────┘  └────┬───┘  └────┬─────┘           │ │
│  │       │            │           │                  │ │
│  └───────┼────────────┼───────────┼──────────────────┘ │
│          │            │           │                    │
│  ┌───────▼────────────▼───────────▼──────────────────┐ │
│  │              Data Plane (Envoy Proxies)           │ │
│  │                                                    │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │ │
│  │  │  Pod A   │  │  Pod B   │  │  Pod C   │        │ │
│  │  │┌────────┐│  │┌────────┐│  │┌────────┐│        │ │
│  │  ││  App   ││  ││  App   ││  ││  App   ││        │ │
│  │  │└────────┘│  │└────────┘│  │└────────┘│        │ │
│  │  │┌────────┐│  │┌────────┐│  │┌────────┐│        │ │
│  │  ││ Envoy  ││  ││ Envoy  ││  ││ Envoy  ││        │ │
│  │  │└────────┘│  │└────────┘│  │└────────┘│        │ │
│  │  └──────────┘  └──────────┘  └──────────┘        │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Key Concepts
- **Envoy proxy**: High-performance proxy deployed as sidecar
- **Pilot (istiod)**: Service discovery, traffic management
- **Citadel**: Certificate authority for mTLS
- **Galley**: Configuration validation and distribution
- **Virtual Service**: Traffic routing rules
- **Destination Rule**: Traffic policies (load balancing, connection pool)
- **Gateway**: Ingress/egress configuration
- **Service Entry**: External service registration

## Advanced Configuration

### Installation profiles
```yaml
- name: Install Istio (demo profile)
  shell: istioctl install --set profile=demo -y

# Available profiles:
# - default: Production with recommended features
# - demo: All features enabled, low resource
# - minimal: Basic control plane only
# - remote: Multi-cluster remote cluster
# - empty: Base for custom configuration
```

### Production installation
```yaml
- name: Install Istio CLI
  preset: istio

- name: Install Istio control plane
  shell: |
    istioctl install \
      --set profile=default \
      --set values.pilot.resources.requests.cpu=500m \
      --set values.pilot.resources.requests.memory=2048Mi \
      --set values.global.proxy.resources.requests.cpu=100m \
      --set values.global.proxy.resources.requests.memory=128Mi
  register: istio_install

- name: Enable sidecar injection
  shell: kubectl label namespace {{ item }} istio-injection=enabled
  loop:
    - production
    - staging
    - default

- name: Install observability add-ons
  shell: |
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/prometheus.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/grafana.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/jaeger.yaml
```

### Custom installation with IstioOperator
```yaml
- name: Create IstioOperator configuration
  template:
    src: istio-operator.yml.j2
    dest: /tmp/istio-operator.yml
  vars:
    istio_namespace: istio-system
    enable_tracing: true
    enable_access_logs: true

- name: Apply IstioOperator
  shell: istioctl install -f /tmp/istio-operator.yml -y
```

**IstioOperator example (istio-operator.yml.j2)**:
```yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  namespace: istio-system
  name: istio-production
spec:
  profile: default
  meshConfig:
    accessLogFile: /dev/stdout
    enableTracing: true
    defaultConfig:
      tracing:
        sampling: 100
  components:
    pilot:
      k8s:
        resources:
          requests:
            cpu: 500m
            memory: 2Gi
    ingressGateways:
    - name: istio-ingressgateway
      enabled: true
      k8s:
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
        service:
          type: LoadBalancer
  values:
    global:
      proxy:
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

## Traffic Management

### Virtual Service (routing rules)
```yaml
- name: Create virtual service
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: reviews-route
    spec:
      hosts:
      - reviews
      http:
      - match:
        - headers:
            end-user:
              exact: jason
        route:
        - destination:
            host: reviews
            subset: v2
      - route:
        - destination:
            host: reviews
            subset: v1
    EOF
```

### Destination Rule (traffic policies)
```yaml
- name: Create destination rule
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: DestinationRule
    metadata:
      name: reviews-destination
    spec:
      host: reviews
      trafficPolicy:
        loadBalancer:
          simple: RANDOM
      subsets:
      - name: v1
        labels:
          version: v1
      - name: v2
        labels:
          version: v2
        trafficPolicy:
          connectionPool:
            tcp:
              maxConnections: 100
    EOF
```

### Ingress Gateway
```yaml
- name: Create gateway
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: Gateway
    metadata:
      name: bookinfo-gateway
    spec:
      selector:
        istio: ingressgateway
      servers:
      - port:
          number: 80
          name: http
          protocol: HTTP
        hosts:
        - "bookinfo.example.com"
    ---
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: bookinfo
    spec:
      hosts:
      - "bookinfo.example.com"
      gateways:
      - bookinfo-gateway
      http:
      - match:
        - uri:
            prefix: /productpage
        route:
        - destination:
            host: productpage
            port:
              number: 9080
    EOF
```

### Traffic splitting (canary deployment)
```yaml
- name: Canary deployment with 10% traffic
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: reviews
    spec:
      hosts:
      - reviews
      http:
      - route:
        - destination:
            host: reviews
            subset: v1
          weight: 90
        - destination:
            host: reviews
            subset: v2
          weight: 10
    EOF
```

### Circuit breaking
```yaml
- name: Configure circuit breaker
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: DestinationRule
    metadata:
      name: httpbin
    spec:
      host: httpbin
      trafficPolicy:
        connectionPool:
          tcp:
            maxConnections: 1
          http:
            http1MaxPendingRequests: 1
            maxRequestsPerConnection: 1
        outlierDetection:
          consecutive5xxErrors: 1
          interval: 1s
          baseEjectionTime: 3m
          maxEjectionPercent: 100
    EOF
```

### Retries and timeouts
```yaml
- name: Configure retries
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: ratings
    spec:
      hosts:
      - ratings
      http:
      - route:
        - destination:
            host: ratings
        timeout: 10s
        retries:
          attempts: 3
          perTryTimeout: 2s
          retryOn: 5xx
    EOF
```

## Security

### Enable mutual TLS (mTLS)
```yaml
- name: Enable strict mTLS globally
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: security.istio.io/v1beta1
    kind: PeerAuthentication
    metadata:
      name: default
      namespace: istio-system
    spec:
      mtls:
        mode: STRICT
    EOF

- name: Enable permissive mTLS for specific namespace
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: security.istio.io/v1beta1
    kind: PeerAuthentication
    metadata:
      name: default
      namespace: production
    spec:
      mtls:
        mode: PERMISSIVE
    EOF
```

### Authorization policies
```yaml
- name: Deny all traffic by default
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: security.istio.io/v1beta1
    kind: AuthorizationPolicy
    metadata:
      name: deny-all
      namespace: production
    spec:
      {}
    EOF

- name: Allow specific service access
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: security.istio.io/v1beta1
    kind: AuthorizationPolicy
    metadata:
      name: allow-frontend
      namespace: production
    spec:
      selector:
        matchLabels:
          app: backend
      action: ALLOW
      rules:
      - from:
        - source:
            principals: ["cluster.local/ns/production/sa/frontend"]
        to:
        - operation:
            methods: ["GET", "POST"]
    EOF
```

### JWT authentication
```yaml
- name: Configure JWT authentication
  shell: |
    cat <<EOF | kubectl apply -f -
    apiVersion: security.istio.io/v1beta1
    kind: RequestAuthentication
    metadata:
      name: jwt-auth
      namespace: production
    spec:
      selector:
        matchLabels:
          app: api
      jwtRules:
      - issuer: "https://auth.example.com"
        jwksUri: "https://auth.example.com/.well-known/jwks.json"
    ---
    apiVersion: security.istio.io/v1beta1
    kind: AuthorizationPolicy
    metadata:
      name: require-jwt
      namespace: production
    spec:
      selector:
        matchLabels:
          app: api
      action: ALLOW
      rules:
      - from:
        - source:
            requestPrincipals: ["*"]
    EOF
```

## Observability

### Enable access logs
```yaml
- name: Enable access logs for all proxies
  shell: |
    istioctl install \
      --set meshConfig.accessLogFile=/dev/stdout \
      --set meshConfig.accessLogEncoding=JSON
```

### Metrics collection
```yaml
- name: Install Prometheus
  shell: kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/prometheus.yaml

- name: Query metrics
  shell: |
    kubectl -n istio-system port-forward svc/prometheus 9090:9090 &
    curl http://localhost:9090/api/v1/query?query=istio_requests_total

# Key metrics:
# - istio_requests_total - Total requests
# - istio_request_duration_milliseconds - Request latency
# - istio_tcp_connections_opened_total - TCP connections
# - pilot_xds_pushes - Configuration pushes
```

### Distributed tracing
```yaml
- name: Install Jaeger
  shell: kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/jaeger.yaml

- name: Access Jaeger UI
  shell: istioctl dashboard jaeger
```

### Kiali dashboard
```yaml
- name: Install Kiali
  shell: kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml

- name: Access Kiali
  shell: istioctl dashboard kiali
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Istio CLI |

## Platform Support
- ✅ Linux (all distributions) - via binary download
- ✅ macOS (Homebrew)
- ✅ Docker/Kubernetes (required for service mesh)
- ❌ Windows (use WSL2)

## Configuration

### Mesh configuration
```bash
# Get mesh config
kubectl -n istio-system get configmap istio -o yaml

# Update mesh config
istioctl install --set meshConfig.enableTracing=true
```

### Sidecar injection
```bash
# Enable automatic injection for namespace
kubectl label namespace default istio-injection=enabled

# Disable injection for specific pod
kubectl annotate pod mypod sidecar.istio.io/inject="false"

# Manual injection
istioctl kube-inject -f deployment.yaml | kubectl apply -f -
```

### Resource requests/limits
```yaml
# Set global proxy resources
istioctl install \
  --set values.global.proxy.resources.requests.cpu=100m \
  --set values.global.proxy.resources.requests.memory=128Mi \
  --set values.global.proxy.resources.limits.cpu=2000m \
  --set values.global.proxy.resources.limits.memory=1024Mi
```

## Use Cases

### Microservices traffic management
```yaml
- name: Install Istio
  preset: istio

- name: Deploy Istio to cluster
  shell: istioctl install --set profile=default -y

- name: Enable sidecar injection
  shell: kubectl label namespace production istio-injection=enabled

- name: Deploy microservices
  shell: kubectl apply -f manifests/

- name: Configure traffic routing
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: user-service
    spec:
      hosts:
      - user-service
      http:
      - match:
        - headers:
            version:
              exact: "beta"
        route:
        - destination:
            host: user-service
            subset: v2
      - route:
        - destination:
            host: user-service
            subset: v1
    EOF
```

### Zero-trust security
```yaml
- name: Enable strict mTLS
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: security.istio.io/v1beta1
    kind: PeerAuthentication
    metadata:
      name: default
      namespace: istio-system
    spec:
      mtls:
        mode: STRICT
    EOF

- name: Default deny authorization
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: security.istio.io/v1beta1
    kind: AuthorizationPolicy
    metadata:
      name: deny-all
      namespace: production
    spec:
      {}
    EOF

- name: Allow specific service communication
  shell: kubectl apply -f authorization-policies/
```

### Canary deployment with monitoring
```yaml
- name: Deploy canary version
  shell: kubectl apply -f deployment-v2.yaml

- name: Configure 10% traffic to canary
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: myapp
    spec:
      hosts:
      - myapp
      http:
      - route:
        - destination:
            host: myapp
            subset: stable
          weight: 90
        - destination:
            host: myapp
            subset: canary
          weight: 10
    EOF

- name: Monitor canary metrics
  shell: |
    kubectl -n istio-system port-forward svc/prometheus 9090:9090 &
    # Check error rate for canary
    curl "http://localhost:9090/api/v1/query?query=sum(rate(istio_requests_total{destination_version=\"canary\",response_code=~\"5..\"}[5m]))"

- name: Increase canary traffic if healthy
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: myapp
    spec:
      hosts:
      - myapp
      http:
      - route:
        - destination:
            host: myapp
            subset: stable
          weight: 50
        - destination:
            host: myapp
            subset: canary
          weight: 50
    EOF
```

## CLI Commands

### Installation and management
```bash
# Install with specific profile
istioctl install --set profile=demo

# Install from manifest
istioctl install -f istio-operator.yaml

# Upgrade Istio
istioctl upgrade

# Uninstall Istio
istioctl uninstall --purge

# Verify installation
istioctl verify-install

# Check version
istioctl version
```

### Configuration analysis
```bash
# Analyze configuration for issues
istioctl analyze

# Analyze specific namespace
istioctl analyze -n production

# Analyze YAML file
istioctl analyze deployment.yaml

# Validate configuration
istioctl validate -f virtual-service.yaml
```

### Proxy management
```bash
# Check proxy status
istioctl proxy-status

# Get proxy configuration
istioctl proxy-config cluster <pod-name>
istioctl proxy-config listener <pod-name>
istioctl proxy-config route <pod-name>
istioctl proxy-config endpoint <pod-name>

# Get proxy logs
istioctl proxy-config log <pod-name>

# Enable debug logging
istioctl proxy-config log <pod-name> --level debug
```

### Debugging
```bash
# Open dashboards
istioctl dashboard kiali
istioctl dashboard grafana
istioctl dashboard jaeger
istioctl dashboard prometheus

# Describe pod configuration
istioctl experimental describe pod <pod-name>

# Check mesh connectivity
istioctl experimental check-inject

# Get metrics
istioctl experimental metrics <pod-name>
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Istio CLI
  preset: istio
```

### Development cluster setup
```yaml
- name: Install Istio CLI
  preset: istio

- name: Install Istio with demo profile
  shell: istioctl install --set profile=demo -y
  register: istio_install

- name: Wait for Istio to be ready
  shell: kubectl -n istio-system rollout status deployment/istiod

- name: Install observability add-ons
  shell: |
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/prometheus.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/grafana.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml

- name: Enable injection for default namespace
  shell: kubectl label namespace default istio-injection=enabled
```

### Production deployment
```yaml
- name: Install Istio CLI
  preset: istio

- name: Create Istio configuration
  template:
    src: istio-operator.yml.j2
    dest: /tmp/istio-operator.yml

- name: Install Istio
  shell: istioctl install -f /tmp/istio-operator.yml -y

- name: Verify installation
  shell: istioctl verify-install
  register: istio_verify

- name: Enable mTLS
  shell: |
    kubectl apply -f - <<EOF
    apiVersion: security.istio.io/v1beta1
    kind: PeerAuthentication
    metadata:
      name: default
      namespace: istio-system
    spec:
      mtls:
        mode: STRICT
    EOF
```

## Agent Use
- **Service mesh deployment**: Install and configure Istio on Kubernetes clusters
- **Traffic management**: Implement canary deployments, A/B testing, traffic splitting
- **Zero-trust security**: Enable mTLS, implement authorization policies
- **Observability**: Set up distributed tracing, metrics, and logs
- **API gateway**: Configure ingress/egress gateways for cluster traffic
- **Fault injection**: Test application resilience with delays and errors
- **Multi-cluster**: Connect services across multiple Kubernetes clusters

## Troubleshooting

### Sidecar not injected
```bash
# Check namespace label
kubectl get namespace -L istio-injection

# Enable injection
kubectl label namespace default istio-injection=enabled

# Check pod annotations
kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations}'

# Manual injection
istioctl kube-inject -f deployment.yaml | kubectl apply -f -
```

### Configuration not applied
```bash
# Analyze for issues
istioctl analyze

# Check pilot logs
kubectl -n istio-system logs -l app=istiod

# Verify proxy received config
istioctl proxy-status

# Force config sync
istioctl proxy-config cluster <pod-name> --fqdn <service>
```

### Connection failures with mTLS
```bash
# Check peer authentication
kubectl get peerauthentication -A

# Check destination rules
kubectl get destinationrule -A

# Verify certificates
istioctl proxy-config secret <pod-name>

# Check if mTLS is working
istioctl authn tls-check <pod-name> <service>
```

### High latency or timeouts
```bash
# Check virtual service timeouts
kubectl get virtualservice -A -o yaml | grep timeout

# Check destination rule connection pools
kubectl get destinationrule -A -o yaml | grep -A 5 connectionPool

# View proxy stats
istioctl dashboard envoy <pod-name>

# Check circuit breaker status
istioctl proxy-config cluster <pod-name> --fqdn <service> -o json | grep outlier
```

### Memory/CPU issues
```bash
# Check proxy resource usage
kubectl top pod -l security.istio.io/tlsMode=istio

# Reduce resource requests
istioctl install --set values.global.proxy.resources.requests.cpu=50m

# Disable unused features
istioctl install --set meshConfig.accessLogFile="" --set values.telemetry.enabled=false
```

## Best Practices

1. **Start with demo profile for testing**, use default or custom for production
2. **Enable sidecar injection per-namespace**, not globally
3. **Use strict mTLS in production** for zero-trust security
4. **Implement authorization policies** with default-deny approach
5. **Monitor resource usage** of Envoy sidecars and tune accordingly
6. **Use circuit breakers** to prevent cascade failures
7. **Configure retries and timeouts** for all external dependencies
8. **Enable distributed tracing** for end-to-end visibility
9. **Use Kiali** for topology visualization and debugging
10. **Test configuration changes** with `istioctl analyze` before applying

## Monitoring

### Key metrics to track
```promql
# Request rate
sum(rate(istio_requests_total[5m])) by (destination_service)

# Error rate
sum(rate(istio_requests_total{response_code=~"5.."}[5m])) by (destination_service)

# Latency (p95)
histogram_quantile(0.95, sum(rate(istio_request_duration_milliseconds_bucket[5m])) by (destination_service, le))

# Proxy memory usage
container_memory_usage_bytes{container="istio-proxy"}
```

### Alerts
- High error rate (>1% 5xx responses)
- High latency (p95 >1s)
- Circuit breaker triggered
- Certificate expiration (<7 days)
- Control plane unavailable

## Uninstall
```yaml
- name: Uninstall Istio
  shell: istioctl uninstall --purge -y

- name: Delete namespace
  shell: kubectl delete namespace istio-system

- name: Remove Istio CLI
  preset: istio
  with:
    state: absent
```

**Note**: Removes Istio control plane and CRDs. Application pods need manual restart to remove sidecars.

## Resources
- Official: https://istio.io/
- Documentation: https://istio.io/latest/docs/
- GitHub: https://github.com/istio/istio
- Community: https://slack.istio.io/
- Examples: https://github.com/istio/istio/tree/master/samples
- Blog: https://istio.io/latest/blog/
- Training: https://academy.tetrate.io/
- Search: "istio traffic management", "istio security", "istio troubleshooting"

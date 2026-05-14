# envoy - Cloud-Native Proxy

High-performance service proxy designed for microservices and cloud-native applications.

## Quick Start
```yaml
- preset: envoy
```

## Features
- **Layer 7 proxy**: HTTP/1.1, HTTP/2, gRPC
- **Load balancing**: Advanced algorithms
- **Service discovery**: Dynamic endpoint discovery
- **Health checking**: Active and passive health checks
- **Observability**: Metrics, logging, tracing
- **TLS termination**: Automatic certificate management

## Basic Usage
```yaml
# envoy.yaml
static_resources:
  listeners:
  - address:
      socket_address:
        address: 0.0.0.0
        port_value: 8080
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: backend
              domains: ["*"]
              routes:
              - match: {prefix: "/"}
                route: {cluster: backend_cluster}
          http_filters:
          - name: envoy.filters.http.router

  clusters:
  - name: backend_cluster
    connect_timeout: 0.25s
    type: STRICT_DNS
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: backend_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: backend
                port_value: 8000

admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9901
```

```bash
# Start Envoy
envoy -c envoy.yaml

# Check admin interface
curl http://localhost:9901/stats
curl http://localhost:9901/clusters
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Cloud (Kubernetes, Docker)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### API Gateway
```yaml
# Rate limiting + authentication
route_config:
  virtual_hosts:
  - name: api
    domains: ["api.example.com"]
    routes:
    - match: {prefix: "/api/"}
      route:
        cluster: api_cluster
        rate_limits:
        - actions:
          - generic_key:
              descriptor_value: "api_limit"
```

### Service Mesh Sidecar
```yaml
# Outbound proxy for microservice
clusters:
- name: user_service
  type: STRICT_DNS
  connect_timeout: 1s
  lb_policy: ROUND_ROBIN
  load_assignment:
    cluster_name: user_service
    endpoints:
    - lb_endpoints:
      - endpoint:
          address:
            socket_address:
              address: user-service.default.svc.cluster.local
              port_value: 8080
```

## Agent Use
- API gateway for microservices
- Service mesh data plane
- Load balancer for cloud applications
- TLS termination proxy
- Observability integration

## Uninstall
```yaml
- preset: envoy
  with:
    state: absent
```

## Resources
- Official docs: https://www.envoyproxy.io/docs/
- GitHub: https://github.com/envoyproxy/envoy
- Search: "envoy proxy tutorial"

# Kong Gateway - API Gateway and Service Mesh

Cloud-native API gateway built on NGINX. Manage APIs, microservices, and service mesh with authentication, rate limiting, transformations, and 200+ plugins. Used by Fortune 500 companies to handle trillions of requests.

## Quick Start
```yaml
- preset: kong
```

Access Admin API: `http://localhost:8001`
Proxy requests: `http://localhost:8000`

## Features
- **API management**: Centralized routing, load balancing, and discovery
- **200+ plugins**: Authentication, rate limiting, caching, logging, transformations
- **Service mesh**: Connect and secure microservices with mTLS
- **Multi-protocol**: HTTP, HTTPS, gRPC, WebSocket, TCP, TLS
- **High performance**: Built on NGINX, handles 100k+ req/sec per node
- **Declarative config**: Manage via YAML files or Admin API
- **Multi-tenancy**: Workspaces and RBAC (Enterprise)
- **Hybrid deployment**: Control plane + data plane architecture
- **Extensible**: Write custom plugins in Lua, Go, Python, JavaScript

## Architecture

```
┌──────────────────────────────────────────────┐
│              Kong Gateway                     │
│                                               │
│  Client → Proxy (8000) → Plugins → Upstream │
│             ↓                                 │
│         Admin API (8001)                      │
└──────────────────────────────────────────────┘

Request Flow:
  Client
    ↓
  Kong Proxy :8000
    ↓
  Route Matching
    ↓
  Plugin Chain (Auth → Rate Limit → Transform)
    ↓
  Load Balancing
    ↓
  Upstream Service
```

### Key Concepts
- **Service**: Upstream API/microservice (e.g., users-api)
- **Route**: Path to reach service (e.g., /users → users-api)
- **Upstream**: Load balancer with multiple targets
- **Target**: Individual backend server (IP:port)
- **Plugin**: Functionality added to routes/services (auth, rate limit, etc.)
- **Consumer**: API client with credentials

## Basic Usage

### Start Kong
```bash
# Initialize database (PostgreSQL)
kong migrations bootstrap

# Start Kong
kong start

# Check status
kong health

# Reload configuration (zero downtime)
kong reload

# Stop Kong
kong stop
```

### Admin API
```bash
# Check Kong status
curl http://localhost:8001/status

# List services
curl http://localhost:8001/services

# List routes
curl http://localhost:8001/routes

# List plugins
curl http://localhost:8001/plugins
```

## Services and Routes

### Create service
```bash
# Add service
curl -i -X POST http://localhost:8001/services \
  --data name=my-service \
  --data url='http://api.example.com'

# Or with separate fields
curl -i -X POST http://localhost:8001/services \
  --data name=my-service \
  --data protocol=http \
  --data host=api.example.com \
  --data port=80 \
  --data path=/v1
```

### Create route
```bash
# Add route to service
curl -i -X POST http://localhost:8001/services/my-service/routes \
  --data 'paths[]=/api' \
  --data name=my-route

# Route with methods
curl -i -X POST http://localhost:8001/services/my-service/routes \
  --data 'paths[]=/users' \
  --data 'methods[]=GET' \
  --data 'methods[]=POST'

# Route with host
curl -i -X POST http://localhost:8001/services/my-service/routes \
  --data 'hosts[]=api.example.com' \
  --data 'paths[]=/api'
```

### Test route
```bash
# Proxy request through Kong
curl http://localhost:8000/api

# With host header
curl -H 'Host: api.example.com' http://localhost:8000/api
```

## Plugins

### Authentication

#### API Key
```bash
# Enable key-auth plugin
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=key-auth \
  --data config.key_names[]=apikey

# Create consumer
curl -X POST http://localhost:8001/consumers \
  --data username=john

# Add API key to consumer
curl -X POST http://localhost:8001/consumers/john/key-auth \
  --data key=secret-api-key

# Test
curl http://localhost:8000/api -H 'apikey: secret-api-key'
```

#### JWT
```bash
# Enable JWT plugin
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=jwt

# Create JWT credential for consumer
curl -X POST http://localhost:8001/consumers/john/jwt \
  --data algorithm=HS256 \
  --data secret=my-secret-key

# Test with JWT token
curl http://localhost:8000/api \
  -H 'Authorization: Bearer <jwt-token>'
```

#### OAuth 2.0
```bash
# Enable OAuth plugin
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=oauth2 \
  --data config.scopes[]=email \
  --data config.scopes[]=profile \
  --data config.mandatory_scope=true
```

### Rate Limiting
```bash
# Rate limit: 100 requests per minute
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=rate-limiting \
  --data config.minute=100 \
  --data config.policy=local

# Advanced rate limiting
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=rate-limiting \
  --data config.second=10 \
  --data config.minute=100 \
  --data config.hour=1000 \
  --data config.policy=redis \
  --data config.redis_host=redis.local
```

### Request/Response Transformation
```bash
# Add request header
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=request-transformer \
  --data config.add.headers[]=X-Custom-Header:value

# Remove response header
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=response-transformer \
  --data config.remove.headers[]=X-Internal-Header
```

### CORS
```bash
# Enable CORS
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=cors \
  --data config.origins=* \
  --data config.methods[]=GET \
  --data config.methods[]=POST \
  --data config.methods[]=PUT \
  --data config.methods[]=DELETE \
  --data config.headers[]=Authorization \
  --data config.exposed_headers[]=X-Auth-Token \
  --data config.credentials=true \
  --data config.max_age=3600
```

### Logging
```bash
# HTTP log
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=http-log \
  --data config.http_endpoint=http://logs.example.com/log

# File log
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=file-log \
  --data config.path=/var/log/kong/access.log
```

## Load Balancing

### Create upstream
```bash
# Create upstream with ring-balancer
curl -X POST http://localhost:8001/upstreams \
  --data name=my-upstream \
  --data algorithm=round-robin

# Add targets (backend servers)
curl -X POST http://localhost:8001/upstreams/my-upstream/targets \
  --data target=10.0.1.10:8080 \
  --data weight=100

curl -X POST http://localhost:8001/upstreams/my-upstream/targets \
  --data target=10.0.1.11:8080 \
  --data weight=100

# Point service to upstream
curl -X PATCH http://localhost:8001/services/my-service \
  --data host=my-upstream
```

### Health checks
```bash
# Add health checks to upstream
curl -X PATCH http://localhost:8001/upstreams/my-upstream \
  --data healthchecks.active.healthy.interval=10 \
  --data healthchecks.active.unhealthy.interval=10 \
  --data healthchecks.active.http_path=/health
```

## Advanced Configuration

### Kong with PostgreSQL
```yaml
- name: Install PostgreSQL
  preset: postgres
  with:
    databases:
      - kong
    users:
      - name: kong
        password: "{{ kong_db_password }}"

- name: Install Kong
  preset: kong

- name: Configure Kong database
  template:
    src: kong.conf.j2
    dest: /etc/kong/kong.conf
    mode: '0644'
  become: true

- name: Initialize Kong database
  shell: kong migrations bootstrap
  become: true

- name: Start Kong
  shell: kong start
  become: true
```

### Declarative configuration
```yaml
# kong.yml
_format_version: "3.0"

services:
  - name: users-api
    url: http://users-service:8080
    routes:
      - name: users-route
        paths:
          - /users
        methods:
          - GET
          - POST
    plugins:
      - name: key-auth
      - name: rate-limiting
        config:
          minute: 100

  - name: orders-api
    url: http://orders-service:8080
    routes:
      - name: orders-route
        paths:
          - /orders

consumers:
  - username: mobile-app
    keyauth_credentials:
      - key: mobile-app-key

plugins:
  - name: cors
    config:
      origins:
        - "*"
```

### Load declarative config
```bash
# Start Kong in DB-less mode
kong start -c kong.conf --declarative-config kong.yml

# Reload config without restart
curl -i -X POST http://localhost:8001/config \
  --form config=@kong.yml
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Kong |

## Platform Support
- ✅ Linux (apt, yum, dnf) - Ubuntu, Debian, RHEL, CentOS
- ✅ Docker (official kong images)
- ✅ Kubernetes (Kong Ingress Controller)
- ❌ macOS (use Docker)
- ❌ Windows (use Docker)

## Configuration

### kong.conf
```bash
# /etc/kong/kong.conf

# Database (PostgreSQL)
database = postgres
pg_host = 127.0.0.1
pg_port = 5432
pg_database = kong
pg_user = kong
pg_password = secret

# DB-less mode (declarative)
database = off
declarative_config = /etc/kong/kong.yml

# Proxy
proxy_listen = 0.0.0.0:8000, 0.0.0.0:8443 ssl
admin_listen = 0.0.0.0:8001

# Logging
log_level = notice
proxy_access_log = /var/log/kong/access.log
proxy_error_log = /var/log/kong/error.log
admin_access_log = /var/log/kong/admin_access.log
admin_error_log = /var/log/kong/admin_error.log

# Performance
nginx_worker_processes = auto
nginx_daemon = on
```

## Use Cases

### API Gateway for Microservices
```yaml
- name: Install Kong
  preset: kong

- name: Create services
  shell: |
    # Users service
    curl -X POST http://localhost:8001/services \
      --data name=users \
      --data url=http://users-service:8080

    curl -X POST http://localhost:8001/services/users/routes \
      --data 'paths[]=/api/users'

    # Orders service
    curl -X POST http://localhost:8001/services \
      --data name=orders \
      --data url=http://orders-service:8080

    curl -X POST http://localhost:8001/services/orders/routes \
      --data 'paths[]=/api/orders'

    # Products service
    curl -X POST http://localhost:8001/services \
      --data name=products \
      --data url=http://products-service:8080

    curl -X POST http://localhost:8001/services/products/routes \
      --data 'paths[]=/api/products'
```

### Multi-Environment Setup
```yaml
- name: Configure Kong for staging
  hosts: staging
  tasks:
    - preset: kong

    - name: Deploy staging routes
      shell: |
        curl -X POST http://localhost:8001/services \
          --data name=api \
          --data url=http://staging-api.internal

    - name: Enable rate limiting (higher limits)
      shell: |
        curl -X POST http://localhost:8001/plugins \
          --data name=rate-limiting \
          --data config.minute=1000

- name: Configure Kong for production
  hosts: production
  tasks:
    - preset: kong

    - name: Deploy production routes
      shell: |
        curl -X POST http://localhost:8001/services \
          --data name=api \
          --data url=http://prod-api.internal

    - name: Enable rate limiting (stricter)
      shell: |
        curl -X POST http://localhost:8001/plugins \
          --data name=rate-limiting \
          --data config.minute=100
```

### API Security Stack
```yaml
- name: Secure API with Kong
  shell: |
    # Create service
    curl -X POST http://localhost:8001/services \
      --data name=secure-api \
      --data url=http://api.internal

    # Create route
    curl -X POST http://localhost:8001/services/secure-api/routes \
      --data 'paths[]=/api'

    # Enable JWT authentication
    curl -X POST http://localhost:8001/services/secure-api/plugins \
      --data name=jwt

    # Enable rate limiting per consumer
    curl -X POST http://localhost:8001/services/secure-api/plugins \
      --data name=rate-limiting \
      --data config.minute=60 \
      --data config.policy=redis

    # Enable request size limiting
    curl -X POST http://localhost:8001/services/secure-api/plugins \
      --data name=request-size-limiting \
      --data config.allowed_payload_size=10

    # Enable bot detection
    curl -X POST http://localhost:8001/services/secure-api/plugins \
      --data name=bot-detection

    # Enable IP restriction
    curl -X POST http://localhost:8001/services/secure-api/plugins \
      --data name=ip-restriction \
      --data config.allow[]=10.0.0.0/8
```

## Kubernetes Integration

### Kong Ingress Controller
```yaml
# Install via Helm
- name: Install Kong Ingress Controller
  shell: |
    helm repo add kong https://charts.konghq.com
    helm repo update
    helm install kong kong/kong \
      --set ingressController.enabled=true \
      --set proxy.type=LoadBalancer

# Use via Ingress resource
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-api
  annotations:
    konghq.com/strip-path: "true"
    konghq.com/plugins: rate-limiting,key-auth
spec:
  ingressClassName: kong
  rules:
    - host: api.example.com
      http:
        paths:
          - path: /users
            pathType: Prefix
            backend:
              service:
                name: users-service
                port:
                  number: 8080
```

## Monitoring

### Prometheus plugin
```bash
# Enable Prometheus plugin
curl -X POST http://localhost:8001/plugins \
  --data name=prometheus

# Scrape metrics
curl http://localhost:8001/metrics
```

### Key metrics
- `kong_http_requests_total` - Total HTTP requests
- `kong_latency_ms` - Request latency
- `kong_upstream_target_health` - Backend health status
- `kong_bandwidth_bytes` - Bandwidth usage

## Admin API

### Service management
```bash
# List services
curl http://localhost:8001/services

# Get service
curl http://localhost:8001/services/my-service

# Update service
curl -X PATCH http://localhost:8001/services/my-service \
  --data url=http://new-backend:8080

# Delete service
curl -X DELETE http://localhost:8001/services/my-service
```

### Plugin management
```bash
# List plugins
curl http://localhost:8001/plugins

# Enable plugin globally
curl -X POST http://localhost:8001/plugins \
  --data name=cors

# Enable plugin on service
curl -X POST http://localhost:8001/services/my-service/plugins \
  --data name=rate-limiting

# Update plugin
curl -X PATCH http://localhost:8001/plugins/{plugin-id} \
  --data config.minute=200

# Delete plugin
curl -X DELETE http://localhost:8001/plugins/{plugin-id}
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Kong
  preset: kong
```

### Production setup with PostgreSQL
```yaml
- name: Setup Kong with database
  hosts: api-gateway
  tasks:
    - preset: postgres
      with:
        databases:
          - kong
        users:
          - name: kong
            password: "{{ kong_password }}"

    - preset: kong

    - name: Configure Kong
      template:
        src: kong.conf.j2
        dest: /etc/kong/kong.conf
      become: true

    - name: Initialize database
      shell: kong migrations bootstrap
      become: true

    - name: Start Kong
      service:
        name: kong
        state: started
      become: true
```

### DB-less deployment
```yaml
- name: Deploy Kong in DB-less mode
  hosts: api-gateway
  tasks:
    - preset: kong

    - name: Create declarative config
      copy:
        src: kong.yml
        dest: /etc/kong/kong.yml
      become: true

    - name: Configure DB-less mode
      lineinfile:
        path: /etc/kong/kong.conf
        line: "{{ item }}"
      loop:
        - "database = off"
        - "declarative_config = /etc/kong/kong.yml"
      become: true

    - name: Start Kong
      shell: kong start
      become: true
```

## Agent Use
- **API gateway**: Centralized routing and management for microservices
- **Authentication**: Unified auth across all APIs (JWT, OAuth, API keys)
- **Rate limiting**: Protect backends from abuse and DDoS
- **Transformation**: Modify requests/responses without changing backends
- **Logging**: Centralized logging and analytics
- **Caching**: Response caching for improved performance
- **Load balancing**: Distribute traffic across backend instances
- **Service mesh**: Secure service-to-service communication

## Troubleshooting

### Kong won't start
```bash
# Check configuration
kong check /etc/kong/kong.conf

# Check logs
tail -f /var/log/kong/error.log

# Verify database connection
psql -h localhost -U kong -d kong

# Check port availability
netstat -tulpn | grep -E '8000|8001'
```

### Plugin not working
```bash
# Verify plugin is enabled
curl http://localhost:8001/plugins | jq '.data[] | select(.name=="rate-limiting")'

# Check plugin configuration
curl http://localhost:8001/plugins/{plugin-id}

# Test without Kong (direct to backend)
curl http://backend:8080/api
```

### High latency
```bash
# Check upstream health
curl http://localhost:8001/upstreams/my-upstream/health

# Disable plugins to isolate issue
curl -X DELETE http://localhost:8001/plugins/{plugin-id}

# Check NGINX metrics
curl http://localhost:8001/status
```

### Database connection errors
```bash
# Test database connectivity
psql -h DB_HOST -U kong -d kong

# Run migrations
kong migrations up

# Check Kong database config
grep -E 'pg_host|pg_database|pg_user' /etc/kong/kong.conf
```

## Best Practices
- **Use upstreams**: Better load balancing and health checking than direct service URLs
- **Enable health checks**: Automatic failover for unhealthy backends
- **Rate limit per consumer**: More granular control than global limits
- **Cache responses**: Use proxy-cache plugin for cacheable endpoints
- **Monitor metrics**: Enable Prometheus plugin for observability
- **Use declarative config**: Easier to version control and deploy
- **Secure Admin API**: Restrict access with firewall or key-auth plugin
- **Test in staging**: Validate plugins and config before production
- **Use workspaces**: Separate environments (Enterprise feature)

## Comparison

| Feature | Kong | Nginx | Traefik | API Gateway (AWS) |
|---------|------|-------|---------|-------------------|
| Protocol support | HTTP, gRPC, TCP | HTTP, TCP | HTTP, TCP, gRPC | HTTP only |
| Plugins | 200+ | Limited | 50+ | AWS integrations |
| Service discovery | ✅ | ❌ | ✅ | ✅ |
| Kubernetes native | ✅ | Manual | ✅ | N/A |
| Open source | ✅ | ✅ | ✅ | ❌ (managed) |
| Performance | Very high | Highest | High | High |

## Uninstall
```yaml
- preset: kong
  with:
    state: absent
```

**Note**: PostgreSQL database and configuration files are preserved. Remove manually if needed.

## Resources
- Official: https://konghq.com/
- Documentation: https://docs.konghq.com/
- Plugin Hub: https://docs.konghq.com/hub/
- GitHub: https://github.com/Kong/kong
- Forum: https://discuss.konghq.com/
- Kubernetes: https://github.com/Kong/kubernetes-ingress-controller
- Search: "kong api gateway tutorial", "kong plugins", "kong kubernetes"

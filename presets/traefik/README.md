# Traefik - Cloud Native Edge Router

Modern HTTP reverse proxy and load balancer with automatic HTTPS, Docker/Kubernetes integration, and dynamic configuration.

## Quick Start

```yaml
- preset: traefik
```

## Features

- **Automatic HTTPS**: Let's Encrypt integration with automatic certificate renewal
- **Dynamic Configuration**: Auto-discovery for Docker, Kubernetes, Consul, etcd
- **Load Balancing**: Multiple algorithms (round-robin, weighted, sticky sessions)
- **Middleware**: Rate limiting, circuit breakers, authentication, compression
- **Dashboard**: Real-time web UI for monitoring and configuration
- **Metrics**: Prometheus, Datadog, StatsD integration
- **WebSocket Support**: Native WebSocket and HTTP/2 support
- **Cross-platform**: Linux and macOS support

## Basic Usage

```bash
# Start Traefik with config
traefik --configFile=traefik.yml

# Check configuration
traefik --configFile=traefik.yml --dry-run

# Version and help
traefik version
traefik --help

# Access dashboard
open http://localhost:8080
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove traefik |
| version | string | v3.0 | Traefik version to install |
| http_port | string | 80 | HTTP entrypoint port |
| https_port | string | 443 | HTTPS entrypoint port |
| dashboard_port | string | 8080 | Dashboard UI port |
| enable_dashboard | bool | true | Enable web dashboard |

## Advanced Configuration

### Basic Setup
```yaml
- preset: traefik
```

### Custom Ports
```yaml
- preset: traefik
  with:
    http_port: "8080"
    https_port: "8443"
    dashboard_port: "9000"
```

### Production Setup
```yaml
- preset: traefik
  with:
    version: "v3.0"
    enable_dashboard: true
    http_port: "80"
    https_port: "443"
```

## Platform Support

- ✅ Linux (binary, package managers, Docker)
- ✅ macOS (Homebrew, binary, Docker)
- ✅ Windows (binary, Docker)
- ✅ Kubernetes (Helm, manifests)

## Configuration

- **Config file**: `~/traefik/traefik.yml` (static configuration)
- **Dynamic config**: `~/traefik/dynamic/` (service definitions)
- **Dashboard**: `http://localhost:8080` (default)
- **Certificates**: `~/traefik/acme.json` (Let's Encrypt)
- **Logs**: stdout or file-based

## Real-World Examples

### Docker Compose Setup
```yaml
# docker-compose.yml
version: '3'

services:
  traefik:
    image: traefik:v3.0
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik.yml:/traefik.yml:ro
      - ./acme.json:/acme.json

  whoami:
    image: traefik/whoami
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.whoami.rule=Host(`whoami.localhost`)"
      - "traefik.http.routers.whoami.entrypoints=web"

  webapp:
    image: nginx:alpine
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.webapp.rule=Host(`app.example.com`)"
      - "traefik.http.routers.webapp.entrypoints=websecure"
      - "traefik.http.routers.webapp.tls.certresolver=letsencrypt"
      - "traefik.http.services.webapp.loadbalancer.server.port=80"
```

Start: `docker-compose up -d`
Test: `curl http://whoami.localhost`

### Microservices Architecture
```yaml
# Multiple services with routing
services:
  api:
    image: myapi:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api.rule=Host(`api.example.com`)"
      - "traefik.http.routers.api.middlewares=api-auth,api-ratelimit"
      - "traefik.http.middlewares.api-auth.basicauth.users=admin:$$apr1$$..."
      - "traefik.http.middlewares.api-ratelimit.ratelimit.average=100"

  frontend:
    image: myfrontend:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.frontend.rule=Host(`example.com`)"
      - "traefik.http.routers.frontend.middlewares=frontend-compress"
      - "traefik.http.middlewares.frontend-compress.compress=true"

  admin:
    image: myadmin:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.admin.rule=Host(`admin.example.com`)"
      - "traefik.http.routers.admin.middlewares=admin-whitelist"
      - "traefik.http.middlewares.admin-whitelist.ipwhitelist.sourcerange=192.168.1.0/24"
```

### Kubernetes Ingress
```yaml
# traefik-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls.certresolver: letsencrypt
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: myapp
            port:
              number: 80
  tls:
  - hosts:
    - app.example.com
    secretName: myapp-tls
```

## Static Configuration

`~/traefik/traefik.yml`:
```yaml
# Entry points
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https

  websecure:
    address: ":443"

# Docker provider
providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
  file:
    directory: /etc/traefik/dynamic
    watch: true

# API and dashboard
api:
  dashboard: true
  insecure: true  # Only for dev

# Let's Encrypt
certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@example.com
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web

# Logging
log:
  level: INFO
  filePath: /var/log/traefik/traefik.log

accessLog:
  filePath: /var/log/traefik/access.log
```

## Dynamic Configuration

`~/traefik/dynamic/services.yml`:
```yaml
http:
  routers:
    api:
      rule: "Host(`api.example.com`)"
      service: api
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
      middlewares:
        - api-auth
        - api-ratelimit

  services:
    api:
      loadBalancer:
        servers:
          - url: "http://localhost:8080"
          - url: "http://localhost:8081"
          - url: "http://localhost:8082"
        healthCheck:
          path: /health
          interval: 10s
          timeout: 3s
        sticky:
          cookie:
            name: server_id

  middlewares:
    api-auth:
      basicAuth:
        users:
          - "admin:$apr1$H6uskkkW$IgXLP6ewTrSuBkTrqE8wj/"

    api-ratelimit:
      rateLimit:
        average: 100
        burst: 50

    api-circuitbreaker:
      circuitBreaker:
        expression: "ResponseCodeRatio(500, 600, 0, 600) > 0.25"

    compress:
      compress: {}

    secure-headers:
      headers:
        sslRedirect: true
        stsSeconds: 315360000
        browserXssFilter: true
        contentTypeNosniff: true
        frameDeny: true
```

## Middleware Examples

### Authentication
```yaml
http:
  middlewares:
    # Basic auth
    basic-auth:
      basicAuth:
        users:
          - "user:$apr1$..."

    # Forward auth (OAuth2)
    oauth:
      forwardAuth:
        address: "http://auth-service:8080/verify"
        trustForwardHeader: true
```

### Rate Limiting
```yaml
http:
  middlewares:
    rate-limit:
      rateLimit:
        average: 100      # requests per second
        burst: 200        # burst size
        period: 1s

    rate-limit-ip:
      rateLimit:
        sourceCriterion:
          ipStrategy:
            depth: 1
```

### Security Headers
```yaml
http:
  middlewares:
    security:
      headers:
        customRequestHeaders:
          X-Forwarded-Proto: "https"
        customResponseHeaders:
          X-Custom-Header: "value"
        sslRedirect: true
        stsSeconds: 315360000
        browserXssFilter: true
        contentTypeNosniff: true
```

## Monitoring and Observability

### Prometheus Metrics
```yaml
# traefik.yml
metrics:
  prometheus:
    entryPoint: metrics
    buckets:
      - 0.1
      - 0.3
      - 1.2
      - 5.0

entryPoints:
  metrics:
    address: ":8082"
```

### Access Logs
```yaml
accessLog:
  filePath: "/var/log/traefik/access.log"
  format: json
  filters:
    statusCodes:
      - "400-499"
      - "500-599"
    retryAttempts: true
    minDuration: "10ms"
```

## Troubleshooting

### Check configuration syntax
```bash
traefik --configFile=traefik.yml --dry-run
```

### Enable debug mode
```yaml
log:
  level: DEBUG
```

### View active routes
```bash
# Via API
curl http://localhost:8080/api/http/routers

# Via dashboard
open http://localhost:8080/dashboard/
```

### Certificate issues
```bash
# Check acme.json
cat ~/traefik/acme.json | jq

# Remove and regenerate
rm ~/traefik/acme.json
# Restart traefik

# Check permissions
chmod 600 ~/traefik/acme.json
```

## Agent Use

- Automatic HTTPS for microservices
- Dynamic service discovery in containers
- Load balancing with health checks
- API gateway with authentication
- Rate limiting and circuit breakers
- Canary deployments and A/B testing
- Multi-cloud ingress controller

## Uninstall

```yaml
- preset: traefik
  with:
    state: absent
```

## Resources

- Official docs: https://doc.traefik.io/traefik/
- GitHub: https://github.com/traefik/traefik
- Community: https://community.traefik.io/
- Plugins: https://plugins.traefik.io/
- Search: "traefik docker", "traefik kubernetes", "traefik middleware"

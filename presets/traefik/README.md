# Traefik Preset

Install Traefik - a modern HTTP reverse proxy and load balancer with automatic HTTPS.

## Quick Start

```yaml
- preset: traefik
  with:
    enable_dashboard: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `v3.0` | Traefik version |
| `http_port` | string | `80` | HTTP port |
| `https_port` | string | `443` | HTTPS port |
| `dashboard_port` | string | `8080` | Dashboard port |
| `enable_dashboard` | bool | `true` | Enable dashboard |

## Usage

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

## Start Traefik

```bash
cd ~/traefik
traefik --configFile=traefik.yml
```

## Docker Integration

```yaml
# docker-compose.yml
version: '3'

services:
  whoami:
    image: traefik/whoami
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.whoami.rule=Host(`whoami.localhost`)"
      - "traefik.http.routers.whoami.entrypoints=web"

  traefik:
    image: traefik:v3.0
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik.yml:/etc/traefik/traefik.yml:ro
```

Start: `docker-compose up -d`  
Test: `curl http://whoami.localhost`

## Configuration

Edit `~/traefik/traefik.yml`:

```yaml
# Enable ACME (Let's Encrypt)
certificatesResolvers:
  letsencrypt:
    acme:
      email: your@email.com
      storage: acme.json
      httpChallenge:
        entryPoint: web
```

## Dynamic Configuration

Edit `~/traefik/dynamic/myservice.yml`:

```yaml
http:
  routers:
    myapp:
      rule: "Host(`myapp.example.com`)"
      service: myapp
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
  
  services:
    myapp:
      loadBalancer:
        servers:
          - url: "http://localhost:3000"
```

## Middleware

```yaml
http:
  middlewares:
    auth:
      basicAuth:
        users:
          - "user:$apr1$H6uskkkW$IgXLP6ewTrSuBkTrqE8wj/"
    
    ratelimit:
      rateLimit:
        average: 100
        burst: 50
```

## Load Balancing

```yaml
http:
  services:
    myapp:
      loadBalancer:
        servers:
          - url: "http://localhost:3000"
          - url: "http://localhost:3001"
          - url: "http://localhost:3002"
        healthCheck:
          path: /health
          interval: 10s
```

## Common Commands

```bash
# Start Traefik
traefik --configFile=traefik.yml

# Check configuration
traefik --configFile=traefik.yml --dry-run

# View dashboard
open http://localhost:8080
```

## Kubernetes

```yaml
# Install with Helm
helm repo add traefik https://traefik.github.io/charts
helm install traefik traefik/traefik

# Or use as Ingress Controller
kubectl apply -f https://raw.githubusercontent.com/traefik/traefik/v3.0/docs/content/reference/dynamic-configuration/kubernetes-crd-definition-v1.yml
```

## Uninstall

```yaml
- preset: traefik
  with:
    state: absent
```

**Note:** Configuration files preserved in `~/traefik/`.

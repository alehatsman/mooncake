# Apache APISIX - Cloud-Native API Gateway

High-performance API gateway built on NGINX and OpenResty with dynamic routing, 100+ plugins, and multi-protocol support. Configure routes, upstreams, and plugins via RESTful Admin API without restarts. Open-source alternative to Kong and AWS API Gateway.

## Quick Start
```yaml
- preset: apisix
```

Start APISIX: `apisix start`
Admin API: `http://127.0.0.1:9180/apisix/admin`
Data plane: `http://127.0.0.1:9080`

## Features
- **Dynamic routing**: Update configuration without restart via Admin API
- **100+ plugins**: Authentication, rate limiting, transformations, observability
- **Multi-protocol**: HTTP/HTTPS, HTTP/2, gRPC, WebSocket, MQTT, Dubbo
- **Service mesh**: Integrate with Istio, Consul, Nacos, Eureka
- **Dashboard**: Web UI for visual configuration management
- **High performance**: 140k+ RPS on single core (NGINX + LuaJIT)
- **Load balancing**: Round-robin, consistent hashing, weighted, least connections
- **Hot reload**: Zero-downtime configuration updates
- **Multi-tenant**: Route isolation and resource quotas
- **Serverless**: Function-as-a-Service with serverless plugins

## Basic Usage
```bash
# Start APISIX
apisix start

# Create route
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "upstream": {
      "type": "roundrobin",
      "nodes": {
        "backend.example.com:8080": 1
      }
    }
  }'

# Test route
curl http://127.0.0.1:9080/api/users

# List routes
curl http://127.0.0.1:9180/apisix/admin/routes \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'

# Stop APISIX
apisix stop
```

## Architecture

### Components
```
┌──────────────────────────────────────────────────┐
│              APISIX Architecture                 │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │        APISIX Dashboard (Optional)         │ │
│  │         Web UI for management              │ │
│  └────────────────┬───────────────────────────┘ │
│                   │                              │
│  ┌────────────────▼───────────────────────────┐ │
│  │           Admin API (9180)                 │ │
│  │     RESTful API for configuration          │ │
│  └────────────────┬───────────────────────────┘ │
│                   │                              │
│  ┌────────────────▼───────────────────────────┐ │
│  │              etcd Cluster                  │ │
│  │        Configuration storage               │ │
│  └────────────────┬───────────────────────────┘ │
│                   │                              │
│  ┌────────────────▼───────────────────────────┐ │
│  │         APISIX Data Plane (9080)          │ │
│  │  ┌──────────────────────────────────────┐ │ │
│  │  │    Route Matching & Load Balancing   │ │ │
│  │  └──────────────┬───────────────────────┘ │ │
│  │  ┌──────────────▼───────────────────────┐ │ │
│  │  │         Plugin Runner               │ │ │
│  │  │  (Auth, Rate Limit, Transform)      │ │ │
│  │  └──────────────┬───────────────────────┘ │ │
│  │  ┌──────────────▼───────────────────────┐ │ │
│  │  │      Upstream Connection Pool        │ │ │
│  │  └──────────────────────────────────────┘ │ │
│  └────────────────────────────────────────────┘ │
│                   │                              │
│  ┌────────────────▼───────────────────────────┐ │
│  │          Backend Services                  │ │
│  └────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

### Key Concepts
- **Route**: Request matching rules (URI, method, host) → upstream
- **Upstream**: Backend service cluster with load balancing
- **Service**: Abstract upstream + plugins (reusable)
- **Consumer**: API client with authentication credentials
- **Plugin**: Request/response processing logic
- **Global Rules**: Plugins that apply to all routes

## Advanced Configuration

### Production deployment
```yaml
- name: Install APISIX
  preset: apisix

- name: Install etcd
  shell: |
    wget https://github.com/etcd-io/etcd/releases/download/v3.5.0/etcd-v3.5.0-linux-amd64.tar.gz
    tar xzf etcd-v3.5.0-linux-amd64.tar.gz
    mv etcd-v3.5.0-linux-amd64/etcd* /usr/local/bin/
  become: true

- name: Start etcd
  shell: |
    etcd --listen-client-urls http://0.0.0.0:2379 \
      --advertise-client-urls http://127.0.0.1:2379
  async: true

- name: Configure APISIX
  template:
    src: apisix-config.yml.j2
    dest: /usr/local/apisix/conf/config.yaml
  become: true

- name: Start APISIX
  shell: apisix start
  become: true
```

### Configuration file (config.yaml)
```yaml
apisix:
  node_listen: 9080
  enable_admin: true
  admin_key:
    - name: admin
      key: edd1c9f034335f136f87ad84b625c8f1
      role: admin

deployment:
  role: traditional
  role_traditional:
    config_provider: etcd
  admin:
    admin_listen:
      ip: 0.0.0.0
      port: 9180
  etcd:
    host:
      - "http://127.0.0.1:2379"
    prefix: /apisix
    timeout: 30

nginx_config:
  error_log_level: warn
  worker_processes: auto
  worker_connections: 10620
```

### High availability cluster
```yaml
# Deploy etcd cluster (3 nodes)
- name: Deploy etcd cluster
  shell: |
    etcd --name etcd-{{ ansible_hostname }} \
      --listen-client-urls http://0.0.0.0:2379 \
      --advertise-client-urls http://{{ ansible_default_ipv4.address }}:2379 \
      --listen-peer-urls http://0.0.0.0:2380 \
      --initial-advertise-peer-urls http://{{ ansible_default_ipv4.address }}:2380 \
      --initial-cluster etcd-1=http://node1:2380,etcd-2=http://node2:2380,etcd-3=http://node3:2380
  async: true

# Deploy APISIX on multiple nodes
- name: Deploy APISIX cluster
  shell: apisix start
  become: true
  delegate_to: "{{ item }}"
  loop:
    - node1
    - node2
    - node3
```

## Routes and Upstreams

### Create route with upstream
```bash
# Basic route
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/users/*",
    "methods": ["GET", "POST"],
    "upstream": {
      "type": "roundrobin",
      "nodes": {
        "backend-1.example.com:8080": 1,
        "backend-2.example.com:8080": 1
      }
    }
  }'
```

### Advanced routing
```bash
# Host-based routing
curl http://127.0.0.1:9180/apisix/admin/routes/2 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "host": "api.example.com",
    "uri": "/*",
    "upstream": {
      "type": "roundrobin",
      "nodes": {
        "backend:8080": 1
      }
    }
  }'

# Priority-based routing
curl http://127.0.0.1:9180/apisix/admin/routes/3 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "priority": 10,
    "uri": "/api/special",
    "upstream": {
      "nodes": {
        "special-backend:8080": 1
      }
    }
  }'
```

### Upstream with health checks
```bash
curl http://127.0.0.1:9180/apisix/admin/upstreams/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "type": "roundrobin",
    "nodes": {
      "backend-1:8080": 1,
      "backend-2:8080": 1
    },
    "checks": {
      "active": {
        "http_path": "/health",
        "healthy": {
          "interval": 2,
          "successes": 2
        },
        "unhealthy": {
          "interval": 1,
          "http_failures": 2
        }
      }
    }
  }'
```

## Plugins

### Authentication plugins

#### API Key
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "key-auth": {}
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'

# Create consumer with API key
curl http://127.0.0.1:9180/apisix/admin/consumers \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "username": "john",
    "plugins": {
      "key-auth": {
        "key": "user-api-key-12345"
      }
    }
  }'

# Use API key
curl http://127.0.0.1:9080/api/users -H 'apikey: user-api-key-12345'
```

#### JWT
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "jwt-auth": {
        "secret": "my-secret-key"
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

#### OAuth 2.0
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "oauth": {
        "client_id": "your-client-id",
        "client_secret": "your-client-secret",
        "authorize_url": "https://auth.example.com/oauth/authorize",
        "token_url": "https://auth.example.com/oauth/token"
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

### Traffic control plugins

#### Rate limiting
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "limit-req": {
        "rate": 100,
        "burst": 50,
        "key": "remote_addr"
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

#### IP restriction
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "ip-restriction": {
        "whitelist": ["10.0.0.0/8", "192.168.0.0/16"]
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

### Transformation plugins

#### Request/Response rewrite
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "proxy-rewrite": {
        "regex_uri": ["^/api/(.*)", "/$1"],
        "headers": {
          "X-Forwarded-For": "$remote_addr"
        }
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

#### CORS
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "cors": {
        "allow_origins": "*",
        "allow_methods": "GET,POST,PUT,DELETE",
        "allow_headers": "Content-Type,Authorization"
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

### Observability plugins

#### Prometheus metrics
```bash
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "prometheus": {}
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'

# Scrape metrics
curl http://127.0.0.1:9091/apisix/prometheus/metrics
```

#### Logging
```bash
# HTTP logger
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "uri": "/api/*",
    "plugins": {
      "http-logger": {
        "uri": "http://log-collector:8080/logs"
      }
    },
    "upstream": {
      "nodes": {"backend:8080": 1}
    }
  }'
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove APISIX |

## Platform Support
- ✅ Linux (all distributions) - native package
- ✅ macOS (Homebrew)
- ✅ Docker (official images)
- ✅ Kubernetes (Helm charts)
- ❌ Windows (use WSL2 or Docker)

## Configuration

### Admin API security
```yaml
# Change default admin key
apisix:
  admin_key:
    - name: admin
      key: your-secure-random-key-here
      role: admin
    - name: viewer
      key: viewer-key-here
      role: viewer
```

### SSL/TLS configuration
```bash
# Upload SSL certificate
curl http://127.0.0.1:9180/apisix/admin/ssl/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "cert": "'"$(cat cert.pem)"'",
    "key": "'"$(cat key.pem)"'",
    "snis": ["api.example.com"]
  }'
```

### Service discovery
```yaml
# Consul integration
discovery:
  consul:
    servers:
      - "http://consul-server:8500"

# Nacos integration
discovery:
  nacos:
    host:
      - "http://nacos-server:8848"
```

## Dashboard

### Install dashboard
```yaml
- name: Install APISIX Dashboard
  shell: |
    wget https://github.com/apache/apisix-dashboard/releases/download/v3.0.0/apisix-dashboard-3.0.0-linux-amd64.tar.gz
    tar xzf apisix-dashboard-3.0.0-linux-amd64.tar.gz
    mv apisix-dashboard /usr/local/
  become: true

- name: Start dashboard
  shell: |
    cd /usr/local/apisix-dashboard
    ./manager-api
  async: true
```

Access dashboard: `http://localhost:9000`
Default credentials: `admin / admin`

## Use Cases

### API Gateway for microservices
```yaml
- name: Install APISIX
  preset: apisix

- name: Create routes for services
  shell: |
    # User service
    curl http://127.0.0.1:9180/apisix/admin/routes/1 \
      -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
      -X PUT -d '{
        "uri": "/users/*",
        "upstream": {"nodes": {"user-service:8080": 1}}
      }'

    # Order service
    curl http://127.0.0.1:9180/apisix/admin/routes/2 \
      -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
      -X PUT -d '{
        "uri": "/orders/*",
        "upstream": {"nodes": {"order-service:8080": 1}}
      }'
```

### Rate limiting and authentication
```yaml
- name: Configure API with rate limiting
  shell: |
    curl http://127.0.0.1:9180/apisix/admin/routes/1 \
      -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
      -X PUT -d '{
        "uri": "/api/*",
        "plugins": {
          "key-auth": {},
          "limit-req": {
            "rate": 100,
            "burst": 50,
            "key": "consumer_name"
          }
        },
        "upstream": {"nodes": {"backend:8080": 1}}
      }'
```

### Canary deployment
```yaml
- name: Configure canary routing
  shell: |
    curl http://127.0.0.1:9180/apisix/admin/routes/1 \
      -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
      -X PUT -d '{
        "uri": "/api/*",
        "plugins": {
          "traffic-split": {
            "rules": [
              {
                "weighted_upstreams": [
                  {
                    "upstream": {"nodes": {"v1-backend:8080": 1}},
                    "weight": 90
                  },
                  {
                    "upstream": {"nodes": {"v2-backend:8080": 1}},
                    "weight": 10
                  }
                ]
              }
            ]
          }
        }
      }'
```

## CLI Commands

### Service management
```bash
# Start APISIX
apisix start

# Stop APISIX
apisix stop

# Restart APISIX
apisix restart

# Reload configuration
apisix reload

# Check status
apisix status

# Test configuration
apisix test
```

### Admin API operations
```bash
# List routes
curl http://127.0.0.1:9180/apisix/admin/routes \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'

# Get route details
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'

# Delete route
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X DELETE

# List upstreams
curl http://127.0.0.1:9180/apisix/admin/upstreams \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'
```

## Monitoring

### Prometheus metrics
```bash
# Enable Prometheus plugin globally
curl http://127.0.0.1:9180/apisix/admin/global_rules/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1' \
  -X PUT -d '{
    "plugins": {
      "prometheus": {}
    }
  }'

# Scrape metrics
curl http://127.0.0.1:9091/apisix/prometheus/metrics
```

### Key metrics
```promql
# Request count
apisix_http_requests_total

# Request latency
apisix_http_latency

# Bandwidth
apisix_bandwidth

# Upstream status
apisix_http_status
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install APISIX
  preset: apisix
```

### Production deployment
```yaml
- name: Install APISIX
  preset: apisix

- name: Install etcd
  shell: |
    wget https://github.com/etcd-io/etcd/releases/download/v3.5.0/etcd-v3.5.0-linux-amd64.tar.gz
    tar xzf etcd-v3.5.0-linux-amd64.tar.gz
    mv etcd-v3.5.0-linux-amd64/etcd* /usr/local/bin/
  become: true

- name: Start etcd
  shell: etcd
  async: true

- name: Configure APISIX
  template:
    src: apisix-config.yml.j2
    dest: /usr/local/apisix/conf/config.yaml
  become: true

- name: Start APISIX
  shell: apisix start
  become: true

- name: Wait for APISIX
  shell: |
    for i in {1..30}; do
      curl -f http://127.0.0.1:9080 && break
      sleep 1
    done
```

### Kubernetes deployment
```yaml
- name: Add APISIX Helm repository
  shell: |
    helm repo add apisix https://charts.apiseven.com
    helm repo update

- name: Install APISIX
  shell: |
    helm install apisix apisix/apisix \
      --namespace apisix \
      --create-namespace \
      --set gateway.type=LoadBalancer
```

## Agent Use
- **API gateway**: Route and secure microservices traffic
- **Rate limiting**: Protect APIs from abuse
- **Authentication**: Centralized auth (API keys, JWT, OAuth)
- **Load balancing**: Distribute traffic across backends
- **Canary deployments**: Gradual rollout with traffic splitting
- **Service mesh ingress**: Entry point for Istio/Consul mesh
- **Observability**: Metrics, logging, tracing integration

## Troubleshooting

### APISIX won't start
```bash
# Check logs
tail -f /usr/local/apisix/logs/error.log

# Check etcd connection
curl http://127.0.0.1:2379/health

# Test configuration
apisix test

# Check port availability
netstat -tuln | grep -E "9080|9180"
```

### Route not working
```bash
# Check route configuration
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'

# Test upstream directly
curl http://backend:8080/health

# Check APISIX logs
tail -f /usr/local/apisix/logs/access.log
tail -f /usr/local/apisix/logs/error.log
```

### Plugin not working
```bash
# Verify plugin is enabled
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'

# Check plugin configuration
cat /usr/local/apisix/conf/config.yaml | grep -A 10 plugins

# Reload APISIX
apisix reload
```

### High memory usage
```bash
# Check worker processes
ps aux | grep apisix

# Reduce worker connections
# Edit config.yaml
nginx_config:
  worker_connections: 5000

# Restart APISIX
apisix restart
```

## Best Practices

1. **Change default admin key** immediately after installation
2. **Use SSL/TLS** for all production routes
3. **Enable health checks** on upstreams
4. **Implement rate limiting** to prevent abuse
5. **Use authentication plugins** (API key, JWT) for security
6. **Monitor with Prometheus** metrics
7. **Deploy etcd cluster** (3+ nodes) for high availability
8. **Use service discovery** for dynamic upstream management
9. **Enable access logs** for debugging and auditing
10. **Test configuration** before applying (`apisix test`)

## Uninstall
```yaml
- name: Stop APISIX
  shell: apisix stop

- name: Remove APISIX
  preset: apisix
  with:
    state: absent

- name: Remove configuration
  file:
    path: /usr/local/apisix
    state: absent
  become: true
```

**Note**: Uninstalling does not remove etcd data. Clean up etcd separately if needed.

## Resources
- Official: https://apisix.apache.org/
- Documentation: https://apisix.apache.org/docs/apisix/getting-started/
- GitHub: https://github.com/apache/apisix
- Dashboard: https://github.com/apache/apisix-dashboard
- Plugins: https://apisix.apache.org/docs/apisix/plugins/batch-requests/
- Community: https://apisix.apache.org/docs/general/join/
- Blog: https://apisix.apache.org/blog/
- Search: "apisix tutorial", "apisix vs kong", "apisix plugins"

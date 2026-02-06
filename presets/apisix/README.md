# Apache APISIX - API Gateway

Cloud-native API gateway with dynamic routing, plugin ecosystem, and high performance.

## Quick Start
```yaml
- preset: apisix
```

## Features
- **Dynamic routing**: Change routes without restart
- **100+ plugins**: Authentication, rate limiting, transformation
- **Multi-protocol**: HTTP, HTTPS, gRPC, WebSocket, MQTT
- **Service discovery**: Consul, Eureka, Nacos integration
- **Dashboard**: Web UI for configuration
- **High performance**: Built on NGINX/OpenResty
- **Multi-cloud**: Works across cloud providers

## Basic Usage
```bash
# Start APISIX
apisix start

# Create route via API
curl http://127.0.0.1:9180/apisix/admin/routes/1 \
  -H 'X-API-KEY: admin-key' -X PUT -d '
{
  "uri": "/api/*",
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "backend:8080": 1
    }
  }
}'

# List routes
curl http://127.0.0.1:9180/apisix/admin/routes -H 'X-API-KEY: admin-key'

# Check status
apisix status
```

## Advanced Configuration
```yaml
- preset: apisix
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: apisix
  with:
    state: absent
```

## Resources
- Search: "apisix documentation", "apisix tutorial"

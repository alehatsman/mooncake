# HAProxy - The Reliable, High Performance TCP/HTTP Load Balancer

Fast, reliable load balancer and reverse proxy for TCP and HTTP applications. Powers some of the world's busiest websites.

## Quick Start
```yaml
- preset: haproxy
```

## Features
- **High performance**: Handles millions of requests per second with minimal overhead
- **Layer 4/7 load balancing**: TCP (L4) and HTTP/HTTPS (L7) support
- **SSL/TLS termination**: Offload SSL/TLS encryption from backend servers
- **Health checking**: Automatic failover with sophisticated health checks
- **Advanced routing**: Content-based routing, URL rewriting, header manipulation
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Check version
haproxy -v

# Test configuration
haproxy -c -f /etc/haproxy/haproxy.cfg

# Start with config file
haproxy -f /etc/haproxy/haproxy.cfg

# Reload without dropping connections
haproxy -f /etc/haproxy/haproxy.cfg -sf $(pidof haproxy)

# Check running status (systemd)
systemctl status haproxy
```

## Configuration
- **Config file**: `/etc/haproxy/haproxy.cfg` (Linux)
- **Stats socket**: `/var/run/haproxy.sock`
- **Default ports**: 80 (HTTP), 443 (HTTPS), 8404 (stats)
- **Logs**: `/var/log/haproxy.log` (Linux)

## Real-World Examples

### Basic HTTP Load Balancer
```
# /etc/haproxy/haproxy.cfg
global
    log /dev/log local0
    maxconn 4096
    user haproxy
    group haproxy
    daemon

defaults
    log global
    mode http
    option httplog
    option dontlognull
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

frontend http_front
    bind *:80
    default_backend http_back

backend http_back
    balance roundrobin
    server web1 192.168.1.10:8080 check
    server web2 192.168.1.11:8080 check
    server web3 192.168.1.12:8080 check
```

### SSL Termination
```
frontend https_front
    bind *:443 ssl crt /etc/haproxy/certs/site.pem
    default_backend https_back

backend https_back
    balance leastconn
    option httpchk GET /health
    server app1 10.0.1.10:8080 check
    server app2 10.0.1.11:8080 check backup
```

### Advanced Routing with ACLs
```
frontend app_front
    bind *:80

    # Route based on URL path
    acl is_api path_beg /api/
    acl is_static path_beg /static/

    use_backend api_servers if is_api
    use_backend static_servers if is_static
    default_backend web_servers

backend api_servers
    balance leastconn
    server api1 10.0.2.10:3000 check
    server api2 10.0.2.11:3000 check

backend static_servers
    balance roundrobin
    server static1 10.0.3.10:80 check
    server static2 10.0.3.11:80 check

backend web_servers
    balance roundrobin
    server web1 10.0.1.10:8080 check
    server web2 10.0.1.11:8080 check
```

### Health Checks and Failover
```
backend app_back
    balance roundrobin
    option httpchk GET /health
    http-check expect status 200

    # Primary servers
    server app1 10.0.1.10:8080 check inter 2000 rise 2 fall 3
    server app2 10.0.1.11:8080 check inter 2000 rise 2 fall 3

    # Backup server (only used if primaries fail)
    server backup 10.0.1.99:8080 check backup
```

### Stats Dashboard
```
frontend stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 30s
    stats auth admin:password123
```

## CI/CD Integration

### Configuration Validation in Pipeline
```yaml
- name: Validate HAProxy config
  shell: haproxy -c -f haproxy.cfg
  register: validation

- name: Fail if config invalid
  assert:
    command:
      cmd: haproxy -c -f haproxy.cfg
      exit_code: 0
```

### Zero-Downtime Reload
```bash
#!/bin/bash
# Reload HAProxy without dropping connections

# Validate new config first
haproxy -c -f /etc/haproxy/haproxy.cfg || exit 1

# Graceful reload
systemctl reload haproxy
```

## Agent Use
- Deploy load balancers for web applications and microservices
- Implement SSL/TLS termination and certificate management
- Set up blue/green deployments with backend switching
- Create API gateways with path-based routing
- Monitor backend health and automate failover
- Implement rate limiting and DDoS protection

## Advanced Configuration
```yaml
- preset: haproxy
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove HAProxy |

## Troubleshooting

### Configuration Errors
```bash
# Test config syntax
haproxy -c -f /etc/haproxy/haproxy.cfg

# Check logs for errors
journalctl -u haproxy -n 50
tail -f /var/log/haproxy.log
```

### Connection Issues
```bash
# Verify HAProxy is listening
netstat -tlnp | grep haproxy
ss -tlnp | grep haproxy

# Test backend connectivity
curl -v http://backend-ip:port/health
```

### Performance Monitoring
```bash
# View stats via socket
echo "show stat" | socat stdio /var/run/haproxy.sock

# Check current connections
echo "show sess" | socat stdio /var/run/haproxy.sock
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Uninstall
```yaml
- preset: haproxy
  with:
    state: absent
```

## Resources
- Official docs: https://www.haproxy.org/
- Configuration manual: https://docs.haproxy.org/
- GitHub: https://github.com/haproxy/haproxy
- Search: "haproxy load balancing tutorial", "haproxy ssl termination", "haproxy best practices"

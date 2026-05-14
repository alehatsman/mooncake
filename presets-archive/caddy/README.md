# Caddy - Modern Web Server

Fast, production-ready web server with automatic HTTPS, modern protocols, and simple configuration.

## Quick Start
```yaml
- preset: caddy
```

## Features
- **Automatic HTTPS**: Free SSL/TLS certificates via Let's Encrypt
- **HTTP/2 & HTTP/3**: Modern protocol support out of the box
- **Simple config**: Easy-to-read Caddyfile format
- **Reverse proxy**: Built-in reverse proxy and load balancing
- **File server**: Static file serving with directory browsing
- **API**: JSON API for dynamic configuration

## Basic Usage
```bash
# Check version
caddy version

# Start server (Caddyfile in current directory)
caddy run

# Start in background
caddy start

# Stop server
caddy stop

# Reload configuration
caddy reload

# Validate Caddyfile
caddy validate

# Format Caddyfile
caddy fmt --overwrite
```

## Simple Configuration

### Static File Server
```caddyfile
# Caddyfile
localhost:8080

file_server
```

```bash
caddy run
# Visit http://localhost:8080
```

### Reverse Proxy
```caddyfile
example.com {
    reverse_proxy localhost:8080
}
```

### Multiple Sites
```caddyfile
site1.com {
    root * /var/www/site1
    file_server
}

site2.com {
    reverse_proxy localhost:3000
}

api.site3.com {
    reverse_proxy localhost:8080
}
```

## Advanced Configuration

### HTTPS with Custom Domain
```caddyfile
myapp.example.com {
    # Automatic HTTPS
    reverse_proxy localhost:8080

    # Custom headers
    header {
        X-Frame-Options "DENY"
        X-Content-Type-Options "nosniff"
    }

    # Access logging
    log {
        output file /var/log/caddy/access.log
    }
}
```

### Load Balancing
```caddyfile
example.com {
    reverse_proxy {
        to localhost:8080 localhost:8081 localhost:8082
        lb_policy round_robin
        health_uri /health
        health_interval 10s
    }
}
```

### Rate Limiting
```caddyfile
example.com {
    rate_limit {
        zone api {
            key {remote_host}
            events 100
            window 1m
        }
    }

    reverse_proxy localhost:8080
}
```

### File Server with Authentication
```caddyfile
files.example.com {
    root * /var/www/files

    basicauth {
        alice JDJhJDEwJEVCNmdaNEg2Ti5iejRMYkF3MFZhZ3VtV3E1SzBWZEZ5Q3VWc0tzOEJwZE9TaFlZdEVkZDhX
    }

    file_server browse
}
```

## Real-World Examples

### Deploy Static Site
```yaml
- name: Install Caddy
  preset: caddy
  become: true

- name: Create site directory
  file:
    path: /var/www/mysite
    state: directory
    mode: "0755"
  become: true

- name: Deploy site files
  copy:
    src: ./build/
    dest: /var/www/mysite/
  become: true

- name: Configure Caddy
  template:
    dest: /etc/caddy/Caddyfile
    content: |
      mysite.com {
          root * /var/www/mysite
          file_server
          encode gzip
      }
  become: true

- name: Restart Caddy
  service:
    name: caddy
    state: restarted
  become: true
```

### API Gateway
```yaml
- name: Configure Caddy as API gateway
  template:
    dest: /etc/caddy/Caddyfile
    content: |
      api.example.com {
          # Users API
          handle /users/* {
              reverse_proxy localhost:8001
          }

          # Orders API
          handle /orders/* {
              reverse_proxy localhost:8002
          }

          # Default
          handle {
              respond "API Gateway" 200
          }
      }
  become: true

- name: Reload Caddy
  shell: caddy reload --config /etc/caddy/Caddyfile
  become: true
```

### Development Server
```caddyfile
# Caddyfile
localhost:8080 {
    # Serve frontend
    handle /app/* {
        root * /var/www
        file_server
    }

    # Proxy API to backend
    handle /api/* {
        reverse_proxy localhost:3000
    }

    # WebSocket support
    handle /ws {
        reverse_proxy localhost:3000
    }
}
```

## Service Management

### Linux (systemd)
```bash
# Start service
sudo systemctl start caddy

# Enable on boot
sudo systemctl enable caddy

# Check status
sudo systemctl status caddy

# View logs
sudo journalctl -u caddy -f

# Reload config
sudo systemctl reload caddy
```

### macOS (launchd)
```bash
# Start
brew services start caddy

# Stop
brew services stop caddy

# Restart
brew services restart caddy
```

## Configuration File Locations
- **Caddyfile**: `/etc/caddy/Caddyfile` (Linux), `/usr/local/etc/Caddyfile` (macOS)
- **Data directory**: `/var/lib/caddy` (Linux), `~/Library/Application Support/Caddy` (macOS)
- **Logs**: `/var/log/caddy/` (Linux), `~/Library/Logs/caddy/` (macOS)

## JSON API

### Dynamic Configuration
```bash
# Get current config
curl http://localhost:2019/config/

# Load config via API
curl http://localhost:2019/load \
  -X POST \
  -H "Content-Type: application/json" \
  -d @caddy-config.json

# Add route
curl http://localhost:2019/config/apps/http/servers/srv0/routes \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"handle": [{"handler": "static_response", "body": "Hello"}]}'
```

## Troubleshooting

### Certificate issues
```bash
# Check certificate status
caddy trust

# Manual certificate
caddy trust install

# View logs for certificate errors
journalctl -u caddy | grep -i cert
```

### Port already in use
```bash
# Check what's using port 80/443
sudo lsof -i :80
sudo lsof -i :443

# Use different port in Caddyfile
:8080 {
    file_server
}
```

### Config not loading
```bash
# Validate Caddyfile
caddy validate --config /etc/caddy/Caddyfile

# Check for syntax errors
caddy fmt --check

# Test config
caddy run --config /etc/caddy/Caddyfile
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ✅ Windows (chocolatey)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Deploy web servers with automatic HTTPS
- Set up reverse proxies for microservices
- Host static sites with modern protocols
- Create API gateways with load balancing
- Serve files with authentication
- Configure web servers without complex syntax

## Uninstall
```yaml
- preset: caddy
  with:
    state: absent
```

## Resources
- Official site: https://caddyserver.com
- Documentation: https://caddyserver.com/docs/
- GitHub: https://github.com/caddyserver/caddy
- Community forum: https://caddy.community
- Search: "caddy web server", "caddy tutorial", "caddy reverse proxy"

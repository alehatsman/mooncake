# Nginx - High-Performance Web Server

High-performance HTTP server, reverse proxy, and load balancer with low resource consumption.

## Quick Start

```yaml
- preset: nginx
```

## Features

- **HTTP server**: Serve static files with high concurrency
- **Reverse proxy**: Forward requests to backend applications
- **Load balancing**: Distribute traffic across multiple servers
- **SSL/TLS termination**: Handle HTTPS encryption
- **HTTP/2 support**: Modern protocol for faster page loads
- **Caching**: Speed up responses with built-in caching
- **URL rewriting**: Flexible request routing

## Basic Usage

```bash
# Check version
nginx -v

# Test configuration syntax
sudo nginx -t

# Start Nginx
sudo systemctl start nginx      # Linux
brew services start nginx       # macOS

# Stop Nginx
sudo systemctl stop nginx       # Linux
brew services stop nginx        # macOS

# Reload configuration (no downtime)
sudo nginx -s reload

# View logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log
```

## Advanced Configuration

```yaml
# Install Nginx with custom configuration
- preset: nginx
  with:
    start_service: true
    port: "8080"
    server_name: "example.com"
    root_dir: "/var/www/myapp"
    enable_ssl: true
```

```yaml
# Install and configure as reverse proxy
- preset: nginx
  with:
    start_service: true
    port: "80"
    ssl_port: "443"

- name: Deploy reverse proxy config
  template:
    src_template: templates/nginx-proxy.conf.j2
    dest: /etc/nginx/sites-available/myapp.conf
  become: true

- name: Enable site
  file:
    src: /etc/nginx/sites-available/myapp.conf
    dest: /etc/nginx/sites-enabled/myapp.conf
    state: link
  become: true

- name: Reload Nginx
  shell: nginx -s reload
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Nginx |
| start_service | bool | true | Start Nginx service after installation |
| port | string | "80" | Default HTTP port |
| ssl_port | string | "443" | Default HTTPS port |
| server_name | string | "_" | Server name (default is catch-all) |
| root_dir | string | /var/www/html | Document root directory |
| enable_ssl | bool | false | Enable SSL configuration |

## Platform Support

- ✅ Linux (apt, dnf, yum, zypper, systemd)
- ✅ macOS (Homebrew, launchd)
- ❌ Windows (use official Windows build)

## Configuration

- **Config file**: `/etc/nginx/nginx.conf` (Linux), `/usr/local/etc/nginx/nginx.conf` (macOS)
- **Site configs**: `/etc/nginx/sites-available/` and `/etc/nginx/sites-enabled/` (Linux)
- **Document root**: `/var/www/html` (Linux), `/usr/local/var/www` (macOS)
- **Logs**: `/var/log/nginx/` (Linux), `/usr/local/var/log/nginx/` (macOS)

## Real-World Examples

### Static Website Hosting
```nginx
# /etc/nginx/sites-available/static-site
server {
    listen 80;
    server_name example.com www.example.com;
    root /var/www/example.com;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }

    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

### Reverse Proxy for Node.js App
```nginx
# /etc/nginx/sites-available/nodejs-app
upstream nodejs_backend {
    server 127.0.0.1:3000;
    server 127.0.0.1:3001;  # Multiple backends for load balancing
}

server {
    listen 80;
    server_name app.example.com;

    location / {
        proxy_pass http://nodejs_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### SSL/TLS with Let's Encrypt
```nginx
# /etc/nginx/sites-available/secure-site
server {
    listen 80;
    server_name example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    root /var/www/example.com;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}
```

### Load Balancer with Health Checks
```nginx
upstream backend_servers {
    least_conn;  # Use least connections algorithm
    server backend1.example.com:8080 max_fails=3 fail_timeout=30s;
    server backend2.example.com:8080 max_fails=3 fail_timeout=30s;
    server backend3.example.com:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    server_name lb.example.com;

    location / {
        proxy_pass http://backend_servers;
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
        proxy_connect_timeout 5s;
        proxy_send_timeout 10s;
        proxy_read_timeout 10s;
    }

    location /health {
        access_log off;
        return 200 "healthy\n";
    }
}
```

### API Gateway with Rate Limiting
```nginx
# Rate limiting zone
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;

server {
    listen 80;
    server_name api.example.com;

    # Apply rate limiting
    location /api/ {
        limit_req zone=api_limit burst=20 nodelay;

        proxy_pass http://api_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # CORS headers
        add_header Access-Control-Allow-Origin *;
        add_header Access-Control-Allow-Methods "GET, POST, PUT, DELETE";
    }
}
```

### CI/CD Deployment
```yaml
# Deploy web application with Nginx
- name: Install Nginx
  preset: nginx
  with:
    start_service: true
  become: true

- name: Deploy application files
  copy:
    src: dist/
    dest: /var/www/myapp/
    recursive: true
  become: true

- name: Deploy Nginx configuration
  template:
    src_template: nginx.conf.j2
    dest: /etc/nginx/sites-available/myapp.conf
  become: true

- name: Enable site
  file:
    src: /etc/nginx/sites-available/myapp.conf
    dest: /etc/nginx/sites-enabled/myapp.conf
    state: link
  become: true

- name: Test configuration
  shell: nginx -t
  become: true

- name: Reload Nginx
  shell: nginx -s reload
  become: true
```

## Common Configurations

### Performance Tuning
```nginx
# /etc/nginx/nginx.conf
worker_processes auto;
worker_connections 1024;

http {
    # Enable gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript
               application/json application/javascript application/xml+rss;

    # Client body size
    client_max_body_size 20M;

    # Timeouts
    client_body_timeout 12;
    client_header_timeout 12;
    keepalive_timeout 15;
    send_timeout 10;

    # File caching
    open_file_cache max=1000 inactive=20s;
    open_file_cache_valid 30s;
    open_file_cache_min_uses 2;
    open_file_cache_errors on;
}
```

### Security Headers
```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
add_header Content-Security-Policy "default-src 'self' http: https: data: blob: 'unsafe-inline'" always;
```

## Agent Use

- Deploy web applications with automated Nginx configuration
- Set up reverse proxies for microservices architectures
- Configure load balancers with health checks
- Implement SSL/TLS termination for backend services
- Create API gateways with rate limiting and authentication
- Serve static assets with optimal caching strategies

## Troubleshooting

### Configuration test failed
```bash
# Test configuration for errors
sudo nginx -t

# Check specific config file
sudo nginx -t -c /etc/nginx/sites-available/mysite.conf

# View detailed error
sudo nginx -t -v
```

### Port already in use
```bash
# Check what's using port 80
sudo lsof -i :80

# Use different port in config
listen 8080;
```

### Permission denied errors
```bash
# Check Nginx user
ps aux | grep nginx

# Fix file permissions
sudo chown -R www-data:www-data /var/www/myapp
sudo chmod -R 755 /var/www/myapp
```

### Service won't start
```bash
# Check logs
sudo journalctl -u nginx -n 50

# Check error log
sudo tail -f /var/log/nginx/error.log

# Verify no syntax errors
sudo nginx -t
```

## Uninstall

```yaml
- preset: nginx
  with:
    state: absent
```

## Resources

- Official docs: https://nginx.org/en/docs/
- Admin guide: https://docs.nginx.com/nginx/admin-guide/
- Config examples: https://www.nginx.com/resources/wiki/start/
- GitHub: https://github.com/nginx/nginx
- Search: "nginx reverse proxy", "nginx ssl setup", "nginx load balancing"

# Nginx Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check status
sudo systemctl status nginx  # Linux
brew services list | grep nginx  # macOS

# Test server
curl http://localhost:8080

# View logs
tail -f /var/log/nginx/access.log  # Linux
tail -f /usr/local/var/log/nginx/access.log  # macOS
```

## Configuration

- **Config file:** `/etc/nginx/nginx.conf` (Linux), `/usr/local/etc/nginx/nginx.conf` (macOS)
- **Document root:** `/var/www/html` (Linux), `/usr/local/var/www` (macOS)
- **Default port:** 80 (or as specified)

## Common Operations

```bash
# Restart service
sudo systemctl restart nginx  # Linux
brew services restart nginx  # macOS

# Reload config
sudo nginx -s reload

# Test config syntax
sudo nginx -t

# Stop service
sudo systemctl stop nginx  # Linux
brew services stop nginx  # macOS
```

## Uninstall

```yaml
- preset: nginx
  with:
    state: absent
```

## Next Steps

1. Edit `/etc/nginx/sites-available/default` to configure your site
2. Create SSL certificates with `certbot` for HTTPS
3. Set up reverse proxy or static file serving as needed

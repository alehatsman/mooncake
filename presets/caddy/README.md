# Caddy Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check status
sudo systemctl status caddy  # Linux
brew services list | grep caddy  # macOS

# Test server
curl http://localhost

# View Caddyfile
cat /etc/caddy/Caddyfile  # Linux
cat /usr/local/etc/Caddyfile  # macOS
```

## Configuration

- **Caddyfile:** `/etc/caddy/Caddyfile` (Linux), `/usr/local/etc/Caddyfile` (macOS)
- **Document root:** `/var/www/html` (default)
- **Automatic HTTPS:** Enabled when using valid domain names

## Common Operations

```bash
# Reload config
sudo caddy reload --config /etc/caddy/Caddyfile

# Format Caddyfile
caddy fmt --overwrite /etc/caddy/Caddyfile

# Test config
caddy validate --config /etc/caddy/Caddyfile

# Restart service
sudo systemctl restart caddy  # Linux
brew services restart caddy  # macOS
```

## Features

- ✅ Automatic HTTPS with Let's Encrypt
- ✅ HTTP/3 support out of the box
- ✅ Simple configuration format
- ✅ Built-in file server

## Uninstall

```yaml
- preset: caddy
  with:
    state: absent
```

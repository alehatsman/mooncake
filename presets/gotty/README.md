# GoTTY - Share Your Terminal as a Web Application

GoTTY is a simple command-line tool that turns CLI tools into web applications, allowing you to share terminal access via a web browser with authentication and SSL support.

## Quick Start

```yaml
- preset: gotty
```

```bash
# Share terminal
gotty -w bash

# Share command output
gotty top

# With authentication
gotty -w -c user:password bash
```

## Features

- **Web-based terminal**: Access CLI tools through a web browser
- **Authentication**: Basic authentication and random URL generation
- **SSL/TLS**: Built-in SSL support
- **Read-only mode**: Share command output without allowing input
- **Customizable**: Reconnection, timeout, and UI options
- **Lightweight**: Single binary with minimal dependencies

## Basic Usage

```bash
# Start web terminal
gotty -w bash

# Access at http://localhost:8080

# Share specific command
gotty top
gotty htop
gotty watch -n 1 df -h

# Custom port
gotty -p 9000 bash

# With authentication
gotty -c user:password bash

# Random URL path (security through obscurity)
gotty -r bash
# Access at http://localhost:8080/random-path

# Read-only mode
gotty top  # Default is read-only for non-shell commands

# Allow write to non-shell commands
gotty -w top
```

## Advanced Configuration

```yaml
- preset: gotty
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove gotty |

## Platform Support

- ✅ Linux (binary install)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

Create `~/.gotty` configuration file:

```ini
# ~/.gotty
port = "8080"
permit_write = false
credential = "user:password"
enable_basic_auth = true
enable_random_url = false
enable_reconnect = true
reconnect_time = 10
timeout = 0
max_connection = 0
once = false
permit_arguments = false
close_signal = 1  # SIGHUP
```

## Real-World Examples

### Remote System Monitoring

```bash
# Share system metrics dashboard
gotty -p 8080 -c admin:secret \
  htop

# Access from browser at http://server-ip:8080
```

### CI/CD Build Logs

```bash
# Stream build output
gotty -w tail -f /var/log/build.log

# With auto-reconnect
gotty -r -w --reconnect tail -f /var/log/deployment.log
```

### Remote Debugging Session

```bash
# Share debugging terminal with team
gotty -w -c team:password \
  --reconnect \
  --timeout 3600 \
  bash
```

### Docker Container Access

```bash
# Provide web-based access to container
gotty docker exec -it mycontainer bash

# Or run gotty inside container
docker run -p 8080:8080 myimage \
  gotty -w -a 0.0.0.0 bash
```

### SSL/TLS Configuration

```bash
# With SSL certificate
gotty --tls \
  --tls-crt /path/to/cert.pem \
  --tls-key /path/to/key.pem \
  -w bash

# Access at https://server:8080
```

### One-Time Session

```bash
# Terminal closes after first session ends
gotty --once -w bash
```

## Agent Use

- Provide web-based access to CLI tools in automation
- Share build/deployment logs via browser
- Create debugging portals for remote teams
- Expose monitoring dashboards without VPN
- Temporary system access for support
- Interactive demos of CLI applications

## Troubleshooting

### Address already in use

Change port:
```bash
gotty -p 9000 bash
```

### Can't connect from other machines

Bind to all interfaces:
```bash
gotty -a 0.0.0.0 bash
```

### Security concerns

Always use authentication and random URLs:
```bash
gotty -r -c username:strong-password bash
```

Consider SSL/TLS for production:
```bash
gotty --tls --tls-crt cert.pem --tls-key key.pem bash
```

### Terminal size issues

Force terminal size:
```bash
gotty --width 120 --height 40 bash
```

## Uninstall

```yaml
- preset: gotty
  with:
    state: absent
```

## Resources

- GitHub: https://github.com/yudai/gotty
- Search: "gotty tutorial", "gotty authentication", "gotty examples"

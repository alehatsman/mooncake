# ttyd - Share Terminal Over the Web

Share your terminal in a web browser over HTTP/HTTPS. Run any command and expose it via web interface.

## Quick Start
```yaml
- preset: ttyd
```

## Features
- **Web-Based Terminal**: Access terminal via browser
- **Secure**: SSL/TLS support with optional authentication
- **Lightweight**: Small footprint, fast performance
- **Cross-platform**: Linux and macOS support
- **Any Command**: Share bash, tmux, vim, or custom commands
- **Multiple Clients**: Multiple users can connect simultaneously

## Basic Usage
```bash
# Share bash shell
ttyd bash

# Share with specific port
ttyd -p 8080 bash

# Share with authentication
ttyd -c user:password bash

# Share read-only
ttyd -W bash

# Share tmux session
ttyd tmux new -A -s shared

# Custom command
ttyd htop
```

## Common Use Cases

### Share Tmux Session
```bash
# Create or attach to shared session
ttyd -p 7681 tmux new -A -s collab

# Access at http://localhost:7681
```

### Remote System Monitoring
```bash
# Share htop
ttyd -p 8080 -c admin:secret htop

# Share system logs
ttyd -p 8080 -W tail -f /var/log/syslog
```

### Demo and Presentations
```bash
# Read-only demo terminal
ttyd -W -p 8080 bash

# Share specific command output
ttyd -p 8080 -W watch -n 1 "kubectl get pods"
```

### Development Environment
```bash
# Share development shell
ttyd -p 7681 bash -c 'cd /project && exec bash'

# Share with environment
ttyd -p 7681 bash -c 'export NODE_ENV=dev && exec bash'
```

## Advanced Options
```bash
# With SSL/TLS
ttyd -S -C cert.pem -K key.pem bash

# Custom interface
ttyd -i 0.0.0.0 -p 8080 bash  # Bind to all interfaces
ttyd -i 127.0.0.1 -p 8080 bash  # Localhost only

# Read-only mode
ttyd -W bash  # Users cannot type

# With authentication
ttyd -c user:password bash
ttyd -c admin:secret -c guest:guest bash  # Multiple users

# Maximum clients
ttyd -m 5 bash  # Max 5 concurrent clients

# Enable reconnection
ttyd -o bash  # Once - disconnect after first client exits
```

## Real-World Examples

### CI/CD Debug Session
```bash
# In CI pipeline, expose debug shell
if [ "$CI_DEBUG" = "true" ]; then
  ttyd -p 8080 -c debug:$DEBUG_PASSWORD bash &
  echo "Debug shell: http://ci-runner:8080"
  sleep 3600  # Keep alive for 1 hour
fi
```

### Remote Pair Programming
```bash
# Host creates session
ttyd -p 7681 tmux new -A -s pair

# Share URL with collaborator
echo "Connect to: http://$(hostname -I | awk '{print $1}'):7681"

# Both can type and see changes
```

### Server Monitoring Dashboard
```bash
# Create monitoring tmux layout
tmux new -s monitoring -d
tmux split-window -h 'htop'
tmux select-pane -t 0
tmux split-window -v 'tail -f /var/log/app.log'

# Share via ttyd
ttyd -W -p 8080 tmux attach -t monitoring
```

### Interactive Documentation
```bash
# Share demo environment
ttyd -W -p 8080 bash -c '
  clear
  cat << EOF
Welcome to the API Demo!
Available commands:
  - api-status: Check API health
  - api-test: Run test suite
  - api-logs: View recent logs
EOF
  exec bash
'
```

### Customer Support
```bash
# Support agent shares terminal
ttyd -p 7681 -c support:$SUPPORT_PASSWORD tmux new -A -s customer-123

# Customer connects via browser
# No SSH or terminal emulator needed
```

## Configuration Options

### Command Line
```bash
ttyd [options] <command> [args...]

Options:
  -p PORT         Port (default: 7681)
  -i INTERFACE    Interface to bind (default: all)
  -c USER:PASS    Basic authentication
  -C CERT         SSL certificate
  -K KEY          SSL key
  -W              Read-only mode
  -o              Once mode (exit after first client)
  -m NUM          Max clients
  -t TITLE        Browser tab title
```

### Systemd Service
```ini
[Unit]
Description=ttyd - Terminal Sharing
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ttyd -p 7681 -c admin:secretpass bash
Restart=always
User=ttyd

[Install]
WantedBy=multi-user.target
```

## Security Considerations
```bash
# Always use authentication for public access
ttyd -c user:strongpassword bash

# Use SSL in production
ttyd -S -C cert.pem -K key.pem -c user:password bash

# Bind to localhost only
ttyd -i 127.0.0.1 -p 7681 bash

# Use read-only mode for monitoring
ttyd -W htop

# Behind reverse proxy (nginx)
location /terminal {
    proxy_pass http://localhost:7681;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

## Advanced Configuration
```yaml
- preset: ttyd
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove ttyd |

## Platform Support
- ✅ Linux (build from source or package managers)
- ✅ macOS (Homebrew or build from source)
- ❌ Windows (WSL recommended)

## Browser Support
- Chrome/Chromium
- Firefox
- Safari
- Edge
- Mobile browsers (iOS Safari, Chrome Mobile)

## Troubleshooting

### Connection refused
```bash
# Check if ttyd is running
ps aux | grep ttyd

# Check port
netstat -tlnp | grep 7681

# Test locally
curl http://localhost:7681
```

### WebSocket errors
```bash
# Check firewall
sudo ufw allow 7681

# Verify websocket upgrade
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:7681
```

### Authentication not working
```bash
# Verify credentials format
ttyd -c user:pass bash  # Correct
ttyd -c user pass bash  # Wrong

# Multiple users
ttyd -c admin:secret -c user:password bash
```

## Agent Use
- Remote debugging of CI/CD pipelines
- Customer support with browser-based terminal access
- Interactive demos and presentations
- Server monitoring dashboards
- Remote pair programming sessions
- Training and educational environments

## Uninstall
```yaml
- preset: ttyd
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/tsl0922/ttyd
- Official docs: https://tsl0922.github.io/ttyd/
- Search: "ttyd examples", "ttyd ssl", "ttyd authentication"

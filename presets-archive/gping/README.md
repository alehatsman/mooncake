# gping - Ping with a Graph

Interactive ping tool with a real-time graph. Visual network latency monitoring written in Rust.

## Quick Start
```yaml
- preset: gping
```

## Features
- **Visual graph**: Real-time latency visualization
- **Multiple hosts**: Ping multiple hosts simultaneously
- **Fast**: Written in Rust, minimal resource usage
- **Color-coded**: Easy identification of packet loss and latency spikes
- **Interactive**: Scroll through history, zoom in/out
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Ping single host
gping google.com

# Ping multiple hosts
gping google.com cloudflare.com 1.1.1.1

# Specify interval (default: 500ms)
gping --interval 1000 example.com

# Set buffer size (number of pings to keep)
gping --buffer 100 example.com

# Watch mode (no graph, just output)
gping --watch google.com

# Custom packet size
gping --packet-size 1024 example.com
```

## Advanced Configuration
```yaml
- preset: gping
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove gping |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, snap)
- ✅ macOS (Homebrew)
- ✅ Windows (Scoop, Chocolatey, binary)

## Configuration
- **No config file**: All options via CLI flags
- **Colors**: Automatic based on latency thresholds
- **History**: Scrollable graph with arrow keys

## Real-World Examples

### Monitor Multiple Servers
```bash
# Monitor website, API, and database
gping \
  example.com \
  api.example.com \
  db.example.com

# Monitor different regions
gping \
  us-east-1.example.com \
  eu-west-1.example.com \
  ap-southeast-1.example.com
```

### Network Troubleshooting
```bash
# Compare ISP DNS vs public DNS
gping \
  192.168.1.1 \
  8.8.8.8 \
  1.1.1.1

# Trace network path
gping \
  192.168.1.1 \
  gateway.local \
  isp-router.net \
  google.com

# Monitor during network changes
gping --interval 100 8.8.8.8
```

### Performance Testing
```bash
# Monitor during load test
gping --buffer 500 api.example.com &
# Run load test
ab -n 10000 -c 100 https://api.example.com/

# Compare CDN performance
gping \
  cdn1.example.com \
  cdn2.example.com \
  origin.example.com
```

### CI/CD Health Checks
```bash
# Pre-deployment connectivity check
if gping --watch --count 10 production-db.example.com | grep -q "100% packet loss"; then
  echo "ERROR: Cannot reach production database"
  exit 1
fi

# Monitor during deployment
gping \
  old-version.example.com \
  new-version.example.com \
  load-balancer.example.com
```

## Agent Use
- Monitor network connectivity to critical services
- Visualize latency during debugging sessions
- Compare performance across multiple endpoints
- Detect network instability or packet loss
- Troubleshoot DNS or routing issues
- Monitor API endpoint health in real-time

## Troubleshooting

### Permission denied
```bash
# On Linux, ping requires elevated privileges
sudo gping example.com

# Or set capabilities
sudo setcap cap_net_raw+ep $(which gping)

# On macOS, use sudo or grant permissions
sudo gping example.com
```

### Graph not updating
```bash
# Increase interval if network is slow
gping --interval 2000 example.com

# Check if host is reachable
ping -c 4 example.com

# Try with IP address instead of hostname
gping 8.8.8.8
```

### Terminal display issues
```bash
# Use watch mode if graph doesn't render
gping --watch example.com

# Ensure terminal supports colors
export TERM=xterm-256color

# Resize terminal window
# Graph adapts to terminal size
```

## Uninstall
```yaml
- preset: gping
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/orf/gping
- Cargo: https://crates.io/crates/gping
- Search: "gping network monitoring", "gping visual ping"

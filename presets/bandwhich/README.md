# bandwhich - Network Bandwidth Monitor

Terminal bandwidth utilization tool that displays real-time network usage per process, connection, and remote address.

## Quick Start
```yaml
- preset: bandwhich
```

## Features
- **Per-process monitoring**: See which processes use bandwidth
- **Connection tracking**: Monitor individual network connections
- **Remote address display**: View traffic by destination IP/domain
- **Real-time updates**: Live bandwidth usage display
- **Minimal overhead**: Efficient Rust implementation
- **Cross-platform**: Linux, macOS support

## Basic Usage
```bash
# Run bandwhich (requires sudo/root)
sudo bandwhich

# Monitor specific interface
sudo bandwhich --interface eth0

# Show raw addresses (no DNS resolution)
sudo bandwhich --no-resolve

# Sort by download bandwidth
# Press 'd' while running

# Sort by upload bandwidth
# Press 'u' while running

# Toggle DNS resolution
# Press 'r' while running
```

## Keyboard Controls

| Key | Action |
|-----|--------|
| `Space` | Pause display |
| `Tab` | Cycle through views (processes, connections, remote addresses) |
| `d` | Sort by download bandwidth |
| `u` | Sort by upload bandwidth |
| `t` | Sort by total bandwidth |
| `q` | Quit |
| `r` | Toggle DNS resolution |

## Advanced Configuration

```yaml
# Install bandwhich
- preset: bandwhich
  become: true

# Verify installation
- name: Check bandwhich version
  shell: bandwhich --version
  register: version
  become: true

# Run network monitoring
- name: Monitor network for 60 seconds
  shell: timeout 60 bandwhich --interface eth0 > /tmp/network-usage.txt
  become: true
```

## Interface Selection

```bash
# List available interfaces
ip link show                    # Linux
ifconfig                        # macOS

# Monitor specific interface
sudo bandwhich --interface eth0      # Wired
sudo bandwhich --interface wlan0     # Wireless
sudo bandwhich --interface docker0   # Docker bridge
```

## Output Views

### Processes View
Shows bandwidth usage grouped by process:
```
Process Name         Download    Upload      Total
firefox              1.2 MB/s    200 KB/s    1.4 MB/s
chrome               800 KB/s    100 KB/s    900 KB/s
ssh                  50 KB/s     50 KB/s     100 KB/s
```

### Connections View
Shows individual network connections:
```
Local Address:Port      Remote Address:Port     Download    Upload
192.168.1.100:54321     93.184.216.34:443       500 KB/s    50 KB/s
192.168.1.100:54322     172.217.1.1:443         300 KB/s    30 KB/s
```

### Remote Addresses View
Shows traffic grouped by destination:
```
Remote Address          Download    Upload      Total
93.184.216.34           800 KB/s    80 KB/s     880 KB/s
172.217.1.1             400 KB/s    40 KB/s     440 KB/s
```

## Configuration

bandwhich doesn't use a config file but supports environment variables:

```bash
# Disable DNS resolution (faster startup)
export BANDWHICH_NO_RESOLVE=1
sudo -E bandwhich

# Set interface via environment
export BANDWHICH_INTERFACE=eth0
sudo -E bandwhich
```

## Real-World Examples

### Troubleshoot Slow Network
```bash
# Run bandwhich to identify bandwidth hogs
sudo bandwhich

# Look for processes with high bandwidth usage
# Press 'd' to sort by download
# Press Tab to cycle through views
```

### Monitor Docker Container Traffic
```yaml
# Monitor Docker network traffic
- preset: bandwhich
  become: true

- name: Monitor Docker bridge
  shell: timeout 120 bandwhich --interface docker0 > /tmp/docker-network.txt
  become: true

- name: Analyze results
  shell: cat /tmp/docker-network.txt
  register: docker_traffic

- name: Display traffic
  print: "{{ docker_traffic.stdout }}"
```

### CI/CD Network Validation
```bash
# Verify deployment doesn't cause excessive network usage
sudo bandwhich --interface eth0 &
BANDWHICH_PID=$!

# Run deployment
./deploy.sh

# Stop monitoring
sudo kill $BANDWHICH_PID

# Analyze results
# Check if any process exceeded threshold
```

### Development Debugging
```bash
# Monitor application network behavior during tests
sudo bandwhich --interface lo &  # Loopback for local services
MONITOR_PID=$!

# Run integration tests
npm test

# Stop monitoring
sudo kill $MONITOR_PID
```

## Permissions

bandwhich requires elevated privileges to capture network packets:

### Linux
```bash
# Run with sudo
sudo bandwhich

# Or grant capabilities (persistent)
sudo setcap cap_net_raw,cap_net_admin+ep $(which bandwhich)

# Now run without sudo
bandwhich
```

### macOS
```bash
# Always requires sudo on macOS
sudo bandwhich
```

## Comparison with Other Tools

| Feature | bandwhich | iftop | nethogs |
|---------|-----------|-------|---------|
| Per-process | ✅ | ❌ | ✅ |
| Per-connection | ✅ | ✅ | ❌ |
| Remote addresses | ✅ | ✅ | ❌ |
| DNS resolution | ✅ | ✅ | ✅ |
| Modern UI | ✅ | ❌ | ❌ |
| Installation | Rust/Cargo | Package | Package |

## Troubleshooting

### Permission Denied
```bash
# Error: "You don't have permission to capture on that device"
# Solution: Run with sudo
sudo bandwhich

# Or on Linux, grant capabilities
sudo setcap cap_net_raw,cap_net_admin+ep $(which bandwhich)
```

### Interface Not Found
```bash
# List available interfaces
ip link show                    # Linux
ifconfig                        # macOS

# Use correct interface name
sudo bandwhich --interface <name>
```

### High CPU Usage
```bash
# Disable DNS resolution for better performance
sudo bandwhich --no-resolve

# Or use environment variable
export BANDWHICH_NO_RESOLVE=1
sudo -E bandwhich
```

### No Traffic Shown
- Ensure you're monitoring the correct interface
- Check if traffic is actually flowing: `ping google.com`
- Verify interface is up: `ip link show` or `ifconfig`
- Try without DNS resolution: `sudo bandwhich --no-resolve`

## Platform Support
- ✅ Linux (apt, dnf, cargo, Homebrew)
- ✅ macOS (Homebrew, cargo)
- ❌ Windows (not supported)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Monitor network bandwidth during deployments
- Identify bandwidth-intensive processes in production
- Debug network performance issues
- Validate application network behavior in CI/CD
- Track container network usage
- Audit egress traffic for security
- Troubleshoot slow network connections

## Uninstall
```yaml
- preset: bandwhich
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/imsnif/bandwhich
- Releases: https://github.com/imsnif/bandwhich/releases
- Search: "bandwhich tutorial", "bandwhich monitor network", "bandwhich vs nethogs"

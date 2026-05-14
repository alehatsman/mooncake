# MTR - Network Diagnostic Tool

Full-featured network diagnostic utility combining traceroute and ping functionality with real-time latency, packet loss, and route visualization for comprehensive network troubleshooting.

## Quick Start

```yaml
- preset: mtr
```

## Features

- **Combined Traceroute + Ping**: Shows route and latency in single view
- **Real-time Monitoring**: Live updating network statistics with multiple sample runs
- **Packet Loss Detection**: Identifies problematic hops with loss percentages
- **Multiple Display Formats**: Interactive TUI, text output, CSV, JSON, XML
- **Platform Independent**: Works across Linux, macOS, and BSD systems
- **Lightweight**: Minimal dependencies, fast startup
- **Powerful Diagnostics**: Customizable packet size, count, wait time, and protocol selection

## Basic Usage

```bash
# Trace route to host with real-time statistics
mtr example.com

# One-time trace (non-interactive)
mtr -r -c 10 example.com

# Show only packet loss (good for quick check)
mtr -r -c 5 -l 8080

# Trace to IP address with specific packet count
mtr -r -c 100 google.com

# Show JSON output for parsing
mtr --json example.com

# IPv6 trace
mtr -6 example.com

# Use alternate port
mtr --port 80 example.com

# Reverse DNS lookup disabled (faster)
mtr -n example.com

# Increase packet size
mtr -s 1500 example.com
```

## Advanced Configuration

```yaml
- preset: mtr
  with:
    state: present
```

Note: MTR is installed without configuration by default. Usage is command-line driven, allowing flexible network diagnostic options per invocation.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (mtr) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ BSD (pkg)

## Configuration

- **Default behavior**: Interactive mode with real-time updates
- **No configuration file**: All options passed via command-line flags
- **Requires**: Elevated privileges (sudo) for some packet operations
- **Data retention**: No persistent data storage

## Real-World Examples

### Quick Network Health Check

```bash
# Fast check to key service (5 pings)
mtr -r -c 5 api.example.com

# Output shows route and latency for each hop
# Identify if issue is local, ISP, or destination
```

### Latency-Sensitive Monitoring

```bash
# Extended sampling for video streaming service
mtr -r -c 50 video-cdn.example.com

# Analyze packet loss and latency patterns
# Look for consistent vs intermittent problems
```

### DNS and Connection Debugging

```bash
# Disable reverse DNS to speed up trace
mtr -n -r -c 10 problematic-host.com

# Get numeric output for automated parsing
mtr --json -r -c 20 monitoring-target.com > mtr_report.json
```

### Automated Network Diagnostics in Scripts

```bash
#!/bin/bash
# Collect network diagnostics for troubleshooting

HOSTS=("api.prod.local" "db.prod.local" "cache.prod.local")

for host in "${HOSTS[@]}"; do
  echo "=== MTR Report for $host ==="
  mtr --json -r -c 10 "$host" > "mtr_${host}.json"

  # Extract loss percentage (requires jq)
  loss=$(jq '.report.hops[] | select(.loss_pct > 0) | .loss_pct' "mtr_${host}.json")

  if [ ! -z "$loss" ]; then
    echo "WARNING: Packet loss detected on $host"
  fi
done
```

### CI/CD Network Validation

```bash
# Verify connectivity before deployment
if mtr -r -c 5 production-server.com | grep -q "100.0 %"; then
  echo "ERROR: Cannot reach production server"
  exit 1
fi

echo "Network connectivity verified"
```

### Cross-Border Connectivity Testing

```bash
# Test international link quality (important for CDN/cloud)
mtr --json -r -c 30 "international-pop.example.com" > international_mtr.json

# Analyze latency by hop to identify international border
jq '.report.hops[] | {hop: .count, ip: .ip, avg: .avg, loss: .loss_pct}' international_mtr.json
```

## Agent Use

- Network connectivity validation in deployment automation
- Latency baseline collection for performance monitoring
- Automated route diagnostics for troubleshooting CI failures
- ISP/network provider issue verification in SLA testing
- Data center interconnect quality verification
- Geographic latency analysis for multi-region deployments
- Pre-deployment network health checks in infrastructure setup

## Troubleshooting

### Permission denied errors

MTR may require elevated privileges:

```bash
# Run with sudo
sudo mtr example.com

# On some systems, allow non-root use
sudo setcap cap_net_raw=ep /usr/bin/mtr
```

### Reverse DNS lookups slow down trace

Disable reverse DNS for faster results:

```bash
# Skip DNS lookups
mtr -n -r -c 10 example.com
```

### Can't reach specific host

Verify the host is reachable before running mtr:

```bash
# Simple connectivity test first
ping -c 3 example.com

# Then run mtr for detailed trace
mtr -r -c 10 example.com
```

### JSON output parsing fails

Ensure jq is installed for JSON parsing:

```bash
# Install jq if needed
sudo apt-get install jq  # Linux

# Parse mtr JSON output
mtr --json example.com | jq '.report.hops[] | {ip, avg, loss_pct}'
```

### IPv6 not working

Some systems require explicit IPv6 flag:

```bash
# Use explicit IPv6 mode
mtr -6 ipv6.example.com

# Or use IPv6 address directly
mtr -6 2001:db8::1
```

## Uninstall

```yaml
- preset: mtr
  with:
    state: absent
```

## Resources

- **Official Docs**: https://www.bitwizard.nl/mtr/
- **GitHub**: https://github.com/traviscross/mtr
- **Man Page**: `man mtr`
- **Output Formats**: https://www.bitwizard.nl/mtr/files.html
- **Search**: "mtr network troubleshooting", "mtr packet loss analysis", "network diagnostics Linux"

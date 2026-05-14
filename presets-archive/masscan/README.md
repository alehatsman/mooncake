# masscan - Fast TCP Port Scanner

Ultra-fast TCP port scanner capable of scanning the entire Internet in under 6 minutes, transmitting 10 million packets per second.

## Quick Start
```yaml
- preset: masscan
```

## Features
- **Blazing fast**: Scans 10 million packets/second
- **Asynchronous transmission**: Can scan entire Internet in minutes
- **Flexible output**: Text, XML, JSON, binary formats
- **Banner grabbing**: Identifies services on open ports
- **IPv4/IPv6**: Supports both protocols
- **Nmap compatible**: Similar command-line syntax to nmap

## Basic Usage
```bash
# Scan single IP for common ports
sudo masscan 192.168.1.1 -p80,443,22

# Scan IP range
sudo masscan 192.168.1.0/24 -p1-1000

# Scan with max rate
sudo masscan 10.0.0.0/8 -p80,443 --rate=10000

# Scan all 65535 ports
sudo masscan 192.168.1.1 -p0-65535

# Output to file
sudo masscan 192.168.1.0/24 -p80,443 -oJ scan.json

# Banner grabbing
sudo masscan 192.168.1.0/24 -p22,80,443 --banners
```

## Advanced Configuration
```yaml
- preset: masscan
  with:
    state: present              # Install or remove (present/absent)
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether masscan should be installed (present) or removed (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, source)
- ✅ macOS (Homebrew, source)
- ⚠️  Windows (requires WSL or Cygwin)

**Note**: Requires root/sudo for raw socket access.

## Configuration

**Config file**: `/etc/masscan/masscan.conf` or `~/.masscan.conf`

Example config:
```conf
# Default options
rate = 100000
output-format = json
output-filename = scan.json
ports = 0-65535
range = 10.0.0.0/8
excludefile = exclude.txt
```

**Rate limiting**:
- `--rate 1000`: 1,000 packets/second (safe for most networks)
- `--rate 10000`: 10,000 packets/second (local networks)
- `--rate 100000`: 100,000 packets/second (fast scans, may cause issues)

## Real-World Examples

### Network Discovery
```bash
# Find web servers on local network
sudo masscan 192.168.0.0/16 -p80,443,8080,8443 --rate=10000

# Scan common ports
sudo masscan 10.0.0.0/8 -p21,22,23,80,443,3389,3306,5432 --rate=50000
```

### Security Auditing
```bash
# Full port scan with banners
sudo masscan 192.168.1.0/24 -p0-65535 --banners -oJ results.json

# Scan for specific vulnerabilities
sudo masscan 10.0.0.0/8 -p445,3389,1433,3306 --rate=10000
```

### CI/CD Integration
```bash
# Quick security check
sudo masscan $TARGET_IP -p80,443,22 --rate=1000 -oJ scan.json
if grep -q "open" scan.json; then
  echo "Open ports detected"
fi
```

### Exclude Ranges
```bash
# Scan but exclude certain IPs
echo "192.168.1.100-192.168.1.110" > exclude.txt
sudo masscan 192.168.1.0/24 -p80,443 --excludefile exclude.txt
```

## Agent Use
- Perform rapid network reconnaissance for security assessments
- Discover open services across large IP ranges
- Validate firewall rules and network segmentation
- Generate inventory of exposed services
- Automate penetration testing workflows

## Troubleshooting

### Permission denied
masscan requires root privileges for raw socket access:
```bash
sudo masscan 192.168.1.1 -p80
```

### Rate too high causing network issues
Reduce the rate:
```bash
sudo masscan 192.168.1.0/24 -p80,443 --rate=1000
```

### No results
Check if firewall is blocking packets, try reducing rate or disabling firewall temporarily.

## Security Considerations
- **Authorization**: Only scan networks you own or have permission to scan
- **Rate limiting**: High scan rates can disrupt networks
- **Legal**: Unauthorized scanning may violate laws (CFAA in US)
- **Detection**: Masscan generates significant traffic and will be detected by IDS/IPS

## Uninstall
```yaml
- preset: masscan
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/robertdavidgraham/masscan
- Search: "masscan tutorial", "masscan examples", "masscan rate limiting"

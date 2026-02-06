# duf - Disk Usage/Free Utility

Modern disk usage utility with a beautiful interface. Better alternative to `df` with human-readable output and color-coded information.

## Quick Start
```yaml
- preset: duf
```

## Features
- **Beautiful output**: Color-coded, easy-to-read tables
- **Smart sorting**: Automatically sorts by mount point
- **Usage warnings**: Color-coded warnings (green/yellow/red)
- **Flexible filtering**: Hide/show specific filesystems and mount points
- **JSON output**: Machine-readable format for scripting
- **Cross-platform**: Linux, macOS, Windows, BSD

## Basic Usage
```bash
# Show all mounted filesystems
duf

# Show specific filesystem
duf /home

# Show multiple filesystems
duf / /home /var

# Show all filesystems (including pseudo)
duf --all

# Show only local filesystems
duf --only local

# Hide specific filesystems
duf --hide-fs tmpfs,devtmpfs
```

## Output Formats
```bash
# Default table format
duf

# JSON output (for scripting)
duf --json

# JSON with pretty-print
duf --json | jq '.'

# Only show specific columns
duf --output filesystem,size,used,avail,usage
```

## Filtering Options
```bash
# Show only local filesystems
duf --only local

# Show only network filesystems
duf --only network

# Show only fuse filesystems
duf --only fuse

# Hide specific filesystem types
duf --hide-fs tmpfs,devtmpfs,squashfs

# Hide specific mount points
duf --hide /boot,/snap

# Show all (including pseudo/temp filesystems)
duf --all
```

## Sorting
```bash
# Sort by mountpoint (default)
duf --sort mountpoint

# Sort by size
duf --sort size

# Sort by used space
duf --sort used

# Sort by available space
duf --sort avail

# Sort by usage percentage
duf --sort usage

# Sort by filesystem type
duf --sort filesystem
```

## Color-Coded Output
- **Green**: 0-50% usage (healthy)
- **Yellow**: 50-75% usage (warning)
- **Orange**: 75-90% usage (critical)
- **Red**: 90-100% usage (danger)

## Advanced Usage
```bash
# Monitor specific disk
duf /dev/sda1

# Show inodes instead of blocks
duf --inodes

# Set custom warning threshold (default 70%)
duf --warn-threshold 60

# Set custom critical threshold (default 90%)
duf --critical-threshold 85

# Combine filters
duf --only local --hide-fs tmpfs --sort usage

# Watch disk usage (refresh every 2 seconds)
watch -n 2 duf
```

## JSON Output for Scripting
```bash
# Get JSON output
duf --json

# Parse with jq - find nearly full disks
duf --json | jq '.[] | select(.usage > 90)'

# Get specific filesystem info
duf --json | jq '.[] | select(.filesystem == "/dev/sda1")'

# Calculate total disk space
duf --json | jq '[.[] | .size] | add'

# List all mount points
duf --json | jq '.[].mount_point'

# Find largest filesystem
duf --json | jq 'max_by(.size) | {mount: .mount_point, size: .size}'
```

## Comparison with df
```bash
# Traditional df command
df -h

# duf equivalent (much prettier)
duf

# df showing all filesystems
df -a

# duf equivalent
duf --all

# df showing only local filesystems
df -l

# duf equivalent
duf --only local

# df with specific format
df -h --output=source,size,used,avail,pcent,target

# duf equivalent
duf --output=filesystem,size,used,avail,usage,mount_point
```

## Practical Examples
```bash
# Check root partition usage
duf /

# Check if any disk is over 90% full
duf --json | jq '.[] | select(.usage > 90) | .mount_point'

# Monitor home directory disk
duf /home

# Get total used space across all disks
duf --json | jq '[.[] | .used] | add'

# List disks sorted by available space
duf --sort avail

# Check network-mounted filesystems
duf --only network

# Hide snap and loop devices
duf --hide-fs squashfs,tmpfs

# Export disk info to file
duf --json > disk-usage.json
```

## Monitoring and Alerting
```bash
# Simple disk space check script
#!/bin/bash
THRESHOLD=90
duf --json | jq -r ".[] | select(.usage > $THRESHOLD) |
  \"ALERT: \(.mount_point) is \(.usage)% full\""

# Get critical filesystems
duf --json | jq '.[] | select(.usage > 90) |
  {mount: .mount_point, usage: .usage, avail: .available}'

# Watch specific partition
watch -n 5 "duf /home"

# Email alert on high usage
if duf --json | jq -e '.[] | select(.usage > 95)' > /dev/null; then
  echo "Disk space critical!" | mail -s "Alert" admin@example.com
fi
```

## Configuration
duf can be configured via environment variables and flags, but has no config file. It's designed to work well out of the box.

## Common Use Cases

### Developer Machine
```bash
# Quick check of main partitions
duf / /home

# Hide system pseudo-filesystems
duf --hide-fs tmpfs,devtmpfs,squashfs
```

### Server Monitoring
```bash
# Check all disks, sort by usage
duf --sort usage

# JSON output for monitoring systems
duf --json | curl -X POST monitoring-api/disk-usage -d @-

# Only show concerning disks (>70% full)
duf --json | jq '.[] | select(.usage > 70)'
```

### CI/CD Pipelines
```bash
# Fail build if disk space too low
USAGE=$(duf --json / | jq '.[0].usage')
if [ $USAGE -gt 90 ]; then
  echo "ERROR: Disk usage at ${USAGE}%"
  exit 1
fi
```

## Output Columns
Available columns for `--output`:
- `filesystem` - Device name
- `type` - Filesystem type
- `size` - Total size
- `used` - Used space
- `avail` - Available space
- `usage` - Usage percentage
- `inodes` - Total inodes
- `inodes_used` - Used inodes
- `inodes_avail` - Available inodes
- `inodes_usage` - Inode usage percentage
- `mount_point` - Where filesystem is mounted

## Tips
- Use `--json` for scripting and automation
- Combine with `watch` for real-time monitoring
- Use `--hide-fs` to reduce noise from pseudo-filesystems
- Sort by `--sort usage` to quickly identify full disks
- Set custom thresholds for your environment

## Integration Examples

### Nagios/Icinga
```bash
#!/bin/bash
CRITICAL=90
WARNING=75

USAGE=$(duf --json / | jq '.[0].usage')

if [ $USAGE -gt $CRITICAL ]; then
  echo "CRITICAL: Disk usage ${USAGE}%"
  exit 2
elif [ $USAGE -gt $WARNING ]; then
  echo "WARNING: Disk usage ${USAGE}%"
  exit 1
else
  echo "OK: Disk usage ${USAGE}%"
  exit 0
fi
```

### Prometheus (node_exporter alternative)
```bash
# Export duf metrics
duf --json | jq -r '.[] |
  "disk_usage_percent{mount=\"\(.mount_point)\"} \(.usage)\n
   disk_size_bytes{mount=\"\(.mount_point)\"} \(.size)\n
   disk_used_bytes{mount=\"\(.mount_point)\"} \(.used)"'
```

## Agent Use
- Pre-deployment disk space validation
- Post-deployment monitoring
- Build pipeline disk checks
- Automated cleanup triggers
- Capacity planning data collection
- Infrastructure health checks

## Uninstall
```yaml
- preset: duf
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/muesli/duf
- Search: "duf disk usage", "duf vs df"

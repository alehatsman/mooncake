# dstat - System Resource Monitor

Versatile resource statistics tool combining vmstat, iostat, netstat, and ifstat functionality.

## Quick Start
```yaml
- preset: dstat
```

## Features
- **Multi-metric**: CPU, disk, network, memory, processes in one view
- **Real-time**: Continuous monitoring with customizable intervals
- **Colored output**: Easy-to-read color-coded display
- **CSV export**: Log data for analysis
- **Plugins**: Extensible with custom plugins
- **Comparison**: See resource usage side-by-side

## Basic Usage
```bash
# Default view (CPU, disk, network, paging, system)
dstat

# Update every 5 seconds
dstat 5

# Show 10 updates at 2-second intervals
dstat 2 10

# CPU and memory only
dstat -cm

# Disk and network
dstat -dn

# All metrics
dstat -a

# Most comprehensive view
dstat -cdngy

# Show specific disk
dstat --disk sda

# Show specific network interface
dstat --net eth0
```

## Advanced Monitoring
```bash
# CPU usage per core
dstat -C 0,1,2,3

# Top CPU process
dstat --top-cpu

# Top memory process
dstat --top-mem

# Top I/O process
dstat --top-io

# Top latency
dstat --top-latency

# Filesystem operations
dstat --fs

# Lock statistics
dstat --lock

# Raw numbers (no colors)
dstat --nocolor

# CSV output
dstat --output report.csv
```

## Common Option Combinations
```bash
# Web server monitoring
dstat -taf --tcp --udp

# Database server
dstat -cdlmn --disk-util --io --top-io

# General system health
dstat -cdngy --load --proc-count

# Network-intensive workload
dstat --net --tcp --udp --socket

# Disk-intensive workload
dstat --disk --io --disk-tps --disk-util
```

## Advanced Configuration
```yaml
# Install dstat
- preset: dstat

# Uninstall
- preset: dstat
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Option Reference

### System Metrics
```bash
-c, --cpu          CPU stats
-d, --disk         Disk stats
-g, --page         Paging stats
-m, --mem          Memory stats
-n, --net          Network stats
-p, --proc         Process stats
-r, --io           I/O request stats
-s, --swap         Swap stats
-y, --sys          System stats (interrupts, context switches)
```

### Specific Resources
```bash
--disk sda,sdb     Specific disks
--net eth0,wlan0   Specific interfaces
-C 0,1,2,3         Specific CPU cores
```

### Process Information
```bash
--top-cpu          Process using most CPU
--top-mem          Process using most memory
--top-io           Process doing most I/O
--top-latency      Process with highest latency
```

### Network Details
```bash
--tcp              TCP stats
--udp              UDP stats
--socket           Socket stats
--unix             Unix socket stats
```

### Advanced Stats
```bash
--fs               Filesystem stats
--ipc              IPC stats
--lock             File lock stats
--raw              Raw disk stats
--vm               VM stats
```

## Real-World Examples

### Monitor During Load Test
```bash
# Watch CPU, memory, disk, network during test
dstat -cdnm 1

# Export to CSV for analysis
dstat -cdnm 1 --output loadtest.csv &
./run-load-test.sh
pkill dstat
```

### Debug Performance Issue
```bash
# Find which process is using resources
dstat --top-cpu --top-mem --top-io 1

# Watch specific disk during backup
dstat --disk sdb -d 1
```

### Database Monitoring
```bash
# Monitor disk I/O and latency
dstat -cd --io --disk-util --top-io 5

# Watch memory and swap
dstat -ms --top-mem 2
```

### Network Traffic Analysis
```bash
# Monitor network with TCP/UDP stats
dstat -n --tcp --udp --socket 1

# Watch specific interface
dstat --net eth0 -n 1
```

## Agent Use
- Monitor system performance during automated deployments
- Collect resource metrics for capacity planning
- Identify performance bottlenecks in CI/CD pipelines
- Track resource usage trends over time with CSV exports
- Automated alerting when thresholds are exceeded
- Generate performance reports for infrastructure audits

## CSV Export for Analysis
```bash
# Log for 1 hour (update every 5 seconds)
dstat -cdnm 5 --output server-metrics.csv &
sleep 3600
pkill dstat

# Analyze with Python/pandas
python3 << 'EOF'
import pandas as pd
df = pd.read_csv('server-metrics.csv', skiprows=6)
print(df.describe())
EOF
```

## Resources
- Manual: `man dstat`
- GitHub: https://github.com/dagwieers/dstat
- Search: "dstat tutorial", "dstat monitoring examples"

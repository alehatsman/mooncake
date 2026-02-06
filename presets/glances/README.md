# Glances - Cross-Platform System Monitoring

A cross-platform curses-based system monitoring tool written in Python that provides comprehensive system resource visibility in a single interface.

## Quick Start

```yaml
- preset: glances
```

```bash
# Launch interactive monitoring
glances

# Web server mode (access via browser)
glances -w
# Then open http://localhost:61208

# Export mode (for data collection)
glances --export influxdb
```

## Features

- **Comprehensive monitoring**: CPU, memory, disk, network, processes, sensors
- **Web interface**: Built-in web server for remote monitoring
- **Multiple export formats**: JSON, CSV, InfluxDB, Prometheus, and more
- **Alerting**: Configurable thresholds with color-coded warnings
- **Extensible**: Plugin architecture for custom monitoring
- **Cross-platform**: Linux, macOS, BSD, Windows

## Basic Usage

```bash
# Start monitoring
glances

# Web server mode (access remotely)
glances -w
# Access at http://localhost:61208

# Export to file
glances --export csv --export-csv-file /tmp/glances.csv

# Client/server mode
glances -s  # Server
glances -c <server-ip>  # Client

# Show specific information
glances --help
```

## Interactive Keys

```
h: Help
q: Quit
1-5: Sort processes by CPU, MEM, NAME, I/O, TIME
a: Auto-sort processes
c: Sort by CPU
m: Sort by MEM
p: Sort by NAME
i: Sort by I/O
t: View mode (compact/wide)
f: Show/hide filesystem stats
n: Show/hide network stats
s: Show/hide sensors
d: Show/hide disk I/O
```

## Advanced Configuration

```yaml
- preset: glances
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove glances |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Config file**: `~/.config/glances/glances.conf` (Linux), `~/Library/Application Support/glances/glances.conf` (macOS)
- **Web UI port**: 61208 (default)
- **Plugins**: `~/.config/glances/plugins/`

## Real-World Examples

### Remote Monitoring Setup

```bash
# On monitored server (start as service)
glances -w -B 0.0.0.0 --password

# Access from browser
open http://server-ip:61208
```

### CI/CD Health Checks

```bash
# Check if system resources are healthy before build
glances --stdout cpu.user,mem.percent | awk -F, '{
  if ($1 > 80 || $2 > 85) {
    print "System overloaded: CPU=" $1 "% MEM=" $2 "%"
    exit 1
  }
}'
```

### Export to InfluxDB

```bash
# Continuous monitoring with InfluxDB export
glances --export influxdb \
  --influxdb-host localhost \
  --influxdb-port 8086 \
  --influxdb-db glances
```

### Performance Baseline

```bash
# Capture 60 seconds of system metrics
glances --time 60 --export csv --export-csv-file baseline.csv
```

## Agent Use

- Monitor server health in deployment pipelines
- Collect performance baselines before/after changes
- Trigger alerts when resource thresholds exceeded
- Export metrics to time-series databases
- Generate system health reports
- Identify resource bottlenecks in CI/CD

## Troubleshooting

### psutil errors on startup

Install Python psutil:
```bash
pip install psutil
```

### Web interface not accessible

Check firewall rules:
```bash
# Allow port 61208
sudo ufw allow 61208/tcp  # Linux
```

### High CPU usage

Reduce refresh rate:
```bash
glances --time 5  # Refresh every 5 seconds
```

## Uninstall

```yaml
- preset: glances
  with:
    state: absent
```

## Resources

- Official docs: https://glances.readthedocs.io/
- GitHub: https://github.com/nicolargo/glances
- Search: "glances tutorial", "glances monitoring", "glances configuration"

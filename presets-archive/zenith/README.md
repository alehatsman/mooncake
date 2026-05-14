# zenith - Cross-Platform System Monitor

A cross-platform system monitoring tool featuring CPU, GPU, network, and disk usage with interactive charts and zoomable historical data.

## Quick Start
```yaml
- preset: zenith
```

## Features
- **GPU monitoring**: NVIDIA GPU utilization and temperature tracking
- **Interactive charts**: Zoomable historical graphs for all metrics
- **Process management**: Sort and filter processes with detailed statistics
- **Network monitoring**: Per-interface bandwidth usage and totals
- **Disk I/O**: Real-time disk read/write statistics
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Launch zenith
zenith

# Disable GPU monitoring
zenith --disable-gpu

# Set custom refresh rate (in milliseconds)
zenith --refresh-rate 2000

# Display only CPU usage
zenith --cpu-only

# Show help
zenith --help

# Show version
zenith --version
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` | Quit |
| `<Esc>` | Quit |
| `↑` / `↓` | Scroll process list |
| `<PgUp>` / `<PgDn>` | Scroll process list (page) |
| `Home` / `End` | Jump to start/end of process list |
| `+` / `-` | Zoom in/out historical charts |
| `Space` | Pause/resume updates |

## Advanced Configuration
```yaml
# Basic installation
- preset: zenith
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, cargo)
- ✅ macOS (Homebrew, cargo)
- ✅ Windows (cargo, binaries)

## Configuration
- **Binary location**: `/usr/local/bin/zenith` (cargo), `/usr/bin/zenith` (package manager)
- **No config files**: zenith is configured via command-line flags only

## Real-World Examples

### Development Monitoring
```bash
# Monitor resource usage during development
zenith --refresh-rate 1000  # Fast updates for real-time feedback
```

### GPU-Accelerated Workloads
```bash
# Track NVIDIA GPU usage during ML training
zenith  # Shows GPU utilization, temperature, memory
```

### Server Monitoring
```bash
# Low-overhead monitoring on production server
zenith --refresh-rate 5000 --disable-gpu
```

### Process Debugging
```bash
# Identify resource-intensive processes
# Launch zenith, use arrow keys to navigate process list
# Processes sorted by CPU usage by default
```

## Agent Use
- Monitor system resources during automated builds
- Track GPU utilization in ML/AI pipelines
- Verify system capacity before resource-intensive operations
- Debug performance issues in CI/CD environments
- Collect historical resource usage data for analysis

## Troubleshooting

### GPU monitoring not working
Ensure NVIDIA drivers and nvidia-smi are installed:
```bash
# Check NVIDIA drivers
nvidia-smi

# If not available, install drivers
# Ubuntu/Debian
sudo apt install nvidia-driver-XXX

# Fedora
sudo dnf install akmod-nvidia
```

### zenith command not found (cargo install)
Add cargo bin to PATH:
```bash
export PATH="$HOME/.cargo/bin:$PATH"
```

### High CPU usage
Increase refresh interval:
```bash
zenith --refresh-rate 3000  # Update every 3 seconds
```

### Permission errors on Linux
Some metrics may require elevated privileges:
```bash
sudo zenith  # For full system access
```

## Uninstall
```yaml
- preset: zenith
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/bvaisvil/zenith
- Search: "zenith system monitor", "zenith vs btop", "zenith GPU monitoring"

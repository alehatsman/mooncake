# ytop - TUI System Monitor

A TUI (text-based user interface) system monitor written in Rust showing CPU, memory, network, and disk usage with beautiful visualizations.

## Quick Start
```yaml
- preset: ytop
```

## Features
- **Colorful visualizations**: CPU, memory, network, and disk graphs with color coding
- **Real-time monitoring**: Live updates of system resource usage
- **Lightweight**: Written in Rust for minimal system overhead
- **Minimal configuration**: Works out-of-the-box with sensible defaults
- **Cross-platform**: Linux, macOS

## Basic Usage
```bash
# Launch ytop
ytop

# Display CPU usage as percentage (instead of graph)
ytop -p

# Change update interval (default: 1000ms)
ytop -r 500

# Display network speed in bits
ytop -b

# Show version
ytop --version

# Show all options
ytop --help
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` | Quit |
| `<Ctrl+c>` | Quit |
| `<Up>/<Down>` | Scroll process list |
| `<PgUp>/<PgDn>` | Scroll faster |
| `d` | Kill selected process (sends SIGTERM) |
| `k` | Kill selected process (sends SIGKILL) |

## Advanced Configuration
```yaml
# Basic installation
- preset: ytop
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, cargo)
- ✅ macOS (Homebrew, cargo)
- ❌ Windows (not supported)

## Configuration
- **Binary location**: `/usr/local/bin/ytop` (cargo), `/usr/bin/ytop` (package manager)
- **No config files**: ytop is configured via command-line flags only

## Real-World Examples

### CI/CD Health Check
```bash
# Quick system resource check before build
ytop -p | head -10
```

### Development Environment Monitoring
```bash
# Monitor resources while running tests
ytop -r 2000  # Update every 2 seconds
```

### Production Server Monitoring
```bash
# Launch with minimal updates to reduce overhead
ytop -r 5000  # Update every 5 seconds
```

## Agent Use
- Quick visual system health check before deployment
- Monitor resource usage during CI/CD builds
- Verify system capacity before intensive operations
- Debug performance issues in development
- Lightweight alternative to htop/top for quick checks

## Troubleshooting

### ytop not found after installation
Ensure cargo bin directory is in PATH:
```bash
export PATH="$HOME/.cargo/bin:$PATH"
```

### Color display issues
Try running in a terminal with 256-color support (iTerm2, Alacritty, etc.).

### High CPU usage
Increase update interval:
```bash
ytop -r 2000  # Update less frequently
```

## Uninstall
```yaml
- preset: ytop
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/cjbassi/ytop
- Search: "ytop rust", "ytop system monitor", "ytop vs btop"

**Note**: ytop is no longer actively maintained. Consider using **btop** or **bottom** for actively maintained alternatives.

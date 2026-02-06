# btop - Resource Monitor

Modern, beautiful resource monitor showing usage and stats for CPU, memory, disks, network, and processes.

## Quick Start
```yaml
- preset: btop
```

## Features
- **Beautiful TUI**: Mouse support, customizable colors and themes
- **Comprehensive monitoring**: CPU, memory, disks, network, processes
- **Low overhead**: Efficient C++ implementation
- **Flexible**: Multiple layout modes and sorting options
- **Cross-platform**: Linux, macOS, FreeBSD

## Basic Usage
```bash
# Launch btop
btop

# Launch with specific update interval (ms)
btop --update 1000

# Show version
btop --version

# Launch in TTY mode (no mouse)
btop --tty_on

# Launch in low-color mode
btop --low-color
```

## Keyboard Shortcuts

### General
| Key | Action |
|-----|--------|
| `q` | Quit |
| `Esc` | Exit menus/filters |
| `M` | Show menu |
| `+` | Increase update speed |
| `-` | Decrease update speed |
| `h` | Toggle help |

### CPU Box
| Key | Action |
|-----|--------|
| `c` | Focus/show CPU |
| `1-9` | Show specific CPU core |
| `0` | Show all cores |

### Memory Box
| Key | Action |
|-----|--------|
| `m` | Focus/show memory |

### Network Box
| Key | Action |
|-----|--------|
| `n` | Focus/show network |
| `b` | Toggle bytes/bits |
| `a` | Toggle auto-scaling |
| `y` | Toggle sync scaling |
| `z` | Reset peak values |

### Process Box
| Key | Action |
|-----|--------|
| `p` | Focus/show processes |
| `t` | Tree view on/off |
| `r` | Reverse sorting order |
| `f` | Filter processes |
| `k` | Kill selected process |
| `s` | Select signal (TERM, KILL, etc) |
| `e` | Show process command line |
| `Enter` | Show process details |

### Sorting
| Key | Action |
|-----|--------|
| `P` | Sort by PID |
| `N` | Sort by program name |
| `C` | Sort by CPU usage |
| `M` | Sort by memory usage |

## Mouse Support
- **Click boxes**: Switch focus between CPU, memory, network, processes
- **Scroll**: Navigate through process list
- **Click column headers**: Sort by that column
- **Right-click process**: Show process menu (kill, signal, etc.)

## Configuration
```bash
# Config location
~/.config/btop/btop.conf              # Linux/BSD
~/Library/Application Support/btop/   # macOS

# Color themes
~/.config/btop/themes/                # Custom themes directory
```

## Customization

### Themes
btop includes several built-in themes:
- **Default**
- **Default-light**
- **TTY**
- **Gruvbox Dark**
- **Nord**
- **Dracula**
- **Monokai**

Change theme in menu (`M` key) or edit config:
```conf
color_theme = "nord"
```

### Config Options
Key settings in `~/.config/btop/btop.conf`:
```conf
# Update interval in milliseconds
update_ms = 2000

# Show temperatures (requires lm-sensors on Linux)
show_temps = True

# Temperature scale (celsius, fahrenheit, kelvin)
temp_scale = "celsius"

# Process tree view
proc_tree = False

# Show process command line
proc_full_cmd = False

# Filter processes
proc_filter = ""

# Graph symbols (braille, block, tty)
graph_symbol = "braille"

# Network interface to monitor (auto, eth0, wlan0, etc)
net_iface = "auto"

# Show disks as io activity or space used
disks_filter = ""
```

## Performance Tips
```bash
# Reduce update frequency for lower CPU usage
btop --update 5000

# Use TTY mode for even lower overhead
btop --tty_on

# Monitor specific process
btop # then press 'f' and type process name
```

## Comparison with htop
| Feature | btop | htop |
|---------|------|------|
| UI Design | Modern, colorful | Classic, functional |
| Mouse Support | Full | Limited |
| Network Stats | Yes | No |
| Disk Stats | Yes | No |
| Themes | Multiple built-in | Basic |
| Performance | Very efficient | Efficient |

## Monitoring Examples
```bash
# Monitor system during load test
btop  # Watch CPU, memory, processes in real-time

# Check which process is using most memory
# Launch btop, press 'p' to focus processes, press 'M' to sort by memory

# Monitor network activity
# Launch btop, press 'n' to focus network, watch interface traffic

# Kill memory-hogging process
# Launch btop, navigate to process, press 'k', confirm

# Check disk I/O
# Launch btop, scroll to disk section, watch read/write activity
```

## Troubleshooting

### No temperature sensors
```bash
# Linux: Install lm-sensors
sudo apt install lm-sensors      # Debian/Ubuntu
sudo dnf install lm_sensors       # Fedora
sudo pacman -S lm_sensors         # Arch

# Detect sensors
sudo sensors-detect

# Test sensors
sensors
```

### High CPU usage
- Increase update interval: Press `+` repeatedly or set `update_ms = 5000` in config
- Disable mouse: `btop --tty_on`
- Use simpler graph symbols: Set `graph_symbol = "block"` in config

### Missing network interface
- Edit config: `net_iface = "eth0"` (or your interface name)
- Or press `M` in btop, navigate to Settings, change network interface

## Agent Use
- System health monitoring during deployments
- Performance baseline collection
- Resource usage validation
- Troubleshooting high load situations
- Server capacity planning

## Uninstall
```yaml
- preset: btop
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/aristocratos/btop
- Themes: https://github.com/aristocratos/btop#themes
- Search: "btop tutorial", "btop vs htop"

# bottom - System Monitor

Graphical process and system monitor with customizable widgets for CPU, memory, network, and process viewing.

## Quick Start
```yaml
- preset: bottom
```

## Features
- **Customizable layout**: Configure widget arrangement
- **Multiple data sources**: CPU, memory, disk, network, processes, temperatures
- **Graph visualization**: Time-series graphs for metrics
- **Process management**: Search, sort, and kill processes
- **Low overhead**: Efficient resource usage
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Launch bottom
btm

# Launch with default config
btm -C ~/.config/bottom/bottom.toml

# Use basic mode (simpler layout)
btm -b

# Autohide time axis
btm -t

# Hide legend
btm --hide_table_gap

# Set refresh rate (ms)
btm -r 1000

# Enable battery widget
btm --battery
```

## Keyboard Shortcuts

### General
| Key | Action |
|-----|--------|
| `q`, `Ctrl+c` | Quit |
| `Esc` | Close dialog/search |
| `?` | Help menu |
| `/` | Search processes |

### Navigation
| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate list |
| `←`/`→` | Switch between widgets |
| `H`/`J`/`K`/`L` | Vim-style navigation |
| `gg` | Jump to top |
| `G` | Jump to bottom |

### Process Management
| Key | Action |
|-----|--------|
| `dd` | Kill selected process |
| `c` | Sort by CPU |
| `m` | Sort by memory |
| `p` | Sort by PID |
| `n` | Sort by name |
| `Tab` | Group processes |
| `Ctrl+f` | Full command |

### Zooming
| Key | Action |
|-----|--------|
| `+`/`-` | Zoom in/out time range |
| `=` | Reset zoom |

## Configuration

### Config File Location
- **Linux**: `~/.config/bottom/bottom.toml`
- **macOS**: `~/Library/Application Support/bottom/bottom.toml`
- **Windows**: `%APPDATA%\bottom\bottom.toml`

### Example Configuration
```toml
[flags]
# Refresh rate in milliseconds
rate = 1000

# Use basic mode
basic = false

# Hide time axis
hide_time = false

# Show battery widget
battery = true

# Temperature type (celsius, fahrenheit, kelvin)
temperature_type = "celsius"

[colors]
# Color scheme (default, gruvbox, nord, dracula)
color = "default"

# Custom colors
table_header_color = "LightBlue"
selected_text_color = "Black"
selected_bg_color = "LightBlue"

[disk]
# Disk name filter
name_filter = ["nvme0n1", "sda"]

# Mount point filter
mount_filter = ["/", "/home"]

[processes]
# Default sort column
default_widget_type = "proc"
default_widget_count = 1

# Group processes
grouped = true
```

## Advanced Configuration

```yaml
# Install bottom
- preset: bottom

# Create custom configuration
- name: Configure bottom
  template:
    dest: ~/.config/bottom/bottom.toml
    content: |
      [flags]
      rate = 500
      battery = true
      temperature_type = "celsius"

      [colors]
      color = "gruvbox"

      [processes]
      grouped = true

# Verify installation
- name: Check bottom version
  shell: btm --version
  register: version

- name: Display version
  print: "bottom version {{ version.stdout }}"
```

## Color Schemes

### Built-in Themes
```bash
# Default theme
btm

# Gruvbox
btm --color gruvbox

# Nord
btm --color nord

# Dracula (via config)
# Add to bottom.toml: color = "dracula"
```

### Custom Colors
```toml
# bottom.toml
[colors]
table_header_color = "LightBlue"
all_cpu_color = "LightMagenta"
avg_cpu_color = "Red"
cpu_core_colors = ["LightMagenta", "LightYellow", "LightCyan", "LightGreen"]
ram_color = "LightBlue"
swap_color = "LightYellow"
rx_color = "LightCyan"
tx_color = "LightGreen"
widget_title_color = "Gray"
border_color = "Gray"
highlighted_border_color = "LightBlue"
text_color = "Gray"
selected_text_color = "Black"
selected_bg_color = "LightBlue"
```

## Widget Configuration

```toml
[row]
  [[row.child]]
  type = "cpu"

  [[row.child]]
  type = "mem"

[[row]]
  [[row.child]]
  type = "net"

  [[row.child]]
  type = "proc"
  default = true

[[row]]
  [[row.child]]
  type = "disk"
```

## Process Filtering

```bash
# Filter processes by name
btm --regex ".*nginx.*"

# Case-insensitive search
btm --case_sensitive false

# Whole word matching
btm --whole_word

# Show tree view
btm --tree
```

## Real-World Examples

### Monitor Server Performance
```bash
# Full-featured monitoring
btm --battery --tree --regex ".*python.*"

# Focus on specific processes
btm --regex ".*docker.*|.*nginx.*"
```

### CI/CD Resource Monitoring
```yaml
# Monitor resource usage during build
- preset: bottom

- name: Start bottom in background
  shell: btm --basic -r 500 > /tmp/bottom.log 2>&1 &
  register: bottom_pid

- name: Run resource-intensive task
  shell: ./build.sh

- name: Stop bottom
  shell: kill {{ bottom_pid.stdout }}

- name: Analyze resource usage
  shell: cat /tmp/bottom.log
```

### Docker Container Monitoring
```bash
# Monitor Docker processes
btm --regex ".*docker.*|.*containerd.*"

# View container resource usage
btm --tree --grouped
```

### Development Debugging
```bash
# Monitor application during development
btm --regex ".*myapp.*" --tree

# Watch for memory leaks
btm -r 500  # Faster refresh

# Sort by memory to find leaks
# Press 'm' to sort by memory usage
```

## Comparison with Other Tools

| Feature | bottom | htop | btop |
|---------|--------|------|------|
| Graphs | ✅ | ❌ | ✅ |
| Customizable | ✅ | Limited | ✅ |
| Mouse support | ✅ | ✅ | ✅ |
| Battery | ✅ | ❌ | ✅ |
| Config file | ✅ | ❌ | ✅ |
| Cross-platform | ✅ | Limited | ✅ |

## Troubleshooting

### High CPU Usage
```bash
# Increase refresh rate (less frequent updates)
btm -r 2000  # 2 seconds

# Use basic mode
btm -b
```

### Missing Temperature Sensors
```bash
# Linux: Install lm-sensors
sudo apt install lm-sensors
sudo sensors-detect

# Check sensors work
sensors
```

### Widget Not Showing
```bash
# Enable specific widgets
btm --battery  # Show battery widget

# Check config file
cat ~/.config/bottom/bottom.toml

# Reset to defaults
rm ~/.config/bottom/bottom.toml
btm
```

### Permission Issues
```bash
# Some features need elevated permissions
sudo btm

# Or grant capabilities (Linux)
sudo setcap cap_sys_ptrace+ep $(which btm)
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, cargo, Homebrew)
- ✅ macOS (Homebrew, cargo, MacPorts)
- ✅ Windows (Chocolatey, Scoop, cargo, winget)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Monitor system resources during deployments
- Track process resource usage in CI/CD
- Identify performance bottlenecks
- Debug memory leaks in development
- Validate resource limits in containers
- Monitor server health remotely
- Troubleshoot high load situations

## Uninstall
```yaml
- preset: bottom
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/ClementTsang/bottom
- Documentation: https://clementtsang.github.io/bottom/
- Config reference: https://clementtsang.github.io/bottom/stable/configuration/
- Search: "bottom system monitor", "bottom vs htop", "bottom configuration"

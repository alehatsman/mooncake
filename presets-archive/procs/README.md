# procs - Modern Process Viewer

Modern replacement for ps written in Rust with colored output, additional information, and powerful filtering.

## Quick Start

```yaml
- preset: procs
```

## Features

- **Colored Output**: Human-readable with syntax highlighting
- **Additional Info**: TCP/UDP ports, read/write throughput, Docker container names
- **Multi-Column Search**: Search across multiple fields simultaneously
- **Tree View**: Display process hierarchies
- **Watch Mode**: Auto-update like top
- **Pager Support**: Automatic paging for long outputs

## Basic Usage

```bash
# List all processes
procs

# Search by name
procs firefox

# Show tree view
procs --tree

# Watch mode (auto-update every 1s)
procs --watch

# Show only specific columns
procs --only pid,user,cpu,mem,command

# Sort by CPU
procs --sortd cpu

# Filter by user
procs --or user root
```

## Advanced Configuration

```yaml
# Basic installation
- preset: procs
  with:
    state: present

# Uninstall
- preset: procs
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, pacman, cargo, snap)
- ✅ macOS (Homebrew, cargo)
- ✅ Windows (cargo, scoop)

## Configuration

- **Config file**: `~/.config/procs/config.toml`
- **Default columns**: PID, User, CPU, Memory, Time, Command
- **Color scheme**: Customizable via config

Example config:
```toml
[[columns]]
kind = "Pid"
style = "BrightYellow"
numeric_search = true
nonnumeric_search = false

[[columns]]
kind = "User"
style = "BrightGreen"
```

## Real-World Examples

### Find Memory Hogs

```bash
# Top 10 processes by memory
procs --sortd mem | head -10

# Processes using more than 1GB
procs --or 'mem > 1000000'
```

### Monitor Specific Service

```bash
# Watch nginx processes
procs --watch nginx

# Monitor Docker containers
procs --tree docker
```

### CI/CD Monitoring

```yaml
- name: Install procs
  preset: procs

- name: Check for runaway processes
  shell: |
    # Find processes using >90% CPU
    if procs --or 'cpu > 90' | grep -v PID; then
      echo "High CPU usage detected"
      exit 1
    fi
  register: cpu_check
```

### Development Workflow

```bash
# Find processes on specific port
procs --or 'tcp.*:8080'

# Kill all processes matching pattern
procs myapp --no-header | awk '{print $1}' | xargs kill

# Monitor build processes
procs --watch --tree make
```

## Common Use Cases

```bash
# Show all Python processes
procs python

# Show processes with open network connections
procs --or 'tcp|udp'

# Show Docker-related processes
procs docker

# Show processes using specific file
procs --or '/var/log'

# Show zombie processes
procs --or 'state Z'

# Custom columns for debugging
procs --only pid,ppid,state,cpu,mem,time,command nginx
```

## vs Traditional ps

```bash
# procs advantages over ps:
procs firefox               # vs ps aux | grep firefox
procs --tree               # vs ps auxf
procs --sortd cpu          # vs ps aux --sort=-pcpu
procs --watch              # vs watch ps aux
procs --or 'tcp.*:8080'    # vs lsof -i :8080 + ps
```

## Agent Use

- Monitor process resource usage in CI/CD
- Detect resource leaks and runaway processes
- Automated process health checks
- Identify processes on specific ports
- Track Docker container processes
- Debugging production issues with enhanced visibility
- Generate process reports for auditing

## Troubleshooting

### Command not found

Ensure procs is in PATH:
```bash
which procs
procs --version
```

### Colors not showing

Check terminal support:
```bash
echo $TERM
```

Force color output:
```bash
procs --color always
```

### Permission denied for some processes

Run with sudo to see all processes:
```bash
sudo procs
```

## Uninstall

```yaml
- preset: procs
  with:
    state: absent
```

## Resources

- Official docs: https://github.com/dalance/procs
- Crates.io: https://crates.io/crates/procs
- Search: "procs rust process viewer", "procs vs ps", "modern ps replacement"

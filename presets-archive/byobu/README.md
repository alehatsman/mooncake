# Byobu - Terminal Multiplexer Wrapper

Enhanced wrapper for tmux and screen with user-friendly keybindings, status notifications, and improved configuration.

## Quick Start
```yaml
- preset: byobu
```

## Features
- **User-friendly**: Easier keybindings than tmux/screen
- **Enhanced status bar**: System stats, notifications, customizable widgets
- **Session management**: Persistent sessions survive disconnects
- **Multi-backend**: Works with both tmux and screen
- **Zero configuration**: Works out of the box with sensible defaults
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# Start a new session
byobu

# Attach to existing session
byobu attach

# List sessions
byobu list-sessions

# Create named session
byobu new -s myproject

# Detach from session (inside byobu)
# Press F6 or Ctrl-D

# Kill a session
byobu kill-session -t myproject
```

## Key Bindings

### Function Keys (Default)
| Key | Action |
|-----|--------|
| `F1` | Show help menu |
| `F2` | Create new window |
| `F3` | Move to previous window |
| `F4` | Move to next window |
| `F5` | Reload profile |
| `F6` | Detach session |
| `F7` | Enter scrollback/search mode |
| `F8` | Rename current window |
| `F9` | Open byobu configuration |
| `F11` | Zoom pane (fullscreen) |
| `F12` | Toggle mouse support |

### Window Management
| Key | Action |
|-----|--------|
| `Shift-F2` | Split window horizontally |
| `Ctrl-F2` | Split window vertically |
| `Shift-F3` | Move focus to previous pane |
| `Shift-F4` | Move focus to next pane |
| `Shift-F11` | Zoom out (exit fullscreen) |
| `Ctrl-Shift-F3` | Move pane left |
| `Ctrl-Shift-F4` | Move pane right |

### Session Management
| Key | Action |
|-----|--------|
| `Alt-PgUp` | Scroll up |
| `Alt-PgDn` | Scroll down |
| `Shift-F6` | Detach but keep session running |
| `Alt-F6` | Detach all clients except current |

## Configuration

### Config Directories
```bash
# System config
/usr/share/byobu/

# User config
~/.byobu/

# Backend selection
~/.byobu/backend    # "tmux" or "screen"
```

### Status Bar Configuration
```bash
# Enable/disable status widgets
byobu-config

# Or edit manually
vim ~/.byobu/status

# Available status items
~/.byobu/statusrc
```

### Custom Status Widgets
```bash
# Enable custom widgets
cd ~/.byobu/
ls status*

# Common widgets
arch          # CPU architecture
battery       # Battery status
cpu_count     # Number of CPUs
cpu_freq      # CPU frequency
cpu_temp      # CPU temperature
disk          # Disk usage
fan_speed     # Fan speed
hostname      # System hostname
ip_address    # IP address
load_average  # System load
logo          # Distribution logo
memory        # Memory usage
network       # Network traffic
raid          # RAID status
reboot_required  # System reboot indicator
time_utc      # UTC time
uptime        # System uptime
whoami        # Current user
wifi_quality  # WiFi signal strength
```

Enable widgets:
```bash
# Edit status configuration
byobu-enable battery
byobu-enable cpu_temp
byobu-enable network
byobu-disable logo
```

## Backend Selection

### Choose Backend
```bash
# Select backend (tmux or screen)
byobu-select-backend

# Set tmux as backend
echo "tmux" > ~/.byobu/backend

# Set screen as backend
echo "screen" > ~/.byobu/backend
```

### Tmux (Recommended)
- Modern, actively developed
- Better pane management
- More features

### Screen
- More widely available
- Lower resource usage
- Simpler

## Advanced Configuration

### Custom Key Bindings
```bash
# ~/.byobu/keybindings.tmux (if using tmux backend)
# Unbind F2, bind Ctrl-T for new window
unbind-key -n F2
bind-key -n C-t new-window

# Custom pane split
bind-key -n C-\ split-window -h
bind-key -n C-_ split-window -v
```

### Color Schemes
```bash
# Enable 256 color support
byobu-config

# Or manually
echo "set -g default-terminal \"screen-256color\"" >> ~/.byobu/.tmux.conf
```

### Profile Scripts
```bash
# ~/.byobu/profile.tmux
# Run commands on session start
set-option -g status-right-length 100
set-option -g status-left-length 50
```

## Real-World Examples

### Remote Server Management
```yaml
- name: Install Byobu
  preset: byobu
  become: true

- name: Set default shell to byobu
  shell: byobu-enable
  become: true

- name: Create persistent monitoring session
  shell: |
    byobu new-session -d -s monitoring
    byobu send-keys -t monitoring "htop" C-m
    byobu split-window -t monitoring -v
    byobu send-keys -t monitoring "tail -f /var/log/syslog" C-m
```

### Development Workflow
```bash
# Create development session with multiple panes
byobu new -s dev

# Inside byobu, split into 3 panes:
# Press Shift-F2 (horizontal split)
# Press Ctrl-F2 (vertical split)

# Pane 1: Editor
vim src/main.go

# Pane 2: Build watcher
while true; do make build; sleep 1; done

# Pane 3: Application
./myapp

# Detach: F6
# Later: byobu attach -t dev
```

### CI/CD Agent with Byobu
```yaml
- name: Install Byobu
  preset: byobu
  become: true

- name: Create build session
  shell: |
    byobu new-session -d -s ci-builds
    byobu send-keys -t ci-builds "cd /app && ./watch-builds.sh" C-m

- name: Configure byobu for headless
  shell: byobu-enable-pr-userinitd
  become: true
```

### SSH Connection Manager
```bash
# Create session with SSH connections
byobu new -s servers

# Split into multiple panes and connect
# In each pane, SSH to different servers
ssh user@server1
ssh user@server2
ssh user@server3

# Synchronize panes (type in all at once)
# Ctrl-F9 -> select "Synchronize panes"
```

## Comparison with tmux/screen

| Feature | Byobu | tmux | screen |
|---------|-------|------|--------|
| Setup | Zero config | Requires config | Requires config |
| Status bar | Rich, customizable | Basic | Basic |
| Keybindings | Function keys | Ctrl-B prefix | Ctrl-A prefix |
| Learning curve | Easy | Medium | Medium |
| Mouse support | Built-in | Requires config | Limited |
| Notifications | Yes | No | No |

## Troubleshooting

### Function keys not working
```bash
# Check terminal type
echo $TERM

# Enable function keys in terminal
# For PuTTY: Terminal -> Keyboard -> Function keys
# For iTerm2: Preferences -> Keys -> Load Preset

# Alternative: Use Ctrl-A prefix mode
byobu-ctrl-a
```

### Status bar not showing
```bash
# Reload configuration
byobu-reload

# Check status items
byobu-config

# Verify backend
cat ~/.byobu/backend

# Reset to defaults
rm -rf ~/.byobu/
byobu
```

### Session not persisting
```bash
# Check backend is running
ps aux | grep tmux

# Ensure proper detach (F6, not exit)
# Verify session exists
byobu list-sessions

# Reconnect
byobu attach
```

### High CPU usage
```bash
# Disable expensive status widgets
byobu-disable network
byobu-disable cpu_temp
byobu-disable battery

# Increase refresh interval
# In ~/.byobu/statusrc
BYOBU_REFRESH_RATE=5  # Default is 5 seconds
```

## Auto-start on Login

```bash
# Enable byobu on login
byobu-enable

# Disable auto-start
byobu-disable

# Check status
byobu-status
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (WSL only)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Maintain persistent SSH sessions on remote servers
- Monitor multiple services simultaneously
- Create development environments with multiple panes
- Manage long-running processes that survive disconnects
- Run commands on multiple servers simultaneously
- Provide terminal multiplexing for headless systems

## Uninstall
```yaml
- preset: byobu
  with:
    state: absent
```

## Resources
- Official site: https://www.byobu.org
- Documentation: https://www.byobu.org/documentation
- GitHub: https://github.com/dustinkirkland/byobu
- Ubuntu wiki: https://help.ubuntu.com/community/Byobu
- Search: "byobu tutorial", "byobu vs tmux", "byobu keybindings"

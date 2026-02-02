# Tmux Preset

Install and configure tmux (terminal multiplexer) with sensible defaults for productivity.

## Features

- ✅ Installs tmux via system package manager
- ✅ Optional sensible configuration
- ✅ Customizable prefix key (default: C-a)
- ✅ Mouse support enabled by default
- ✅ Vi key bindings in copy mode
- ✅ Better pane split keys (| and -)
- ✅ Vim-like pane navigation (hjkl)
- ✅ Modern status bar with colors
- ✅ Cross-platform (Linux, macOS)

## Usage

### Install tmux with sensible defaults
```yaml
- name: Install tmux
  preset: tmux
```

### Install without configuration
```yaml
- name: Install tmux (no config)
  preset: tmux
  with:
    configure: false
```

### Customize configuration
```yaml
- name: Install tmux with custom settings
  preset: tmux
  with:
    prefix_key: "C-b"      # Use default prefix
    mouse_mode: false      # Disable mouse
    vi_mode: true          # Enable vi bindings
```

### Uninstall
```yaml
- name: Remove tmux
  preset: tmux
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `configure` | bool | `true` | Install configuration file |
| `config_path` | string | `~/.tmux.conf` | Path to config file |
| `prefix_key` | string | `C-a` | Tmux prefix key |
| `mouse_mode` | bool | `true` | Enable mouse support |
| `vi_mode` | bool | `true` | Use vi key bindings |

## What is Tmux?

Tmux is a terminal multiplexer that lets you:
- Run multiple terminal sessions in one window
- Split your terminal into panes
- Detach and reattach to sessions
- Keep programs running when you disconnect
- Share sessions with others

Perfect for:
- Remote server work (SSH sessions)
- Development workflows
- System administration
- Pair programming

## Configuration Features

The generated `~/.tmux.conf` includes:

### Key Bindings
- **Prefix**: `C-a` (Ctrl+a) instead of default `C-b`
- **Split panes**: `C-a |` (vertical), `C-a -` (horizontal)
- **Navigate panes**: `C-a h/j/k/l` (Vim-style)
- **Resize panes**: `C-a H/J/K/L`
- **New window**: `C-a c`
- **Switch windows**: `C-a 0-9`
- **Reload config**: `C-a r`

### Features
- ✅ Mouse support (scroll, select, resize)
- ✅ Vi key bindings for copy mode
- ✅ 256 color support
- ✅ 50,000 line scrollback buffer
- ✅ Window numbering starts at 1
- ✅ Auto-renumber windows
- ✅ Fast escape time (no delay)
- ✅ Status bar with date/time
- ✅ Colored pane borders

## Quick Start Guide

### Basic Usage
```bash
# Start tmux
tmux

# Create new window
C-a c

# Split pane vertically
C-a |

# Split pane horizontally
C-a -

# Navigate between panes
C-a h  # left
C-a j  # down
C-a k  # up
C-a l  # right

# Detach from session
C-a d

# List sessions
tmux ls

# Attach to session
tmux attach

# Attach to specific session
tmux attach -t mysession
```

### Session Management
```bash
# Create named session
tmux new -s development

# Create session with window name
tmux new -s work -n editor

# Kill session
tmux kill-session -t development

# Rename session
C-a $

# Switch between sessions
C-a (  # previous session
C-a )  # next session
C-a s  # list sessions
```

### Window Management
```bash
# Create new window
C-a c

# Rename window
C-a ,

# Close window
C-a &

# Switch to window
C-a 0-9  # by number
C-a n    # next window
C-a p    # previous window
C-a l    # last window

# Move window
C-a .

# Find window
C-a f
```

### Pane Management
```bash
# Split panes
C-a |  # vertical split
C-a -  # horizontal split

# Navigate panes
C-a h/j/k/l  # vim-style

# Resize panes
C-a H/J/K/L  # vim-style (hold and repeat)

# Toggle pane zoom
C-a z

# Close pane
C-a x

# Show pane numbers
C-a q

# Swap panes
C-a {  # swap with previous
C-a }  # swap with next
```

### Copy Mode (Vi-style)
```bash
# Enter copy mode
C-a [

# Navigate (vi keys)
h/j/k/l  # move cursor
w/b      # word forward/back
gg/G     # top/bottom

# Select and copy
v        # start selection
y        # yank (copy)

# Paste
C-a ]

# Search
/        # search forward
?        # search backward
n        # next match
N        # previous match
```

## Common Workflows

### Remote Development
```bash
# On remote server
tmux new -s dev

# ... work in tmux ...

# Detach when done
C-a d

# Later, reconnect and resume
ssh server
tmux attach -t dev
```

### Multi-pane Development
```bash
# Start session
tmux new -s project

# Split into 3 panes:
# - Editor (top)
# - Server (bottom-left)
# - Shell (bottom-right)

C-a -      # Horizontal split
C-a j      # Move to bottom
C-a |      # Vertical split
```

### Pair Programming
```bash
# Person 1: Start shared session
tmux new -s pair

# Person 2: Attach to same session
tmux attach -t pair

# Both see and control same terminal
```

## Platform Support

- ✅ Linux (all distributions)
- ✅ macOS
- ❌ Windows (use WSL)

## Learn More

- [Tmux GitHub](https://github.com/tmux/tmux)
- [Tmux Cheat Sheet](https://tmuxcheatsheet.com/)
- [Tmux Book](https://pragprog.com/titles/bhtmux2/tmux-2/)
- [Oh My Tmux](https://github.com/gpakosz/.tmux) - Advanced config

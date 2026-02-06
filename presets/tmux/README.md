# Tmux - Terminal Multiplexer

Powerful terminal multiplexer for managing multiple terminal sessions. Split panes, create windows, detach/reattach sessions, and survive SSH disconnections.

## Quick Start

```yaml
- preset: tmux
```

## Features

- **Session Management**: Create, detach, and reattach terminal sessions
- **Split Panes**: Vertical and horizontal pane splitting
- **Multiple Windows**: Organize work across multiple windows
- **Plugin System**: TPM (Tmux Plugin Manager) with tmux-resurrect, tmux-continuum
- **Customizable**: Extensive configuration options
- **Persistent Sessions**: Sessions survive disconnections
- **Vi/Emacs Modes**: Copy mode with familiar keybindings
- **Cross-platform**: Linux and macOS support

## Basic Usage

```bash
# Sessions
tmux                    # Start new session
tmux new -s dev         # Start named session
tmux ls                 # List sessions
tmux attach             # Attach to last session
tmux attach -t dev      # Attach to named session
tmux kill-session -t dev # Kill session

# Detach from session
Ctrl-b d

# Inside tmux - Windows
Ctrl-b c         # Create new window
Ctrl-b n         # Next window
Ctrl-b p         # Previous window
Ctrl-b 0-9       # Switch to window number
Ctrl-b w         # List windows
Ctrl-b ,         # Rename window
Ctrl-b &         # Kill window

# Panes
Ctrl-b |         # Split horizontally
Ctrl-b -         # Split vertically
Ctrl-b arrow     # Navigate panes
Alt-arrow        # Navigate without prefix
Ctrl-b x         # Kill pane
Ctrl-b z         # Toggle pane zoom
Ctrl-b Space     # Cycle layouts
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tmux |
| install_tpm | bool | true | Install Tmux Plugin Manager |
| prefix_key | string | C-b | Prefix key (C-b or C-a) |
| mouse_mode | bool | true | Enable mouse support |
| history_limit | string | 10000 | Scrollback history lines |

## Advanced Configuration

### Basic Installation
```yaml
- preset: tmux
```

### Custom Prefix (like GNU Screen)
```yaml
- preset: tmux
  with:
    prefix_key: "C-a"
```

### Minimal Setup
```yaml
- preset: tmux
  with:
    install_tpm: false
    mouse_mode: false
```

### Full Configuration
```yaml
- preset: tmux
  with:
    prefix_key: "C-a"
    mouse_mode: true
    history_limit: "50000"
    install_tpm: true
```

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper, source)
- ✅ macOS (Homebrew, MacPorts, source)
- ✅ BSD (pkg, ports)

## Configuration

- **Config file**: `~/.tmux.conf`
- **Plugins**: `~/.tmux/plugins/` (with TPM)
- **Default prefix**: `Ctrl-b`
- **Sessions persist**: After disconnection

## Real-World Examples

### Development Environment
```bash
# Create project session
tmux new -s webapp

# Window 1: Editor
vim

# New window for server
Ctrl-b c
npm run dev

# New window for logs
Ctrl-b c
tail -f logs/app.log

# Split pane for database
Ctrl-b |
psql myapp_dev

# Detach and continue later
Ctrl-b d

# Reattach anytime
tmux attach -t webapp
```

### Remote Server Management
```bash
# SSH to server
ssh user@server

# Start tmux session
tmux new -s admin

# Monitor system
htop

# New window for logs
Ctrl-b c
tail -f /var/log/syslog

# Disconnect safely (session continues)
Ctrl-b d
exit

# Reconnect later
ssh user@server
tmux attach -t admin
```

### Pair Programming
```bash
# Host creates session
tmux new -s pair

# Second user SSH and attaches
ssh user@host
tmux attach -t pair

# Both see same screen, can type simultaneously
```

### CI/CD Integration
```yaml
# Run tests in tmux for debugging
- name: Run tests in tmux
  shell: |
    tmux new-session -d -s ci-test
    tmux send-keys -t ci-test "npm test" Enter
    tmux send-keys -t ci-test "npm run lint" Enter
    sleep 60
    tmux kill-session -t ci-test
```

## Copy Mode (Vi-style)

1. Enter copy mode: `C-b [`
2. Navigate with Vi keys: `h j k l`
3. Start selection: `Space`
4. Copy selection: `Enter`
5. Paste: `C-b ]`

## Plugin Manager (TPM)

Included plugins:
- `tmux-sensible` - Sensible defaults
- `tmux-resurrect` - Save/restore sessions
- `tmux-continuum` - Auto-save sessions

### Commands:
- `C-b I` - Install plugins
- `C-b U` - Update plugins
- `C-b alt-u` - Uninstall plugins

### Add More Plugins:

Edit `~/.tmux.conf`:
```tmux
set -g @plugin 'tmux-plugins/tmux-yank'
set -g @plugin 'tmux-plugins/tmux-copycat'
```

Then press `C-b I` to install.

## Popular Plugins

```tmux
# Better mouse support
set -g @plugin 'nhdaly/tmux-better-mouse-mode'

# Copy to system clipboard
set -g @plugin 'tmux-plugins/tmux-yank'

# Search and highlight
set -g @plugin 'tmux-plugins/tmux-copycat'

# Status bar theme
set -g @plugin 'jimeh/tmux-themepack'
set -g @themepack 'powerline/default/cyan'

# Sidebar file tree
set -g @plugin 'tmux-plugins/tmux-sidebar'
```

## Session Management

```bash
# Save session (manual)
C-b C-s

# Restore session
C-b C-r

# Auto-restore (with tmux-continuum)
# Sessions automatically saved every 15 minutes
```

## Configuration Tips

Edit `~/.tmux.conf`:

```tmux
# Use 256 colors
set -g default-terminal "screen-256color"

# No delay for escape key
set -sg escape-time 0

# Aggressive resize
setw -g aggressive-resize on

# Custom status bar
set -g status-right '#[fg=yellow]#(uptime | cut -d "," -f 3-)'
```

## Copy Mode (Vi-style)

```bash
# Enter copy mode
Ctrl-b [

# Navigate with Vi keys
h j k l        # Move cursor
0 $            # Start/end of line
w b            # Word forward/backward
/ ?            # Search forward/backward
g G            # Top/bottom of buffer

# Start selection
Space

# Copy selection
Enter

# Paste
Ctrl-b ]

# Exit copy mode
q or Escape
```

## Productivity Tips

### Custom Key Bindings
Edit `~/.tmux.conf`:
```tmux
# Better split commands
bind | split-window -h
bind - split-window -v
unbind '"'
unbind %

# Reload config
bind r source-file ~/.tmux.conf \; display "Config reloaded"

# Vim-like pane navigation
bind h select-pane -L
bind j select-pane -D
bind k select-pane -U
bind l select-pane -R

# Resize panes
bind -r H resize-pane -L 5
bind -r J resize-pane -D 5
bind -r K resize-pane -U 5
bind -r L resize-pane -R 5
```

### Status Bar Customization
```tmux
# Status bar position
set -g status-position top

# Status bar colors
set -g status-bg black
set -g status-fg white

# Status bar content
set -g status-right '#[fg=yellow]#(uptime | cut -d "," -f 3-)'
set -g status-left '[#S]'

# Window status
setw -g window-status-current-style 'fg=black bg=green'
```

### Session Management Script
```bash
#!/bin/bash
# tmux-dev.sh - Quick development setup

SESSION="dev"

# Create session
tmux new-session -d -s $SESSION

# Window 1: Editor
tmux rename-window -t $SESSION:0 'editor'
tmux send-keys -t $SESSION:0 'cd ~/project && vim' C-m

# Window 2: Server
tmux new-window -t $SESSION:1 -n 'server'
tmux send-keys -t $SESSION:1 'cd ~/project && npm run dev' C-m

# Window 3: Logs
tmux new-window -t $SESSION:2 -n 'logs'
tmux send-keys -t $SESSION:2 'cd ~/project && tail -f logs/app.log' C-m

# Window 4: Database
tmux new-window -t $SESSION:3 -n 'database'
tmux split-window -h -t $SESSION:3
tmux send-keys -t $SESSION:3.0 'psql myapp_dev' C-m
tmux send-keys -t $SESSION:3.1 'redis-cli' C-m

# Attach to session
tmux attach -t $SESSION
```

## Troubleshooting

### Config not loading
```bash
# Reload config manually
Ctrl-b :source-file ~/.tmux.conf

# Or from command line
tmux source-file ~/.tmux.conf
```

### Colors look wrong
```bash
# Add to ~/.tmux.conf
set -g default-terminal "screen-256color"
set -ga terminal-overrides ",xterm-256color:Tc"
```

### Mouse not working
```bash
# Enable in config
set -g mouse on

# Then reload
Ctrl-b r
```

### Sessions not persisting
```bash
# Install tmux-resurrect plugin
# Add to ~/.tmux.conf
set -g @plugin 'tmux-plugins/tmux-resurrect'
set -g @plugin 'tmux-plugins/tmux-continuum'
set -g @continuum-restore 'on'

# Save: Ctrl-b Ctrl-s
# Restore: Ctrl-b Ctrl-r
```

## Agent Use

- Persistent terminal sessions for long-running tasks
- Remote server administration without screen
- Development environment automation
- CI/CD test isolation
- Pair programming and collaboration
- Multi-service application management
- Session recovery after disconnections

## Uninstall

```yaml
- preset: tmux
  with:
    state: absent
```

## Resources

- Official site: https://github.com/tmux/tmux
- Wiki: https://github.com/tmux/tmux/wiki
- Man page: `man tmux`
- Cheat sheet: https://tmuxcheatsheet.com/
- Search: "tmux tutorial", "tmux configuration", "tmux plugins"

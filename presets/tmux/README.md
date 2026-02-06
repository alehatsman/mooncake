# Tmux Preset

Install and configure Tmux - a powerful terminal multiplexer for managing multiple terminal sessions.

## Quick Start

```yaml
- preset: tmux
  with:
    prefix_key: "C-a"
    mouse_mode: true
    install_tpm: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `install_tpm` | bool | `true` | Install Plugin Manager |
| `prefix_key` | string | `C-b` | Prefix key (C-b or C-a) |
| `mouse_mode` | bool | `true` | Enable mouse support |
| `history_limit` | string | `10000` | Scrollback history |

## Usage

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

## Quick Reference

### Sessions
```bash
tmux                    # Start new session
tmux new -s mysession   # Start named session
tmux ls                 # List sessions
tmux attach             # Attach to last session
tmux attach -t mysession # Attach to named session
tmux kill-session -t mysession # Kill session
```

### Key Bindings (with default C-b prefix)

**Windows:**
- `C-b c` - Create new window
- `C-b n` - Next window
- `C-b p` - Previous window
- `C-b 0-9` - Switch to window number
- `C-b w` - List windows
- `C-b ,` - Rename window
- `C-b &` - Kill window

**Panes:**
- `C-b |` - Split horizontally
- `C-b -` - Split vertically
- `C-b arrow` - Navigate panes
- `Alt-arrow` - Navigate without prefix
- `C-b x` - Kill pane
- `C-b z` - Toggle pane zoom

**Other:**
- `C-b d` - Detach session
- `C-b r` - Reload config
- `C-b [` - Enter copy mode (scroll)
- `C-b ]` - Paste buffer

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

## Workflow Example

```bash
# Start development session
tmux new -s dev

# Create windows
C-b c  # Editor window
C-b c  # Server window
C-b c  # Database window

# Split panes for monitoring
C-b |  # Split for logs
C-b -  # Split for system monitor

# Detach and continue later
C-b d

# Reattach
tmux attach -t dev
```

## Uninstall

```yaml
- preset: tmux
  with:
    state: absent
```

# Kitty - GPU Terminal Emulator

Fast, feature-rich, GPU-accelerated terminal emulator with support for images, ligatures, and extensive customization.

## Quick Start
```yaml
- preset: kitty
```

## Features
- **GPU-accelerated**: Rendering via OpenGL for smooth performance
- **Images**: Display images directly in terminal
- **Ligatures**: Programming font ligatures support
- **True color**: 24-bit color with transparency
- **Tabs and splits**: Built-in window management
- **Unicode**: Full Unicode support including emojis
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Start kitty
kitty

# Open with specific directory
kitty --directory=/path/to/dir

# Run command
kitty --hold sh -c "ls -la"

# New window in existing instance
kitty @ new-window

# New tab
kitty @ new-tab

# Split window
kitty @ launch --location=split
```

## Advanced Configuration
```yaml
# Basic installation
- preset: kitty

# Install and verify
- preset: kitty
  register: kitty_result

- name: Check kitty version
  shell: kitty --version
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kitty |

## Platform Support
- ✅ Linux (apt, pacman, official binary)
- ✅ macOS (Homebrew, official app)
- ❌ Windows (not supported, use WSL)

## Configuration
- **Config file**: `~/.config/kitty/kitty.conf`
- **Themes**: `~/.config/kitty/themes/`
- **Keybindings**: Defined in kitty.conf
- **Session files**: Store window/tab layouts

## Configuration Examples

### Basic kitty.conf
```conf
# Font
font_family      JetBrains Mono
font_size        13.0

# Theme
background_opacity 0.95
background #1e1e2e
foreground #cdd6f4

# Cursor
cursor_shape block
cursor_blink_interval 0

# Tabs
tab_bar_style powerline
tab_powerline_style round

# Window
remember_window_size yes
window_padding_width 5
```

### Keyboard Shortcuts
```conf
# Tabs
map ctrl+shift+t new_tab
map ctrl+shift+w close_tab
map ctrl+shift+right next_tab
map ctrl+shift+left previous_tab

# Windows
map ctrl+shift+enter new_window
map ctrl+shift+n new_os_window

# Splits
map ctrl+shift+minus launch --location=split
map ctrl+shift+backslash launch --location=vsplit

# Font size
map ctrl+shift+equal change_font_size all +2.0
map ctrl+shift+minus change_font_size all -2.0
map ctrl+shift+0 change_font_size all 0
```

### Theme Configuration
```conf
# Use theme from file
include themes/gruvbox-dark.conf

# Or define inline
background #282828
foreground #ebdbb2
cursor #ebdbb2
color0 #282828
color1 #cc241d
color2 #98971a
color3 #d79921
# ... more colors
```

## Real-World Examples

### Development Setup
```bash
# Create session file: ~/.config/kitty/sessions/dev.conf
layout tall
cd ~/projects/myapp

launch zsh
launch --cwd=current htop
launch --cwd=current --type=tab npm run dev

# Load session
kitty --session ~/.config/kitty/sessions/dev.conf
```

### Image Display
```bash
# View images in terminal
kitty +kitten icat image.png

# Preview multiple images
for img in *.png; do
  kitty +kitten icat "$img"
  echo "$img"
done

# Image in neovim/vim
# Use image.nvim or similar plugins
```

### Remote Development
```bash
# SSH with kitty integration
kitty +kitten ssh user@remote

# This enables:
# - File transfer via drag-and-drop
# - Shell integration features
# - Proper terminfo on remote
```

### Diff Tool
```bash
# Visual diff
kitty +kitten diff file1.txt file2.txt

# Git difftool integration
git config --global diff.tool kitty
git config --global difftool.kitty.cmd 'kitty +kitten diff $LOCAL $REMOTE'
```

## Kittens (Built-in Tools)

```bash
# Image viewer
kitty +kitten icat image.png

# Diff viewer
kitty +kitten diff file1 file2

# Unicode input
kitty +kitten unicode_input

# Clipboard
kitty +kitten clipboard --get-clipboard

# Hints (click URLs/paths)
kitty +kitten hints

# SSH wrapper
kitty +kitten ssh hostname

# Panel (split terminal)
kitty +kitten panel sh -c 'htop'

# Broadcast (type to all windows)
kitty +kitten broadcast
```

## Agent Use
- Automated terminal session creation for development environments
- Scripted image display in CI/CD dashboards
- Terminal-based data visualization
- SSH automation with built-in file transfer
- Integration testing with terminal UI applications
- Remote development environment provisioning

## Troubleshooting

### GPU Acceleration Not Working
```bash
# Check OpenGL support
glxinfo | grep "OpenGL version"

# Force software rendering
kitty --config NONE --override "linux_display_server=x11"

# Disable GPU (fallback)
# Add to kitty.conf:
# gpu_acceleration no
```

### Font Ligatures Not Showing
```bash
# Ensure font supports ligatures
# Install Fira Code, JetBrains Mono, or Cascadia Code

# Enable in kitty.conf
disable_ligatures never

# Test
echo "==> != === <="
```

### Colors Wrong in Vim/Neovim
```bash
# Set TERM correctly
echo $TERM  # Should be xterm-kitty

# In shell config (.zshrc, .bashrc)
export TERM=xterm-kitty

# Or in kitty.conf
term xterm-kitty
```

### Copy/Paste Not Working
```bash
# Check clipboard settings in kitty.conf
clipboard_control write-clipboard write-primary read-clipboard read-primary

# Use kitty's clipboard
kitty +kitten clipboard --get-clipboard
echo "text" | kitty +kitten clipboard
```

## Comparison with Other Terminals

### Kitty vs Alacritty
- **Kitty**: More features (tabs, images, kittens), slightly slower
- **Alacritty**: Minimal, faster, requires external tab manager

### Kitty vs iTerm2 (macOS)
- **Kitty**: Cross-platform, GPU-accelerated, FOSS
- **iTerm2**: macOS-only, more mature, native integration

### Kitty vs WezTerm
- **Kitty**: Simpler config, faster startup
- **WezTerm**: Lua configuration, more programmable

## Uninstall
```yaml
- preset: kitty
  with:
    state: absent
```

## Resources
- Official docs: https://sw.kovidgoyal.net/kitty/
- GitHub: https://github.com/kovidgoyal/kitty
- Themes: https://github.com/dexpota/kitty-themes
- Config examples: https://github.com/kovidgoyal/kitty/discussions
- Search: "kitty terminal tutorial", "kitty configuration examples", "kitty vs alacritty"

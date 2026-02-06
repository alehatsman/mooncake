# alacritty - GPU Terminal Emulator

Fast, GPU-accelerated terminal emulator. Cross-platform, highly configurable, minimal latency for responsive terminal experience.

## Quick Start
```yaml
- preset: alacritty
```

## Basic Usage
```bash
# Launch Alacritty
alacritty

# With specific command
alacritty -e vim

# With shell
alacritty -e /bin/zsh

# With working directory
alacritty --working-directory /path/to/dir
```

## Command Line Options
```bash
# Run command
alacritty -e nvim file.txt
alacritty --command ssh user@server

# Working directory
alacritty --working-directory ~/projects

# Config file
alacritty --config-file ~/.config/alacritty/custom.toml

# Title
alacritty -t "My Terminal"
alacritty --title "Dev Environment"

# Window options
alacritty --class MyClass
alacritty --option 'window.dimensions.columns=120'

# Hold window open
alacritty --hold -e ./script.sh
```

## Configuration File
```toml
# ~/.config/alacritty/alacritty.toml

[window]
opacity = 0.95
padding = { x = 10, y = 10 }
decorations = "Full"  # Full, None, Transparent, Buttonless
startup_mode = "Maximized"  # Windowed, Maximized, Fullscreen

[window.dimensions]
columns = 120
lines = 40

[font]
size = 14.0

[font.normal]
family = "FiraCode Nerd Font"
style = "Regular"

[font.bold]
family = "FiraCode Nerd Font"
style = "Bold"

[colors.primary]
background = "#1e1e2e"
foreground = "#cdd6f4"

[cursor]
style = { shape = "Block", blinking = "On" }
blink_interval = 750
```

## Font Configuration
```toml
[font]
size = 14.0

[font.normal]
family = "JetBrains Mono"
style = "Regular"

[font.bold]
family = "JetBrains Mono"
style = "Bold"

[font.italic]
family = "JetBrains Mono"
style = "Italic"

# Offset
[font.offset]
x = 0
y = 1

# Glyph offset
[font.glyph_offset]
x = 0
y = 0
```

## Color Schemes
```toml
# Tokyo Night
[colors.primary]
background = "#1a1b26"
foreground = "#c0caf5"

[colors.normal]
black = "#15161e"
red = "#f7768e"
green = "#9ece6a"
yellow = "#e0af68"
blue = "#7aa2f7"
magenta = "#bb9af7"
cyan = "#7dcfff"
white = "#a9b1d6"

# Dracula
[colors.primary]
background = "#282a36"
foreground = "#f8f8f2"

# Gruvbox Dark
[colors.primary]
background = "#282828"
foreground = "#ebdbb2"
```

## Keyboard Shortcuts
```toml
[[keyboard.bindings]]
key = "N"
mods = "Command"
action = "SpawnNewInstance"

[[keyboard.bindings]]
key = "Plus"
mods = "Command"
action = "IncreaseFontSize"

[[keyboard.bindings]]
key = "Minus"
mods = "Command"
action = "DecreaseFontSize"

[[keyboard.bindings]]
key = "Key0"
mods = "Command"
action = "ResetFontSize"

[[keyboard.bindings]]
key = "V"
mods = "Command"
action = "Paste"

[[keyboard.bindings]]
key = "C"
mods = "Command"
action = "Copy"

[[keyboard.bindings]]
key = "Q"
mods = "Command"
action = "Quit"
```

## Window Configuration
```toml
[window]
# Opacity
opacity = 0.9

# Padding
padding = { x = 15, y = 15 }

# Decorations
decorations = "Full"  # Full, None, Transparent

# Startup mode
startup_mode = "Windowed"  # Windowed, Maximized, Fullscreen

# Dynamic title
dynamic_title = true

# Dimensions
[window.dimensions]
columns = 120
lines = 40

# Position
[window.position]
x = 100
y = 100
```

## Cursor Configuration
```toml
[cursor]
# Style
style = { shape = "Block", blinking = "On" }
# Shapes: Block, Underline, Beam

# Blink interval (ms)
blink_interval = 750

# Unfocused behavior
unfocused_hollow = true
```

## Scrolling
```toml
[scrolling]
# History
history = 10000

# Multiplier
multiplier = 3

# Auto-scroll
auto_scroll = false
```

## Mouse Bindings
```toml
[[mouse.bindings]]
mouse = "Right"
action = "PasteSelection"

[[mouse.bindings]]
mouse = "Middle"
action = "PasteSelection"

[mouse]
hide_when_typing = true

[mouse.double_click]
threshold = 300

[mouse.triple_click]
threshold = 300
```

## Hints (URL/Path Detection)
```toml
[[hints.enabled]]
command = "open"
regex = "(ipfs:|ipns:|magnet:|mailto:|gemini://|gopher://|https://|http://|news:|file:|git://|ssh:|ftp://)[^\u0000-\u001F\u007F-\u009F<>\"\\s{-}\\^⟨⟩`]+"
post_processing = true
mouse.enabled = true
binding = { key = "U", mods = "Control|Shift" }

[[hints.enabled]]
command = "open"
regex = "[^ -~]?[^ ]+[.](?:txt|md|log|json|yaml|toml|conf|ini)[^ -~]?"
mouse.enabled = true
```

## Performance Tuning
```toml
[debug]
# Rendering
render_timer = false

# Print events
print_events = false

# Log level
log_level = "Warn"  # Off, Error, Warn, Info, Debug, Trace

[env]
TERM = "xterm-256color"
```

## Integration Examples
```bash
# Tmux
alacritty -e tmux

# Neovim
alacritty -e nvim

# SSH sessions
alacritty -t "Production Server" -e ssh prod.example.com

# Custom shell
alacritty -e fish

# Project-specific terminal
alacritty --working-directory ~/projects/myapp -t "MyApp Dev"

# Development environment
alacritty -e zsh -c "cd ~/projects && nvim"
```

## Tips & Tricks
```bash
# Copy config to project
cp ~/.config/alacritty/alacritty.toml ./alacritty-custom.toml

# Use project-specific config
alacritty --config-file ./alacritty-custom.toml

# Test config changes
# Alacritty live reloads on config file changes

# Check for errors
alacritty --print-events
```

## Comparison
| Feature | Alacritty | iTerm2 | Kitty | WezTerm |
|---------|-----------|--------|-------|---------|
| GPU-accelerated | Yes | Yes | Yes | Yes |
| Config format | TOML | GUI/plist | conf | Lua |
| Startup speed | Fastest | Slow | Fast | Fast |
| Platform | All | macOS | All | All |
| Ligatures | Yes | Yes | Yes | Yes |

## Best Practices
- **Use Nerd Fonts** for icon support
- **Enable opacity carefully** (can impact readability)
- **Set reasonable history** (10000-50000 lines)
- **Use vi mode** if you're vim user
- **Configure hints** for clickable URLs/paths
- **Use tmux/zellij** for splits (Alacritty = simple)
- **Version control** your config file

## Tips
- Fastest terminal startup time
- GPU rendering = smooth scrolling
- Cross-platform consistency
- Simple, focused on terminal emulation
- No tabs/splits (by design, use multiplexer)
- Hot reload configuration
- Ligature support for coding fonts

## Agent Use
- Automated terminal launching
- CI/CD interactive sessions
- SSH session management
- Development environment setup
- Container debugging
- Remote development

## Uninstall
```yaml
- preset: alacritty
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/alacritty/alacritty
- Docs: https://github.com/alacritty/alacritty/blob/master/docs/features.md
- Search: "alacritty config", "alacritty themes"

# wezterm - GPU-Accelerated Terminal

Modern cross-platform terminal emulator with GPU acceleration, built-in multiplexer, and Lua configuration.

## Quick Start
```yaml
- preset: wezterm
```

## Basic Usage
```bash
# Start terminal
wezterm

# Start with command
wezterm start -- bash

# List fonts
wezterm ls-fonts

# Show config
wezterm show-keys
```

## Configuration

Configuration file: `~/.config/wezterm/wezterm.lua` (Linux/macOS) or `%USERPROFILE%\.config\wezterm\wezterm.lua` (Windows)

### Basic Configuration
```lua
local wezterm = require 'wezterm'
local config = {}

-- Font
config.font = wezterm.font('JetBrains Mono')
config.font_size = 12.0

-- Colors
config.color_scheme = 'Tokyo Night'

-- Window
config.window_background_opacity = 0.95
config.window_padding = {
  left = 10,
  right = 10,
  top = 10,
  bottom = 10,
}

-- Tabs
config.hide_tab_bar_if_only_one_tab = true
config.use_fancy_tab_bar = false

return config
```

## Fonts
```lua
-- Use specific font
config.font = wezterm.font('JetBrains Mono', { weight = 'Bold' })

-- Font with fallbacks
config.font = wezterm.font_with_fallback({
  'JetBrains Mono',
  'FiraCode Nerd Font',
  'Menlo',
})

-- Enable ligatures
config.harfbuzz_features = { 'calt=1', 'clig=1', 'liga=1' }

-- List available fonts
wezterm ls-fonts

-- List fonts matching pattern
wezterm ls-fonts --list-system | grep Mono
```

## Color Schemes
```lua
-- Built-in color scheme
config.color_scheme = 'Tokyo Night'

-- Popular schemes:
-- 'Dracula'
-- 'Gruvbox Dark'
-- 'Nord'
-- 'Solarized Dark'
-- 'Catppuccin'
-- 'One Dark'

-- Custom colors
config.colors = {
  foreground = '#c0caf5',
  background = '#1a1b26',
  cursor_bg = '#c0caf5',
  cursor_border = '#c0caf5',
  selection_fg = '#c0caf5',
  selection_bg = '#33467c',

  ansi = {
    '#15161e', '#f7768e', '#9ece6a', '#e0af68',
    '#7aa2f7', '#bb9af7', '#7dcfff', '#a9b1d6',
  },
  brights = {
    '#414868', '#f7768e', '#9ece6a', '#e0af68',
    '#7aa2f7', '#bb9af7', '#7dcfff', '#c0caf5',
  },
}
```

## Key Bindings
```lua
local act = wezterm.action

config.keys = {
  -- Tabs
  { key = 't', mods = 'CTRL|SHIFT', action = act.SpawnTab 'CurrentPaneDomain' },
  { key = 'w', mods = 'CTRL|SHIFT', action = act.CloseCurrentTab{ confirm = true } },
  { key = '[', mods = 'CTRL|SHIFT', action = act.ActivateTabRelative(-1) },
  { key = ']', mods = 'CTRL|SHIFT', action = act.ActivateTabRelative(1) },

  -- Panes
  { key = '|', mods = 'CTRL|SHIFT', action = act.SplitHorizontal{ domain = 'CurrentPaneDomain' } },
  { key = '_', mods = 'CTRL|SHIFT', action = act.SplitVertical{ domain = 'CurrentPaneDomain' } },
  { key = 'h', mods = 'CTRL|SHIFT', action = act.ActivatePaneDirection 'Left' },
  { key = 'l', mods = 'CTRL|SHIFT', action = act.ActivatePaneDirection 'Right' },
  { key = 'k', mods = 'CTRL|SHIFT', action = act.ActivatePaneDirection 'Up' },
  { key = 'j', mods = 'CTRL|SHIFT', action = act.ActivatePaneDirection 'Down' },

  -- Copy/Paste
  { key = 'c', mods = 'CTRL|SHIFT', action = act.CopyTo 'Clipboard' },
  { key = 'v', mods = 'CTRL|SHIFT', action = act.PasteFrom 'Clipboard' },

  -- Font size
  { key = '+', mods = 'CTRL', action = act.IncreaseFontSize },
  { key = '-', mods = 'CTRL', action = act.DecreaseFontSize },
  { key = '0', mods = 'CTRL', action = act.ResetFontSize },
}
```

## Multiplexer (Built-in)
```lua
-- Enable multiplexing
config.enable_tab_bar = true

-- SSH integration
config.ssh_domains = {
  {
    name = 'production',
    remote_address = 'prod.example.com',
    username = 'admin',
  },
}

-- Connect to SSH domain
wezterm connect production
```

## Tabs Configuration
```lua
-- Tab bar appearance
config.use_fancy_tab_bar = false
config.hide_tab_bar_if_only_one_tab = true
config.tab_bar_at_bottom = false
config.tab_max_width = 25

-- Tab title
wezterm.on('format-tab-title', function(tab, tabs, panes, config, hover, max_width)
  local title = tab.tab_title
  if title and #title > 0 then
    return title
  end
  return tab.active_pane.title
end)

-- Tab colors
config.colors.tab_bar = {
  background = '#1a1b26',
  active_tab = {
    bg_color = '#7aa2f7',
    fg_color = '#1a1b26',
  },
  inactive_tab = {
    bg_color = '#292e42',
    fg_color = '#545c7e',
  },
}
```

## Window Configuration
```lua
-- Window appearance
config.window_decorations = 'RESIZE'  -- or 'NONE', 'TITLE', 'TITLE | RESIZE'
config.window_background_opacity = 0.95
config.text_background_opacity = 1.0

-- Blur (macOS only)
config.macos_window_background_blur = 20

-- Padding
config.window_padding = {
  left = 10,
  right = 10,
  top = 10,
  bottom = 10,
}

-- Initial size
config.initial_rows = 30
config.initial_cols = 120
```

## Performance
```lua
-- GPU acceleration
config.front_end = 'WebGpu'  -- or 'OpenGL'
config.max_fps = 60

-- Scrollback
config.scrollback_lines = 10000

-- Animation
config.animation_fps = 60
config.cursor_blink_rate = 800
config.cursor_blink_ease_in = 'Constant'
config.cursor_blink_ease_out = 'Constant'
```

## Hyperlinks
```lua
-- Clickable URLs
config.hyperlink_rules = {
  -- HTTP(S) URLs
  {
    regex = '\\b\\w+://[\\w.-]+\\.[a-z]{2,15}\\S*\\b',
    format = '$0',
  },
  -- File paths
  {
    regex = '\\b\\w+@[\\w-]+(\\.[\\w-]+)+\\b',
    format = 'mailto:$0',
  },
}
```

## Domains (SSH/Docker/WSL)
```lua
-- SSH domains
config.ssh_domains = {
  {
    name = 'dev',
    remote_address = 'dev.example.com',
    username = 'deploy',
  },
  {
    name = 'prod',
    remote_address = 'prod.example.com',
    username = 'admin',
    multiplexing = 'WezTerm',
  },
}

-- Connect to domain
wezterm connect dev
wezterm connect ssh://user@host
```

## Advanced Features

### Smart Tab Switching
```lua
-- Switch to last active tab
{ key = 'Tab', mods = 'CTRL', action = act.ActivateLastTab },

-- Go to specific tab
{ key = '1', mods = 'ALT', action = act.ActivateTab(0) },
{ key = '2', mods = 'ALT', action = act.ActivateTab(1) },
```

### Custom Events
```lua
-- Trigger custom action
wezterm.on('trigger-vim-with-scrollback', function(window, pane)
  local scrollback = pane:get_lines_as_text()
  local name = os.tmpname()
  local f = io.open(name, 'w+')
  f:write(scrollback)
  f:flush()
  f:close()
  window:perform_action(act.SpawnCommandInNewTab{ args = { 'vim', name }}, pane)
end)
```

### Launch Menu
```lua
config.launch_menu = {
  {
    label = 'Bash',
    args = { 'bash', '-l' },
  },
  {
    label = 'PowerShell',
    args = { 'pwsh' },
  },
  {
    label = 'Top',
    args = { 'top' },
  },
}
```

## Workspace Management
```lua
-- Save workspace
wezterm.on('save-workspace', function(window)
  local workspace = window:active_workspace()
  -- Save to file
end)

-- Restore workspace
wezterm.on('restore-workspace', function(window)
  -- Load from file
end)
```

## Status Bar
```lua
wezterm.on('update-right-status', function(window, pane)
  local date = wezterm.strftime '%Y-%m-%d %H:%M:%S'
  window:set_right_status(wezterm.format {
    { Text = date },
  })
end)
```

## Image Protocol Support
```bash
# Display images (iTerm2 protocol)
wezterm imgcat image.png

# In scripts
#!/bin/bash
for img in *.png; do
  wezterm imgcat "$img"
done
```

## Serial Port Support
```lua
-- Connect to serial device
config.serial_ports = {
  {
    port = '/dev/ttyUSB0',
    baud = 115200,
  },
}
```

## Comparison
| Feature | WezTerm | Alacritty | Kitty | iTerm2 |
|---------|---------|-----------|-------|--------|
| GPU | WebGPU | OpenGL | OpenGL | Metal |
| Config | Lua | TOML | Config | GUI |
| Multiplexer | Built-in | No | Yes | Yes |
| Ligatures | Yes | Yes | Yes | Yes |
| Images | Yes | No | Yes | Yes |
| SSH | Built-in | No | Yes | Yes |
| Platform | All | All | All | macOS |
| Speed | Very Fast | Fastest | Very Fast | Fast |

## Troubleshooting
```lua
-- Debug overlay
{ key = 'L', mods = 'CTRL|SHIFT', action = act.ShowDebugOverlay },

-- Check GPU
wezterm --version
wezterm show-config

-- Performance issues
config.front_end = 'OpenGL'  -- Try if WebGpu has issues
config.max_fps = 30  -- Reduce if needed

-- Font issues
wezterm ls-fonts --list-system
```

## Productivity Workflows

### Development Setup
```lua
-- Project launcher
config.default_prog = { 'zsh', '-l' }

config.launch_menu = {
  {
    label = 'Backend',
    args = { 'bash', '-c', 'cd ~/backend && vim' },
  },
  {
    label = 'Frontend',
    args = { 'bash', '-c', 'cd ~/frontend && npm run dev' },
  },
}
```

### Quick Actions
```lua
-- Quick commands
config.keys = {
  {
    key = 'E',
    mods = 'CTRL|SHIFT',
    action = act.PromptInputLine {
      description = 'Enter new tab name',
      action = wezterm.action_callback(function(window, pane, line)
        if line then
          window:active_tab():set_title(line)
        end
      end),
    },
  },
}
```

## CI/CD Integration
```bash
# Test config
wezterm --config-file ./test-config.lua

# Non-interactive use
wezterm start -- bash -c "make test"

# Screenshot
wezterm cli capture-pane --pane-id 0
```

## Best Practices
- **Use Lua for config** - powerful and flexible
- **Enable GPU acceleration** for smooth rendering
- **Configure font fallbacks** for emoji support
- **Use built-in multiplexer** instead of tmux/screen
- **Leverage SSH domains** for remote connections
- **Set reasonable scrollback** (balance memory/history)
- **Use key bindings** for common tasks

## Tips
- Rust-based (fast and reliable)
- WebGPU rendering (modern graphics)
- Cross-platform (Linux, macOS, Windows)
- Lua configuration (full programming language)
- Built-in image protocol
- SSH multiplexing
- Serial port support
- Active development

## Agent Use
- Automated terminal sessions
- SSH connection management
- Development environment setup
- Build and test automation
- Remote server administration
- Serial device communication

## Uninstall
```yaml
- preset: wezterm
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/wez/wezterm
- Docs: https://wezfurlong.org/wezterm/
- Config: https://wezfurlong.org/wezterm/config/files.html
- Search: "wezterm config", "wezterm lua"

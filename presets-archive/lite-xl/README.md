# lite-xl - Lightweight Text Editor

Lightweight, simple, fast text editor written in Lua with minimal memory footprint and highly extensible plugin system.

## Quick Start
```yaml
- preset: lite-xl
```

## Features
- **Ultra-lightweight**: ~3MB binary, minimal resource usage
- **Fast startup**: Instant launch, no splash screen
- **Lua-based plugins**: Easy to write and customize
- **Syntax highlighting**: Support for 50+ languages
- **Multiple cursors**: Edit in multiple places simultaneously
- **Project navigation**: Quick file switching and search

## Basic Usage
```bash
# Launch editor
lite-xl

# Open file
lite-xl myfile.txt

# Open directory
lite-xl /path/to/project

# Show version
lite-xl --version

# Run with specific config
lite-xl --config /path/to/config
```

## Advanced Configuration
```yaml
- preset: lite-xl
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove lite-xl |

## Platform Support
- ✅ Linux (AppImage, package managers)
- ✅ macOS (Homebrew, DMG)
- ✅ Windows (installer, portable)

## Configuration
- **User directory**: `~/.config/lite-xl` (Linux/macOS), `%APPDATA%\lite-xl` (Windows)
- **Init file**: `init.lua` in user directory
- **Plugins**: `plugins/` subdirectory
- **Themes**: `colors/` subdirectory

## Real-World Examples

### Custom Configuration
```lua
-- ~/.config/lite-xl/init.lua
local core = require "core"
local style = require "core.style"

-- Increase font size
style.code_font = renderer.font.load("monospace", 14)

-- Disable line wrapping
config.line_limit = 80

-- Set color scheme
core.reload_module("colors.gruvbox")

-- Enable plugins
require "plugins.autosave"
require "plugins.bracketmatch"
```

### Plugin Installation
```bash
# Clone plugin repository
cd ~/.config/lite-xl/plugins
git clone https://github.com/lite-xl/lite-xl-lsp lsp

# Or download single plugin
curl -o ~/.config/lite-xl/plugins/autosave.lua \
  https://raw.githubusercontent.com/lite-xl/lite-xl-plugins/master/plugins/autosave.lua
```

### Development Setup
```yaml
# Install lite-xl with custom plugins
- name: Install lite-xl
  preset: lite-xl

- name: Create plugins directory
  file:
    path: ~/.config/lite-xl/plugins
    state: directory

- name: Install LSP plugin
  shell: |
    cd ~/.config/lite-xl/plugins
    git clone https://github.com/lite-xl/lite-xl-lsp lsp
```

### Portable Installation
```bash
# Create portable version
mkdir lite-xl-portable
cd lite-xl-portable
wget https://github.com/lite-xl/lite-xl/releases/download/v2.1.0/lite-xl-v2.1.0-linux-x86_64-portable.tar.gz
tar xf lite-xl-*.tar.gz
cd lite-xl
./lite-xl
```

## Agent Use
- Lightweight code editor for automation tasks
- Text file editing in resource-constrained environments
- Quick configuration file editing
- Log file viewing and analysis
- Minimal IDE for scripting environments

## Troubleshooting

### AppImage won't run
Make executable and check FUSE:
```bash
chmod +x LiteXL-*.AppImage

# If FUSE not available
./LiteXL-*.AppImage --appimage-extract
./squashfs-root/AppRun
```

### Fonts not rendering correctly
Install recommended fonts:
```bash
# Ubuntu/Debian
sudo apt install fonts-jetbrains-mono fonts-firacode

# macOS
brew install --cask font-jetbrains-mono
```

### Plugin not loading
Check plugin syntax:
```bash
# Run with verbose output
lite-xl --verbose
```

Verify plugin location:
```bash
ls -la ~/.config/lite-xl/plugins/
```

### High DPI scaling issues
Set scale factor in init.lua:
```lua
-- ~/.config/lite-xl/init.lua
SCALE = 2
```

## Uninstall
```yaml
- preset: lite-xl
  with:
    state: absent
```

## Resources
- Official site: https://lite-xl.com/
- GitHub: https://github.com/lite-xl/lite-xl
- Plugins: https://github.com/lite-xl/lite-xl-plugins
- Themes: https://github.com/lite-xl/lite-xl-colors
- Search: "lite-xl text editor", "lite-xl plugins"

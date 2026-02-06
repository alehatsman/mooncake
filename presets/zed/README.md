# Zed - High-Performance Code Editor

A next-generation, high-performance code editor built in Rust, featuring real-time collaboration, AI integration, and blazing-fast performance.

## Quick Start
```yaml
- preset: zed
```

## Features
- **Blazing fast**: GPU-accelerated rendering, instant startup
- **Real-time collaboration**: Built-in multiplayer editing
- **AI-powered**: Integrated AI assistant for code generation and refactoring
- **Language support**: LSP-based intelligent completion for all major languages
- **Modern UI**: Beautiful, minimalist interface with intuitive workflows
- **Cross-platform**: Linux, macOS

## Basic Usage
```bash
# Launch Zed
zed

# Open file or directory
zed /path/to/file
zed /path/to/project

# Open file at specific line
zed file.rs:42

# Show version
zed --version

# Get help
zed --help
```

## Keyboard Shortcuts

### Essential
| Shortcut | Action |
|----------|--------|
| `Cmd+P` / `Ctrl+P` | Quick open file |
| `Cmd+Shift+P` / `Ctrl+Shift+P` | Command palette |
| `Cmd+B` / `Ctrl+B` | Toggle file tree |
| `Cmd+K` / `Ctrl+K` | Clear search |
| `Cmd+,` / `Ctrl+,` | Open settings |

### Editing
| Shortcut | Action |
|----------|--------|
| `Cmd+D` / `Ctrl+D` | Add selection to next match |
| `Cmd+Shift+L` / `Ctrl+Shift+L` | Select all occurrences |
| `Cmd+/` / `Ctrl+/` | Toggle line comment |
| `Alt+Up/Down` | Move line up/down |
| `Cmd+Shift+K` / `Ctrl+Shift+K` | Delete line |

### Navigation
| Shortcut | Action |
|----------|--------|
| `Cmd+T` / `Ctrl+T` | Go to symbol |
| `F12` | Go to definition |
| `Cmd+Click` / `Ctrl+Click` | Go to definition |
| `Cmd+Alt+Left/Right` | Navigate back/forward |

### Collaboration
| Shortcut | Action |
|----------|--------|
| `Cmd+Shift+C` / `Ctrl+Shift+C` | Share project |
| `Cmd+J` / `Ctrl+J` | Join project |

## Advanced Configuration
```yaml
# Basic installation
- preset: zed
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (AppImage, package managers)
- ✅ macOS (DMG, Homebrew)
- ❌ Windows (in development)

## Configuration
- **Config file**: `~/.config/zed/settings.json` (Linux), `~/Library/Application Support/Zed/settings.json` (macOS)
- **Keybindings**: `~/.config/zed/keymap.json` (Linux), `~/Library/Application Support/Zed/keymap.json` (macOS)
- **Extensions**: `~/.config/zed/extensions/` (Linux), `~/Library/Application Support/Zed/extensions/` (macOS)

### Sample Configuration
```json
{
  "theme": "One Dark",
  "ui_font_size": 16,
  "buffer_font_size": 14,
  "tab_size": 2,
  "format_on_save": "on",
  "vim_mode": false,
  "telemetry": {
    "diagnostics": false,
    "metrics": false
  }
}
```

## Real-World Examples

### Solo Development
```bash
# Open project directory
zed ~/projects/my-app

# Use AI assistant
# Cmd+Shift+A (or Ctrl+Shift+A) to open assistant
# Ask: "Refactor this function to use async/await"
```

### Pair Programming
```bash
# Start Zed and share project
# 1. Open project: zed ~/projects/shared-project
# 2. Press Cmd+Shift+C to generate share link
# 3. Share link with collaborator
# 4. Both users can edit simultaneously with live cursors
```

### Code Review Workflow
```bash
# Open specific file at line number
zed src/main.rs:142

# Navigate to definition
# Cmd+Click on function name to jump to definition
# Cmd+Alt+Left to go back
```

## Agent Use
- Open and edit code files in automated workflows
- Leverage AI assistant for code generation in development pipelines
- Integrate with CI/CD for code review and refactoring
- Use command-line interface for scripted file manipulation
- Collaborate on code changes in real-time during deployment

## Troubleshooting

### Zed won't launch on Linux
Ensure required dependencies are installed:
```bash
# Ubuntu/Debian
sudo apt install libgl1-mesa-glx libxcb-xfixes0

# Fedora
sudo dnf install mesa-libGL libxcb
```

### LSP server not working
Restart the language server:
```
Cmd+Shift+P → "Restart Language Server"
```

### High GPU usage
Disable GPU acceleration in settings:
```json
{
  "gpu": false
}
```

### Collaboration not working
Check firewall settings and ensure ports are open. Zed uses WebRTC for peer-to-peer connections.

## Uninstall
```yaml
- preset: zed
  with:
    state: absent
```

**Manual cleanup (optional):**
```bash
# Remove configuration
rm -rf ~/.config/zed  # Linux
rm -rf ~/Library/Application\ Support/Zed  # macOS
```

## Resources
- Official site: https://zed.dev
- GitHub: https://github.com/zed-industries/zed
- Documentation: https://zed.dev/docs
- Search: "zed editor tutorial", "zed vs vscode", "zed collaboration"

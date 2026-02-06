# lapce - Lightning-Fast Modern Code Editor

Native, GPU-accelerated code editor written in Rust with built-in LSP support, modal editing, and remote development capabilities.

## Quick Start
```yaml
- preset: lapce
```

## Features
- **GPU-accelerated rendering**: Smooth scrolling and UI with Druid UI framework
- **Built-in LSP**: Language Server Protocol support out of the box
- **Modal editing**: Vim-like keybindings with intuitive commands
- **Remote development**: Edit code on remote machines via SSH
- **Plugin system**: Extend functionality with plugins
- **Modern UI**: Clean, customizable interface with themes

## Basic Usage
```bash
# Launch editor
lapce

# Open file
lapce myfile.rs

# Open directory
lapce /path/to/project

# Show version
lapce --version

# List available plugins
lapce plugins list

# Install plugin
lapce plugins install <plugin-name>
```

## Advanced Configuration
```yaml
- preset: lapce
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove lapce |

## Platform Support
- ✅ Linux (Flatpak, AppImage)
- ✅ macOS (Homebrew, DMG)
- ❌ Windows (not yet supported by this preset)

## Configuration
- **Config directory**: `~/.config/lapce` (Linux), `~/Library/Application Support/lapce` (macOS)
- **Settings file**: `settings.toml` in config directory
- **Themes**: User themes in `themes/` subdirectory
- **Plugins**: Installed in `plugins/` subdirectory

## Real-World Examples

### Custom Settings
```toml
# ~/.config/lapce/settings.toml
[core]
modal = true
color-theme = "Lapce Dark"

[editor]
font-family = "Cascadia Code"
font-size = 14
tab-width = 4
show-line-numbers = true

[terminal]
font-family = "JetBrains Mono"
font-size = 13
```

### Development Workflow
```yaml
# Set up development environment
- name: Install Lapce
  preset: lapce

- name: Configure editor settings
  copy:
    dest: ~/.config/lapce/settings.toml
    content: |
      [core]
      modal = true

      [editor]
      font-family = "FiraCode Nerd Font"
      tab-width = 2
```

### Remote Development
```bash
# Edit files on remote server
lapce ssh://user@server:/path/to/project

# Remote development with specific key
lapce ssh://user@server:/path/to/project --ssh-key ~/.ssh/id_rsa
```

## Agent Use
- IDE setup in development environments
- Remote code editing automation
- Consistent editor configuration across teams
- Plugin installation and management
- Development environment standardization

## Troubleshooting

### AppImage not executable
Make it executable:
```bash
chmod +x Lapce-*.AppImage
./Lapce-*.AppImage
```

### Missing fonts
Install recommended fonts:
```bash
# Ubuntu/Debian
sudo apt install fonts-firacode fonts-jetbrains-mono

# macOS
brew install --cask font-fira-code font-jetbrains-mono
```

### LSP not working
Check language server installation:
```bash
# For Rust
rustup component add rust-analyzer

# For Python
pip install python-lsp-server
```

### GPU acceleration issues
Try software rendering:
```bash
LAPCE_LOG=lapce_app=debug lapce
```

## Uninstall
```yaml
- preset: lapce
  with:
    state: absent
```

## Resources
- Official site: https://lapce.dev/
- GitHub: https://github.com/lapce/lapce
- Documentation: https://docs.lapce.dev/
- Plugins: https://plugins.lapce.dev/
- Search: "lapce editor tutorial", "lapce vs code alternative"

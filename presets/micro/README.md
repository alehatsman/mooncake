# micro - Terminal Text Editor with Mouse Support

A modern, intuitive terminal-based text editor with mouse support, syntax highlighting, and sensible defaults that just work.

## Quick Start

```yaml
- preset: micro
```

## Features

- **Mouse support**: Click to position cursor, drag to select, scroll with wheel
- **Syntax highlighting**: Automatic language detection and color themes
- **Keybindings**: Vi/Vim-like, Emacs-like, or Nano-like (user configurable)
- **Plugins**: Extend functionality with third-party plugins
- **Multiple cursors**: Edit multiple locations simultaneously
- **Undo/Redo**: Full undo history with redo support
- **Cross-platform**: Linux, macOS, and Windows support

## Basic Usage

```bash
# Start editor
micro

# Open a file
micro file.txt

# Open multiple files
micro file1.txt file2.txt

# Check version
micro --version

# Get help
micro --help

# Open at specific line and column
micro +10:5 file.txt

# Run plugin command
micro -plugin list
```

## Advanced Configuration

```yaml
# Advanced installation with all options
- preset: micro
  with:
    state: present              # present or absent
    version: latest             # or specific version
    plugins:
      - enabled                 # Plugin suite
      - aspell                  # Spell checker
      - detectindent            # Auto indentation
    theme: monokai              # or other theme
    keybinding_mode: vi         # vi, emacs, or default
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) |
| version | string | latest | Version to install (e.g., "2.0.13", "latest") |
| plugins | array | [] | List of plugins to install/enable |
| theme | string | default | Color theme (monokai, solarized, etc.) |
| keybinding_mode | string | default | Keybinding scheme (vi, emacs, or default) |

## Basic Usage Examples

### Essential Commands

```bash
# Save file
Ctrl+s

# Quit without saving
Ctrl+q

# Undo
Ctrl+z

# Redo
Ctrl+y

# Copy line
Ctrl+c

# Cut line
Ctrl+x

# Paste
Ctrl+v

# Find text
Ctrl+f

# Replace text
Ctrl+h

# Go to line
Ctrl+g
```

### Navigation

```bash
# Move cursor
Arrow keys or hjkl (Vi mode)

# Page up/down
PgUp / PgDn

# Start/end of line
Home / End

# Start/end of file
Ctrl+Home / Ctrl+End

# Jump to word
Ctrl+Right / Ctrl+Left
```

### Editing

```bash
# Select all
Ctrl+a

# Select line
Ctrl+l

# Delete line
Alt+d or Ctrl+Shift+k

# Duplicate line
Alt+u or Ctrl+d

# Insert line above
Alt+Up

# Insert line below
Alt+Down

# Join lines
Alt+j
```

## Configuration

Micro uses two main configuration paths:

**Linux:**
- **Config directory**: `~/.config/micro/`
- **Settings file**: `~/.config/micro/settings.json`
- **Keybindings**: `~/.config/micro/bindings.json`
- **Plugins**: `~/.config/micro/plugins/`

**macOS:**
- **Config directory**: `~/.config/micro/`
- **Settings file**: `~/.config/micro/settings.json`
- **Keybindings**: `~/.config/micro/bindings.json`
- **Plugins**: `~/.config/micro/plugins/`

**Windows:**
- **Config directory**: `%APPDATA%\micro\`
- **Settings file**: `%APPDATA%\micro\settings.json`
- **Plugins**: `%APPDATA%\micro\plugins\`

### Customizing Settings

Edit `~/.config/micro/settings.json`:

```json
{
  "colorscheme": "monokai",
  "tabsize": 4,
  "indentchar": " ",
  "autoindent": true,
  "autoformat": true,
  "gofmt": true,
  "wordwrap": false,
  "statusline": true,
  "tabbar": true,
  "syntax": true,
  "mouse": true
}
```

### Common Settings

```json
{
  "autoindent": true,      # Maintain indentation on new lines
  "mouse": true,           # Enable mouse support
  "syntax": true,          # Enable syntax highlighting
  "wordwrap": false,       # Wrap long lines
  "colorscheme": "monokai",# Color theme
  "tabsize": 4,            # Tab width
  "ruler": true,           # Show line/column ruler
  "statusline": true       # Show status bar
}
```

## Real-World Examples

### Quick Config Editing

```bash
# Edit shell configuration
micro ~/.bashrc

# Edit nginx config
micro /etc/nginx/nginx.conf

# Edit git config
micro ~/.gitconfig
```

### Development Workflow

```bash
# Create and edit Python script
micro script.py
# ... write code ...
# Save with Ctrl+s, exit with Ctrl+q

# Edit multiple files simultaneously
micro main.go util.go types.go
```

### Using Multiple Cursors

```bash
# Place first cursor at position
# Press Ctrl+m to create/remove cursor
# Edit at multiple locations simultaneously
# Useful for refactoring variable names
```

## Platform Support

- ✅ Linux (apt, dnf, pacman via package managers or curl installer)
- ✅ macOS (Homebrew or curl installer)
- ✅ Windows (download or package managers)

## Agent Use

- **Text editing automation**: Script-based file modifications
- **Configuration management**: Deploy and modify config files
- **Quick edits**: One-off file changes in automation pipelines
- **Log inspection**: View and analyze logs during troubleshooting
- **Template rendering**: Edit and validate template files
- **Code generation**: Modify auto-generated code files
- **CI/CD integration**: Edit files as part of deployment workflows

## Troubleshooting

### Mouse not working

Check if mouse support is enabled in `~/.config/micro/settings.json`:
```json
{
  "mouse": true
}
```

If still not working, restart micro or try:
```bash
# Rebuild micro
micro -plugin rebuild
```

### Syntax highlighting not working

Verify syntax highlighting is enabled:
```bash
# In micro, press Ctrl+e and type:
set syntax true
```

Ensure your file has the correct extension (`.py`, `.go`, `.js`, etc.)

### Keybindings not responding

Check keybinding configuration:
```bash
# View current keybindings in micro
micro -help | grep bind
```

Edit `~/.config/micro/bindings.json` if custom keybindings are desired.

### Plugin installation issues

List available plugins:
```bash
micro -plugin list
```

Install specific plugin:
```bash
micro -plugin install pluginname
```

## Uninstall

```yaml
- preset: micro
  with:
    state: absent
```

**Note:** Configuration files in `~/.config/micro/` are preserved after uninstall.

## Resources

- Official docs: https://micro-editor.github.io/
- GitHub: https://github.com/zyedidia/micro
- Plugin repository: https://github.com/micro-editor/plugin-channel
- Search: "micro editor tutorial", "micro editor plugins", "micro editor configuration"

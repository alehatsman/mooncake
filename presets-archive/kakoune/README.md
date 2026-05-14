# Kakoune - Modal Code Editor

Code editor with orthogonal design, multiple selections, and strong Unix integration. Inspired by Vim but with interactive feedback.

## Quick Start
```yaml
- preset: kakoune
```

## Features
- **Multiple selections**: Edit multiple locations simultaneously
- **Orthogonal design**: Clear separation between selection and action
- **Interactive**: See changes as you type with live feedback
- **Client-server**: Built-in client-server architecture
- **Language support**: Syntax highlighting for 100+ languages
- **Customizable**: Extensive scripting with Kakoune's language
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Open file
kak file.txt

# Open multiple files
kak file1.txt file2.txt

# Open at specific line
kak +42 file.txt

# Read from stdin
echo "text" | kak

# Start in client-server mode
kak -s mysession file.txt
kak -c mysession other.txt  # Connect to session
```

## Advanced Configuration
```yaml
# Basic installation
- preset: kakoune

# Install and verify
- preset: kakoune
  register: kak_result

- name: Check version
  shell: kak -version
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kakoune |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (WSL recommended)

## Configuration
- **Config file**: `~/.config/kak/kakrc`
- **Autoload**: `~/.config/kak/autoload/`
- **Colors**: `~/.config/kak/colors/`
- **Plugins**: Managed via plugin managers or manual installation

## Key Bindings (Normal Mode)
```
Selection:
  w, W          - Select next word/WORD
  b, B          - Select previous word/WORD
  %             - Select whole buffer
  s             - Select regex matches
  S             - Split selections by regex
  <a-s>         - Split selections by line

Editing:
  i, I          - Insert mode (before/start of selection)
  a, A          - Insert mode (after/end of selection)
  c             - Change (delete and insert)
  d             - Delete selection
  y             - Yank (copy) selection
  p, P          - Paste after/before

Multiple Selections:
  <a-k>         - Keep selections matching regex
  <a-K>         - Remove selections matching regex
  <space>       - Clear all but main selection
  <a-space>     - Clear main selection

Navigation:
  h, j, k, l    - Move cursor
  gg, G         - Go to first/last line
  %             - Go to matching bracket
  m             - Select to matching bracket
```

## Real-World Examples

### Multiple Cursor Editing
```bash
# Example: Add quotes around words
kak file.txt
# In normal mode:
# 1. Type '%' to select all
# 2. Type 's\w+<ret>' to select all words
# 3. Type 'c"<c-r>."<esc>' to wrap in quotes
```

### Refactoring Code
```bash
# Replace function name across file
kak app.js
# In normal mode:
# 1. Type '%' to select buffer
# 2. Type 's\boldFunction\b<ret>' to find matches
# 3. Type 'cnewFunction<esc>' to replace
```

### Git Integration
```bash
# Edit files from git diff
git diff --name-only | kak

# Edit conflict files
git diff --name-only --diff-filter=U | xargs kak
```

### Configuration Example (~/.config/kak/kakrc)
```kak
# Set color scheme
colorscheme gruvbox

# Line numbers
add-highlighter global/ number-lines -relative

# Soft tabs (spaces)
set-option global tabstop 2
set-option global indentwidth 2

# Enable auto-pairs
hook global InsertChar \( %{ exec -draft h<a-k>\(<ret>; exec a)<esc> }
hook global InsertChar \{ %{ exec -draft h<a-k>\{<ret>; exec a}<esc> }
hook global InsertChar \[ %{ exec -draft h<a-k>\[<ret>; exec a]<esc> }

# Save with Ctrl-s
map global normal <c-s> ':w<ret>'

# Format on save
hook global BufWritePre .* %{
  try %{ execute-keys -draft \%s\h+$<ret>d }  # Remove trailing whitespace
}
```

### Language Server Protocol (LSP)
```bash
# Install kak-lsp
cargo install kak-lsp

# Configure in kakrc
eval %sh{kak-lsp --kakoune -s $kak_session}
lsp-enable

# Usage: hover, goto definition, rename
# Use :lsp-hover, :lsp-definition, :lsp-rename
```

## Agent Use
- Script-driven code transformations in CI/CD
- Batch editing of configuration files
- Automated refactoring tasks
- Template generation from stdin
- Interactive code review tooling
- Syntax highlighting for log analysis

## Plugins and Extensions
```bash
# Popular plugin managers
# 1. plug.kak (recommended)
git clone https://github.com/andreyorst/plug.kak.git ~/.config/kak/plugins/plug.kak

# 2. Manual installation
mkdir -p ~/.config/kak/autoload
git clone <plugin-repo> ~/.config/kak/autoload/plugin-name
```

## Troubleshooting

### Terminal Colors Not Working
```bash
# Ensure TERM is set correctly
echo $TERM  # Should be xterm-256color or similar

# Test colors
kak -e 'colorscheme gruvbox'
```

### Session Already Exists
```bash
# List sessions
kak -l

# Kill session
kak -c session -e 'kill'

# Force new session
kak -d -s newsession file.txt
```

### Clipboard Not Working
```bash
# Install clipboard tool
# macOS: pbcopy/pbpaste (built-in)
# Linux: xclip or xsel

sudo apt install xclip  # Debian/Ubuntu
sudo dnf install xclip  # Fedora

# Configure in kakrc
hook global RegisterModified '"' %{ nop %sh{
  printf %s "$kak_main_reg_dquote" | xclip -selection clipboard
}}
```

## Comparison with Vim

### Kakoune Advantages
- Selection-first workflow (see what you're changing)
- Multiple selections as first-class feature
- Composable commands with interactive feedback
- Simpler scripting language
- Client-server by default

### Vim Advantages
- Ubiquitous (installed everywhere)
- Larger plugin ecosystem
- More learning resources
- Mature language server integration

## Uninstall
```yaml
- preset: kakoune
  with:
    state: absent
```

## Resources
- Official docs: https://kakoune.org/
- GitHub: https://github.com/mawww/kakoune
- Wiki: https://github.com/mawww/kakoune/wiki
- Community: https://discuss.kakoune.com/
- Search: "kakoune tutorial", "kakoune vs vim", "kakoune plugins"

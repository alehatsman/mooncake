# Helix - Post-modern Modal Text Editor

A modern, Kakoune-inspired modal text editor written in Rust with built-in LSP support and tree-sitter syntax highlighting.

## Quick Start
```yaml
- preset: helix
```

## Features
- **Built-in LSP**: Language Server Protocol support out of the box
- **Tree-sitter syntax**: Accurate syntax highlighting and code navigation
- **Multiple selections**: Edit multiple locations simultaneously
- **Modal editing**: Vim-like modal interface with improved ergonomics
- **Cross-platform**: Linux, macOS, and BSD support
- **No configuration required**: Works great with defaults

## Basic Usage
```bash
# Open file
hx file.txt

# Open multiple files
hx file1.txt file2.txt file3.txt

# Open file at specific line
hx file.txt:42

# Open directory (file picker)
hx /path/to/project

# Create new file
hx new-file.txt

# Read from stdin
echo "content" | hx -
```

## Modal Editing Basics

### Normal Mode (default)
| Key | Action |
|-----|--------|
| `i` | Insert before cursor |
| `a` | Insert after cursor |
| `o` | Insert line below |
| `O` | Insert line above |
| `v` | Select mode |
| `x` | Select line |
| `w` | Move forward by word |
| `b` | Move backward by word |
| `d` | Delete selection |
| `c` | Change selection (delete + insert) |
| `y` | Yank (copy) |
| `p` | Paste after |
| `P` | Paste before |
| `u` | Undo |
| `U` | Redo |

### Selection and Navigation
```bash
# Multiple selections (Helix superpower)
%     # Select entire file
s     # Select pattern (regex)
C     # Copy selection to new cursor
,     # Remove primary selection
;     # Reduce to single selection

# Extend selections
w     # Extend to next word
e     # Extend to word end
b     # Extend to word start
f<ch> # Find character forward
t<ch> # Till character forward

# Jump
gd    # Go to definition (LSP)
gi    # Go to implementation
gr    # Go to references
Space+j # Jump to symbol in file
Space+s # Search in file
Space+/ # Global search
```

### File Operations
```bash
:w        # Write file
:q        # Quit
:wq       # Write and quit
:q!       # Quit without saving
:o file   # Open file
:buffer-close  # Close current buffer
:buffer-next   # Next buffer
:buffer-previous # Previous buffer
```

## Language Server Protocol (LSP)

### Automatic Language Support
Helix automatically detects and uses LSP servers:
- **Rust**: rust-analyzer
- **Python**: pyright, pylsp
- **JavaScript/TypeScript**: typescript-language-server
- **Go**: gopls
- **C/C++**: clangd
- **Java**: jdtls

### LSP Features
```bash
# Code navigation
gd        # Go to definition
gi        # Go to implementation
gr        # Go to references
Space+k   # Hover documentation
Space+r   # Rename symbol

# Code actions
Space+a   # Code actions menu
Space+d   # Show diagnostics

# Autocompletion
# Triggers automatically while typing
Ctrl+Space  # Manual completion trigger
```

### Installing LSP Servers
```bash
# Rust
rustup component add rust-analyzer

# Python
pip install pyright

# JavaScript/TypeScript
npm install -g typescript-language-server

# Go
go install golang.org/x/tools/gopls@latest

# C/C++
# Install clangd via package manager
```

## Configuration
```bash
# Config location
~/.config/helix/config.toml     # Linux/BSD
~/Library/Application Support/helix/config.toml  # macOS
```

### Basic config.toml
```toml
theme = "onedark"

[editor]
line-number = "relative"
mouse = true
cursorline = true
auto-save = true

[editor.cursor-shape]
insert = "bar"
normal = "block"
select = "underline"

[editor.file-picker]
hidden = false  # Show hidden files

[editor.lsp]
display-messages = true
```

## Themes
```bash
# List available themes
:theme [Tab]

# Change theme
:theme onedark
:theme gruvbox
:theme nord
:theme dracula
```

Popular themes: onedark, gruvbox, nord, dracula, monokai, solarized

## Advanced Configuration
```yaml
- preset: helix
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Helix editor |

## Real-World Examples

### Development Workflow
```bash
# Open project
hx ~/projects/myapp

# Navigate to file (fuzzy finder)
Space+f

# Search across project
Space+/

# Go to definition
gd

# Rename variable
Space+r

# Format code
:format

# Run shell command
:sh cargo build
```

### Multi-cursor Editing
```bash
# Select all occurrences of word under cursor
*

# Add cursor at each match
%s pattern<Enter>

# Edit all matches simultaneously
c new-text<Esc>

# Remove selections
,  # Remove primary
;  # Keep only primary
```

### Working with Multiple Files
```bash
# Open multiple files
hx *.rs

# Switch between buffers
Space+b  # Buffer picker

# Split windows
Ctrl+w s  # Horizontal split
Ctrl+w v  # Vertical split
Ctrl+w h/j/k/l  # Navigate splits
```

## Keyboard Shortcuts Cheatsheet

### Essential Commands
| Key | Action |
|-----|--------|
| `Space+f` | Find file |
| `Space+b` | Switch buffer |
| `Space+/` | Global search |
| `Space+w` | Write file |
| `Space+q` | Quit |
| `gd` | Go to definition |
| `gr` | Find references |
| `:` | Command mode |

### Movement
| Key | Action |
|-----|--------|
| `h/j/k/l` | Left/Down/Up/Right |
| `w/b` | Word forward/backward |
| `gg/ge` | File start/end |
| `gh/gl` | Line start/end |
| `%` | Match bracket |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported in preset)

## Comparison with Other Editors
| Feature | Helix | Neovim | Vim |
|---------|-------|--------|-----|
| LSP | Built-in | Plugin | Plugin |
| Tree-sitter | Built-in | Plugin | N/A |
| Config | Minimal TOML | Lua/Vimscript | Vimscript |
| Learning curve | Moderate | Steep | Steep |
| Multiple cursors | Native | Plugin | Plugin |

## Agent Use
- Modern text editor for code editing tasks
- Built-in LSP for intelligent code operations
- Scriptable via command mode
- Efficient for multi-file refactoring
- Tree-sitter for accurate syntax operations

## Troubleshooting

### LSP not working
```bash
# Check LSP status
:lsp-workspace-command

# Verify LSP server installed
which rust-analyzer  # or appropriate server

# Check logs
:log-open
```

### Performance issues
```toml
# Reduce tree-sitter timeout
[editor.lsp]
timeout = 5  # seconds

# Disable features
[editor]
auto-pairs = false
auto-completion = false
```

### Custom key bindings
```toml
# ~/.config/helix/config.toml
[keys.normal]
C-s = ":w"  # Ctrl+S to save
```

## Uninstall
```yaml
- preset: helix
  with:
    state: absent
```

## Resources
- Official docs: https://docs.helix-editor.com/
- GitHub: https://github.com/helix-editor/helix
- Book: https://docs.helix-editor.com/
- Search: "helix editor tutorial", "helix vs neovim"

# Modern Unix Tools - Fast, Friendly Replacements for Classic Commands

Modern Unix tools offer significant improvements over their classic counterparts: faster performance, better UX with colors and icons, sensible defaults, and active maintenance. This preset installs curated modern alternatives that respect your workflow (like .gitignore).

## Quick Start

```yaml
- preset: modern-unix
```

## Features

- **Fast**: Rewritten in Rust/modern languages - often 10-100x faster than originals
- **User-Friendly**: Colored output, helpful defaults, intuitive flags
- **Respects Conventions**: Automatically honors .gitignore, .ignore files
- **Drop-in Compatible**: Mostly same flags/syntax as originals (can create aliases)
- **Well-Maintained**: Active development, responsive maintainers
- **Cross-Platform**: Available on Linux and macOS via package managers

## Basic Usage

```bash
# bat - Syntax highlighting cat
bat script.py               # Auto-detect language
bat --line-number file.txt  # Show line numbers
bat --style=numbers,grid    # Different output styles

# ripgrep - Fast recursive grep (respects .gitignore)
rg "TODO" src/             # Search respecting .gitignore
rg -t rust "fn main"       # Search specific file type
rg -c "error"              # Count matches
rg --stats "pattern"       # Show search statistics

# fd - Simpler find replacement
fd "test.*\.py"            # Find Python test files
fd -e txt                  # Find all .txt files
fd "config" -x cat {}      # Execute command on matches

# exa - Colorful ls replacement
exa -la                    # List with details
exa --tree -L 2            # Tree view, 2 levels
exa --git                  # Show git status

# zoxide - Smart directory jumping (cd replacement)
z documents                # Jump to Documents
z proj test                # Fuzzy match ~/Projects/test-project
zi                         # Interactive selection of frecent dirs

# dust - Intuitive disk usage
dust                       # Top-level disk usage
dust -d 3                  # Depth 3 tree view
dust /var -c               # Count files instead of size

# duf - Pretty disk usage df
duf                        # List all filesystems
duf -hide /dev             # Hide /dev filesystems
duf -only local            # Show only local filesystems

# bottom - System monitor (top replacement)
btm                        # Start monitoring
btm -C config.toml         # Use custom config
btm -r 5                   # Refresh every 5 seconds
```

## Advanced Configuration

```yaml
# Install all tools
- preset: modern-unix

# Install specific tools only
- preset: modern-unix
  with:
    tools:
      - bat
      - ripgrep
      - fd
      - duf

# Uninstall all tools
- preset: modern-unix
  with:
    state: absent

# Uninstall specific tools
- preset: modern-unix
  with:
    state: absent
    tools:
      - zoxide
      - bottom
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tools |
| tools | array | [bat, ripgrep, fd, exa, zoxide, dust, duf, bottom] | List of tools to install/remove |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (available via Chocolatey/Scoop - not included in this preset)

## Tool Reference

| Tool | Replaces | Key Feature | Language |
|------|----------|-------------|----------|
| **bat** | cat | Syntax highlighting, git integration | Rust |
| **ripgrep** (rg) | grep | Respects .gitignore, blazingly fast | Rust |
| **fd** | find | Simpler syntax, colorful output | Rust |
| **exa** | ls | Git integration, icons, colors | Rust |
| **zoxide** (z) | cd | Frecency-based jumping, interactive mode | Rust |
| **dust** | du | Intuitive tree display, fast | Rust |
| **duf** | df | Pretty colors, better formatting | Go |
| **bottom** (btm) | top/htop | Beautiful UI, resource overview | Rust |

## Configuration

**Tool Installation Locations:**
- **Linux**: `/usr/bin/` or `/usr/local/bin/`
- **macOS**: `~/.brew/bin/` (homebrew)

**Shell Integration (Optional):**

Add to `~/.bashrc`, `~/.zshrc`, or `~/.config/fish/config.fish`:

```bash
# zoxide shell integration (required for z command)
eval "$(zoxide init bash)"   # For bash
eval "$(zoxide init zsh)"    # For zsh
eval "$(zoxide init fish)"   # For fish

# Useful aliases (optional)
alias cat='bat'
alias ls='exa'
alias find='fd'
alias grep='rg'
alias du='dust'
alias df='duf'
alias top='btm'
```

**bottom Configuration File:**
- Linux: `~/.config/bottom/bottom.toml`
- macOS: `~/Library/Application Support/bottom/bottom.toml`

## Real-World Examples

### Fast Code Search Pipeline

```bash
# Find all Python files modified in last 7 days with TODO comments
fd -e py --changed-within 7d | xargs rg "TODO" -n

# Search with context and line numbers
rg "function definition" -B 2 -A 2 --line-number

# Count occurrences across codebase
rg "import requests" -c
```

### System Monitoring During Development

```bash
# Start bottom system monitor
btm

# Or with focus on specific tab
btm --default_widget_type cpu

# In another terminal, monitor disk usage in real-time
watch "dust /home -d 1"
```

### Project Navigation

```bash
# Jump to frequently used project
z myproject

# Interactively select from recent directories
zi

# List files in project using exa
exa --tree --level 2 --long

# Find config files
fd "config|settings" --hidden
```

### File Comparison and Viewing

```bash
# Syntax-highlighted diff
diff <(bat file1.py) <(bat file2.py)

# Show git-annotated files
exa --long --git

# Cat with line numbers and git info
bat --line-number --diff <file>
```

## Agent Use

- Rapidly search large codebases with ripgrep (10-100x faster than grep)
- Navigate directory hierarchies with zoxide (auto-learns frequently used paths)
- Parse structured output from modern tools for automation scripts
- Monitor system resources during deployments using bottom
- Find files by pattern and execute bulk operations
- Analyze disk usage and identify bottlenecks quickly
- Build fast CI/CD pipeline scripts with modern tools

## Troubleshooting

### zoxide not working after installation

The `z` command requires shell integration. Add to your shell config:

```bash
eval "$(zoxide init bash)"  # for bash
eval "$(zoxide init zsh)"   # for zsh
```

Then restart your shell or run `source ~/.bashrc`

### Shell aliases conflict with function names

If you use `alias cat='bat'` but need original cat:

```bash
# Use full path to original
/bin/cat file.txt

# Or call with backslash
\cat file.txt

# Or remove alias temporarily
unalias cat
```

### bottom won't display correctly

Check terminal size (needs at least 80x20):

```bash
# See current terminal dimensions
echo $COLUMNS x $LINES

# Expand terminal window and try again
btm
```

### ripgrep not respecting .gitignore

By default rg respects .gitignore. To ignore it:

```bash
rg --no-ignore "pattern"           # Ignore .gitignore
rg --no-ignore-vcs "pattern"       # Ignore VCS files only
rg -uu "pattern"                   # Unrestricted search
```

### exa showing strange characters instead of icons

Terminal font doesn't support Unicode icons. Either:

```bash
# Disable icons
exa --icons=never

# Or install Nerd Font: https://www.nerdfonts.com/
```

## Uninstall

```yaml
- preset: modern-unix
  with:
    state: absent
```

Or remove specific tools:

```yaml
- preset: modern-unix
  with:
    state: absent
    tools:
      - zoxide
      - bottom
```

## Resources

- **bat** (syntax highlighting): https://github.com/sharkdp/bat
- **ripgrep** (fast grep): https://github.com/BurntSushi/ripgrep
- **fd** (simple find): https://github.com/sharkdp/fd
- **exa** (modern ls): https://github.com/ogham/exa
- **zoxide** (smart cd): https://github.com/ajeetdsouza/zoxide
- **dust** (disk usage): https://github.com/bootandy/dust
- **duf** (df replacement): https://github.com/muesli/duf
- **bottom** (system monitor): https://github.com/ClementTsang/bottom
- Search: "modern unix tools", "rust cli tools", "command line productivity"

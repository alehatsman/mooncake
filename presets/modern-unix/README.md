# Modern Unix Tools Preset

Install modern replacements for classic Unix commands with better defaults, colors, and performance.

## What's Included

| Tool | Replaces | Description |
|------|----------|-------------|
| **bat** | cat | Syntax highlighting, git integration, line numbers |
| **ripgrep (rg)** | grep | Blazing fast search, respects .gitignore |
| **fd** | find | Simple syntax, fast, colorful output |
| **exa** | ls | Modern ls with git integration, icons |
| **zoxide (z)** | cd | Smart directory jumping based on frequency |
| **dust** | du | Intuitive disk usage with tree view |
| **duf** | df | Pretty disk usage with colors |
| **bottom (btm)** | top/htop | Graphical system monitor |

## Usage

### Install all tools
```yaml
- name: Install modern Unix tools
  preset: modern-unix
```

### Install specific tools only
```yaml
- name: Install just bat and ripgrep
  preset: modern-unix
  with:
    tools:
      - bat
      - ripgrep
```

### Uninstall
```yaml
- name: Remove modern Unix tools
  preset: modern-unix
  with:
    state: absent
```

## Platform Support

- ✅ macOS (via Homebrew)
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ❌ Windows (most tools available via Chocolatey/Scoop - add if needed)

## Quick Command Reference

```bash
# bat - cat with syntax highlighting
bat file.js

# ripgrep - fast grep
rg "TODO" --type rust

# fd - simple find
fd "test.*\.py"

# exa - better ls
exa -la --git

# zoxide - smart cd (tracks your most used directories)
z documents  # jumps to ~/Documents after first use
z proj       # fuzzy matches ~/Projects

# dust - disk usage tree
dust

# duf - pretty df
duf

# bottom - system monitor
btm
```

## Shell Integration

Some tools work better with shell integration:

### zoxide
Add to your `.bashrc` or `.zshrc`:
```bash
eval "$(zoxide init bash)"  # for bash
eval "$(zoxide init zsh)"   # for zsh
```

### Aliases
Consider adding to your shell config:
```bash
alias cat='bat'
alias ls='exa'
alias find='fd'
alias grep='rg'
```

## Learn More

- [bat](https://github.com/sharkdp/bat)
- [ripgrep](https://github.com/BurntSushi/ripgrep)
- [fd](https://github.com/sharkdp/fd)
- [exa](https://github.com/ogham/exa)
- [zoxide](https://github.com/ajeetdsouza/zoxide)
- [dust](https://github.com/bootandy/dust)
- [duf](https://github.com/muesli/duf)
- [bottom](https://github.com/ClementTsang/bottom)

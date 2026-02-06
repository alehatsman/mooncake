# fzf Preset

Install fzf - a blazingly fast command-line fuzzy finder that helps you search files, command history, processes, and more with interactive filtering.

## Quick Start

```yaml
# Basic installation
- preset: fzf

# With shell extensions (Ctrl+R, Ctrl+T, Alt+C)
- preset: fzf
  with:
    install_shell_extensions: true
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`

## Basic Usage

```bash
# Find and open file in vim
vim $(fzf)

# Search command history (Ctrl+R)
# Press Ctrl+R in your shell to search through command history

# Navigate directories (Alt+C)
# Press Alt+C to fuzzy find and cd into a directory

# Paste files/dirs (Ctrl+T)
# Type a command, press Ctrl+T to select files to paste
```

## Advanced Configuration
```yaml
- preset: fzf
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | Install (`present`) or uninstall (`absent`) |
| `install_shell_extensions` | bool | `true` | Install key bindings and fuzzy completion |
| `install_vim_plugin` | bool | `false` | Install vim/neovim plugin |


## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)
## Usage Examples

### Basic File Finding

```bash
# Find and open file in vim
vim $(fzf)

# Find and edit file
fzf | xargs nvim

# With preview
fzf --preview 'cat {}'
fzf --preview 'bat --color=always {}'
```

### Multi-Selection

```bash
# Select multiple files with Tab
fzf --multi

# Delete multiple files
rm $(fzf --multi)

# Open multiple files in vim
vim $(fzf --multi)
```

### Command History (Ctrl+R)

```bash
# Press Ctrl+R in shell to search history
# Type to filter commands
# Enter to execute selected command
```

### Directory Navigation (Alt+C)

```bash
# Press Alt+C to fuzzy find and cd into directory
# Searches from current directory down
```

### File/Directory Paste (Ctrl+T)

```bash
# Type command and press Ctrl+T
# Select files/directories to paste onto command line
vim <Ctrl+T>  # Opens fuzzy finder, pastes selected files
```

## Advanced Examples

### Kill Process

```bash
# Interactive process killer
kill -9 $(ps aux | fzf | awk '{print $2}')

# With preview showing process details
ps aux | fzf --preview 'echo {}' --preview-window=down:3:wrap
```

### Git Branch Checkout

```bash
# Fuzzy find and checkout git branch
git checkout $(git branch | fzf)

# With preview showing recent commits
git checkout $(git branch | fzf --preview 'git log --oneline --color {} | head -20')
```

### Search and Edit Files

```bash
# Find files by name and edit
fd --type f | fzf --preview 'bat --color=always {}' | xargs nvim

# Search file contents and edit
rg --files-with-matches "searchterm" | fzf --preview 'bat {}' | xargs nvim
```

### Docker Container Management

```bash
# Stop container
docker stop $(docker ps | fzf | awk '{print $1}')

# View logs
docker logs -f $(docker ps -a | fzf | awk '{print $1}')

# Exec into container
docker exec -it $(docker ps | fzf | awk '{print $1}') /bin/bash
```

### Environment Variable Explorer

```bash
# Search and display environment variables
env | fzf

# Export selected variable
eval "export $(env | fzf | cut -d= -f1)"
```

## Integration with Other Tools

### With ripgrep (rg)

```bash
# Search file contents and open in editor
rg --line-number . | fzf | cut -d: -f1 | xargs nvim

# Search with context
rg --line-number --color=always . | fzf --ansi
```

### With fd (find alternative)

```bash
# Find files
fd --type f | fzf

# Find directories only
fd --type d | fzf

# Find with extension
fd --extension js | fzf
```

### With bat (cat alternative)

```bash
# Preview files with syntax highlighting
fzf --preview 'bat --style=numbers --color=always {}'

# Set as default preview
export FZF_DEFAULT_OPTS="--preview 'bat --style=numbers --color=always --line-range :500 {}'"
```

## Shell Integration

### Bash/Zsh Configuration

```bash
# Add to ~/.bashrc or ~/.zshrc

# Use fd instead of find
export FZF_DEFAULT_COMMAND='fd --type f --hidden --follow --exclude .git'

# Ctrl+T configuration
export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
export FZF_CTRL_T_OPTS="--preview 'bat --color=always --line-range :500 {}'"

# Alt+C configuration
export FZF_ALT_C_COMMAND='fd --type d --hidden --follow --exclude .git'
export FZF_ALT_C_OPTS="--preview 'tree -C {} | head -100'"

# Color scheme
export FZF_DEFAULT_OPTS='
  --color=fg:#f8f8f2,bg:#282a36,hl:#bd93f9
  --color=fg+:#f8f8f2,bg+:#44475a,hl+:#bd93f9
  --color=info:#ffb86c,prompt:#50fa7b,pointer:#ff79c6
  --color=marker:#ff79c6,spinner:#ffb86c,header:#6272a4
'
```

### Key Bindings

| Key | Action |
|-----|--------|
| `Ctrl+R` | Search command history |
| `Ctrl+T` | Paste selected files/directories |
| `Alt+C` | cd into selected directory |

Within fzf:
| Key | Action |
|-----|--------|
| `Ctrl+J/K` or `↓/↑` | Navigate |
| `Enter` | Select |
| `Tab` | Multi-select (with --multi) |
| `Shift+Tab` | Deselect |
| `Ctrl+A` | Select all (with --multi) |
| `Ctrl+D` | Deselect all |
| `Ctrl+/` | Toggle preview |

## Vim/Neovim Integration

```vim
" Add to ~/.vimrc or ~/.config/nvim/init.vim

" Basic fzf
set rtp+=/usr/local/opt/fzf  " macOS Homebrew path
" or
set rtp+=~/.fzf              " Git installation path

" File finder
nnoremap <C-p> :FZF<CR>

" Buffer finder
nnoremap <leader>b :Buffers<CR>

" Ripgrep search
nnoremap <leader>f :Rg<CR>

" Command history
nnoremap <leader>h :History:<CR>
```

## Common Options

```bash
# Preview window
--preview 'cat {}'
--preview-window=right:50%
--preview-window=down:40%:wrap

# Multi-selection
--multi
--bind 'ctrl-a:select-all'

# Height and layout
--height 40%
--layout reverse
--border

# Case sensitivity
-i    # Case-insensitive
+i    # Case-sensitive
```

## Real-World Workflows

### 1. Project File Navigation

```bash
#!/bin/bash
# Save as: fzf-edit

# Fuzzy find and edit with preview
fd --type f --hidden --follow --exclude .git | \
  fzf --preview 'bat --color=always {}' \
      --bind 'enter:become(nvim {})'
```

### 2. Git Interactive

```bash
# Fuzzy git log browser
git log --oneline --color=always | \
  fzf --ansi --preview 'git show --color=always {1}' | \
  awk '{print $1}' | \
  xargs git show

# Interactive staging
git status --short | \
  fzf --multi --preview 'git diff --color=always {2}' | \
  awk '{print $2}' | \
  xargs git add
```

### 3. SSH Host Selector

```bash
# Parse ~/.ssh/config and connect
grep "^Host " ~/.ssh/config | \
  awk '{print $2}' | \
  fzf --preview 'grep -A 10 "^Host {}" ~/.ssh/config' | \
  xargs -I {} ssh {}
```

### 4. Directory Bookmarks

```bash
# Add to ~/.bashrc
bookmark() {
  echo "$PWD" >> ~/.bookmarks
}

jump() {
  local dir=$(cat ~/.bookmarks | fzf)
  [ -n "$dir" ] && cd "$dir"
}
```

## Performance Tips

1. **Use with fd instead of find**: Much faster for large directories
2. **Limit preview size**: `--preview-window=:500` to preview first 500 lines
3. **Use --ansi**: When piping colored output
4. **Cache results**: For frequently searched directories

## Troubleshooting

### Key Bindings Not Working

```bash
# Reinstall shell extensions
$(brew --prefix)/opt/fzf/install --key-bindings --completion --update-rc

# Or add manually to ~/.bashrc:
[ -f ~/.fzf.bash ] && source ~/.fzf.bash
```

### Preview Not Showing

```bash
# Check if bat/cat is available
which bat
which cat

# Test preview manually
ls | fzf --preview 'cat {}'
```

### Slow Performance

```bash
# Use fd for better performance
export FZF_DEFAULT_COMMAND='fd --type f'

# Reduce preview size
export FZF_DEFAULT_OPTS='--preview-window=:100'
```

## Agent Use
- Automated environment setup
- CI/CD pipeline integration
- Development environment provisioning
- Infrastructure automation

## Uninstall

```yaml
- preset: fzf
  with:
    state: absent
```

## Resources

- **GitHub**: https://github.com/junegunn/fzf
- **Wiki**: https://github.com/junegunn/fzf/wiki
- **Examples**: https://github.com/junegunn/fzf/wiki/examples

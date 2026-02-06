# zoxide - Smarter cd Command

Faster way to navigate directories. Remembers frequently used directories, jump with partial names, smarter than autojump.

## Quick Start
```yaml
- preset: zoxide
```

## Basic Usage
```bash
# Jump to directory
z proj       # Jumps to ~/projects
z doc        # Jumps to ~/Documents
z config     # Jumps to ~/.config

# Interactive selection (with fzf)
zi proj      # Shows menu of matches

# Add current directory
z --add .

# Remove directory
z --remove /path
```

## Shell Integration
```bash
# Bash (~/.bashrc)
eval "$(zoxide init bash)"

# Zsh (~/.zshrc)
eval "$(zoxide init zsh)"

# Fish (~/.config/fish/config.fish)
zoxide init fish | source

# PowerShell
Invoke-Expression (& { (zoxide init powershell | Out-String) })
```

## How It Works
```bash
# zoxide learns as you cd
cd ~/projects/myapp
cd ~/Documents/work
cd ~/.config/nvim

# Now you can jump
z myapp     # Goes to ~/projects/myapp
z work      # Goes to ~/Documents/work
z nvim      # Goes to ~/.config/nvim

# Partial matches
z pro       # Matches ~/projects
z con       # Matches ~/.config
```

## Commands
```bash
# Jump (alias: z)
zoxide query proj

# Interactive (with fzf)
zi proj

# Add directory manually
zoxide add /path/to/dir

# Remove directory
zoxide remove /path/to/dir

# Edit database
zoxide edit

# Query matches
zoxide query --list proj

# Show statistics
zoxide query --stat
```

## Advanced Usage
```bash
# Jump to subdirectory
z myapp src

# Multiple keywords
z proj myapp config

# Exclude directories
z --exclude node_modules proj

# Score threshold
zoxide query --score 10 proj

# List all matches
zoxide query --list --score 0

# Interactive with preview (requires fzf)
zi
```

## Aliases
```bash
# Built-in aliases after init
z     # Jump
zi    # Interactive jump
zoxide  # Full command

# Custom aliases
alias zz='z -'  # Jump to previous dir
alias zh='z ~'  # Jump to home

# Integration
alias proj='z ~/projects'
alias conf='z ~/.config'
```

## Configuration
```bash
# Custom data directory
export _ZO_DATA_DIR=~/custom/path

# Exclude directories
export _ZO_EXCLUDE_DIRS="/tmp/*:$HOME/temp/*"

# Max number of results
export _ZO_MAXAGE=10000

# FZF options
export _ZO_FZF_OPTS="--height=40% --layout=reverse"

# Resolve symlinks
export _ZO_RESOLVE_SYMLINKS=1

# Echo on cd
export _ZO_ECHO=1
```

## Interactive Mode (zi)
```bash
# Start interactive mode
zi

# With query
zi proj

# FZF key bindings:
# Enter - Jump to selected
# Ctrl+C - Cancel
# Ctrl+N - Next match
# Ctrl+P - Previous match
```

## Statistics
```bash
# View database
zoxide query --stat

# Output example:
# 12.5  /home/user/projects
# 8.3   /home/user/Documents
# 6.1   /home/user/.config
# 4.2   /home/user/projects/myapp

# Export to JSON
zoxide query --stat --list | jq
```

## Integration Examples
```bash
# With fzf
function zf() {
  local dir
  dir=$(zoxide query -l | fzf)
  [[ -n "$dir" ]] && cd "$dir"
}

# Quick project opener
function proj() {
  z $1 && code .
}

# Git integration
function zgit() {
  z $1 && git status
}

# Create and jump
function mkz() {
  mkdir -p "$1" && zoxide add "$1" && cd "$1"
}
```

## Comparison with Autojump
```bash
# Autojump
j proj

# Zoxide (same usage)
z proj

# Both work similarly, but zoxide is:
# - Faster (written in Rust)
# - Better algorithm (frecency)
# - More actively maintained
# - Better fzf integration
```

## Advanced Patterns
```bash
# Substring matching
z app      # Matches myapp, webapp, application

# Multi-word
z web front  # Matches web/frontend

# Partial path
z pro/my   # Matches projects/myapp

# Case insensitive (default)
z PROJ     # Matches projects

# Jump to parent
z ..       # Same as cd ..
z ../..    # Same as cd ../..
```

## Database Management
```bash
# Location
# Linux: ~/.local/share/zoxide/db.zo
# macOS: ~/Library/Application Support/zoxide/db.zo

# Backup
cp ~/.local/share/zoxide/db.zo ~/backup/

# Reset
rm ~/.local/share/zoxide/db.zo
# Rebuild by cd'ing to directories

# Edit manually
zoxide edit
```

## Shell Integration Options
```bash
# Custom aliases
eval "$(zoxide init bash --cmd j)"  # Use 'j' instead of 'z'

# Hook all cd commands
eval "$(zoxide init bash --hook pwd)"

# No aliases
eval "$(zoxide init bash --no-aliases)"
```

## Productivity Workflows
```bash
# Rapid navigation
z api && code .          # Jump and open
z doc && ls -la          # Jump and list
z conf && nvim init.vim  # Jump and edit

# Project switching
z myapp
git pull
npm install
npm start

# Quick access
z down      # Downloads
z drop      # Dropbox
z desk      # Desktop
```

## Migration
```bash
# From autojump
# Works the same way, just start using z

# From z
# Compatible, just install and use

# Import from autojump
cat ~/.local/share/autojump/autojump.txt | \
  while read score path; do
    zoxide add "$path"
  done
```

## Comparison Table
| Feature | zoxide | autojump | z | fasd |
|---------|--------|----------|---|------|
| Speed | Fastest | Fast | Fast | Moderate |
| Language | Rust | Python | Shell | C |
| Algorithm | Frecency | Weighted | Frecency | Frecency |
| FZF | Native | Manual | Manual | Manual |
| Active | Yes | Yes | Limited | No |
| Maintained | Very | Yes | Limited | No |

## Troubleshooting
```bash
# Not learning directories
# Make sure shell integration is loaded
eval "$(zoxide init bash)"

# Database location
echo $_ZO_DATA_DIR

# Debug mode
export _ZO_ECHO=1
z proj  # Shows score and path

# Clear and rebuild
rm ~/.local/share/zoxide/db.zo
# cd to directories to rebuild
```

## Best Practices
- **Use short queries** (2-4 chars usually enough)
- **Let it learn** (use cd for first few visits)
- **Use zi** for ambiguous matches
- **Add important dirs** manually (`zoxide add`)
- **Combine with fzf** for best experience
- **Set custom aliases** for frequent paths
- **Export/backup database** periodically

## Tips
- 3-10x faster than Python-based tools
- Smarter frecency algorithm
- Native fzf integration
- Cross-platform (Linux, macOS, Windows, BSD)
- Shell-agnostic (bash, zsh, fish, PowerShell)
- Minimal overhead (< 5ms)
- Learns from your habits

## Agent Use
- Automated directory navigation
- Workspace automation
- Development environment setup
- Script optimization
- Project management
- Path resolution

## Uninstall
```yaml
- preset: zoxide
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/ajeetdsouza/zoxide
- Wiki: https://github.com/ajeetdsouza/zoxide/wiki
- Search: "zoxide vs autojump", "zoxide fzf"

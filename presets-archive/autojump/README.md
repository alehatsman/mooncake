# autojump - Smart Directory Navigation

Fast way to navigate filesystem. Jump to frequently used directories with partial names instead of typing full paths.

## Quick Start
```yaml
- preset: autojump
```

## Features
- **Smart jumping**: Jump to frequently used directories with partial names
- **Learning algorithm**: Tracks directory usage frequency automatically
- **Fast**: Sub-10ms directory switching
- **Cross-shell**: Works with bash, zsh, fish, tcsh
- **Weighted ranking**: More visited directories rank higher
- **Tab completion**: Complete directory names with fuzzy matching
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# Jump to directory
j proj  # Jumps to ~/projects
j doc   # Jumps to ~/Documents
j down  # Jumps to ~/Downloads

# Multiple matches
j pro   # Shows menu if multiple matches

# Open in file manager
jo proj  # Opens ~/projects in Finder/Explorer

# Child directory
jc test  # Jump to child directory matching "test"
```

## How It Works
```bash
# autojump learns as you cd
cd ~/projects/myapp
cd ~/Documents/work
cd ~/Downloads

# Now you can jump
j myapp    # Goes to ~/projects/myapp
j work     # Goes to ~/Documents/work
j down     # Goes to ~/Downloads

# Partial matches work
j pro      # Matches ~/projects
j doc      # Matches ~/Documents
```

## Shell Integration
```bash
# Bash (~/.bashrc)
[[ -s ~/.autojump/etc/profile.d/autojump.sh ]] && source ~/.autojump/etc/profile.d/autojump.sh

# Zsh (~/.zshrc)
[[ -s ~/.autojump/etc/profile.d/autojump.sh ]] && source ~/.autojump/etc/profile.d/autojump.sh

# Fish (~/.config/fish/config.fish)
begin
    set --local AUTOJUMP_PATH $HOME/.autojump/share/autojump/autojump.fish
    if test -e $AUTOJUMP_PATH
        source $AUTOJUMP_PATH
    end
end
```

## Commands
```bash
# Jump to directory
j pattern

# Jump to child directory
jc pattern

# Open in file manager
jo pattern
jco pattern  # Child directory

# Show statistics
j -s
j --stat

# Add directory manually
j -a /path/to/dir
j --add /path/to/dir

# Increase weight
j -i 100  # Increase current dir weight

# Decrease weight
j -d 15   # Decrease current dir weight

# Purge non-existent directories
j --purge

# Complete (tab completion)
j proj<TAB>
```

## Statistics
```bash
# View database
j -s

# Output example:
10.0:   /home/user/projects
8.0:    /home/user/Documents
6.0:    /home/user/Downloads
5.0:    /home/user/projects/myapp

# Total key weight: 29
```

## Advanced Usage
```bash
# Multiple matches
j pro
# Shows:
# 1. ~/projects
# 2. ~/projects/prototype
# 3. ~/programs
# Select: [1-3]

# Exact match
j __e__ pattern

# Prefer child directories
jc pattern

# Case-sensitive
j __cs__ pattern

# Complete pattern
j proj<TAB>  # Shows completions
```

## Configuration
```bash
# Environment variables

# Increase/decrease weight on directory access
export AUTOJUMP_KEEP_SYMLINKS=1

# Don't update database when PWD prefix matches
export AUTOJUMP_IGNORE_CASE=1

# Database location
export AUTOJUMP_DATA_DIR=~/.local/share/autojump
```

## Workflow Examples
```bash
# Project navigation
j myapp      # Jump to project
code .       # Open in editor
j test       # Jump to test directory
npm test     # Run tests

# Quick document access
j doc        # Jump to Documents
j work       # Jump to work folder
j report     # Jump to reports

# Development workflow
j api        # Jump to API project
docker-compose up -d
j frontend   # Jump to frontend
npm start
```

## Tips & Tricks
```bash
# Learn faster (manually add)
j -a ~/important/project

# Clean up database
j --purge  # Remove non-existent dirs

# Increase priority
cd ~/main/project
j -i 1000  # Give high weight

# Tab completion
j pro<TAB>  # See all matches

# Combine with other commands
cd $(autojump proj)
ls $(autojump doc)
```

## Integration Examples
```bash
# With fzf
j() {
  local dir
  dir=$(autojump -s | sed '/_____/Q; s/^[0-9,.:]*\s*//' | fzf)
  [[ -n "$dir" ]] && cd "$dir"
}

# With git
jgit() {
  j $1 && git status
}

# Quick project opener
proj() {
  j $1 && code .
}
```

## Comparison
| Feature | autojump | z | zoxide | fasd |
|---------|----------|---|--------|------|
| Algorithm | Weighted | Frecency | Frecency | Frecency |
| Speed | Fast | Fast | Fastest | Moderate |
| Language | Python | Shell | Rust | C |
| Active | Yes | Limited | Yes | No |

## Troubleshooting
```bash
# Database location
cat ~/.local/share/autojump/autojump.txt

# Debug mode
export AUTOJUMP_DEBUG=1
j pattern

# Reset database
rm ~/.local/share/autojump/autojump.txt
# Rebuild by using cd

# Not working after install
# Source the script in shell config
source ~/.autojump/etc/profile.d/autojump.sh
```

## Migration
```bash
# From z
# autojump learns automatically, just start using it

# Export autojump database
j -s > autojump-backup.txt

# Import
# Just cd to directories, autojump learns
```

## Best Practices
- **Use short patterns** (2-4 chars usually enough)
- **Let it learn** (use cd normally for first few times)
- **Purge regularly** (`j --purge`)
- **Use jc** for child directories
- **Combine with fzf** for fuzzy selection
- **Add important dirs manually** (`j -a`)

## Tips
- Learns from your cd history
- Weighted algorithm (more visits = higher priority)
- Partial name matching
- Works across sessions
- Cross-platform
- Tab completion support
- Very fast (< 10ms)

## Platform Support
- ✅ Linux (apt, Homebrew)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### Development Workflow
```bash
# Quick project switching
j api          # Jump to API project
docker-compose up -d
j frontend     # Jump to frontend
npm run dev
j docs         # Jump to documentation
mkdocs serve
```

### CI/CD Setup Scripts
```bash
#!/bin/bash
# Navigate and run tasks efficiently
j infra && terraform apply
j api && ./deploy.sh production
j monitoring && docker-compose restart grafana
```

### Multi-Repo Management
```bash
# Update all microservices
for service in auth users payments notifications; do
  j $service && git pull && npm install
done

# Run tests across projects
j api-test && npm test
j frontend-test && npm test
j integration && npm run test:e2e
```

### Directory Organization
```bash
# Add important directories manually
j -a ~/critical-project
j -a ~/client-deliverables
j -a ~/infrastructure

# Now access them instantly
j critical    # → ~/critical-project
j client      # → ~/client-deliverables
j infra       # → ~/infrastructure
```

## Agent Use
- Automated directory navigation in deployment scripts
- Workspace switching in multi-project environments
- Quick access to frequently used project directories
- CI/CD pipeline navigation optimization
- Development environment setup automation

## Uninstall

## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install autojump
  preset: autojump

- name: Use autojump in automation
  shell: |
    # Custom configuration here
    echo "autojump configured"
```

```yaml
- preset: autojump
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/wting/autojump
- Wiki: https://github.com/wting/autojump/wiki
- Search: "autojump tutorial", "autojump vs zoxide"

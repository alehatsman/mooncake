# atuin - Magical Shell History

Sync, search, and backup shell history with context. SQLite-based history with full-text search, sync across machines, statistics.

## Quick Start
```yaml
- preset: atuin
```

## Basic Usage
```bash
# Search history (interactive)
Ctrl+R  # Opens atuin search

# Search with query
atuin search docker

# Show statistics
atuin stats

# Sync history
atuin sync
```

## Shell Integration
```bash
# Bash (~/.bashrc)
eval "$(atuin init bash)"

# Zsh (~/.zshrc)
eval "$(atuin init zsh)"

# Fish (~/.config/fish/config.fish)
atuin init fish | source

# After adding, restart shell
```

## Interactive Search
```
Ctrl+R - Open search interface

In search mode:
  Type to search
  Enter - Execute command
  Tab - Edit command before running
  Ctrl+C - Cancel
  ↑/↓ - Navigate results
  Ctrl+R - Cycle filter modes
```

## Search Modes
```bash
# Full text search (default)
Ctrl+R
> docker ps

# Directory filter
Ctrl+R (cycle to directory mode)
> Shows commands run in current dir

# Session filter
Ctrl+R (cycle to session mode)
> Shows commands from current session

# Host filter
Ctrl+R (cycle to host mode)
> Shows commands from current host
```

## Command Line Search
```bash
# Search history
atuin search docker

# Limit results
atuin search --limit 10 git

# Search by command
atuin search --cmd "git commit"

# Search by directory
atuin search --cwd /home/user/projects

# Exclude
atuin search --exclude "rm -rf"

# Before/after date
atuin search --before "2024-01-01"
atuin search --after "2023-12-01"
```

## Statistics
```bash
# Show stats
atuin stats

# Output example:
# Total commands: 12,453
# Unique commands: 892
# Top commands:
#   1. git status (347)
#   2. ls (289)
#   3. cd .. (178)

# Stats for period
atuin stats --period day
atuin stats --period week
atuin stats --period month
atuin stats --period year

# Top N commands
atuin stats --count 20
```

## History Management
```bash
# Import existing history
atuin import auto

# Import from specific shell
atuin import bash
atuin import zsh
atuin import fish

# List history
atuin history list

# Show specific command
atuin history show <id>

# Delete command
atuin history delete <id>

# Clear all history (dangerous!)
atuin history clear
```

## Sync Across Machines
```bash
# Register account
atuin register -u <username> -e <email>

# Login
atuin login -u <username>

# Sync history
atuin sync

# Auto-sync (in config)
auto_sync = true

# Sync down only
atuin sync --force
```

## Configuration
```toml
# ~/.config/atuin/config.toml

# Search mode
search_mode = "fuzzy"  # fuzzy, exact, prefix, suffix, skim

# Filter mode
filter_mode = "global"  # global, host, session, directory

# Style
style = "compact"  # compact, full, auto

# Inline height
inline_height = 20

# Show preview
show_preview = true

# Update check
update_check = false

# Auto sync
auto_sync = true
sync_frequency = "5m"

# History format
history_format = "{time} - [{duration}] - {command}"

# Filter
filter_mode_shell_up_key_binding = "directory"
```

## Key Bindings
```toml
# Custom key bindings
[keys]
scroll_exits = false

[keys.bindings]
up = "search"
ctrl_r = "search"
ctrl_n = "next"
ctrl_p = "previous"
```

## Advanced Search
```bash
# Regular expressions
atuin search --regex "^git (commit|push)"

# Case sensitive
atuin search --case-sensitive Docker

# Reverse order
atuin search --reverse git

# Interactive filter
atuin search --interactive

# Multiple filters
atuin search \
  --cwd /home/user/projects \
  --after "2024-01-01" \
  --cmd git
```

## Statistics Queries
```bash
# Command frequency
atuin stats | head -20

# Most used in directory
cd ~/projects
atuin stats --cwd $(pwd)

# Recent activity
atuin stats --period day

# Hourly breakdown
atuin history list --format "{time}" | \
  cut -d: -f1 | sort | uniq -c
```

## Productivity Workflows
```bash
# Find forgotten command
Ctrl+R
> ssh prod  # Find the exact ssh command

# Repeat complex command
Ctrl+R
> docker-compose  # Find previous docker-compose command
Tab  # Edit before running

# Learn command patterns
atuin stats
# See what commands you use most

# Cross-machine workflow
# On laptop:
docker run -d myapp
atuin sync

# On desktop:
atuin sync
Ctrl+R > docker run  # Find the exact command
```

## Self-Hosted Sync Server
```bash
# Run your own server
docker run -d \
  -p 8888:8888 \
  -v atuin-data:/data \
  ghcr.io/atuinsh/atuin:latest

# Configure client
atuin register -u user -e user@example.com

# ~/.config/atuin/config.toml
sync_address = "http://localhost:8888"
```

## Privacy & Security
```toml
# Don't record certain patterns
history_filter = [
  "^secret",
  "^password",
  "export.*SECRET",
  "export.*PASSWORD",
  "export.*TOKEN"
]

# Disable sync
auto_sync = false

# Local only mode
sync_address = ""

# Encrypted sync
# History is encrypted before sync
# Only you can decrypt your history
```

## Migration
```bash
# From bash/zsh history
atuin import auto

# From fzf history
# Atuin works alongside fzf

# Export atuin history
atuin history list > history-backup.txt

# Backup database
cp ~/.local/share/atuin/history.db ~/backup/
```

## Comparison
| Feature | atuin | mcfly | hstr | fzf |
|---------|-------|-------|------|-----|
| Sync | Yes | No | No | No |
| SQLite | Yes | Yes | No | No |
| Context | Full | Partial | Basic | N/A |
| Stats | Rich | Basic | Basic | No |
| Fuzzy search | Yes | Yes | Yes | Yes |

## Tips
- SQLite-based (fast, queryable)
- Sync across all machines
- Encrypted cloud sync
- Rich context (directory, time, duration, exit code)
- Full-text search
- Statistics and insights
- Works offline
- Privacy-focused

## Best Practices
- **Enable auto-sync** for cross-machine history
- **Use filter modes** (Ctrl+R cycles through)
- **Check stats** to optimize workflow
- **Filter sensitive commands** in config
- **Backup database** periodically
- **Use Tab** to edit before executing
- **Self-host** for complete privacy

## Agent Use
- Command history analysis
- Workflow optimization
- Team command sharing (self-hosted)
- Audit logging
- Command pattern discovery
- Productivity metrics

## Uninstall
```yaml
- preset: atuin
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/atuinsh/atuin
- Docs: https://atuin.sh/
- Search: "atuin shell history", "atuin sync"

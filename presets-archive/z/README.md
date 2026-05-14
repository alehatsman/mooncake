# z - Smart Directory Navigation

A shell extension that tracks your most frequently used directories and lets you jump to them using partial names (frecency-based).

## Quick Start
```yaml
- preset: z
```

## Features
- **Frecency-based navigation**: Combines frequency and recency to rank directories
- **Partial matching**: Jump to directories with just a few characters
- **No configuration needed**: Automatically learns from your usage
- **Shell integration**: Works seamlessly with bash, zsh
- **Fast**: Implemented as a shell script for minimal overhead

## Basic Usage
```bash
# After z learns your directories (by cd'ing into them)...

# Jump to most frecent directory matching "project"
z project

# Jump to directory matching multiple keywords
z work project

# Show list of matching directories without jumping
z -l project

# Jump to highest-ranked subdirectory of current directory
z -c subdir

# Jump to most recent directory (instead of most frecent)
z -t project

# Jump by rank instead of frecency
z -r project

# Remove current directory from z database
z -x .

# List all tracked directories
z -l
```

## How It Works

z maintains a database of directories you visit, ranking them by **frecency** (frequency + recency):
1. Every time you `cd` into a directory, z increments its rank
2. Directories you visit often and recently appear first
3. Use `z <partial-name>` to jump to the best match

**Example workflow:**
```bash
# Normal cd commands - z learns in the background
cd ~/projects/web-app
cd ~/projects/mobile-app
cd ~/work/client-project

# Later, jump with partial names
z web        # Goes to ~/projects/web-app
z mobile     # Goes to ~/projects/mobile-app
z client     # Goes to ~/work/client-project
```

## Advanced Configuration
```yaml
# Basic installation
- preset: z
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (bash, zsh)
- ✅ macOS (bash, zsh)
- ❌ Windows (not supported)

## Configuration
- **Installation**: Adds source line to `~/.bashrc` or `~/.zshrc`
- **Database**: `~/.z` (plain text file tracking directory rankings)
- **Script location**: Varies by installation method

## Real-World Examples

### Daily Development Workflow
```bash
# Jump to frequently accessed project directories
z api       # Jump to backend API project
z frontend  # Jump to frontend project
z config    # Jump to configuration directory
```

### Project Navigation
```bash
# Working across multiple projects
z proj blog    # Jump to ~/projects/my-blog
z proj ecom    # Jump to ~/projects/ecommerce-site
z proj tool    # Jump to ~/projects/dev-tools
```

### Quick Access to Common Paths
```bash
# Jump to deeply nested directories quickly
z logs         # Instead of cd /var/log/application/production/logs
z nginx        # Instead of cd /etc/nginx/sites-available
z tmp          # Instead of cd /var/tmp/work-dir
```

## Agent Use
- Navigate to project directories in automated scripts
- Jump to common paths in CI/CD environments (after initial setup)
- Simplify navigation in interactive terminal sessions
- Reduce path typing in development workflows
- Enable context-aware directory switching

## Troubleshooting

### z command not found
Restart your shell or source your rc file:
```bash
source ~/.zshrc  # or ~/.bashrc
```

### z not tracking directories
Make sure you're using `cd` to change directories (z hooks into cd):
```bash
# This is tracked
cd ~/projects/myapp

# This is NOT tracked (direct shell command)
builtin cd ~/projects/myapp
```

### Wrong directory selected
z ranks by frecency. If wrong directory is chosen:
```bash
# List all matches
z -l keyword

# Use more specific keywords
z proj mobile  # Instead of just "mobile"

# Remove incorrect entry
z -x /path/to/wrong/directory
```

### Database cleanup
Remove old/deleted directories:
```bash
# z automatically cleans entries for deleted directories
# Force cleanup by trying to jump to deleted paths
z deleted-dir  # Will show error and remove from database
```

## Uninstall
```yaml
- preset: z
  with:
    state: absent
```

**Note**: Manual cleanup may be required:
```bash
# Remove database
rm ~/.z

# Remove from shell config
# Edit ~/.zshrc or ~/.bashrc and remove z source line
```

## Resources
- GitHub: https://github.com/rupa/z
- Search: "z shell navigation", "z vs autojump", "frecency directory jumping"

**Alternatives**: Consider **zoxide** (modern Rust rewrite with more features) or **autojump** (Python-based alternative).

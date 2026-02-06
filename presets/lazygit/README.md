# lazygit Preset

Install lazygit - a simple terminal UI for git commands with an intuitive interface for staging, committing, branching, and more.

## Quick Start

```yaml
# Basic installation
- preset: lazygit

# With default configuration
- preset: lazygit
  with:
    create_config: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | Install (`present`) or uninstall (`absent`) |
| `create_config` | bool | `false` | Create default configuration file |

## Usage

### Launch lazygit

```bash
# In current git repository
lazygit

# In specific repository
lazygit -p ~/myproject

# With custom config
lazygit --use-config-file ~/.config/lazygit/custom.yml
```

## Key Features

### Files Panel
- **Stage/unstage files**: Press `space` on individual files
- **Stage all**: Press `a`
- **Discard changes**: Press `d`
- **Ignore file**: Press `i`
- **View diff**: Press `enter`
- **Stage hunks**: Enter file with `enter`, then `space` on individual hunks

### Commits Panel
- **View commit diff**: Press `enter`
- **Checkout commit**: Press `space`
- **Cherry-pick**: Press `c`
- **Revert**: Press `t`
- **Reset to commit**: Press `g` then choose reset type
- **Copy commit SHA**: Press `Ctrl+o`
- **Create tag**: Press `T`

### Branches Panel
- **Create branch**: Press `n`
- **Checkout branch**: Press `space`
- **Merge into current**: Press `M`
- **Rebase current onto**: Press `r`
- **Delete branch**: Press `d`
- **Rename branch**: Press `R`
- **Set upstream**: Press `u`

### Remotes Panel
- **Fetch**: Press `f`
- **Pull**: Press `g` then `p`
- **Push**: Press `P` or `g` then `P`
- **Force push**: Press `P` then select force push

### Stash Panel
- **Create stash**: Press `s` in files panel
- **Apply stash**: Press `space`
- **Pop stash**: Press `g` then `a`
- **Drop stash**: Press `d`
- **Rename stash**: Press `r`

## Navigation

### Panel Navigation
| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next/previous panel |
| `1-5` | Jump to specific panel |
| `[` / `]` | Previous/next tab |
| `h` / `l` | Previous/next panel (vim-style) |

### Within Panels
| Key | Action |
|-----|--------|
| `j` / `k` or `↓` / `↑` | Move down/up |
| `g` / `G` | Jump to top/bottom |
| `Ctrl+u` / `Ctrl+d` | Scroll half page up/down |
| `<` / `>` | Scroll to top/bottom |
| `Enter` | View details/select |
| `Space` | Stage/checkout/toggle |

## Common Operations

### Staging and Committing

```bash
# 1. Navigate to Files panel (panel 1)
# 2. Use j/k to move, space to stage
# 3. Press 'c' to commit
# 4. Type commit message
# 5. Press Enter to confirm
```

### Interactive Rebase

```bash
# 1. Go to Commits panel (panel 2)
# 2. Navigate to commit where you want to start
# 3. Press 'i' to start interactive rebase
# 4. Use j/k to move commits
# 5. Press 'e' to edit, 's' to squash, 'd' to drop
# 6. Press 'Enter' to continue rebase
```

### Resolving Merge Conflicts

```bash
# 1. After merge conflict, lazygit shows conflicted files
# 2. Press Enter on conflicted file
# 3. Navigate with j/k, choose version with Space
# 4. Press 'Esc' when done
# 5. Stage resolved file with Space
# 6. Continue merge with 'g' then 'c'
```

### Creating and Switching Branches

```bash
# Create new branch:
# 1. Go to Branches panel (panel 3)
# 2. Press 'n'
# 3. Type branch name
# 4. Press Enter

# Switch branches:
# 1. Go to Branches panel
# 2. Use j/k to navigate
# 3. Press Space to checkout
```

## Advanced Features

### Filtering Commits

```bash
# In Commits panel
# Press '/' to search
# Type search term
# Press 'n' / 'N' for next/previous match
```

### Custom Commands

Add to `~/.config/lazygit/config.yml`:

```yaml
customCommands:
  - key: 'ctrl+a'
    command: 'git add -A'
    context: 'files'
    description: 'Stage all files'

  - key: 'C'
    command: 'git commit --amend --no-edit'
    context: 'files'
    description: 'Amend commit without editing message'

  - key: 'P'
    command: 'git push --force-with-lease'
    context: 'branches'
    subprocess: true
    description: 'Force push with lease'
```

### Git Flow Integration

```yaml
customCommands:
  - key: 'F'
    command: 'git flow feature start {{index .PromptResponses 0}}'
    context: 'branches'
    prompts:
      - type: 'input'
        title: 'Feature name'
    description: 'Start git flow feature'
```

## Configuration

### Location

- **Linux**: `~/.config/lazygit/config.yml`
- **macOS**: `~/.config/lazygit/config.yml`
- **Windows**: `%APPDATA%\lazygit\config.yml`

### Example Configuration

```yaml
gui:
  theme:
    activeBorderColor:
      - green
      - bold
    inactiveBorderColor:
      - white
  showFileTree: true
  showRandomTip: true
  showCommandLog: true
  commandLogSize: 8

git:
  paging:
    colorArg: always
    pager: delta --dark --paging=never

  commit:
    signOff: false

  merging:
    manualCommit: false

  log:
    showGraph: 'always'
    order: 'topo-order'

  autoFetch: true
  autoRefresh: true

update:
  method: prompt

refresher:
  refreshInterval: 10
  fetchInterval: 60

confirmOnQuit: false
```

### Delta Integration

```yaml
git:
  paging:
    colorArg: always
    pager: delta --dark --paging=never
```

### Custom Themes

```yaml
gui:
  theme:
    # Light theme
    lightTheme: true
    activeBorderColor:
      - blue
      - bold
    inactiveBorderColor:
      - default

    # Or use Nord theme
    activeBorderColor:
      - '#88C0D0'
      - bold
    inactiveBorderColor:
      - '#4C566A'
```

## Keybinding Reference

### Universal
| Key | Action |
|-----|--------|
| `?` | Open help menu |
| `x` | Open menu for current item |
| `q` | Quit |
| `Esc` | Cancel/return |
| `R` | Refresh |
| `:` | Execute custom command |
| `z` | Undo |
| `Ctrl+z` | Redo |

### Files
| Key | Action |
|-----|--------|
| `Space` | Stage/unstage |
| `a` | Stage all |
| `A` | Unstage all |
| `c` | Commit |
| `C` | Commit with message from clipboard |
| `d` | Discard changes |
| `s` | Stash all |
| `i` | Add to .gitignore |
| `e` | Edit file |
| `o` | Open file |
| `S` | View stash options |

### Commits
| Key | Action |
|-----|--------|
| `Space` | Checkout commit |
| `c` | Copy commit SHA |
| `C` | Copy commit message |
| `t` | Revert commit |
| `r` | Reword commit |
| `g` | Reset to commit |
| `T` | Create tag |
| `Enter` | View files in commit |

### Branches
| Key | Action |
|-----|--------|
| `Space` | Checkout branch |
| `n` | New branch |
| `d` | Delete branch |
| `D` | Force delete branch |
| `r` | Rebase branch |
| `M` | Merge into current branch |
| `f` | Fast-forward merge |
| `R` | Rename branch |
| `u` | Set upstream |
| `P` | Push branch |

## Integration with Tools

### With GitHub CLI (gh)

```yaml
customCommands:
  - key: 'ctrl+p'
    command: 'gh pr create --web'
    context: 'global'
    description: 'Create pull request'

  - key: 'ctrl+o'
    command: 'gh pr view --web'
    context: 'global'
    description: 'Open PR in browser'
```

### With pre-commit

lazygit respects `.git/hooks/` and runs pre-commit hooks automatically.

### With GPG Signing

```yaml
git:
  commit:
    signOff: false
  # GPG signing is configured via git config
```

## Tips and Tricks

1. **Quick commit**: Stage files and press `c` followed by your message
2. **Amend last commit**: Press `A` in files panel
3. **Interactive staging**: Enter file with `Enter`, stage hunks with `Space`
4. **View file history**: Select file, press `Enter`, then `Ctrl+l`
5. **Compare branches**: Switch to Branches panel, select two branches
6. **Undo in lazygit**: Press `z` to undo last action

## Troubleshooting

### lazygit doesn't start

```bash
# Check git repository
git status

# Check lazygit installation
lazygit --version

# Run with debug
lazygit --debug
```

### Keybindings not working

```bash
# Check config file syntax
lazygit --help

# Reset to default config
mv ~/.config/lazygit/config.yml ~/.config/lazygit/config.yml.bak
```

### Delta/pager issues

```yaml
# Disable custom pager
git:
  paging:
    pager: ''
```

## Uninstall

```yaml
- preset: lazygit
  with:
    state: absent
```

Configuration will be preserved at `~/.config/lazygit/`

## Resources

- **GitHub**: https://github.com/jesseduffield/lazygit
- **Docs**: https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md
- **Keybindings**: https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md
- **Videos**: https://www.youtube.com/results?search_query=lazygit+tutorial

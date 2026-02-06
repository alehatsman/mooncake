# lazygit - Terminal UI for Git

Simple terminal UI for git commands with keyboard shortcuts, visual branch graphs, and intuitive workflows.

## Quick Start
```yaml
- preset: lazygit
```

## Features
- **Interactive interface**: Mouse and keyboard navigation
- **Visual branch tree**: ASCII branch visualization
- **Staging made easy**: Stage files, hunks, or individual lines
- **Commit management**: Amend, reword, squash, rebase
- **Fast workflows**: Common git operations with single keystrokes
- **Customizable**: Themes, keybindings, and custom commands

## Basic Usage
```bash
# Launch lazygit
lazygit

# Keyboard shortcuts (in UI):
# 1-5 - switch panels (status, files, branches, commits, stash)
# space - stage/unstage file or hunk
# a - stage all
# c - commit
# P - push
# p - pull
# n - new branch
# m - merge
# r - rebase
# d - delete/drop
# e - edit file
# o - open file
# ? - help
# q - quit
```

## Advanced Configuration
```yaml
- preset: lazygit
  with:
    state: present
    create_config: true          # Create default config
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove lazygit |
| create_config | bool | false | Create default configuration file |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ✅ Windows (Scoop, Chocolatey)

## Configuration
- **Config file**: `~/.config/lazygit/config.yml` (Linux), `~/Library/Application Support/lazygit/config.yml` (macOS)
- **Themes**: Built-in themes or custom color schemes
- **Custom commands**: Define in config file

## Real-World Examples

### Quick Commit Workflow
```bash
# 1. Launch lazygit
lazygit

# 2. Stage changes (space on files)
# 3. Press 'c' to commit
# 4. Write message, save and close
# 5. Press 'P' to push
```

### Interactive Rebase
```bash
# 1. Go to commits panel (press 2)
# 2. Select commits to rebase
# 3. Press 'i' for interactive rebase
# 4. Use 's' to squash, 'r' to reword, 'd' to drop
# 5. Confirm with enter
```

### Branch Management
```bash
# 1. Press 3 for branches panel
# 2. Navigate with j/k
# 3. Press 'n' for new branch
# 4. Press 'm' to merge
# 5. Press 'd' to delete
```

### Stash Operations
```bash
# 1. Press 5 for stash panel
# 2. Stage changes you want to keep
# 3. Press 's' to stash unstaged
# 4. Later: press 'g' to pop stash
```

## Agent Use
- Rapid git operations in development workflows
- Visual conflict resolution
- Interactive commit history management
- Branch strategy enforcement
- Code review preparation

## Troubleshooting

### Config file not found
Create default config:
```bash
mkdir -p ~/.config/lazygit
touch ~/.config/lazygit/config.yml
```

### Cannot push/pull
Check git remote:
```bash
git remote -v
git remote set-url origin <url>
```

### Merge conflicts
lazygit shows conflicts visually:
1. Navigate to conflicted files
2. Press 'e' to edit in $EDITOR
3. Resolve conflicts
4. Stage with space
5. Commit with 'c'

## Uninstall
```yaml
- preset: lazygit
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/jesseduffield/lazygit
- Documentation: https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md
- Keybindings: https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md
- Search: "lazygit tutorial", "git terminal ui"

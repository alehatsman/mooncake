# fish - Friendly Interactive Shell

Smart and user-friendly command line shell with autosuggestions, syntax highlighting, and web-based configuration. No configuration required to be productive.

## Quick Start
```yaml
- preset: fish
```

## Features
- **Autosuggestions**: Suggests commands as you type based on history
- **Syntax highlighting**: Real-time validation of commands before execution
- **Web-based configuration**: Configure via browser at `fish_config`
- **No configuration needed**: Sensible defaults work out of the box
- **Cross-platform**: Linux, macOS, BSD support

## Basic Usage
```bash
# Launch fish shell
fish

# Set as default shell
chsh -s $(which fish)

# Configure via web interface
fish_config

# Check configuration
fish --version
echo $SHELL
```

## Advanced Configuration
```yaml
- preset: fish
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove fish |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ BSD (pkg)
- ❌ Windows (WSL only)

## Configuration
- **Config file**: `~/.config/fish/config.fish`
- **Functions directory**: `~/.config/fish/functions/`
- **Completions**: `~/.config/fish/completions/`
- **Configuration**: Run `fish_config` to open web interface

## Real-World Examples

### Custom Prompt with Git Status
```fish
# ~/.config/fish/config.fish
function fish_prompt
    set_color blue
    echo -n (prompt_pwd)
    set_color normal
    echo -n (fish_git_prompt)
    echo -n ' > '
end
```

### Useful Aliases
```fish
# ~/.config/fish/config.fish
alias gs='git status'
alias gp='git pull'
alias ll='ls -lah'
alias k='kubectl'
```

### Custom Function
```fish
# ~/.config/fish/functions/mkcd.fish
function mkcd
    mkdir -p $argv[1]
    cd $argv[1]
end
```

### Environment Variables
```fish
# ~/.config/fish/config.fish
set -x EDITOR vim
set -x GOPATH ~/go
set -gx PATH $PATH $GOPATH/bin
```

## Agent Use
- Provide interactive shell for development environments
- Configure user shells in automated provisioning
- Setup developer workstations with modern shell features
- Enable shell autosuggestions for improved CLI productivity
- Install in containers for debugging sessions

## Troubleshooting

### Command not found after switching shells
```bash
# Add directories to PATH in fish config
# ~/.config/fish/config.fish
set -gx PATH /usr/local/bin $PATH
set -gx PATH $HOME/.local/bin $PATH
```

### Switch back to previous shell
```bash
# Change default shell back to bash/zsh
chsh -s /bin/bash
# or
chsh -s /bin/zsh
```

### Disable autosuggestions
```fish
# Disable in current session
set -e fish_autosuggestion_enabled

# Permanently disable (add to config.fish)
set -U fish_autosuggestion_enabled 0
```

### Import bash aliases
```fish
# Convert bash aliases to fish functions
# Instead of: alias ll='ls -lah'
# Use:
function ll
    ls -lah $argv
end
```

## Uninstall
```yaml
- preset: fish
  with:
    state: absent
```

## Resources
- Official docs: https://fishshell.com/docs/current/
- GitHub: https://github.com/fish-shell/fish-shell
- Configuration examples: https://github.com/fish-shell/fish-shell/wiki/Cookbook
- Search: "fish shell tutorial", "fish shell vs zsh", "fish shell configuration"

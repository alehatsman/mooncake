# starship - Cross-Shell Prompt

Minimal, fast, and infinitely customizable prompt for any shell. Beautiful terminal experience with smart information display.

## Quick Start
```yaml
- preset: starship
```

## Features
- **Cross-shell**: Bash, Zsh, Fish, PowerShell, Ion, Elvish, Tcsh, Nushell, Xonsh
- **Fast**: Written in Rust, instant prompt rendering
- **Git-aware**: Branch, status, stash count, ahead/behind indicators
- **Language versions**: Auto-detects Node, Python, Rust, Go, Java, and 40+ languages
- **Cloud contexts**: AWS, Azure, GCP, Kubernetes current context
- **Customizable**: 100+ modules, extensive TOML configuration
- **Smart**: Only shows relevant information (project language when in project directory)

## Basic Usage
```bash
# Starship initializes on shell start (configured in .bashrc/.zshrc)
# No direct commands - configuration via starship.toml

# Apply preset theme
starship preset nerd-font-symbols -o ~/.config/starship.toml

# Show all presets
starship preset list
```

## Advanced Configuration
```yaml
# Install starship (default)
- preset: starship

# Install with shell auto-configuration
- preset: starship
  with:
    configure_shell: true              # Auto-configure shell init file

# Install with preset theme
- preset: starship
  with:
    preset: nerd-font-symbols          # Apply Nerd Fonts theme

# Uninstall starship
- preset: starship
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| configure_shell | bool | true | Auto-configure shell init file |
| preset | string | - | Apply preset theme (nerd-font-symbols, bracketed-segments, etc.) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Windows (scoop, choco)

## Configuration
- **Config file**: `~/.config/starship.toml` (Linux), `~/Library/Application Support/starship/starship.toml` (macOS)
- **Shell init**: Automatically added to `.bashrc`, `.zshrc`, `config.fish`, or `profile.ps1`
- **Presets**: Built-in themes via `starship preset`

## Custom Configuration
```toml
# ~/.config/starship.toml

# Minimal prompt
[line_break]
disabled = true

# Custom prompt character
[character]
success_symbol = "[➜](bold green)"
error_symbol = "[✗](bold red)"

# Show command duration for slow commands
[cmd_duration]
min_time = 500
format = "took [$duration](bold yellow)"

# Git branch styling
[git_branch]
style = "bold blue"
symbol = " "

# Directory settings
[directory]
truncation_length = 3
truncate_to_repo = true
style = "bold cyan"

# Show Python version when in Python project
[python]
format = "via [${symbol}${pyenv_prefix}(${version} )]($style)"
symbol = " "

# Kubernetes context
[kubernetes]
disabled = false
format = "on [⛵ $context](dimmed green)"

# AWS profile
[aws]
format = "on [$symbol($profile )]($style)"
symbol = "  "
```

## Preset Themes
```bash
# Nerd Fonts required
starship preset nerd-font-symbols -o ~/.config/starship.toml

# Bracketed style
starship preset bracketed-segments -o ~/.config/starship.toml

# No special fonts
starship preset plain-text-symbols -o ~/.config/starship.toml

# Hide runtime versions
starship preset no-runtime-versions -o ~/.config/starship.toml

# Pure-inspired
starship preset pure-preset -o ~/.config/starship.toml

# Powerline style
starship preset pastel-powerline -o ~/.config/starship.toml

# List all presets
starship preset list
```

## Common Modules
```toml
# Git status
[git_status]
conflicted = " "
ahead = "⇡${count}"
behind = "⇣${count}"
diverged = "⇕⇡${ahead_count}⇣${behind_count}"
untracked = "?${count}"
stashed = "$${count}"
modified = "!${count}"
staged = "+${count}"
renamed = "»${count}"
deleted = "✘${count}"

# Node.js
[nodejs]
format = "via [ $version](bold green)"

# Rust
[rust]
format = "via [ $version](bold red)"

# Go
[golang]
format = "via [ $version](bold cyan)"

# Docker context
[docker_context]
format = "via [ $context](bold blue)"

# Battery indicator
[battery]
full_symbol = ""
charging_symbol = ""
discharging_symbol = ""

[[battery.display]]
threshold = 30
style = "bold red"
```

## Advanced Features

### Custom Modules
```toml
# Custom command output
[custom.git_email]
command = "git config user.email"
when = "git rev-parse --git-dir 2> /dev/null"
format = "[$output]($style)"
style = "dimmed white"
```

### Environment Variables
```toml
# Show environment variable
[env_var.SHELL]
variable = "SHELL"
format = "with [$env_value]($style)"
style = "dimmed blue"
```

### Right-aligned Modules
```toml
# Right prompt
format = "$all$character"
right_format = "$time"

[time]
disabled = false
format = "[$time]($style)"
```

### Line Break Control
```toml
# Two-line prompt
[line_break]
disabled = false

# Single-line prompt
[line_break]
disabled = true
```

## Real-World Examples

### Developer Prompt
```toml
[character]
success_symbol = "[➜](bold green)"
error_symbol = "[✗](bold red)"

[directory]
truncation_length = 3
truncate_to_repo = true
fish_style_pwd_dir_length = 1

[git_branch]
symbol = " "

[git_status]
ahead = "⇡${count}"
behind = "⇣${count}"
diverged = "⇕"
modified = "!${count}"
untracked = "?${count}"
staged = "+${count}"

[nodejs]
symbol = " "

[python]
symbol = " "

[rust]
symbol = " "
```

### Minimal Prompt
```toml
[character]
success_symbol = "[λ](bold green)"
error_symbol = "[λ](bold red)"

[line_break]
disabled = true

[directory]
truncation_length = 1
```

### Cloud Ops Prompt
```toml
[kubernetes]
disabled = false
format = "on [⛵ $context\\($namespace\\)](dimmed green)"

[aws]
format = "on [$symbol($profile )]($style)"

[gcloud]
format = "on [$symbol$account(@$domain)\\($project\\)]($style)"

[terraform]
format = "[🏗️ $workspace]($style)"
```

## Shell Integration

### Bash
```bash
# ~/.bashrc
eval "$(starship init bash)"
```

### Zsh
```zsh
# ~/.zshrc
eval "$(starship init zsh)"
```

### Fish
```fish
# ~/.config/fish/config.fish
starship init fish | source
```

### PowerShell
```powershell
# $PROFILE
Invoke-Expression (&starship init powershell)
```

## Performance Tips
- **Disable unused modules** to speed up prompt rendering
- **Use when conditions** to only show modules when relevant
- **Limit scan_timeout** for slow commands
- **Cache expensive operations** via custom modules

```toml
# Disable unused modules
[package]
disabled = true

[elixir]
disabled = true

# Limit scan timeout
[cmd_duration]
min_time = 500
scan_timeout = 10
```

## Troubleshooting

### Icons not displaying
Install a Nerd Font:
```bash
# Recommended fonts
brew tap homebrew/cask-fonts
brew install font-fira-code-nerd-font
brew install font-hack-nerd-font
```

### Slow prompt
Check which modules are slow:
```bash
starship timings
```

Disable slow modules in `starship.toml`.

## Agent Use
- Consistent terminal experience across environments
- Visual feedback for development context (git, language, cloud)
- Automated prompt configuration in provisioning scripts
- Environment-aware information display
- Custom modules for deployment status

## Uninstall
```yaml
- preset: starship
  with:
    state: absent
```

**Note:** Configuration file (`~/.config/starship.toml`) is preserved after uninstall.

## Resources
- Official docs: https://starship.rs/
- Configuration: https://starship.rs/config/
- Presets: https://starship.rs/presets/
- Search: "starship prompt examples", "starship configuration"

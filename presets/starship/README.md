# Starship Preset

Minimal, fast, cross-shell prompt. Beautiful, informative, and highly customizable terminal prompt.

## Quick Start

```yaml
- preset: starship

# With preset theme
- preset: starship
  with:
    preset: nerd-font-symbols
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `configure_shell` | Auto-configure shell (default: true) |
| `preset` | Theme preset (default, nerd-font-symbols, etc.) |

## Presets

```bash
starship preset nerd-font-symbols        # Nerd fonts required
starship preset bracketed-segments       # Bracketed style
starship preset plain-text-symbols       # No special fonts
starship preset no-runtime-versions      # Hide versions
starship preset pure-preset              # Pure-inspired
starship preset pastel-powerline         # Powerline style
```

## Configuration

Location: `~/.config/starship.toml`

```toml
# Minimal prompt
[line_break]
disabled = true

# Custom prompt character
[character]
success_symbol = "[➜](bold green)"
error_symbol = "[✗](bold red)"

# Show command duration
[cmd_duration]
min_time = 500

# Git branch styling
[git_branch]
style = "bold blue"

# Custom modules
[directory]
truncation_length = 3
style = "bold cyan"
```

## Features

- **Fast**: Rust-powered, instant prompt
- **Git-aware**: Branch, status, stash count
- **Language versions**: Node, Python, Rust, Go, etc.
- **Cloud contexts**: AWS, Azure, GCP, K8s
- **Custom modules**: Any command output
- **Cross-shell**: Bash, Zsh, Fish, PowerShell

## Resources
- Docs: https://starship.rs/
- Config: https://starship.rs/config/

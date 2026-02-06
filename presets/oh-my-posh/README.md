# Oh My Posh - Prompt Theme Engine

Customizable prompt theme engine for any shell with beautiful themes and Git integration.

## Quick Start

```yaml
- preset: oh-my-posh
```

## Features

- **Cross-platform**: Works on Linux, macOS, Windows
- **Shell agnostic**: Bash, Zsh, Fish, PowerShell, and more
- **Theme library**: Hundreds of pre-built themes
- **Git integration**: Show branch, status, ahead/behind
- **Fast**: Written in Go for performance
- **Customizable**: JSON-based theme configuration
- **Powerline**: Supports Nerd Fonts and Powerline glyphs

## Basic Usage

```bash
# Check version
oh-my-posh --version

# Preview theme
oh-my-posh init bash --config ~/.poshthemes/jandedobbeleer.omp.json

# List available themes
oh-my-posh get themes

# Apply theme to bash
eval "$(oh-my-posh init bash --config ~/.poshthemes/jandedobbeleer.omp.json)"

# Apply theme to zsh
eval "$(oh-my-posh init zsh --config ~/.poshthemes/agnoster.omp.json)"

# Export theme to PNG
oh-my-posh config export image --config ~/.mytheme.omp.json --output theme.png
```

## Advanced Configuration

```yaml
# Install Oh My Posh
- preset: oh-my-posh

# Download themes
- name: Get Oh My Posh themes
  shell: |
    mkdir -p ~/.poshthemes
    oh-my-posh get themes --output ~/.poshthemes

# Configure for Bash
- name: Add Oh My Posh to bashrc
  lineinfile:
    path: ~/.bashrc
    line: 'eval "$(oh-my-posh init bash --config ~/.poshthemes/{{ theme }}.omp.json)"'
    create: true

# Configure for Zsh
- name: Add Oh My Posh to zshrc
  lineinfile:
    path: ~/.zshrc
    line: 'eval "$(oh-my-posh init zsh --config ~/.poshthemes/{{ theme }}.omp.json)"'
    create: true

# Deploy custom theme
- name: Install custom theme
  template:
    src_template: mytheme.omp.json.j2
    dest: ~/.mytheme.omp.json
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Oh My Posh |

## Platform Support

- ✅ Linux (Homebrew, binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Windows (winget, scoop, manual install)

## Configuration

- **Config file**: JSON theme file (e.g., `~/.mytheme.omp.json`)
- **Themes**: `~/.poshthemes/` (downloaded themes)
- **Nerd Fonts**: Required for icons and glyphs

## Real-World Examples

### Bash Configuration
```bash
# ~/.bashrc
eval "$(oh-my-posh init bash --config ~/.poshthemes/paradox.omp.json)"
```

### Zsh Configuration
```bash
# ~/.zshrc
eval "$(oh-my-posh init zsh --config ~/.poshthemes/agnoster.omp.json)"
```

### Custom Theme
```json
{
  "$schema": "https://raw.githubusercontent.com/JanDeDobbeleer/oh-my-posh/main/themes/schema.json",
  "blocks": [
    {
      "type": "prompt",
      "alignment": "left",
      "segments": [
        {
          "type": "path",
          "style": "powerline",
          "powerline_symbol": "\uE0B0",
          "foreground": "#ffffff",
          "background": "#0077c2",
          "properties": {
            "style": "folder"
          }
        },
        {
          "type": "git",
          "style": "powerline",
          "powerline_symbol": "\uE0B0",
          "foreground": "#ffffff",
          "background": "#00c853",
          "properties": {
            "branch_icon": "\uE0A0 ",
            "fetch_status": true
          }
        }
      ]
    }
  ]
}
```

### Development Environment Setup
```yaml
# Setup developer workstation
- name: Install Oh My Posh
  preset: oh-my-posh

- name: Install Nerd Font
  shell: |
    mkdir -p ~/.local/share/fonts
    cd ~/.local/share/fonts
    curl -fLo "Hack Nerd Font.ttf" \
      https://github.com/ryanoasis/nerd-fonts/raw/master/patched-fonts/Hack/Regular/complete/Hack%20Regular%20Nerd%20Font%20Complete.ttf
    fc-cache -fv

- name: Download themes
  shell: oh-my-posh get themes --output ~/.poshthemes

- name: Configure shell
  lineinfile:
    path: ~/.zshrc
    line: 'eval "$(oh-my-posh init zsh --config ~/.poshthemes/atomic.omp.json)"'
```

### CI/CD Theme Deployment
```yaml
# Standardize terminal prompts across team
- name: Install Oh My Posh
  preset: oh-my-posh

- name: Deploy company theme
  copy:
    src: themes/company.omp.json
    dest: ~/.company-theme.omp.json

- name: Configure for all shells
  blockinfile:
    path: "{{ item }}"
    block: |
      # Oh My Posh
      eval "$(oh-my-posh init {{ item | basename | regex_replace('rc$', '') }} --config ~/.company-theme.omp.json)"
    create: true
  loop:
    - ~/.bashrc
    - ~/.zshrc
```

## Popular Themes

- **agnoster**: Classic theme with Git support
- **atomic**: Clean modern look
- **jandedobbeleer**: Default Oh My Posh theme
- **paradox**: Minimal and fast
- **powerlevel10k_rainbow**: Colorful with lots of info
- **spaceship**: Inspired by Spaceship prompt

## Theme Segments

Common segments you can add to your theme:

- **path**: Current directory
- **git**: Git repository status
- **time**: Current time
- **kubectl**: Kubernetes context
- **aws**: AWS profile
- **node**: Node.js version
- **python**: Python version
- **go**: Go version
- **docker**: Docker context
- **exit**: Last command exit code

## Agent Use

- Standardize terminal prompts across development teams
- Deploy custom themes with company branding
- Provide context-aware information (Git, K8s, AWS)
- Improve developer experience with visual cues
- Automate prompt configuration in workstation setups
- Create environment-specific prompt indicators

## Troubleshooting

### Icons not showing
```bash
# Install a Nerd Font
# Download from: https://www.nerdfonts.com/

# Configure terminal to use Nerd Font
# Terminal → Preferences → Font → "Hack Nerd Font"
```

### Prompt not updating
```bash
# Reload shell configuration
source ~/.bashrc  # or ~/.zshrc

# Verify Oh My Posh is installed
which oh-my-posh

# Check theme file exists
ls -la ~/.poshthemes/
```

### Slow prompt
```bash
# Disable slow segments in theme
# Edit theme JSON and remove expensive segments like kubectl, aws

# Or use a minimal theme
eval "$(oh-my-posh init bash --config ~/.poshthemes/pure.omp.json)"
```

## Uninstall

```yaml
- preset: oh-my-posh
  with:
    state: absent
```

## Resources

- Official docs: https://ohmyposh.dev/docs/
- Themes: https://ohmyposh.dev/docs/themes
- GitHub: https://github.com/JanDeDobbeleer/oh-my-posh
- Nerd Fonts: https://www.nerdfonts.com/
- Search: "oh my posh tutorial", "oh my posh themes", "oh my posh custom theme"

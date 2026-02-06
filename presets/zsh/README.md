# Zsh Preset

Install Zsh shell with Oh My Zsh framework for a better terminal experience.

## Quick Start

```yaml
- preset: zsh
  with:
    install_ohmyzsh: true
    theme: "agnoster"
    plugins: ["git", "docker", "kubectl", "terraform"]
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `install_ohmyzsh` | bool | `true` | Install Oh My Zsh |
| `theme` | string | `robbyrussell` | Oh My Zsh theme |
| `plugins` | array | `["git", "docker", "kubectl"]` | Plugins to enable |
| `set_default_shell` | bool | `true` | Set as default shell |

## Popular Themes

- `robbyrussell` - Default, clean
- `agnoster` - Powerline-style
- `powerlevel10k` - Modern, fast (requires fonts)
- `pure` - Minimal
- `spaceship` - Customizable prompt

## Popular Plugins

- `git` - Git aliases and completions
- `docker` - Docker completions
- `kubectl` - Kubernetes completions
- `terraform` - Terraform completions
- `aws` - AWS CLI completions
- `npm` - NPM completions
- `python` - Python completions
- `ruby` - Ruby completions
- `zsh-autosuggestions` - Fish-like suggestions
- `zsh-syntax-highlighting` - Syntax highlighting

## Usage

### Basic Setup
```yaml
- preset: zsh
```

### Developer Setup
```yaml
- preset: zsh
  with:
    theme: "powerlevel10k"
    plugins:
      - git
      - docker
      - kubectl
      - terraform
      - aws
      - npm
      - python
```

### Minimal Setup
```yaml
- preset: zsh
  with:
    install_ohmyzsh: false
```

## Customization

Edit `~/.zshrc`:

```bash
# Change theme
ZSH_THEME="agnoster"

# Add plugins
plugins=(git docker kubectl terraform aws)

# Custom aliases
alias k="kubectl"
alias tf="terraform"
alias dc="docker-compose"

# Environment variables
export EDITOR=nvim
```

## Install Additional Plugins

```bash
# zsh-autosuggestions
git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions

# zsh-syntax-highlighting
git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting

# Add to .zshrc
plugins=(... zsh-autosuggestions zsh-syntax-highlighting)
```

## Powerlevel10k Setup

After installation with `theme: "powerlevel10k"`:

```bash
# Configure theme
p10k configure

# Edit settings
nano ~/.p10k.zsh
```

**Note:** Requires Nerd Fonts. Install:
```bash
# macOS
brew tap homebrew/cask-fonts
brew install font-meslo-lg-nerd-font

# Set in terminal preferences
```

## Useful Aliases

Oh My Zsh includes many aliases:

```bash
# Git
gst    # git status
ga     # git add
gc     # git commit
gp     # git push
gl     # git pull

# Directory navigation
..     # cd ..
...    # cd ../..
~      # cd ~

# List
l      # ls -lah
la     # ls -lAh
ll     # ls -lh
```

## Tips

1. **Reload config**: `source ~/.zshrc`
2. **Update Oh My Zsh**: `omz update`
3. **List themes**: `ls ~/.oh-my-zsh/themes`
4. **List plugins**: `ls ~/.oh-my-zsh/plugins`

## Uninstall

```yaml
- preset: zsh
  with:
    state: absent
```

**Note:** Reverts to bash shell.

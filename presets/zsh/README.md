# Zsh - Z Shell with Oh My Zsh

A powerful Unix shell with extensive customization, plugin support, and the Oh My Zsh framework for enhanced productivity.

## Quick Start
```yaml
- preset: zsh
```

## Features
- **Auto-completion**: Intelligent tab completion for commands and arguments
- **Themes**: Beautiful prompts with Git status, path shortening, and colors
- **Plugin ecosystem**: 300+ plugins for Git, Docker, cloud tools, and more
- **Spelling correction**: Suggests corrections for mistyped commands
- **History sharing**: Share command history across terminal sessions
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# After installation, Zsh becomes your default shell

# Reload configuration
source ~/.zshrc

# Update Oh My Zsh
omz update

# List available themes
ls ~/.oh-my-zsh/themes

# List available plugins
ls ~/.oh-my-zsh/plugins

# Switch themes temporarily
prompt <theme-name>
```

## Built-in Aliases (Oh My Zsh)

### Git Shortcuts
```bash
g      # git
gst    # git status
ga     # git add
gc     # git commit
gp     # git push
gl     # git pull
gco    # git checkout
gb     # git branch
gd     # git diff
glog   # git log --oneline --decorate
```

### Directory Navigation
```bash
..     # cd ..
...    # cd ../..
....   # cd ../../..
~      # cd ~
-      # cd -  (previous directory)
```

### File Listing
```bash
l      # ls -lah
la     # ls -lAh
ll     # ls -lh
ls     # ls with colors
```

## Advanced Configuration
```yaml
# Basic installation with defaults
- preset: zsh

# Developer setup with popular theme and plugins
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

# Minimal setup without Oh My Zsh
- preset: zsh
  with:
    install_ohmyzsh: false

# Custom theme and plugins
- preset: zsh
  with:
    theme: "agnoster"
    plugins:
      - git
      - docker
      - kubectl
      - zsh-autosuggestions
      - zsh-syntax-highlighting
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| install_ohmyzsh | bool | true | Install Oh My Zsh framework |
| theme | string | robbyrussell | Oh My Zsh theme |
| plugins | array | ["git", "docker", "kubectl"] | Oh My Zsh plugins to enable |
| set_default_shell | bool | true | Set Zsh as default shell |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew, pre-installed on macOS 10.15+)
- ❌ Windows (use WSL)

## Configuration
- **Config file**: `~/.zshrc`
- **Oh My Zsh**: `~/.oh-my-zsh/`
- **Custom plugins**: `~/.oh-my-zsh/custom/plugins/`
- **Custom themes**: `~/.oh-my-zsh/custom/themes/`
- **History**: `~/.zsh_history`

### Sample ~/.zshrc
```bash
# Path to Oh My Zsh installation
export ZSH="$HOME/.oh-my-zsh"

# Theme
ZSH_THEME="agnoster"

# Plugins
plugins=(
  git
  docker
  kubectl
  terraform
  aws
  zsh-autosuggestions
  zsh-syntax-highlighting
)

source $ZSH/oh-my-zsh.sh

# Custom aliases
alias k="kubectl"
alias tf="terraform"
alias dc="docker-compose"

# Environment variables
export EDITOR=nvim
export PATH="$HOME/bin:$PATH"
```

## Popular Themes

### robbyrussell (Default)
Simple, fast, shows Git branch.
```bash
# In ~/.zshrc
ZSH_THEME="robbyrussell"
```

### agnoster
Powerline-style with status indicators (requires Powerline fonts).
```bash
ZSH_THEME="agnoster"
```

### powerlevel10k
Modern, fast, highly customizable (requires Nerd Fonts).
```bash
ZSH_THEME="powerlevel10k/powerlevel10k"

# Configure interactively
p10k configure
```

### spaceship
Clean, async prompt with Git, Node, Python versions.
```bash
ZSH_THEME="spaceship"
```

### pure
Minimal, elegant, no clutter.
```bash
ZSH_THEME="refined"
```

## Popular Plugins

### Built-in Plugins
```bash
# Add to ~/.zshrc plugins array
plugins=(
  git          # Git aliases and completions
  docker       # Docker completions
  kubectl      # Kubernetes completions
  terraform    # Terraform completions
  aws          # AWS CLI completions
  npm          # NPM completions
  python       # Python completions
  ruby         # Ruby completions
  rust         # Rust completions
  golang       # Go completions
)
```

### Community Plugins

#### zsh-autosuggestions (Fish-like suggestions)
```bash
# Install
git clone https://github.com/zsh-users/zsh-autosuggestions \
  ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions

# Add to plugins
plugins=(... zsh-autosuggestions)
```

#### zsh-syntax-highlighting
```bash
# Install
git clone https://github.com/zsh-users/zsh-syntax-highlighting \
  ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-syntax-highlighting

# Add to plugins (must be last!)
plugins=(... zsh-syntax-highlighting)
```

## Real-World Examples

### DevOps Workstation
```yaml
# Full-featured development shell
- preset: zsh
  with:
    theme: "powerlevel10k"
    plugins:
      - git
      - docker
      - kubectl
      - terraform
      - helm
      - aws
      - gcloud
```

### Python Developer Setup
```yaml
- preset: zsh
  with:
    theme: "agnoster"
    plugins:
      - git
      - python
      - pip
      - virtualenv
      - django
```

### CI/CD Agent Setup
```yaml
# Minimal for automated environments
- preset: zsh
  with:
    install_ohmyzsh: false
    set_default_shell: true
```

## Agent Use
- Configure consistent shell environments across teams
- Standardize developer workstation setup
- Automate shell configuration in CI/CD
- Deploy production-ready shell configurations
- Manage plugin installations programmatically

## Troubleshooting

### Theme not displaying correctly
Install Powerline or Nerd Fonts:
```bash
# macOS
brew tap homebrew/cask-fonts
brew install font-meslo-lg-nerd-font

# Linux (Ubuntu/Debian)
sudo apt install fonts-powerline

# Configure terminal to use the font
```

### Slow shell startup
Reduce plugins or use lazy loading:
```bash
# Profile startup time
time zsh -i -c exit

# Disable unused plugins
plugins=(git docker kubectl)  # Keep only essentials
```

### Oh My Zsh not found after installation
Reload shell or source config:
```bash
source ~/.zshrc
# or restart terminal
```

### Custom aliases not working
Ensure they're after `source $ZSH/oh-my-zsh.sh` in ~/.zshrc:
```bash
source $ZSH/oh-my-zsh.sh

# Custom aliases go here
alias k="kubectl"
```

## Uninstall
```yaml
- preset: zsh
  with:
    state: absent
```

**Manual cleanup:**
```bash
# Remove Oh My Zsh
rm -rf ~/.oh-my-zsh

# Remove config (optional)
rm ~/.zshrc ~/.zsh_history

# Change default shell back to bash
chsh -s /bin/bash
```

## Resources
- Official site: https://www.zsh.org
- Oh My Zsh: https://ohmyz.sh
- GitHub: https://github.com/ohmyzsh/ohmyzsh
- Themes: https://github.com/ohmyzsh/ohmyzsh/wiki/Themes
- Plugins: https://github.com/ohmyzsh/ohmyzsh/wiki/Plugins
- Search: "zsh tutorial", "oh my zsh best plugins", "zsh vs bash"

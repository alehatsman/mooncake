# Powerlevel10k - Fast Zsh Theme

Lightning-fast Zsh theme with extensive customization. Displays Git status, command execution time, exit codes, and more with minimal latency.

## Quick Start
```yaml
- preset: p10k
```

## Features
- **Instant prompt**: Near-zero latency with asynchronous rendering
- **Git integration**: Branch, status, and stash information
- **Command timing**: Execution duration for long-running commands
- **Exit codes**: Visual indication of command success/failure
- **Highly customizable**: Interactive configuration wizard
- **Transient prompt**: Compact past prompts to save screen space
- **Cross-platform**: Linux, macOS, WSL

## Basic Usage
```bash
# Run configuration wizard
p10k configure

# Reload configuration
exec zsh

# Show configuration file location
echo $POWERLEVEL9K_CONFIG_FILE

# Disable instant prompt temporarily
POWERLEVEL9K_INSTANT_PROMPT=off

# Show segment timing info
POWERLEVEL9K_DEBUG=true
```

## Advanced Configuration
```yaml
# Install Powerlevel10k (default)
- preset: p10k

# Uninstall Powerlevel10k
- preset: p10k
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (any distro with Zsh)
- ✅ macOS (with Zsh)
- ✅ WSL (Windows Subsystem for Linux)

## Configuration
- **Config file**: `~/.p10k.zsh`
- **Theme directory**: `${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k`
- **Font**: Requires Nerd Font or Powerline font
- **Zsh requirement**: Zsh 5.1 or newer

## Installation Methods

### Manual Zshrc Setup
```bash
# Add to ~/.zshrc before loading Oh My Zsh
source ~/powerlevel10k/powerlevel10k.zsh-theme

# Or with Oh My Zsh
ZSH_THEME="powerlevel10k/powerlevel10k"
```

### Font Installation
```bash
# Install recommended font (MesloLGS NF)
# Download from:
# https://github.com/romkatv/powerlevel10k#fonts

# Set font in terminal:
# - iTerm2: Preferences > Profiles > Text > Font
# - Terminal.app: Preferences > Profiles > Font
# - VS Code: "terminal.integrated.fontFamily": "MesloLGS NF"
```

## Configuration Wizard
```bash
# Run interactive configuration
p10k configure

# Options include:
# - Prompt style (lean, classic, rainbow, pure)
# - Character set (Unicode, ASCII)
# - Prompt colors
# - Prompt flow (one-line, two-line)
# - Transient prompt
# - Instant prompt mode
```

## Customization Examples

### Basic Customization
```bash
# ~/.p10k.zsh

# Show command execution time for commands > 3 seconds
typeset -g POWERLEVEL9K_COMMAND_EXECUTION_TIME_THRESHOLD=3

# Shorten directory paths
typeset -g POWERLEVEL9K_SHORTEN_STRATEGY=truncate_to_last

# Git status colors
typeset -g POWERLEVEL9K_VCS_CLEAN_FOREGROUND=076
typeset -g POWERLEVEL9K_VCS_MODIFIED_FOREGROUND=220
typeset -g POWERLEVEL9K_VCS_UNTRACKED_FOREGROUND=014

# Add custom segment
typeset -g POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(
  status
  command_execution_time
  background_jobs
  direnv
  asdf
  virtualenv
  anaconda
  pyenv
  goenv
  nodenv
  nvm
  nodeenv
  context
  time
)
```

### Show Kubernetes Context
```bash
# Enable kubernetes segment
typeset -g POWERLEVEL9K_KUBECONTEXT_SHOW_ON_COMMAND='kubectl|helm|kubens|kubectx|oc'

# Shorten long cluster names
typeset -g POWERLEVEL9K_KUBECONTEXT_SHORTEN=(
  'gke_*_*_*'   '$4'
  'arn:aws:eks:*:*:cluster/*'  '$6'
)
```

### Custom Prompt Elements
```bash
# Add Docker context
typeset -g POWERLEVEL9K_DOCKER_CONTEXT_SHOW_ON_COMMAND='docker|docker-compose'

# Show AWS profile
typeset -g POWERLEVEL9K_AWS_SHOW_ON_COMMAND='aws|terraform'

# Display Python virtual environment
typeset -g POWERLEVEL9K_VIRTUALENV_SHOW_WITH_PYENV=false
typeset -g POWERLEVEL9K_VIRTUALENV_LEFT_DELIMITER=''
typeset -g POWERLEVEL9K_VIRTUALENV_RIGHT_DELIMITER=''
```

## Prompt Styles

### Lean Style
```bash
typeset -g POWERLEVEL9K_MODE=nerdfont-complete
typeset -g POWERLEVEL9K_PROMPT_ADD_NEWLINE=false
typeset -g POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(
  dir vcs
)
typeset -g POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(
  status command_execution_time
)
```

### Two-Line Prompt
```bash
typeset -g POWERLEVEL9K_PROMPT_ON_NEWLINE=true
typeset -g POWERLEVEL9K_RPROMPT_ON_NEWLINE=false
typeset -g POWERLEVEL9K_MULTILINE_FIRST_PROMPT_PREFIX='╭─'
typeset -g POWERLEVEL9K_MULTILINE_LAST_PROMPT_PREFIX='╰─❯ '
```

## Performance Optimization
```bash
# Instant prompt (fastest)
typeset -g POWERLEVEL9K_INSTANT_PROMPT=verbose

# Disable unused segments
typeset -g POWERLEVEL9K_DISABLE_HOT_RELOAD=true

# Reduce Git status checks
typeset -g POWERLEVEL9K_VCS_MAX_INDEX_SIZE_DIRTY=4096

# Cache command output
typeset -g POWERLEVEL9K_VCS_DISABLE_GITSTATUS_FORMATTING=false
```

## Real-World Examples

### Developer Setup
```yaml
- name: Install Powerlevel10k
  preset: p10k

- name: Configure Zsh
  shell: |
    cat >> ~/.zshrc << 'EOF'
    # Enable Powerlevel10k instant prompt
    if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
      source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
    fi

    # Load Powerlevel10k
    source ~/powerlevel10k/powerlevel10k.zsh-theme

    # Load configuration
    [[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh
    EOF
```

### CI/CD Environment
```bash
# Disable fancy prompt in CI
if [[ -n "$CI" ]]; then
  typeset -g POWERLEVEL9K_MODE=ascii
  typeset -g POWERLEVEL9K_INSTANT_PROMPT=off
fi
```

### Remote Server Setup
```bash
# Minimal configuration for SSH sessions
if [[ -n "$SSH_CONNECTION" ]]; then
  typeset -g POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(
    context dir vcs
  )
  typeset -g POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(
    status
  )
fi
```

## Transient Prompt
```bash
# Enable transient prompt (compact past prompts)
typeset -g POWERLEVEL9K_TRANSIENT_PROMPT=always

# Customize transient prompt
typeset -g POWERLEVEL9K_TRANSIENT_PROMPT_PREFIX='%F{green}❯%f '
```

## Troubleshooting

### Icons not displaying
Install a Nerd Font:
```bash
# Download and install MesloLGS NF
curl -fLo ~/.local/share/fonts/MesloLGS-NF-Regular.ttf \
  https://github.com/romkatv/powerlevel10k-media/raw/master/MesloLGS%20NF%20Regular.ttf

# Rebuild font cache
fc-cache -fv

# Set font in terminal emulator
```

### Slow prompt
Enable instant prompt:
```bash
# Add to top of ~/.zshrc
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi
```

### Configuration not loading
Check file location:
```bash
# Ensure config file exists
ls -la ~/.p10k.zsh

# Verify it's sourced in ~/.zshrc
grep p10k ~/.zshrc
```

### Git status not showing
Check gitstatus:
```bash
# Test gitstatus
cd ~/powerlevel10k/gitstatus
./gitstatusd --parent-pid=$$ --sigwinch-pid=$$
```

## Agent Use
- Development environment setup
- SSH server configuration
- Container terminal customization
- Remote workspace provisioning
- Automated dotfiles deployment
- Team-wide shell standardization

## Uninstall
```yaml
- preset: p10k
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/romkatv/powerlevel10k
- Configuration reference: https://github.com/romkatv/powerlevel10k#configuration
- Font installation: https://github.com/romkatv/powerlevel10k#fonts
- Search: "powerlevel10k configuration", "p10k customization", "powerlevel10k tutorial"

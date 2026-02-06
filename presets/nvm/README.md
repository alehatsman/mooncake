# nvm - Node Version Manager

Manage multiple Node.js versions. Install, switch, and use different Node versions per project.

## Features

- **Multiple Node versions** - Install and switch Node 12-21+
- **Per-project versions** - Automatic switching via .nvmrc
- **Zero sudo** - User-level installation
- **Shell integration** - Bash, Zsh, Fish support
- **Global packages** - Auto-install packages on new Node versions
- **LTS support** - Install latest LTS with --lts
- **Alias system** - Named shortcuts for versions
- **Package migration** - Copy packages between versions

## Quick Start
```yaml
- preset: nvm
```

## Basic Usage
```bash
# Install latest LTS
nvm install --lts

# Install specific version
nvm install 20.10.0
nvm install 18.19.0

# Use version
nvm use 20
nvm use 18

# Set default
nvm alias default 20

# List installed
nvm ls

# List available
nvm ls-remote
```

## Shell Integration
```bash
# Bash (~/.bashrc)
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

# Zsh (~/.zshrc)
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

# Fish (~/.config/fish/config.fish)
set -x NVM_DIR $HOME/.nvm
bass source $NVM_DIR/nvm.sh
```

## Version Management
```bash
# Install latest
nvm install node

# Install LTS
nvm install --lts
nvm install --lts=hydrogen  # Node 18
nvm install --lts=iron      # Node 20

# Install from .nvmrc
nvm install

# Use from .nvmrc
nvm use

# Show current
nvm current

# Show path
nvm which 20
```

## .nvmrc File
```bash
# Create .nvmrc
echo "20.10.0" > .nvmrc

# Or with version name
echo "lts/iron" > .nvmrc

# Auto-use on cd (add to shell)
autoload -U add-zsh-hook
load-nvmrc() {
  if [[ -f .nvmrc && -r .nvmrc ]]; then
    nvm use
  fi
}
add-zsh-hook chpwd load-nvmrc
```

## Common Operations
```bash
# Update npm
nvm install-latest-npm

# Uninstall version
nvm uninstall 18.19.0

# Run with version
nvm run 20 app.js
nvm exec 20 node app.js

# System version
nvm use system

# Reinstall packages
nvm reinstall-packages 18
```

## Project Workflows
```bash
# New project setup
cd myproject
echo "20.10.0" > .nvmrc
nvm install
nvm use
npm init -y

# Clone and setup
git clone repo
cd repo
nvm install  # Reads .nvmrc
npm install

# Switch between projects
cd project-a && nvm use  # Uses Node 18
cd project-b && nvm use  # Uses Node 20
```

## Multiple Versions
```bash
# Install multiple
nvm install 18
nvm install 20
nvm install 21

# Switch quickly
nvm use 18
nvm use 20

# Default version
nvm alias default 20

# Test on multiple
for v in 18 20 21; do
  nvm use $v
  npm test
done
```

## Package Migration
```bash
# List global packages
npm list -g --depth=0

# Copy packages between versions
nvm reinstall-packages 18  # Copy from 18 to current

# Fresh install with packages
nvm install 20 --reinstall-packages-from=18
```

## Aliases
```bash
# Create alias
nvm alias myapp 18.19.0

# Use alias
nvm use myapp

# List aliases
nvm alias

# Remove alias
nvm unalias myapp

# Built-in aliases
nvm use node     # Latest
nvm use --lts    # Latest LTS
nvm use stable   # Latest stable
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Setup Node
  run: |
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
    export NVM_DIR="$HOME/.nvm"
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
    nvm install
    nvm use

# GitLab CI
before_script:
  - curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
  - export NVM_DIR="$HOME/.nvm"
  - source $NVM_DIR/nvm.sh
  - nvm install
  - npm install

# Docker
FROM ubuntu:22.04
ENV NVM_DIR=/root/.nvm
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
RUN . $NVM_DIR/nvm.sh && nvm install --lts
```

## Version Selection
```bash
# Partial versions
nvm install 20      # Latest 20.x.x
nvm install 18.19   # Latest 18.19.x

# LTS codenames
nvm install lts/hydrogen  # 18.x
nvm install lts/iron      # 20.x
nvm install lts/*         # Latest LTS

# Wildcards
nvm use node      # Latest installed
nvm use --lts     # Latest LTS installed
```

## Automatic Switching
```bash
# Zsh auto-switch
# Add to ~/.zshrc
autoload -U add-zsh-hook
load-nvmrc() {
  local node_version="$(nvm version)"
  local nvmrc_path="$(nvm_find_nvmrc)"

  if [ -n "$nvmrc_path" ]; then
    local nvmrc_node_version=$(nvm version "$(cat "${nvmrc_path}")")
    if [ "$nvmrc_node_version" = "N/A" ]; then
      nvm install
    elif [ "$nvmrc_node_version" != "$node_version" ]; then
      nvm use
    fi
  fi
}
add-zsh-hook chpwd load-nvmrc
load-nvmrc

# Bash auto-switch
# Similar approach in ~/.bashrc
```

## Environment Variables
```bash
# Custom install location
export NVM_DIR="$HOME/.config/nvm"

# Default packages file
export NVM_DEFAULT_PACKAGES="$HOME/.nvm-default-packages"

# Symlink current
export NVM_SYMLINK_CURRENT=true

# Colors
export NVM_COLORS='cmgRY'
```

## Default Packages
```bash
# Create default packages file
cat > ~/.nvm-default-packages <<EOF
typescript
ts-node
yarn
pnpm
npm-check-updates
nodemon
EOF

# Now all new Node installs get these packages
nvm install 20  # Installs with default packages
```

## Troubleshooting
```bash
# Slow nvm load
# Use lazy loading in .zshrc
nvm() {
  unset -f nvm
  export NVM_DIR="$HOME/.nvm"
  [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
  nvm "$@"
}

# Verify installation
command -v nvm

# Debug mode
export NVM_DEBUG=1
nvm install 20

# Clear cache
rm -rf ~/.nvm/.cache

# Reinstall nvm
rm -rf ~/.nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
```

## Advanced Configuration

### Lazy Loading (Speed Optimization)
```bash
# Add to .zshrc for faster shell startup
nvm() {
  unset -f nvm
  export NVM_DIR="$HOME/.nvm"
  [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
  nvm "$@"
}
```

### Custom Install Location
```bash
export NVM_DIR="$HOME/.config/nvm"
```

### Default Packages
```bash
# Auto-install these on every new Node version
cat > ~/.nvm-default-packages <<EOF
typescript
ts-node
yarn
pnpm
nodemon
EOF
```

### Build from Source
```bash
# Install with custom flags
NVM_NODEJS_ORG_MIRROR=https://nodejs.org/dist nvm install 20
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `node` | Node version (20, 18, --lts, node) |
| `set_default` | bool | `true` | Set as default version |
| `additional_versions` | array | `[]` | Other versions to install |
| `global_packages` | array | `[]` | Packages to install globally |

## Platform Support

- ✅ **Linux** - All distributions (bash/zsh/fish)
- ✅ **macOS** - Intel and Apple Silicon
- ⚠️ **Windows** - WSL only (use nvm-windows for native)
- ✅ **FreeBSD** - Community supported

**Requirements:**
- curl or wget
- git (for nvm installation)
- C++ compiler (for native modules)

## Comparison
| Feature | nvm | n | volta | fnm |
|---------|-----|---|-------|-----|
| Speed | Slow | Fast | Fast | Fastest |
| Language | Bash | Bash | Rust | Rust |
| .nvmrc | Yes | No | Yes | Yes |
| Windows | WSL only | No | Yes | Yes |
| Auto-switch | Manual | No | Auto | Auto |

## Best Practices
- **Use .nvmrc** in all projects
- **Pin exact versions** for production
- **Use LTS** for production apps
- **Test on multiple versions** before release
- **Use default packages** for common tools
- **Lazy load** nvm in shell for speed
- **Document version** in README

## Tips
- 100M+ users worldwide
- Supports Node 0.10 to latest
- Per-project version control
- Global package isolation
- No sudo needed
- Works with yarn, pnpm, npm
- .nvmrc for team consistency

## Agent Use
- Automated Node version management
- CI/CD pipeline setup
- Multi-version testing
- Development environment setup
- Team version consistency
- Container image builds

## Uninstall
```yaml
- preset: nvm
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/nvm-sh/nvm
- Search: "nvm install", "nvm use"

# Node.js - JavaScript Runtime via nvm

Install and manage Node.js versions using nvm (Node Version Manager). Run JavaScript server-side with the world's most popular runtime.

## Quick Start
```yaml
- preset: nodejs
```

## Features
- **Version management**: Install and switch between Node.js versions
- **Multiple versions**: Run different versions per project
- **LTS support**: Install long-term support releases
- **Global packages**: Install npm packages globally
- **Shell integration**: Automatic PATH configuration
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Check versions
node --version
npm --version
nvm --version

# List installed versions
nvm ls

# Install another version
nvm install 18
nvm install 20

# Switch version
nvm use 18
nvm use 20

# Set default version
nvm alias default 20

# Run command with specific version
nvm exec 18 node script.js

# List available versions
nvm ls-remote
nvm ls-remote --lts
```

## Advanced Configuration

### Install latest LTS
```yaml
- preset: nodejs
  # Uses LTS by default
```

### Install specific version
```yaml
- preset: nodejs
  with:
    version: "20.10.0"
```

### Install multiple versions
```yaml
- preset: nodejs
  with:
    version: lts
    additional_versions:
      - "18.19.0"
      - "20.10.0"
    set_default: true
```

### Install with global packages
```yaml
- preset: nodejs
  with:
    version: lts
    global_packages:
      - typescript
      - eslint
      - prettier
      - nodemon
      - pm2
```

### Development environment
```yaml
- name: Setup Node.js dev environment
  preset: nodejs
  with:
    version: lts
    global_packages:
      - typescript
      - ts-node
      - eslint
      - prettier
      - nodemon
      - npm-check-updates
```

### Production server
```yaml
- name: Setup Node.js production
  preset: nodejs
  with:
    version: "20.10.0"
    global_packages:
      - pm2
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Node.js/nvm |
| version | string | lts | Node version (e.g., "20.10.0", "lts", "latest", "18") |
| set_default | bool | true | Set this version as default |
| additional_versions | array | [] | Additional Node.js versions to install |
| global_packages | array | [] | npm packages to install globally |

## Version Formats

nvm supports various version formats:
- `lts` - Latest LTS (Long Term Support) version
- `latest` - Latest stable version
- `20` - Latest 20.x.x version
- `20.10.0` - Exact version
- `lts/hydrogen` - Specific LTS codename (Node 18)
- `lts/iron` - Specific LTS codename (Node 20)

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS
- ❌ Windows (use nvm-windows separately)

## Configuration
- **nvm directory**: `~/.nvm/`
- **Node installations**: `~/.nvm/versions/node/`
- **Global packages**: `~/.nvm/versions/node/<version>/lib/node_modules/`
- **Shell profiles**: `~/.bashrc`, `~/.zshrc`, `~/.profile`

## Real-World Examples

### CI/CD testing matrix
```yaml
- name: Install Node.js for CI testing
  preset: nodejs
  with:
    version: "20"
    additional_versions:
      - "18"
      - "16"

- name: Test on Node 18
  shell: |
    nvm use 18
    npm install
    npm test

- name: Test on Node 20
  shell: |
    nvm use 20
    npm install
    npm test
```

### Microservices deployment
```yaml
- name: Install Node.js
  preset: nodejs
  with:
    version: "20.10.0"
    global_packages:
      - pm2
  become: true

- name: Deploy services
  shell: |
    pm2 start api/app.js --name api
    pm2 start worker/app.js --name worker
    pm2 save
```

### .nvmrc project setup
```bash
# Create .nvmrc in project
echo "20.10.0" > .nvmrc

# Auto-use on cd (add to .zshrc)
autoload -U add-zsh-hook
load-nvmrc() {
  if [[ -f .nvmrc && -r .nvmrc ]]; then
    nvm use
  fi
}
add-zsh-hook chpwd load-nvmrc
```

## Post-Installation

After installation, restart your terminal or run:
```bash
source ~/.bashrc  # or ~/.zshrc
```

Verify installation:
```bash
node --version
npm --version
nvm --version
```

## Common Operations

### Package management
```bash
# Install packages
npm install express
npm install -g typescript

# Update npm
nvm install-latest-npm

# List global packages
npm list -g --depth=0
```

### Version switching
```bash
# Switch quickly
nvm use 18
nvm use 20

# Default version
nvm alias default 20

# System version (if installed)
nvm use system
```

### Migration between versions
```bash
# Copy packages between versions
nvm reinstall-packages 18

# Fresh install with packages from old version
nvm install 20 --reinstall-packages-from=18
```

## nvm Commands

### Installation
```bash
# Install version
nvm install 20
nvm install --lts
nvm install --lts=hydrogen

# Install from .nvmrc
nvm install
```

### Usage
```bash
# Use version
nvm use 20
nvm use --lts
nvm use  # Uses .nvmrc

# Show current
nvm current

# Show path
nvm which 20
```

### Management
```bash
# List versions
nvm ls
nvm ls-remote
nvm ls-remote --lts

# Uninstall version
nvm uninstall 18.19.0

# Aliases
nvm alias myapp 18.19.0
nvm alias
nvm unalias myapp
```

## Global Packages

### Common development tools
```bash
npm install -g typescript ts-node
npm install -g eslint prettier
npm install -g nodemon
npm install -g npm-check-updates
```

### Production tools
```bash
npm install -g pm2
npm install -g forever
npm install -g node-gyp
```

### Package managers
```bash
npm install -g yarn
npm install -g pnpm
```

## Agent Use
- Automated Node.js version management
- CI/CD pipeline setup with specific versions
- Multi-version testing environments
- Development environment provisioning
- Team version consistency via .nvmrc
- Global tool installation
- Production deployments with pinned versions

## Troubleshooting

### nvm command not found
```bash
# Add to shell profile manually
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

# Reload shell
source ~/.bashrc  # or ~/.zshrc
```

### Permission errors
```bash
# Don't use sudo with nvm
nvm install 20  # ✅ Correct

# Avoid
sudo npm install -g package  # ❌ Wrong with nvm
```

### Slow shell startup
```bash
# Use lazy loading (add to .zshrc)
nvm() {
  unset -f nvm
  export NVM_DIR="$HOME/.nvm"
  [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
  nvm "$@"
}
```

### Version not persisting
```bash
# Set default version
nvm alias default 20

# Check default
nvm ls
```

## Best Practices
- **Use .nvmrc**: Commit to repo for team consistency
- **Pin versions**: Use exact versions in production
- **LTS for production**: Stable, long-term support
- **Latest for development**: Try new features
- **Global packages minimal**: Prefer project-local dependencies
- **Test on multiple versions**: Ensure compatibility
- **Update regularly**: Security patches and features
- **Document versions**: List required versions in README

## Comparison

| Manager | Speed | .nvmrc | Auto-switch | Windows |
|---------|-------|--------|-------------|---------|
| nvm | Slow | ✅ | Manual | WSL only |
| n | Fast | ❌ | No | No |
| volta | Fast | ✅ | ✅ | ✅ |
| fnm | Very fast | ✅ | ✅ | ✅ |

## Uninstall
```yaml
- preset: nodejs
  with:
    state: absent
```

**Note**: This removes nvm and all installed Node.js versions.

## Resources
- nvm GitHub: https://github.com/nvm-sh/nvm
- Node.js: https://nodejs.org/
- npm docs: https://docs.npmjs.com/
- Search: "nvm install", "nvm use", "node version manager"

# fnm - Fast Node Manager

Fast and simple Node.js version manager built in Rust. Cross-platform alternative to nvm with instant version switching.

## Quick Start
```yaml
- preset: fnm
```

## Features
- **Blazing fast**: Written in Rust, instant version switching
- **Cross-platform**: Windows, macOS, Linux support
- **Shell integration**: Works with bash, zsh, fish, PowerShell
- **`.node-version` support**: Automatically switch based on project files
- **Minimal**: Single binary, no system dependencies

## Basic Usage
```bash
# List available Node.js versions
fnm list-remote

# Install Node.js version
fnm install 20
fnm install 18.16.0

# Use specific version
fnm use 20

# Set default version
fnm default 20

# List installed versions
fnm list

# Current version
fnm current

# Install from .node-version or .nvmrc
fnm install
fnm use
```

## Advanced Configuration
```yaml
- preset: fnm
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove fnm |

## Platform Support
- ✅ Linux (shell script installer)
- ✅ macOS (Homebrew, shell script)
- ✅ Windows (Scoop, Chocolatey, shell script)

## Configuration
- **Install directory**: `~/.local/share/fnm` (Linux/macOS), `%LOCALAPPDATA%\fnm` (Windows)
- **Node.js versions**: `~/.local/share/fnm/node-versions/`
- **Shell configuration**: Add to `.bashrc`, `.zshrc`, or `config.fish`
- **Environment variable**: `FNM_DIR` for custom installation path

## Real-World Examples

### Automatic Version Switching
```bash
# Create project with Node.js version
echo "20.10.0" > .node-version

# fnm automatically switches when entering directory
cd my-project  # Automatically uses Node.js 20.10.0
```

### CI/CD Pipeline
```yaml
- name: Install Node.js via fnm
  preset: fnm

- name: Install specific Node version
  shell: fnm install 20.10.0

- name: Use Node version
  shell: fnm use 20.10.0

- name: Run tests
  shell: |
    eval "$(fnm env --use-on-cd)"
    npm test
```

### Multi-version Testing
```bash
# Test against multiple Node.js versions
for version in 18 20 21; do
  echo "Testing with Node.js $version"
  fnm use $version
  npm test
done
```

### Shell Integration
```bash
# Add to ~/.bashrc or ~/.zshrc
eval "$(fnm env --use-on-cd)"

# Add to ~/.config/fish/config.fish
fnm env --use-on-cd | source
```

## Agent Use
- Manage Node.js versions in CI/CD pipelines
- Automate developer environment setup with specific Node versions
- Test applications across multiple Node.js versions
- Switch Node.js versions per project automatically
- Install and configure Node.js in containerized environments

## Troubleshooting

### fnm command not found
```bash
# Add fnm to PATH
# bash/zsh
export PATH="$HOME/.local/share/fnm:$PATH"
eval "$(fnm env)"

# fish
set -gx PATH "$HOME/.local/share/fnm" $PATH
fnm env | source
```

### Version not switching automatically
```bash
# Enable auto-switching in shell
eval "$(fnm env --use-on-cd)"  # bash/zsh
fnm env --use-on-cd | source    # fish
```

### Permission denied
```bash
# Fix installation directory permissions
chmod -R u+w ~/.local/share/fnm
```

### npm not found after install
```bash
# Ensure fnm env is loaded
eval "$(fnm env)"
fnm use 20

# Verify
which node
which npm
```

## Uninstall
```yaml
- preset: fnm
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/Schniz/fnm
- GitHub: https://github.com/Schniz/fnm
- Comparison with nvm: https://github.com/Schniz/fnm#comparison-with-nvm
- Search: "fnm node version manager", "fnm vs nvm", "fnm installation"

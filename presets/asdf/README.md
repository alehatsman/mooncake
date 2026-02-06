# asdf - Universal Version Manager

Manage multiple runtime versions with a single tool. Replace nvm, rbenv, pyenv, and more with one version manager.

## Quick Start
```yaml
- preset: asdf
```

## Features
- **Multi-language**: Support for 500+ tools via plugins (Node, Python, Ruby, Go, Java, etc.)
- **Per-project versions**: `.tool-versions` file for project-specific versions
- **Single tool**: Replace nvm, rbenv, pyenv, goenv with one tool
- **Automatic switching**: Changes versions when entering project directories
- **Global + local**: Set global defaults and per-project overrides
- **Shell integration**: Works with bash, zsh, fish
- **Extensible**: Plugin system for adding new tools

## Basic Usage
```bash
# Add plugin
asdf plugin add nodejs

# List available versions
asdf list all nodejs

# Install version
asdf install nodejs 20.10.0

# Set global version
asdf global nodejs 20.10.0

# Set local version (project-specific)
asdf local nodejs 18.19.0
```

## Plugin Management
```bash
# List available plugins
asdf plugin list all

# Add plugin
asdf plugin add python
asdf plugin add ruby
asdf plugin add golang

# List installed plugins
asdf plugin list

# Update plugin
asdf plugin update python

# Update all plugins
asdf plugin update --all

# Remove plugin
asdf plugin remove python
```

## Version Installation
```bash
# Install latest
asdf install nodejs latest

# Install specific version
asdf install python 3.11.7

# Install from .tool-versions
asdf install

# Install all tools
asdf install
```

## Version Selection
```bash
# Global (default for all directories)
asdf global nodejs 20.10.0

# Local (current directory)
asdf local python 3.11.7

# Shell (current shell session)
asdf shell ruby 3.2.0

# Check current version
asdf current nodejs

# Check all current versions
asdf current
```

## .tool-versions File
```bash
# .tool-versions (project root)
nodejs 20.10.0
python 3.11.7
ruby 3.2.0
golang 1.21.5

# With comments
nodejs 20.10.0  # LTS version
python 3.11.7   # Latest stable

# Legacy version file support
# asdf reads .nvmrc, .ruby-version, etc.
```

## Version Management
```bash
# List installed versions
asdf list nodejs

# List all available versions
asdf list all python

# List versions matching regex
asdf list all python 3.11

# Uninstall version
asdf uninstall python 3.10.0

# Reshim (rebuild shims)
asdf reshim nodejs

# Where is version installed
asdf where nodejs 20.10.0
```

## Common Languages
```bash
# Node.js
asdf plugin add nodejs
asdf install nodejs latest
asdf global nodejs latest

# Python
asdf plugin add python
asdf install python 3.11.7
asdf global python 3.11.7

# Ruby
asdf plugin add ruby
asdf install ruby 3.2.0
asdf global ruby 3.2.0

# Go
asdf plugin add golang
asdf install golang 1.21.5
asdf global golang 1.21.5

# Java
asdf plugin add java
asdf install java openjdk-21
asdf global java openjdk-21

# Rust
asdf plugin add rust
asdf install rust 1.75.0
asdf global rust 1.75.0
```

## Multiple Versions
```bash
# Set multiple global versions
asdf global nodejs 20.10.0 18.19.0

# First one is default
node --version  # Uses 20.10.0

# Switch with shell
asdf shell nodejs 18.19.0
node --version  # Now uses 18.19.0
```

## Project Setup
```bash
# Initialize project
cd myproject
asdf local nodejs 20.10.0
asdf local python 3.11.7

# Team uses same versions
cat .tool-versions
# nodejs 20.10.0
# python 3.11.7

# Install team's versions
asdf install
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Setup asdf
  uses: asdf-vm/actions/install@v3

- name: Install tools
  run: |
    asdf plugin add nodejs
    asdf install

# GitLab CI
before_script:
  - git clone https://github.com/asdf-vm/asdf.git ~/.asdf
  - echo '. "$HOME/.asdf/asdf.sh"' >> ~/.bashrc
  - source ~/.bashrc
  - asdf plugin add nodejs
  - asdf install

# Docker
FROM ubuntu:22.04
RUN git clone https://github.com/asdf-vm/asdf.git ~/.asdf
RUN echo '. "$HOME/.asdf/asdf.sh"' >> ~/.bashrc
WORKDIR /app
COPY .tool-versions .
RUN bash -c 'source ~/.asdf/asdf.sh && \
    asdf plugin add nodejs && \
    asdf install'
```

## Configuration
```bash
# ~/.asdfrc
legacy_version_file = yes  # Read .nvmrc, .ruby-version, etc.
use_release_candidates = no
always_keep_download = no
plugin_repository_last_check_duration = 60

# Disable concurrency
concurrency = 1

# Custom plugin repo
disable_plugin_short_name_repository = no
```

## Plugin Development
```bash
# Clone plugin template
git clone https://github.com/asdf-vm/asdf-plugin-template mytool

# Required scripts
bin/list-all    # List all versions
bin/download    # Download version
bin/install     # Install version

# Optional scripts
bin/latest-stable    # Get latest stable version
bin/help            # Custom help
```

## Troubleshooting
```bash
# Reshim after manual install
asdf reshim nodejs

# Check shims
asdf which node

# Update asdf
asdf update

# Check plugin health
asdf plugin test nodejs

# Debug
ASDF_DEBUG=1 asdf install nodejs 20.10.0
```

## Migration
```bash
# From nvm
cat ~/.nvmrc >> .tool-versions
asdf plugin add nodejs
asdf install

# From rbenv
cat .ruby-version >> .tool-versions
asdf plugin add ruby
asdf install

# From pyenv
cat .python-version >> .tool-versions
asdf plugin add python
asdf install
```

## Shell Integration
```bash
# Bash (~/.bashrc)
. "$HOME/.asdf/asdf.sh"
. "$HOME/.asdf/completions/asdf.bash"

# Zsh (~/.zshrc)
. "$HOME/.asdf/asdf.sh"
fpath=(${ASDF_DIR}/completions $fpath)
autoload -Uz compinit && compinit

# Fish (~/.config/fish/config.fish)
source ~/.asdf/asdf.fish
```

## Performance Tips
```bash
# Use legacy version files for speed
legacy_version_file = yes

# Reduce plugin updates
plugin_repository_last_check_duration = 1440  # 24 hours

# Use latest instead of specific versions
asdf install nodejs latest

# Parallel installs (be careful)
asdf install & asdf install python &
```

## Comparison
| Feature | asdf | nvm | rbenv | pyenv |
|---------|------|-----|-------|-------|
| Multi-language | Yes | No | No | No |
| Single tool | Yes | No | No | No |
| Plugin system | Yes | No | No | No |
| .tool-versions | Yes | No | No | No |
| Legacy files | Yes | .nvmrc | .ruby-version | .python-version |

## Best Practices
- **Use .tool-versions** in all projects
- **Enable legacy_version_file** for compatibility
- **Pin versions** for reproducibility
- **Update plugins regularly**
- **Use latest for development**, specific versions for production
- **Document versions** in README
- **Keep asdf updated**

## Tips
- Single tool for all language versions
- Team synchronization with .tool-versions
- No sudo required
- Per-directory versions
- Legacy file support (.nvmrc, etc.)
- 200+ plugins available
- Fast switching

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Real-World Examples

### Multi-Project Version Management
```bash
#!/bin/bash
# setup-workspace.sh - Setup development environment

# Project A: Node 18, Python 3.11
cd ~/projects/api
cat > .tool-versions <<EOF
nodejs 18.17.0
python 3.11.4
terraform 1.5.0
EOF
asdf install

# Project B: Node 20, Python 3.12
cd ~/projects/frontend
cat > .tool-versions <<EOF
nodejs 20.5.0
python 3.12.0
yarn 1.22.19
EOF
asdf install

# Automatic switching when cd-ing between projects
cd ~/projects/api
node --version  # v18.17.0
cd ~/projects/frontend
node --version  # v20.5.0
```

### CI/CD Pipeline Setup
```yaml
# .github/workflows/test.yml
- name: Install asdf
  preset: asdf

- name: Install project tools
  run: |
    asdf plugin add nodejs
    asdf plugin add python
    asdf install  # Reads .tool-versions

- name: Verify versions
  run: |
    node --version
    python --version
    asdf current
```

### Team Environment Consistency
```bash
# bootstrap-dev-env.sh - One-command setup for new team members
#!/bin/bash

echo "Setting up development environment..."

# Install asdf via Mooncake
cat > setup.yml <<EOF
- name: Install asdf
  preset: asdf
  become: false
EOF

mooncake run -c setup.yml

# Install plugins from .tool-versions
asdf plugin add nodejs https://github.com/asdf-vm/asdf-nodejs.git
asdf plugin add python
asdf plugin add golang https://github.com/asdf-community/asdf-golang.git
asdf plugin add terraform

# Install all versions
asdf install

echo "Development environment ready!"
echo "Current versions:"
asdf current
```

### Docker Development Image
```dockerfile
# Dockerfile.dev
FROM ubuntu:22.04

# Install asdf
RUN apt-get update && apt-get install -y curl git
RUN git clone https://github.com/asdf-vm/asdf.git ~/.asdf --branch v0.13.1

# Copy project tool versions
COPY .tool-versions /app/.tool-versions
WORKDIR /app

# Install tools
RUN . ~/.asdf/asdf.sh && \
    asdf plugin add nodejs && \
    asdf plugin add python && \
    asdf install

# Set up PATH
ENV PATH="/root/.asdf/shims:/root/.asdf/bin:${PATH}"

CMD ["bash"]
```

## Agent Use
- Automated development environment provisioning
- CI/CD version management with project-specific tools
- Team onboarding automation with consistent tool versions
- Multi-language project support in containers
- Version enforcement for compliance and reproducibility
- Legacy project maintenance with isolated tool versions

## Uninstall

## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install asdf
  preset: asdf

- name: Use asdf in automation
  shell: |
    # Custom configuration here
    echo "asdf configured"
```

```yaml
- preset: asdf
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/asdf-vm/asdf
- Docs: https://asdf-vm.com/
- Plugins: https://github.com/asdf-vm/asdf-plugins
- Search: "asdf version manager", "asdf tool-versions"

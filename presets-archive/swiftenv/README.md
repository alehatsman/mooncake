# swiftenv - Swift Version Manager

Manage multiple Swift versions on a single system. Switch between Swift versions per project or globally.

## Quick Start
```yaml
- preset: swiftenv
```

## Features
- **Multiple versions**: Install and manage multiple Swift versions simultaneously
- **Per-project versions**: Use `.swift-version` file for project-specific Swift versions
- **Global version**: Set system-wide default Swift version
- **Shell integration**: Automatic PATH management for active Swift version
- **Version installation**: Download and install official Swift releases
- **Lightweight**: Minimal overhead, uses Swift toolchain binaries directly

## Basic Usage
```bash
# List available Swift versions
swiftenv install --list

# Install specific version
swiftenv install 5.9.0
swiftenv install 5.8.1

# List installed versions
swiftenv versions

# Set global version
swiftenv global 5.9.0

# Set local version (creates .swift-version)
swiftenv local 5.8.1

# Show current version
swiftenv version
```

## Advanced Configuration
```yaml
# Install swiftenv (default)
- preset: swiftenv

# Install with shell configuration
- preset: swiftenv
  with:
    configure_shell: true              # Auto-configure shell init file

# Uninstall swiftenv
- preset: swiftenv
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| configure_shell | bool | true | Auto-configure shell init file |

## Platform Support
- ✅ Linux (apt, dnf, yum)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported - use official Swift installer)

## Configuration
- **Versions**: `~/.swiftenv/versions/` - Installed Swift versions
- **Shims**: `~/.swiftenv/shims/` - Swift executable shims
- **Global version**: `~/.swiftenv/version` - System-wide default version
- **Local version**: `.swift-version` - Project-specific version file

## Version Management
```bash
# Install latest version
swiftenv install $(swiftenv install --list | tail -1)

# Install from snapshot
swiftenv install DEVELOPMENT-SNAPSHOT-2024-01-15-a

# Uninstall version
swiftenv uninstall 5.8.0

# Rehash after manual installation
swiftenv rehash
```

## Project Configuration
```bash
# Set project Swift version
cd my-swift-project
swiftenv local 5.9.0

# This creates .swift-version file
cat .swift-version
# Output: 5.9.0

# Commit .swift-version to git
git add .swift-version
git commit -m "Pin Swift version"
```

## Shell Integration
```bash
# Bash (~/.bashrc)
export PATH="$HOME/.swiftenv/shims:$PATH"
eval "$(swiftenv init -)"

# Zsh (~/.zshrc)
export PATH="$HOME/.swiftenv/shims:$PATH"
eval "$(swiftenv init -)"

# Fish (~/.config/fish/config.fish)
set -gx PATH $HOME/.swiftenv/shims $PATH
status --is-interactive; and source (swiftenv init -|psub)
```

## Real-World Examples

### Multi-Project Workflow
```bash
# Project A uses Swift 5.9
cd ~/projects/app-a
swiftenv local 5.9.0
swift --version
# Swift version 5.9.0

# Project B uses Swift 5.8
cd ~/projects/app-b
swiftenv local 5.8.1
swift --version
# Swift version 5.8.1
```

### CI/CD Integration
```yaml
# GitHub Actions
- name: Setup Swift
  run: |
    # Use project's .swift-version
    swiftenv install $(cat .swift-version)
    swiftenv global $(cat .swift-version)

- name: Build
  run: swift build -c release
```

### Docker Development
```dockerfile
FROM ubuntu:22.04

# Install swiftenv
RUN git clone https://github.com/kylef/swiftenv.git ~/.swiftenv
ENV PATH="/root/.swiftenv/shims:/root/.swiftenv/bin:$PATH"

# Install Swift version
COPY .swift-version /app/.swift-version
WORKDIR /app
RUN swiftenv install $(cat .swift-version)
RUN swiftenv global $(cat .swift-version)
```

## Version Selection
```bash
# Check which version will be used
swiftenv which swift

# Check version precedence
# 1. SWIFTENV_VERSION environment variable
# 2. .swift-version in current directory
# 3. .swift-version in parent directories
# 4. Global version (~/.swiftenv/version)

# Override with environment variable
SWIFTENV_VERSION=5.8.1 swift --version
```

## Troubleshooting

### Command not found after installation
```bash
# Rehash shims
swiftenv rehash

# Verify PATH
echo $PATH | grep swiftenv

# Manually add to PATH
export PATH="$HOME/.swiftenv/shims:$PATH"
```

### Wrong version being used
```bash
# Check version source
swiftenv version
# Shows: 5.9.0 (set by /path/to/.swift-version)

# Check precedence
swiftenv versions
# Shows: * 5.9.0 (current)

# Remove local override if needed
rm .swift-version
```

### Installation fails
```bash
# Check available versions
swiftenv install --list

# Install with verbose output
swiftenv install -v 5.9.0

# Manual installation
# Download from https://swift.org/download/
# Extract to ~/.swiftenv/versions/5.9.0/
swiftenv rehash
```

## Comparison with Official Swift

| Feature | swiftenv | Official Installer |
|---------|----------|--------------------|
| Multiple versions | Yes | No (system-wide) |
| Per-project versions | Yes | No |
| Easy switching | Yes | Manual |
| Shell integration | Yes | Limited |
| Installation | Git clone | PKG/MSI installer |

## Best Practices
- **Commit .swift-version**: Include in version control for consistency
- **Pin versions**: Use specific versions, not "latest"
- **CI/CD**: Install from .swift-version in pipelines
- **Team alignment**: Ensure all developers use same Swift version
- **Update gradually**: Test before updating project Swift version

## Tips
- Use `.swift-version` for project-specific versions
- `swiftenv global` for system-wide default
- Rehash after manual installations
- Check `swiftenv which swift` to debug version issues
- Use snapshot builds for testing pre-release features

## Agent Use
- Automated Swift version management
- CI/CD pipeline Swift version selection
- Multi-project development environments
- Swift version testing matrices
- Development environment provisioning

## Uninstall
```yaml
- preset: swiftenv
  with:
    state: absent
```

**Note:** Installed Swift versions (`~/.swiftenv/versions/`) are preserved after uninstall.

## Resources
- GitHub: https://github.com/kylef/swiftenv
- Swift downloads: https://swift.org/download/
- Search: "swiftenv tutorial", "swift version management"

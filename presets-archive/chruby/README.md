# chruby - Ruby Version Manager

Lightweight Ruby version manager that changes the current Ruby version with minimal overhead.

## Quick Start
```yaml
- preset: chruby
```

## Features
- **Lightweight**: ~90 lines of shell script
- **Simple**: No shims, rehashing, or configuration files
- **Fast**: Changes Ruby by modifying PATH
- **Flexible**: Works with ruby-install, ruby-build
- **Shell integration**: Auto-switching per directory
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# List installed Rubies
chruby

# Switch to specific Ruby
chruby ruby-3.2.0

# Switch to system Ruby
chruby system

# Show current Ruby
chruby --version
ruby --version
```

## Configuration

### Shell Integration
```bash
# Add to ~/.bashrc or ~/.zshrc
source /usr/local/share/chruby/chruby.sh
source /usr/local/share/chruby/auto.sh

# Set default Ruby
chruby ruby-3.2.0
```

### Auto-switching
```bash
# Create .ruby-version in project
echo "ruby-3.2.0" > .ruby-version

# chruby auto-switches when entering directory
cd myproject  # Automatically switches to ruby-3.2.0
```

## Installing Rubies

### With ruby-install
```bash
# Install ruby-install
# Then install Ruby versions
ruby-install ruby 3.2.0
ruby-install ruby 3.1.4

# List available versions
ruby-install --list
```

## Real-World Examples

### Development Environment
```yaml
- name: Install chruby
  preset: chruby

- name: Configure shell
  shell: |
    echo 'source /usr/local/share/chruby/chruby.sh' >> ~/.bashrc
    echo 'source /usr/local/share/chruby/auto.sh' >> ~/.bashrc

- name: Set default Ruby
  shell: chruby ruby-3.2.0
```

### Project Setup
```yaml
- name: Create .ruby-version
  template:
    dest: /app/.ruby-version
    content: ruby-3.2.0

- name: Install dependencies
  shell: |
    source /usr/local/share/chruby/chruby.sh
    chruby ruby-3.2.0
    bundle install
  cwd: /app
```

## Platform Support
- ✅ Linux (package managers, manual)
- ✅ macOS (Homebrew)
- ❌ Windows (WSL only)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Manage Ruby versions across projects
- Auto-switch Ruby versions per directory
- Lightweight alternative to rbenv/rvm
- Simple Ruby version management in CI/CD
- No overhead for Ruby version switching


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install chruby
  preset: chruby

- name: Use chruby in automation
  shell: |
    # Custom configuration here
    echo "chruby configured"
```
## Uninstall
```yaml
- preset: chruby
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/postmodern/chruby
- ruby-install: https://github.com/postmodern/ruby-install
- Search: "chruby tutorial", "ruby version manager", "chruby vs rbenv"

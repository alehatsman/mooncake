# rbenv - Ruby Version Manager

Lightweight Ruby version management. Install and switch Ruby versions per project without sudo.

## Features

- **Multiple Ruby versions** - MRI, JRuby, TruffleRuby support
- **Shim-based** - No PATH pollution
- **Zero config** - Works out of box
- **Per-project versions** - Automatic .ruby-version detection
- **Plugin system** - ruby-build, rbenv-vars, etc.
- **Shell integration** - Bash, Zsh, Fish
- **Bundler compatible** - Works seamlessly
- **Fast** - < 3000 lines of bash

## Quick Start
```yaml
- preset: rbenv
```

## Basic Usage
```bash
# Install Ruby
rbenv install 3.3.0
rbenv install 3.2.2

# Set version
rbenv global 3.3.0    # System-wide
rbenv local 3.2.2     # Project-specific (.ruby-version)
rbenv shell 3.1.0     # Current shell

# List installed
rbenv versions

# List available
rbenv install --list

# Show current
rbenv version
```

## Shell Integration
```bash
# Bash (~/.bashrc)
export PATH="$HOME/.rbenv/bin:$PATH"
eval "$(rbenv init - bash)"

# Zsh (~/.zshrc)
export PATH="$HOME/.rbenv/bin:$PATH"
eval "$(rbenv init - zsh)"

# Fish (~/.config/fish/config.fish)
set -x PATH $HOME/.rbenv/bin $PATH
status --is-interactive; and rbenv init - fish | source
```

## Version Selection
```bash
# Global (default)
rbenv global 3.3.0
cat ~/.rbenv/version

# Local (project)
rbenv local 3.2.2
cat .ruby-version

# Shell (session)
rbenv shell 3.1.0
echo $RBENV_VERSION

# Unset shell version
rbenv shell --unset
```

## .ruby-version File
```bash
# Create manually
echo "3.3.0" > .ruby-version

# Or use rbenv
rbenv local 3.3.0

# Auto-switch on cd
# rbenv automatically detects .ruby-version
cd myproject  # Switches to version in .ruby-version
```

## Installation
```bash
# Install specific version
rbenv install 3.3.0

# Install latest
rbenv install $(rbenv install --list | grep -v - | tail -1)

# Uninstall
rbenv uninstall 3.2.2

# Verify installation
rbenv versions
ruby --version
```

## ruby-build Plugin
```bash
# Update available versions
cd ~/.rbenv/plugins/ruby-build && git pull

# Install with custom options
RUBY_CONFIGURE_OPTS="--with-openssl-dir=/usr/local/opt/openssl" \
  rbenv install 3.3.0

# Install with jemalloc
RUBY_CONFIGURE_OPTS="--with-jemalloc" rbenv install 3.3.0

# Install from definition
rbenv install /path/to/ruby-definition
```

## Shims and Rehashing
```bash
# Rehash shims (after gem install)
rbenv rehash

# Show shim path
rbenv which ruby
rbenv which bundle
rbenv which rake

# List shims
rbenv shims

# Automatic rehashing (with gem-rehash plugin)
# No need to run rbenv rehash after gem install
```

## Project Workflows
```bash
# New Rails project
cd myproject
rbenv local 3.3.0
gem install bundler
bundle init
bundle add rails

# Clone and setup
git clone repo
cd repo
rbenv install  # Reads .ruby-version
gem install bundler
bundle install

# Multiple projects
cd project-a && ruby --version  # Uses 3.2.2
cd project-b && ruby --version  # Uses 3.3.0
```

## Bundler Integration
```bash
# Install bundler
rbenv global 3.3.0
gem install bundler
rbenv rehash

# Project setup
bundle config set --local path 'vendor/bundle'
bundle install

# Run with correct Ruby
bundle exec rails server
bundle exec rake test
```

## Common Commands
```bash
# Show paths
rbenv root         # ~/.rbenv
rbenv prefix       # /Users/you/.rbenv/versions/3.3.0

# Version info
rbenv version      # Active version with source
rbenv versions     # All installed versions
rbenv version-name # Active version number only

# Environment
rbenv commands     # List all commands
rbenv completions  # Shell completions
rbenv help install # Help for specific command
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Setup Ruby
  uses: ruby/setup-ruby@v1
  with:
    ruby-version: '3.3.0'
    bundler-cache: true

# Or manual rbenv
- name: Install rbenv
  run: |
    git clone https://github.com/rbenv/rbenv.git ~/.rbenv
    echo 'export PATH="$HOME/.rbenv/bin:$PATH"' >> $GITHUB_ENV
    echo 'eval "$(rbenv init - bash)"' >> $GITHUB_ENV
    git clone https://github.com/rbenv/ruby-build.git ~/.rbenv/plugins/ruby-build
    rbenv install 3.3.0
    rbenv global 3.3.0

# GitLab CI
image: ruby:3.3.0

before_script:
  - ruby --version
  - gem install bundler
  - bundle install --jobs $(nproc)

# Docker
FROM ubuntu:22.04
RUN git clone https://github.com/rbenv/rbenv.git ~/.rbenv && \
    git clone https://github.com/rbenv/ruby-build.git ~/.rbenv/plugins/ruby-build
ENV PATH="/root/.rbenv/bin:/root/.rbenv/shims:$PATH"
RUN rbenv install 3.3.0 && rbenv global 3.3.0
```

## Plugins
```bash
# Install plugin
git clone https://github.com/rbenv/ruby-build.git \
  ~/.rbenv/plugins/ruby-build

# Popular plugins
# ruby-build - Install Ruby versions
# rbenv-vars - Manage environment variables
# rbenv-gemset - Manage gemsets
# rbenv-default-gems - Auto-install gems
# rbenv-update - Update rbenv and plugins

# Install rbenv-vars
git clone https://github.com/rbenv/rbenv-vars.git \
  ~/.rbenv/plugins/rbenv-vars
```

## Default Gems
```bash
# Create default gems file
cat > ~/.rbenv/default-gems <<EOF
bundler
rake
pry
rubocop
EOF

# Now all new Ruby installs get these gems
rbenv install 3.3.0  # Installs with default gems
```

## Environment Variables
```bash
# Set via rbenv-vars plugin
echo "export RAILS_ENV=development" > .rbenv-vars
echo "export DATABASE_URL=postgres://localhost/mydb" >> .rbenv-vars

# Variables auto-loaded when entering directory
cd myproject  # RAILS_ENV and DATABASE_URL now set
```

## Performance
```bash
# Speed up installation
export RUBY_BUILD_CACHE_PATH=~/.rbenv/cache
export MAKEFLAGS="-j $(nproc)"

# Use Ruby YJIT (3.1+)
rbenv install 3.3.0
# YJIT enabled by default in 3.3+

# Check YJIT status
ruby --yjit -v
```

## Multiple Ruby Implementations
```bash
# Install JRuby
rbenv install jruby-9.4.5.0

# Install TruffleRuby
rbenv install truffleruby-23.1.2

# Switch between
rbenv global ruby-3.3.0      # MRI Ruby
rbenv local jruby-9.4.5.0    # JRuby
rbenv shell truffleruby-23.1.2  # TruffleRuby
```

## Troubleshooting
```bash
# Ruby not found after install
rbenv rehash

# Version not changing
rbenv version  # Check which version and source
rbenv which ruby  # Check ruby path

# Build failed
# Install dependencies (Ubuntu/Debian)
sudo apt-get install -y \
  build-essential libssl-dev libreadline-dev \
  zlib1g-dev libyaml-dev

# Build failed (macOS)
brew install openssl readline
RUBY_CONFIGURE_OPTS="--with-openssl-dir=$(brew --prefix openssl@3)" \
  rbenv install 3.3.0

# Slow gem install
gem install --no-document gem-name

# Clear build cache
rm -rf ~/.rbenv/cache
```

## Comparison
| Feature | rbenv | rvm | chruby | asdf |
|---------|-------|-----|--------|------|
| Complexity | Simple | Complex | Simplest | Moderate |
| Method | Shims | Override | PATH | Universal |
| Gemsets | Plugin | Built-in | No | No |
| Speed | Fast | Slow | Fastest | Fast |
| Ruby only | Yes | Yes | Yes | No (Multi) |

## Advanced Configuration

### Custom RUBY_ROOT
```bash
export RBENV_ROOT=/opt/rbenv
```

### Build with Custom Options
```bash
# JIT compilation
RUBY_CONFIGURE_OPTS="--enable-yjit" rbenv install 3.3.0

# OpenSSL path
RUBY_CONFIGURE_OPTS="--with-openssl-dir=$(brew --prefix openssl@3)" \
  rbenv install 3.3.0

# Jemalloc allocator
RUBY_CONFIGURE_OPTS="--with-jemalloc" rbenv install 3.3.0

# All optimizations
RUBY_CONFIGURE_OPTS="--enable-yjit --with-jemalloc" \
CFLAGS="-O3 -march=native" \
rbenv install 3.3.0
```

### Speed Optimization
```bash
# Parallel builds
export MAKEFLAGS="-j $(nproc)"

# Cache downloads
export RUBY_BUILD_CACHE_PATH=~/.rbenv/cache
```

### Default Gems
```bash
# Auto-install on every new Ruby
cat > ~/.rbenv/default-gems <<EOF
bundler
rake
pry
rubocop
solargraph
EOF
```

### Environment Variables (rbenv-vars plugin)
```bash
# Per-project environment
cat > .rbenv-vars <<EOF
RAILS_ENV=development
DATABASE_URL=postgres://localhost/mydb
REDIS_URL=redis://localhost:6379
EOF
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `3.3.0` | Ruby version to install |
| `set_global` | bool | `true` | Set as global Ruby version |
| `additional_versions` | array | `[]` | Other versions to install |
| `install_bundler` | bool | `true` | Install Bundler gem |

## Platform Support

- ✅ **Linux** - All distributions
- ✅ **macOS** - Intel and Apple Silicon
- ✅ **FreeBSD/OpenBSD** - Community supported
- ⚠️ **Windows** - WSL only

**Build Requirements:**
- GCC or Clang
- Make
- OpenSSL
- readline
- zlib
- libyaml

## Migration from RVM
```bash
# Remove RVM
rvm implode

# Install rbenv
git clone https://github.com/rbenv/rbenv.git ~/.rbenv
git clone https://github.com/rbenv/ruby-build.git ~/.rbenv/plugins/ruby-build

# Update shell config (remove rvm, add rbenv)
# Remove: [[ -s "$HOME/.rvm/scripts/rvm" ]] && source "$HOME/.rvm/scripts/rvm"
# Add: eval "$(rbenv init - bash)"

# Install Ruby versions
rbenv install 3.3.0
rbenv global 3.3.0
```

## Best Practices
- **Use .ruby-version** for project consistency
- **Commit .ruby-version** to git
- **Rehash after gem installs** (or use plugin)
- **Pin bundler version** in Gemfile.lock
- **Use bundler --deployment** in production
- **Set default gems** for common tools
- **Keep ruby-build updated**

## Tips
- Lightweight (< 3000 lines of bash)
- Zero configuration needed
- Works with Bundler out of box
- Per-project version control
- No PATH pollution
- Compatible with ruby-build
- Shell completion support

## Agent Use
- Automated Ruby version management
- CI/CD pipeline setup
- Multi-version testing
- Development environment setup
- Team version consistency
- Container image builds

## Uninstall
```yaml
- preset: rbenv
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/rbenv/rbenv
- ruby-build: https://github.com/rbenv/ruby-build
- Search: "rbenv install", "rbenv local"

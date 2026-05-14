# RVM - Ruby Version Manager

Manage multiple Ruby environments with isolated gemsets. Switch Ruby versions per project, install gems independently.

## Quick Start
```yaml
- preset: rvm
```

## Features
- **Multiple Ruby versions**: Install and switch between any Ruby version
- **Gemset isolation**: Separate gem dependencies per project
- **Automatic switching**: `.ruby-version` and `.ruby-gemset` file support
- **Built-in Ruby installer**: No manual compilation needed
- **Patch management**: Easy security updates
- **Integration**: Works with Bundler, Rails, system shells
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# List known Ruby versions
rvm list known

# Install Ruby
rvm install 3.2.0

# Use specific version
rvm use 3.2.0

# Set default version
rvm use 3.2.0 --default

# List installed versions
rvm list

# Check current version
rvm current
```

## Advanced Configuration
```yaml
# Install RVM (default)
- preset: rvm

# Uninstall RVM
- preset: rvm
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (all versions)
- ✅ BSD
- ❌ Windows (use WSL)

## Configuration
- **RVM directory**: `~/.rvm/`
- **User config**: `~/.rvmrc`
- **Project config**: `.ruby-version`, `.ruby-gemset`
- **Shell integration**: Added to `~/.bashrc`, `~/.zshrc`
- **Rubies**: `~/.rvm/rubies/`
- **Gemsets**: `~/.rvm/gems/`

## Ruby Version Management
```bash
# Install specific version
rvm install 3.2.0
rvm install 3.1.4
rvm install 2.7.8

# Install with custom name
rvm install 3.2.0 --name my-ruby

# Use version
rvm use 3.2.0

# Set default
rvm use 3.2.0 --default

# List versions
rvm list
rvm list rubies

# Remove version
rvm remove 3.1.4

# Upgrade Ruby
rvm upgrade 3.1.4 3.2.0
```

## Gemset Management
```bash
# Create gemset
rvm gemset create myproject

# Use gemset
rvm use 3.2.0@myproject

# List gemsets
rvm gemset list

# Delete gemset
rvm gemset delete myproject

# Empty gemset
rvm gemset empty myproject

# Copy gemset
rvm gemset copy 3.2.0@project1 3.2.0@project2

# Export gemset
rvm gemset export myproject.gems

# Import gemset
rvm gemset import myproject.gems
```

## Project Configuration
```bash
# Create .ruby-version file
echo "3.2.0" > .ruby-version

# Create .ruby-gemset file
echo "myproject" > .ruby-gemset

# RVM auto-switches when entering directory
cd ~/projects/myproject  # Automatically uses 3.2.0@myproject
```

## Installation Options
```bash
# Install latest stable
rvm install ruby

# Install with docs
rvm install 3.2.0 --with-docs

# Install with custom configure flags
rvm install 3.2.0 --with-openssl-dir=/usr/local/opt/openssl

# Reinstall
rvm reinstall 3.2.0

# Install MRI (default)
rvm install ruby-3.2.0

# Install JRuby
rvm install jruby

# Install Rubinius
rvm install rbx
```

## Updating RVM
```bash
# Update RVM itself
rvm get stable

# Update to latest
rvm get head

# Update Ruby definitions
rvm reload
```

## Integration
```bash
# Install Bundler
gem install bundler

# Use with Bundler
rvm use 3.2.0@myproject
bundle install

# Run command in gemset
rvm 3.2.0@myproject do bundle exec rails server

# Run in all gemsets
rvm all do gem update

# Run in multiple versions
rvm 3.2.0,3.1.4 do ruby --version
```

## Aliases
```bash
# Create alias
rvm alias create default 3.2.0

# List aliases
rvm alias list

# Delete alias
rvm alias delete myalias

# Use alias
rvm use default
```

## Requirements
```bash
# Show requirements for current OS
rvm requirements

# Install requirements (macOS)
rvm requirements run

# Install requirements (Linux)
# Automatically installs build dependencies
```

## Real-World Examples

### Development Environment Setup
```bash
# Install latest Ruby
rvm install 3.2.0
rvm use 3.2.0 --default

# Create project gemset
rvm gemset create rails-app
rvm use 3.2.0@rails-app

# Install Rails
gem install rails

# Create project
rails new myapp
cd myapp

# Set project Ruby and gemset
echo "3.2.0" > .ruby-version
echo "rails-app" > .ruby-gemset

# Future directory entries auto-switch
```

### Multiple Projects
```bash
# Project 1 (Rails 7)
cd ~/project1
echo "3.2.0" > .ruby-version
echo "project1" > .ruby-gemset
rvm use 3.2.0@project1
gem install rails -v 7.0.0
bundle install

# Project 2 (Rails 6)
cd ~/project2
echo "3.1.4" > .ruby-version
echo "project2" > .ruby-gemset
rvm use 3.1.4@project2
gem install rails -v 6.1.0
bundle install

# Auto-switching
cd ~/project1  # Uses Ruby 3.2.0@project1
cd ~/project2  # Uses Ruby 3.1.4@project2
```

### CI/CD Integration
```yaml
# .gitlab-ci.yml
test:
  before_script:
    - preset: rvm
    - shell: rvm use 3.2.0
    - shell: gem install bundler
    - shell: bundle install
  script:
    - bundle exec rspec
```

### Testing Multiple Ruby Versions
```bash
# Test on multiple versions
for version in 3.2.0 3.1.4 2.7.8; do
  echo "Testing on Ruby $version"
  rvm use $version
  bundle install
  bundle exec rspec
done
```

## Troubleshooting

### RVM command not found
Shell integration not loaded. Source RVM:
```bash
source ~/.rvm/scripts/rvm
```

Or add to shell config:
```bash
# Add to ~/.bashrc or ~/.zshrc
[[ -s "$HOME/.rvm/scripts/rvm" ]] && source "$HOME/.rvm/scripts/rvm"
```

### Ruby install fails
Install build requirements:
```bash
# macOS
rvm requirements
brew install openssl readline

# Ubuntu/Debian
sudo apt-get install build-essential libssl-dev libreadline-dev

# Fedora/RHEL
sudo dnf install gcc make openssl-devel readline-devel
```

### Gemset not switching
Ensure `.ruby-version` and `.ruby-gemset` exist:
```bash
ls -la .ruby-version .ruby-gemset
cat .ruby-version  # Should show Ruby version
cat .ruby-gemset   # Should show gemset name
```

### Slow shell startup
RVM adds to shell init. Reduce overhead:
```bash
# Use lazy loading in ~/.zshrc
export PATH="$PATH:$HOME/.rvm/bin"
# Remove full RVM initialization
```

## Best Practices
- Use `.ruby-version` and `.ruby-gemset` in projects
- Create separate gemsets for each project
- Keep RVM updated: `rvm get stable`
- Document Ruby version in README
- Use Bundler for gem dependencies
- Test on multiple Ruby versions before release
- Clean old Ruby versions: `rvm remove old-version`

## Security
```bash
# Verify RVM installation
rvm --version

# Update to latest stable (includes security fixes)
rvm get stable

# Update Ruby (security patches)
rvm upgrade 3.1.4 3.1.5

# Check Ruby security advisories
# Visit: https://www.ruby-lang.org/en/security/
```

## Agent Use
- Set up development environments with specific Ruby versions
- Create isolated testing environments for CI/CD
- Automate gemset configuration for deployment
- Test compatibility across Ruby versions
- Manage Ruby dependencies in multi-tenant systems
- Provision consistent development environments
- Automate Ruby security updates across projects

## Uninstall
```yaml
- preset: rvm
  with:
    state: absent
```

Manual uninstall:
```bash
rvm implode
rm -rf ~/.rvm
# Remove RVM lines from ~/.bashrc, ~/.zshrc
```

## Resources
- Official site: https://rvm.io/
- Documentation: https://rvm.io/rvm/basics
- GitHub: https://github.com/rvm/rvm
- Gemsets guide: https://rvm.io/gemsets/basics
- Search: "rvm ruby version", "rvm gemsets", "rvm install ruby"

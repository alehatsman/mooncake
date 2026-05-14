# Ruby Preset

Install Ruby via rbenv (Ruby version manager) with support for multiple versions.

## Features

- **Multiple Ruby versions** - Install Ruby 2.7-3.3+
- **Version manager** - Uses rbenv for isolation
- **Per-project versions** - Automatic .ruby-version support
- **Bundler included** - Dependency management
- **Zero sudo** - User-level installation
- **Build dependencies** - Auto-installed for compilation
- **Shell integration** - Bash, Zsh, Fish support
- **Cross-platform** - Linux and macOS

## Quick Start

```yaml
- preset: ruby
```

## Basic Usage

After installation:
```bash
# Verify installation
ruby --version
gem --version
bundle --version

# Check installed versions
rbenv versions

# Run Ruby code
ruby -e 'puts "Hello, Ruby!"'

# Interactive REPL
irb
>>> puts "Hello from IRB"
>>> exit

# Install gems
gem install rails sinatra rspec

# Show gem info
gem list
gem info rails
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `3.3.0` | Ruby version to install |
| `set_global` | bool | `true` | Set as global Ruby version |
| `additional_versions` | array | `[]` | Other Ruby versions to install |
| `install_bundler` | bool | `true` | Install Bundler gem |

## Usage

### Latest Ruby
```yaml
- preset: ruby
  with:
    version: "3.3.0"
```

### Multiple Versions
```yaml
- preset: ruby
  with:
    version: "3.3.0"
    additional_versions:
      - "3.2.2"
      - "2.7.8"
```

### Rails Development
```yaml
- preset: ruby
  with:
    version: "3.2.2"
    install_bundler: true
```

## Verify Installation

```bash
# Check version
ruby --version
gem --version
bundle --version

# List installed versions
rbenv versions

# Create a simple Ruby script
echo 'puts "Hello from Ruby!"' > hello.rb
ruby hello.rb
```

## Common Operations

```bash
# Manage Ruby versions
rbenv install 3.2.2
rbenv versions
rbenv global 3.2.2
rbenv local 3.3.0

# Gem management
gem install rails
gem list
gem update
gem uninstall package_name

# Bundler
bundle init
bundle install
bundle exec rails server
bundle update
```

## Rails Application

```bash
# Install Rails
gem install rails

# Create new app
rails new myapp
cd myapp

# Install dependencies
bundle install

# Start server
rails server

# Generate model
rails generate model User name:string email:string

# Run migrations
rails db:migrate
```

## Gemfile Example

```ruby
source 'https://rubygems.org'

ruby '3.3.0'

gem 'rails', '~> 7.1.0'
gem 'pg', '~> 1.5'
gem 'puma', '~> 6.0'
gem 'redis', '~> 5.0'

group :development, :test do
  gem 'debug'
  gem 'rspec-rails'
end

group :development do
  gem 'rubocop'
end
```

## Project Setup

```bash
# Create project
mkdir myproject
cd myproject

# Set Ruby version
rbenv local 3.3.0

# Create Gemfile
bundle init

# Add gems
bundle add sinatra

# Run script
ruby app.rb
```

## Version Management

```bash
# Install specific version
rbenv install 3.2.2

# Set global version
rbenv global 3.3.0

# Set local version (creates .ruby-version)
rbenv local 3.2.2

# Show current version
rbenv version

# List available versions
rbenv install --list
```

## Advanced Configuration

### Performance Optimization
```bash
# Build with YJIT (Ruby 3.3+)
RUBY_CONFIGURE_OPTS="--enable-yjit" rbenv install 3.3.0

# With jemalloc
RUBY_CONFIGURE_OPTS="--with-jemalloc" rbenv install 3.3.0

# All optimizations
RUBY_CONFIGURE_OPTS="--enable-yjit --with-jemalloc" \
CFLAGS="-O3 -march=native" \
rbenv install 3.3.0

# Parallel builds
MAKEFLAGS="-j $(nproc)" rbenv install 3.3.0
```

### Default Gems
```bash
# Auto-install on new Ruby versions
cat > ~/.rbenv/default-gems <<EOF
bundler
rake
pry
rubocop
solargraph
EOF

rbenv install 3.3.0  # Gets default gems
```

### Custom OpenSSL (macOS)
```bash
RUBY_CONFIGURE_OPTS="--with-openssl-dir=$(brew --prefix openssl@3)" \
rbenv install 3.3.0
```

### Environment Variables
```bash
# Use rbenv-vars plugin
git clone https://github.com/rbenv/rbenv-vars.git \
  ~/.rbenv/plugins/rbenv-vars

# Set per-project vars
cat > .rbenv-vars <<EOF
RAILS_ENV=development
DATABASE_URL=postgres://localhost/mydb
EOF
```

### Bundler Configuration
```bash
# Use local gems
bundle config set --local path 'vendor/bundle'

# Parallel installs
bundle config set --local jobs 4

# Skip production gems
bundle config set --local without 'production'
```

## Platform Support

- ✅ **Linux** - All distributions
- ✅ **macOS** - Intel and Apple Silicon
- ⚠️ **Windows** - WSL only

**Build Requirements:**
- GCC/Clang
- Make
- OpenSSL
- readline
- zlib
- libyaml

## Agent Use

Ruby + rbenv is useful for agent automation and DevOps tasks:

### Infrastructure Automation
```yaml
# Install Ruby for Chef/Puppet agents
- preset: ruby
  with:
    version: "3.3.0"
    install_bundler: true

- name: Install automation tools
  shell: |
    eval "$(rbenv init -)"
    gem install chef puppet rake
```

### Web Scraping Agents
```ruby
require 'nokogiri'
require 'open-uri'

# Agent scrapes and processes data
def scrape_data(url)
  doc = Nokogiri::HTML(URI.open(url))
  doc.css('.data').map(&:text)
end
```

### CI/CD Agents
```yaml
# Multi-version testing
- preset: ruby
  with:
    version: "3.3.0"
    additional_versions: ["3.2.2", "3.1.4"]

- name: Test on all versions
  shell: |
    for version in 3.3.0 3.2.2 3.1.4; do
      rbenv global $version
      bundle install
      bundle exec rspec
    done
```

### API Clients
```ruby
require 'faraday'

# Agent interacts with APIs
class APIAgent
  def initialize(base_url)
    @conn = Faraday.new(url: base_url)
  end

  def fetch(endpoint)
    @conn.get(endpoint).body
  end
end
```

### Task Automation
```ruby
require 'rake'

# Rakefile for agent tasks
task :deploy do
  sh "git pull"
  sh "bundle install"
  sh "rails db:migrate"
  sh "systemctl restart app"
end
```

Benefits for agents:
- **Isolation** - Per-project Ruby versions
- **Fast scripting** - Rapid prototyping
- **Rich ecosystem** - Gems for everything
- **DSLs** - Clean configuration languages
- **Testing** - RSpec, Minitest built-in

## Troubleshooting

```bash
# Rehash shims after installing gems
rbenv rehash

# Update rbenv
cd ~/.rbenv
git pull

# Update ruby-build
cd ~/.rbenv/plugins/ruby-build
git pull
```

## Uninstall

```yaml
- preset: ruby
  with:
    state: absent
```

**Note:** This removes rbenv and all Ruby versions.

## Resources
- Official site: https://www.ruby-lang.org/
- Documentation: https://docs.ruby-lang.org/
- RubyGems: https://rubygems.org/
- Bundler: https://bundler.io/
- Ruby Style Guide: https://rubystyle.guide/
- Search: "ruby programming tutorial", "bundler guide", "ruby best practices"

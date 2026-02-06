# Ruby Preset

Install Ruby via rbenv (Ruby version manager) with support for multiple versions.

## Quick Start

```yaml
- preset: ruby
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

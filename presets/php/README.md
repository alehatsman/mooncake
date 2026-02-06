# PHP Preset

Install PHP programming language with Composer package manager.

## Quick Start

```yaml
- preset: php
  with:
    version: "8.3"
    install_composer: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `8.3` | PHP version (7.4, 8.0-8.3) |
| `install_composer` | bool | `true` | Install Composer |
| `extensions` | array | `[]` | Extensions to install |

## Usage

### Latest PHP
```yaml
- preset: php
```

### Specific Version with Extensions
```yaml
- preset: php
  with:
    version: "8.2"
    extensions:
      - mbstring
      - xml
      - curl
      - zip
      - gd
      - mysql
```

### Laravel Development
```yaml
- preset: php
  with:
    version: "8.2"
    extensions: [mbstring, xml, curl, zip, gd, mysql, redis]
    install_composer: true
```

## Verify Installation

```bash
php -v
php -m  # List modules
composer --version
```

## Common Operations

```bash
# Run PHP script
php script.php

# Start built-in server
php -S localhost:8000

# Check syntax
php -l script.php

# Interactive shell
php -a
```

## Composer

```bash
# Create project
composer create-project laravel/laravel myapp

# Install dependencies
composer install

# Add package
composer require vendor/package

# Update packages
composer update

# Autoload
composer dump-autoload
```

## Laravel

```bash
# Install Laravel
composer global require laravel/installer

# Create project
laravel new myapp
cd myapp

# Start server
php artisan serve

# Run migrations
php artisan migrate
```

## Common Extensions

- `mbstring` - Multibyte string
- `xml` - XML support
- `curl` - HTTP client
- `zip` - ZIP archive
- `gd` - Image processing
- `mysql` / `pgsql` - Database
- `redis` - Redis support
- `opcache` - Performance

## Configuration

- **php.ini**: `/etc/php/{{ version }}/cli/php.ini` (Linux)
- **php.ini**: `/opt/homebrew/etc/php/{{ version }}/php.ini` (macOS)

## Uninstall

```yaml
- preset: php
  with:
    state: absent
```

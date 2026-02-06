# php - PHP Programming Language

Server-side scripting language designed for web development with Composer package manager.

## Quick Start
```yaml
- preset: php
```

## Features
- **Fast**: JIT compilation in PHP 8+
- **Versatile**: Web applications, CLI scripts, APIs
- **Popular**: Powers WordPress, Laravel, Symfony
- **Composer**: Modern dependency management
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Check version
php -v

# Run script
php script.php

# Start development server
php -S localhost:8000

# Check syntax
php -l script.php

# Interactive shell
php -a

# List loaded extensions
php -m
```

## Advanced Configuration
```yaml
- preset: php
  with:
    version: "8.3"              # PHP version (7.4, 8.0, 8.1, 8.2, 8.3)
    install_composer: true      # Install Composer
    extensions:                 # PHP extensions
      - mbstring
      - xml
      - curl
      - zip
      - gd
      - mysql
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove PHP |
| version | string | 8.3 | PHP version to install |
| install_composer | bool | true | Install Composer package manager |
| extensions | array | [] | PHP extensions to install |

## Platform Support
- ✅ Linux (apt, dnf, yum)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **php.ini (Linux)**: `/etc/php/<version>/cli/php.ini`
- **php.ini (macOS)**: `/opt/homebrew/etc/php/<version>/php.ini`
- **Composer global**: `~/.composer/` or `~/.config/composer/`

## Real-World Examples

### Laravel Development Setup
```yaml
- preset: php
  with:
    version: "8.3"
    extensions:
      - mbstring
      - xml
      - curl
      - zip
      - gd
      - mysql
      - redis
    install_composer: true

- name: Install Laravel
  shell: composer global require laravel/installer

- name: Create Laravel project
  shell: composer create-project laravel/laravel myapp
```

### WordPress Development
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
      - imagick

- name: Download WordPress
  shell: |
    curl -O https://wordpress.org/latest.tar.gz
    tar -xzf latest.tar.gz
    rm latest.tar.gz
```

### API Development
```bash
# Create simple API
cat > api.php <<'EOF'
<?php
header('Content-Type: application/json');
echo json_encode(['message' => 'Hello from PHP API']);
EOF

# Start server
php -S localhost:8000 api.php

# Test
curl http://localhost:8000
```

### Composer Package Management
```bash
# Initialize project
composer init

# Install dependencies
composer require guzzlehttp/guzzle

# Install dev dependencies
composer require --dev phpunit/phpunit

# Update all packages
composer update

# Generate autoloader
composer dump-autoload
```

## Agent Use
- Deploy PHP web applications and APIs
- Run Laravel/Symfony application servers
- Execute PHP scripts for data processing
- Manage dependencies with Composer
- Run PHP-based CI/CD tasks (PHPUnit, PHP CodeSniffer)

## Common Extensions
- **mbstring**: Multibyte string support
- **xml**: XML parsing and generation
- **curl**: HTTP client for API calls
- **zip**: ZIP archive handling
- **gd/imagick**: Image processing
- **mysql/pgsql**: Database connectivity
- **redis**: Redis cache integration
- **opcache**: Performance optimization

## Troubleshooting

### Extension not found
Install specific extension:
```bash
# Debian/Ubuntu
sudo apt-get install php8.3-mbstring

# macOS (via PECL)
pecl install extension-name
```

### Composer command not found
Add to PATH:
```bash
echo 'export PATH="$HOME/.composer/vendor/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Version conflicts
Check active version:
```bash
# Linux
update-alternatives --display php

# macOS
brew link --overwrite --force php@8.3
```

## Uninstall
```yaml
- preset: php
  with:
    state: absent
```

## Resources
- Official docs: https://www.php.net/docs.php
- Composer: https://getcomposer.org/doc/
- Laravel: https://laravel.com/docs
- Search: "php tutorial", "composer guide"

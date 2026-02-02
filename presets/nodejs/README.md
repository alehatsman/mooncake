# Node.js/nvm Preset

Install Node.js via nvm (Node Version Manager) for easy version management.

## Features

- ✅ Installs nvm (Node Version Manager)
- ✅ Installs specified Node.js version
- ✅ Supports installing multiple Node versions
- ✅ Sets default Node version
- ✅ Installs global npm packages
- ✅ Configures shell profiles automatically
- ✅ Cross-platform (Linux, macOS)

## Usage

### Install latest LTS Node.js
```yaml
- name: Install Node.js LTS
  preset: nodejs
```

### Install specific Node.js version
```yaml
- name: Install Node.js 20
  preset: nodejs
  with:
    version: "20.10.0"
```

### Install multiple versions
```yaml
- name: Install Node.js with multiple versions
  preset: nodejs
  with:
    version: lts          # Main version (set as default)
    additional_versions:
      - "18.19.0"
      - "20.10.0"
    set_default: true
```

### Install with global packages
```yaml
- name: Install Node.js with global tools
  preset: nodejs
  with:
    version: lts
    global_packages:
      - typescript
      - eslint
      - prettier
      - nodemon
      - pm2
```

### Uninstall
```yaml
- name: Remove Node.js and nvm
  preset: nodejs
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `lts` | Node version: "20.10.0", "lts", "latest", "18" |
| `set_default` | bool | `true` | Set as default Node version |
| `additional_versions` | array | `[]` | Other Node versions to install |
| `global_packages` | array | `[]` | npm packages to install globally |

## Version Formats

nvm supports various version formats:
- `lts` - Latest LTS (Long Term Support) version
- `latest` - Latest stable version
- `20` - Latest 20.x.x version
- `20.10.0` - Exact version
- `lts/hydrogen` - Specific LTS codename

## Platform Support

- ✅ Linux (all distributions)
- ✅ macOS
- ❌ Windows (use nvm-windows separately)

## What Gets Installed

1. **nvm** - Node Version Manager
   - Installed to `~/.nvm/`
   - Added to `~/.bashrc`, `~/.zshrc`, `~/.profile`

2. **Node.js** - JavaScript runtime
   - Installed via nvm
   - Multiple versions can coexist

3. **npm** - Node package manager
   - Comes with Node.js
   - Used for installing packages

## Common Use Cases

### Development Environment
```yaml
- name: Setup Node.js dev environment
  preset: nodejs
  with:
    version: lts
    global_packages:
      - typescript      # TypeScript compiler
      - ts-node        # TypeScript execution
      - eslint         # Linter
      - prettier       # Code formatter
      - nodemon        # Auto-restart on changes
```

### Production Server
```yaml
- name: Setup Node.js production
  preset: nodejs
  with:
    version: "20.10.0"   # Pin specific version
    global_packages:
      - pm2              # Process manager
```

### Testing Multiple Versions
```yaml
- name: Install Node.js for CI testing
  preset: nodejs
  with:
    version: "20"
    additional_versions:
      - "18"
      - "16"
```

## Post-Installation

After installation, restart your terminal or run:
```bash
source ~/.bashrc  # or ~/.zshrc
```

### Using nvm
```bash
# List installed versions
nvm ls

# Install another version
nvm install 18

# Switch version
nvm use 18

# Set default version
nvm alias default 20

# Run command with specific version
nvm exec 18 node script.js
```

### Verify Installation
```bash
node --version
npm --version
nvm --version
```

## Learn More

- [nvm GitHub](https://github.com/nvm-sh/nvm)
- [Node.js Documentation](https://nodejs.org/docs/)
- [npm Documentation](https://docs.npmjs.com/)

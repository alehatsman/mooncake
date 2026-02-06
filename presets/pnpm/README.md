# pnpm - Fast Disk-Efficient Package Manager

Fast, disk space efficient package manager for Node.js that uses content-addressable storage.

## Quick Start
```yaml
- preset: pnpm
```

## Features
- **Fast**: Up to 2x faster than npm
- **Efficient**: Saves disk space with content-addressable storage
- **Strict**: Non-flat node_modules prevents phantom dependencies
- **Monorepo support**: Built-in workspace support
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Install dependencies from package.json
pnpm install

# Add package
pnpm add express

# Add dev dependency
pnpm add -D typescript

# Remove package
pnpm remove express

# Update packages
pnpm update

# Run script
pnpm run build

# Execute binary
pnpm exec eslint .
```

## Advanced Configuration
```yaml
- preset: pnpm
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove pnpm |

## Platform Support
- ✅ Linux (npm)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **pnpm-workspace.yaml**: Workspace configuration for monorepos
- **. pnpmfile.js**: Hooks for customizing dependency resolution
- **.npmrc**: npm configuration (shared with pnpm)
- **Store location**: `~/.pnpm-store/` (shared across projects)

## Real-World Examples

### New Project
```bash
# Initialize project
pnpm init

# Install dependencies
pnpm add react react-dom
pnpm add -D vite @vitejs/plugin-react

# Run dev server
pnpm run dev
```

### Existing Project
```bash
# Clone and install
git clone https://github.com/org/project.git
cd project
pnpm install

# Run build
pnpm run build

# Run tests
pnpm test
```

### Monorepo Workspace
```yaml
# pnpm-workspace.yaml
packages:
  - 'packages/*'
  - 'apps/*'
```

```bash
# Install all workspace dependencies
pnpm install

# Run command in specific package
pnpm --filter @myorg/api run build

# Run command in all packages
pnpm -r run test
```

### CI/CD Pipeline
```yaml
- preset: pnpm

- name: Install dependencies
  shell: pnpm install --frozen-lockfile
  cwd: /app

- name: Run linter
  shell: pnpm run lint
  cwd: /app

- name: Run tests
  shell: pnpm test
  cwd: /app

- name: Build production
  shell: pnpm run build
  cwd: /app
```

## Agent Use
- Install Node.js project dependencies efficiently
- Build and test JavaScript/TypeScript applications
- Manage monorepo workspaces
- Run package scripts in CI/CD pipelines
- Reduce disk usage in containerized environments

## Common Commands
```bash
# List installed packages
pnpm list

# Check for outdated packages
pnpm outdated

# Interactive package updates
pnpm up --interactive

# Prune unused packages
pnpm prune

# Store management
pnpm store status
pnpm store prune

# Workspace commands
pnpm -r exec -- rm -rf node_modules
pnpm -r update
```

## Troubleshooting

### Store corruption
```bash
# Verify store integrity
pnpm store status

# Prune unreferenced packages
pnpm store prune
```

### Lockfile out of sync
```bash
# Update lockfile
pnpm install

# Or enforce frozen lockfile (CI)
pnpm install --frozen-lockfile
```

### Peer dependency conflicts
```bash
# Show why a package is installed
pnpm why package-name

# Auto-install peer dependencies
pnpm install --auto-install-peers
```

## Uninstall
```yaml
- preset: pnpm
  with:
    state: absent
```

## Resources
- Official docs: https://pnpm.io/
- GitHub: https://github.com/pnpm/pnpm
- Search: "pnpm tutorial", "pnpm vs npm"

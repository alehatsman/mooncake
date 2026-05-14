# Yarn - Fast, Reliable Package Manager

Fast, reliable, and secure dependency management for JavaScript and Node.js projects.

## Quick Start
```yaml
- preset: yarn
```

## Features
- **Fast**: Caches packages for offline installation and parallel downloads
- **Reliable**: Lock file ensures consistent installs across machines
- **Secure**: Checksums verify package integrity before execution
- **Network Performance**: Efficient resolution and fetching algorithms
- **Workspaces**: Monorepo support with workspace management
- **Plug'n'Play**: Optional zero-install mode (Yarn 2+)

## Basic Usage
```bash
# Initialize project
yarn init
yarn init -y

# Install dependencies
yarn install
yarn

# Add dependencies
yarn add react
yarn add --dev jest
yarn add --peer react-dom

# Remove dependencies
yarn remove package-name

# Upgrade dependencies
yarn upgrade
yarn upgrade package-name
yarn upgrade-interactive

# Run scripts
yarn run test
yarn run build
yarn test  # shorthand

# Global packages
yarn global add package-name
yarn global list
```

## Advanced Configuration
```yaml
- preset: yarn
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Yarn |

## Platform Support
- ✅ Linux (npm, binary)
- ✅ macOS (Homebrew, npm)
- ✅ Windows (npm, Scoop, Chocolatey)

## Configuration
- **Config file**: `.yarnrc` or `.yarnrc.yml` (Yarn 2+)
- **Lock file**: `yarn.lock`
- **Cache**: `~/.yarn/cache/` (Yarn 1), `.yarn/cache/` (Yarn 2+)
- **Global packages**: `~/.config/yarn/global/`

## Package Management

```bash
# Install specific version
yarn add package@1.2.3
yarn add package@^1.0.0
yarn add package@latest

# Install from GitHub
yarn add user/repo
yarn add user/repo#branch

# Install from local path
yarn add file:../local-package

# List installed packages
yarn list
yarn list --depth=0
yarn list --pattern eslint

# Info about package
yarn info package-name
yarn info package-name versions

# Check outdated packages
yarn outdated

# Clean cache
yarn cache clean
```

## Scripts

```json
{
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "test": "jest",
    "lint": "eslint ."
  }
}
```

```bash
# Run scripts
yarn dev
yarn build
yarn start
yarn test
yarn lint

# Run with arguments
yarn test --watch
yarn lint --fix
```

## Workspaces (Monorepos)

```json
{
  "private": true,
  "workspaces": [
    "packages/*"
  ]
}
```

```bash
# Install all workspace dependencies
yarn install

# Add dependency to specific workspace
yarn workspace package-a add lodash

# Run command in workspace
yarn workspace package-a run build

# Run command in all workspaces
yarn workspaces run build
yarn workspaces run test

# List workspaces
yarn workspaces info
```

## Real-World Examples

### Project Setup
```bash
# Create new project
mkdir my-app && cd my-app
yarn init -y

# Add dependencies
yarn add react react-dom next
yarn add --dev @types/react typescript jest

# Create scripts
cat >> package.json <<'EOF'
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "test": "jest"
  }
EOF

# Install and run
yarn install
yarn dev
```

### CI/CD Integration
```yaml
- name: Install Yarn
  preset: yarn

- name: Install dependencies (with cache)
  shell: yarn install --frozen-lockfile --prefer-offline
  cwd: /app
  env:
    CI: true

- name: Run tests
  shell: yarn test --ci --coverage
  cwd: /app

- name: Build application
  shell: yarn build
  cwd: /app

- name: Deploy artifacts
  copy:
    src: /app/dist/
    dest: /var/www/html/
  become: true
```

### Monorepo Management
```bash
# Project structure
my-monorepo/
├── package.json
├── packages/
│   ├── app/
│   │   └── package.json
│   ├── shared/
│   │   └── package.json
│   └── api/
│       └── package.json

# Root package.json
{
  "private": true,
  "workspaces": ["packages/*"]
}

# Install all dependencies
yarn install

# Build all packages
yarn workspaces run build

# Run tests in all packages
yarn workspaces run test

# Add dependency to specific package
yarn workspace @myorg/app add react
yarn workspace @myorg/api add express
```

### Docker Integration
```dockerfile
FROM node:18-alpine

WORKDIR /app

# Copy package files
COPY package.json yarn.lock ./

# Install dependencies
RUN yarn install --frozen-lockfile --production

# Copy application
COPY . .

# Build
RUN yarn build

CMD ["yarn", "start"]
```

## Yarn 2+ (Berry)

```bash
# Upgrade to Yarn 2+
yarn set version berry
yarn set version stable

# PnP (Plug'n'Play) mode
yarn install

# Zero-install (commit .yarn/cache)
git add .yarn/cache
git commit -m "Enable zero-install"

# Plugins
yarn plugin import interactive-tools
yarn plugin import workspace-tools
```

## Performance Optimization

```bash
# Frozen lockfile (CI)
yarn install --frozen-lockfile

# Prefer offline
yarn install --prefer-offline

# Ignore scripts
yarn install --ignore-scripts

# Production only
yarn install --production

# Clean install
rm -rf node_modules yarn.lock
yarn install
```

## Agent Use
- Automated dependency installation in CI/CD
- Package version management and upgrades
- Monorepo workspace orchestration
- Build pipeline integration
- Security audit automation
- Cache optimization for faster builds

## Troubleshooting

### Dependency conflicts
```bash
# Clear cache
yarn cache clean

# Remove node_modules and reinstall
rm -rf node_modules yarn.lock
yarn install

# Check for duplicates
yarn list --pattern package-name
```

### Slow installation
```bash
# Use offline mode
yarn install --offline

# Check network
yarn config set network-timeout 600000

# Use different registry
yarn config set registry https://registry.npmjs.org/
```

### Lock file conflicts
```bash
# Merge conflicts in yarn.lock
# 1. Accept changes
# 2. Regenerate lock file
rm yarn.lock
yarn install
```

## Comparison with npm

| Feature | Yarn | npm |
|---------|------|-----|
| Speed | Faster | Slower |
| Lock file | yarn.lock | package-lock.json |
| Workspaces | Yes | Yes |
| Offline | Yes | Limited |
| Deterministic | Yes | Yes |
| PnP Mode | Yes (v2+) | No |

## Uninstall
```yaml
- preset: yarn
  with:
    state: absent
```

## Resources
- Official docs: https://yarnpkg.com/
- GitHub: https://github.com/yarnpkg/yarn
- Search: "yarn package manager", "yarn vs npm"

# Volta - The Hassle-Free JavaScript Tool Manager

Fast, reliable, and secure JavaScript toolchain manager that ensures everyone on your team uses the same versions.

## Quick Start
```yaml
- preset: volta
```

## Features
- **Version Management**: Install and manage multiple Node.js and package manager versions
- **Project Pinning**: Lock tool versions in package.json for consistent environments
- **Fast Switching**: Automatically switches Node/npm/yarn versions per project
- **Cross-platform**: Works seamlessly on Linux, macOS, and Windows
- **Team Consistency**: Ensures entire team uses same tool versions
- **Binary Caching**: Fast installation with built-in binary caching

## Basic Usage
```bash
# Check installed version
volta --version

# Install Node.js
volta install node                 # Latest LTS
volta install node@18              # Specific major version
volta install node@18.17.0         # Exact version

# Install package managers
volta install npm@9.8.0
volta install yarn@1.22.19
volta install pnpm@8.6.12

# Install global packages
volta install typescript
volta install eslint
volta install @vue/cli

# Pin versions for project (adds to package.json)
volta pin node@18.17.0
volta pin npm@9.8.0
volta pin yarn@1.22.19

# List installed tools
volta list
volta list all

# Run specific Node version
volta run --node 16.20.0 -- node server.js
```

## Advanced Configuration
```yaml
- preset: volta
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Volta |

## Platform Support
- ✅ Linux (any distribution)
- ✅ macOS (Intel and Apple Silicon)
- ✅ Windows (PowerShell and cmd)

## Configuration
- **Installation directory**: `~/.volta/` (all platforms)
- **Binary cache**: `~/.volta/cache/`
- **Tools directory**: `~/.volta/tools/`
- **Shims**: `~/.volta/bin/` (must be in PATH)

## Project Configuration

Volta automatically reads tool versions from `package.json`:

```json
{
  "name": "my-project",
  "volta": {
    "node": "18.17.0",
    "npm": "9.8.0",
    "yarn": "1.22.19"
  }
}
```

When you `cd` into this directory, Volta automatically switches to these versions.

## Real-World Examples

### Team Development Setup
```bash
# Team lead pins versions
cd my-project
volta pin node@18.17.0
volta pin npm@9.8.0
# Commits package.json with "volta" section

# Team members clone repo and install
git clone https://github.com/team/my-project
cd my-project
volta install node  # Installs pinned version automatically
npm install
```

### Multiple Projects with Different Versions
```bash
# Project A uses Node 16
cd ~/projects/project-a
volta pin node@16.20.0
node --version  # v16.20.0

# Project B uses Node 18
cd ~/projects/project-b
volta pin node@18.17.0
node --version  # v18.17.0

# Automatic switching - no manual nvm use!
cd ~/projects/project-a
node --version  # v16.20.0 automatically
```

### CI/CD Integration
```yaml
- name: Install Volta
  preset: volta

- name: Install Node.js version from package.json
  shell: volta install node
  cwd: /app

- name: Install dependencies
  shell: npm ci
  cwd: /app

- name: Build application
  shell: npm run build
  cwd: /app
```

### Global Tools Management
```bash
# Install commonly used global tools
volta install typescript eslint prettier
volta install @nestjs/cli @angular/cli create-react-app

# List installed globals
volta list

# Use globally installed tools
tsc --version
eslint --version
create-react-app --version
```

## Agent Use
- Consistent Node.js version management across environments
- Project-specific tool version enforcement
- CI/CD pipeline tool installation and management
- Development environment standardization
- Automated dependency version control
- Multi-project Node.js version switching

## Comparison with Other Tools

| Feature | Volta | nvm | asdf |
|---------|-------|-----|------|
| Speed | ⚡ Fast | Medium | Medium |
| Auto-switching | ✅ Yes | ❌ Manual | ✅ Yes |
| Windows | ✅ Native | ❌ No | ❌ WSL only |
| Package mgr | ✅ Yes | ❌ No | ✅ Yes |
| Binary cache | ✅ Yes | ❌ No | ❌ No |
| Team sync | ✅ package.json | ❌ .nvmrc | ✅ .tool-versions |

## Troubleshooting

### Volta not in PATH
```bash
# Add to shell profile (.bashrc, .zshrc, etc.)
export VOLTA_HOME="$HOME/.volta"
export PATH="$VOLTA_HOME/bin:$PATH"

# Reload shell
source ~/.bashrc  # or ~/.zshrc
```

### Cannot install Node version
```bash
# Check internet connection
curl -I https://nodejs.org

# Clear cache and retry
rm -rf ~/.volta/cache
volta install node@18.17.0
```

### Tool not switching automatically
```bash
# Verify package.json has volta section
cat package.json | grep -A 3 '"volta"'

# Re-pin version
volta pin node@18.17.0

# Check current version
volta list
node --version
```

### Permission errors
```bash
# Fix permissions
sudo chown -R $(whoami) ~/.volta

# Avoid using sudo with volta commands
volta install node  # ✅ Correct
sudo volta install node  # ❌ Wrong
```

## Migration from nvm

```bash
# Find current Node version
nvm current

# Install same version with Volta
volta install node@$(node --version | cut -d'v' -f2)

# Pin for current project
volta pin node

# Remove nvm from shell profile
# Edit ~/.bashrc or ~/.zshrc and remove:
# export NVM_DIR="$HOME/.nvm"
# [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

# Reload shell
source ~/.bashrc  # or ~/.zshrc
```

## Uninstall
```yaml
- preset: volta
  with:
    state: absent
```

Manual cleanup:
```bash
# Remove installation
rm -rf ~/.volta

# Remove from PATH (edit shell profile)
# Remove: export VOLTA_HOME="$HOME/.volta"
# Remove: export PATH="$VOLTA_HOME/bin:$PATH"

# Reload shell
source ~/.bashrc  # or ~/.zshrc
```

## Resources
- Official docs: https://docs.volta.sh/
- GitHub: https://github.com/volta-cli/volta
- Search: "volta javascript tool manager", "volta vs nvm"

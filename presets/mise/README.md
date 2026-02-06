# mise - Development Environment Manager

Fast, flexible polyglot development environment and version manager. Manage Node.js, Python, Ruby, Go, Rust, and 100+ other tools with a single configuration file.

## Quick Start

```yaml
- preset: mise
```

## Features

- **Polyglot**: Support for 100+ programming languages and tools
- **Fast**: Compiled in Rust, instant tool switching with zero startup overhead
- **Compatible**: Drop-in replacement for nvm, rbenv, pyenv, goenv, and others
- **Single Config**: `.mise.toml` replaces multiple version manager configs
- **Shell Integration**: Automatic environment activation via shell hooks
- **Plugin System**: Extensible with community plugins for custom tools

## Basic Usage

```bash
# Check mise version
mise --version

# Show help
mise --help

# List installed tools
mise ls

# List available versions for a tool
mise ls-remote node

# Install a tool version
mise use node@20.11.0

# Set local project version
mise use --local python@3.11

# Install from .mise.toml config
mise install

# Activate in current shell
mise activate
```

## Advanced Configuration

```yaml
- preset: mise
  with:
    state: present          # Install or remove
```

Create `.mise.toml` in your project:

```toml
[tools]
node = "20.11.0"           # Latest 20.x
python = "3.11"            # Latest 3.11
ruby = "3.2"
go = "1.21"
rust = "stable"

[env]
_.node.corepack = true     # Enable npm/yarn/pnpm auto-detection
```

Or use `.node-version`, `.python-version`, `.ruby-version` files (compatible with other managers).

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Configuration

- **Config file**: `.mise.toml` in project root or `~/.config/mise/config.toml` (global)
- **Version files**: `.node-version`, `.python-version`, `.ruby-version` (auto-detected)
- **Data directory**: `~/.local/share/mise/` (Linux), `~/Library/Application Support/mise/` (macOS)
- **Cache directory**: `~/.cache/mise/` (Linux), `~/Library/Caches/mise/` (macOS)
- **Shims directory**: `~/.local/share/mise/shims/` (added to PATH)

## Platform Support

- ✅ Linux (apt via install script, dnf, brew)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Real-World Examples

### Multi-Language Project Setup

Share development environment across a team:

```toml
# .mise.toml in project root
[tools]
node = "20.11.0"
python = "3.11.7"
postgresql = "15"
redis = "7"

[env]
NODE_ENV = "development"
PYTHONUNBUFFERED = "1"
```

Team members activate with single command:

```bash
mise install
mise activate
node --version   # 20.11.0
python --version # 3.11.7
```

### Version Switching Across Projects

Work with different versions simultaneously:

```bash
# Project A requires Python 3.10
cd project-a
mise use python@3.10
python --version  # 3.10.x

# Project B requires Python 3.12
cd project-b
mise use python@3.12
python --version  # 3.12.x

# Automatic switching based on .mise.toml or .python-version
cd project-a
python --version  # Back to 3.10.x
```

### CI/CD Environment Setup

Consistent tooling in continuous integration:

```bash
# GitHub Actions
- name: Setup development environment
  uses: jdx/setup-mise@v1

# Or with Mooncake
- preset: mise
- shell: |
    mise install
    node --version
    python --version
```

### Global Tool Installation

Install tools once, use everywhere:

```bash
# Install to global config
mise use -g bun@1.0

# Use in any project
bun install
bun run build
```

## Agent Use

- **Environment reproducibility**: Ensure consistent tool versions across CI/CD and development
- **Multi-language projects**: Manage Python, Node.js, Ruby dependencies in single config
- **Tool installation**: Automate polyglot development stack setup
- **Version testing**: Quickly switch between versions for compatibility testing
- **Environment validation**: Verify correct tool versions before deployment

## Troubleshooting

### Tools not in PATH

Ensure shell integration is activated:

```bash
# Add to ~/.bashrc or ~/.zshrc
eval "$(mise activate bash)"  # or zsh

# Verify shims directory is in PATH
echo $PATH | grep mise
```

### Version not found

Check available versions:

```bash
# List remote versions
mise ls-remote node

# Install from repository
mise install node@latest
```

### Conflicting version managers

Remove competing managers:

```bash
# If using nvm alongside mise
rm -rf ~/.nvm
# Update shell config to remove nvm initialization

# If using pyenv
brew uninstall pyenv
```

### Caching issues

Clear mise cache:

```bash
# Clear download cache
rm -rf ~/.cache/mise

# Reinstall tools
mise install
```

## Uninstall

```yaml
- preset: mise
  with:
    state: absent
```

**Note:** Data directory containing installed tool versions is preserved. Remove `~/.local/share/mise/` or `~/Library/Application Support/mise/` manually if desired.

## Resources

- Official docs: https://mise.jdx.dev/
- GitHub: https://github.com/jdx/mise
- Plugin registry: https://mise.jdx.dev/plugins.html
- Search: "mise version manager", "mise .mise.toml", "mise migration from nvm/rbenv"

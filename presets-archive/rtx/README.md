# rtx - Polyglot Runtime Manager

rtx (now called mise) is a fast, polyglot tool version manager compatible with asdf plugins.

## Quick Start

```yaml
- preset: rtx
```

## Features

- **Polyglot**: Manage Node.js, Python, Ruby, Go, and 400+ tools
- **Fast**: Written in Rust, 20-200x faster than asdf
- **Compatible**: Works with existing .tool-versions files
- **Automatic activation**: Switches versions based on directory
- **Plugin ecosystem**: Compatible with asdf plugins
- **Simple**: Single binary, no dependencies

## Basic Usage

```bash
# Install a tool
rtx install node@20
rtx install python@3.11

# Set global version
rtx global node@20
rtx global python@3.11

# Set local version (creates .tool-versions)
rtx local node@18
rtx local python@3.10

# List installed versions
rtx list

# List available versions
rtx ls-remote node
rtx ls-remote python

# Use specific version
rtx use node@20.11.0
```

## Advanced Configuration

```yaml
# Simple installation
- preset: rtx

# Remove installation
- preset: rtx
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Config file**: `~/.config/rtx/config.toml`
- **Tool versions**: `.tool-versions` (per-project)
- **Data directory**: `~/.local/share/rtx/`
- **Shell integration**: Add to `~/.bashrc` or `~/.zshrc`:
  ```bash
  eval "$(rtx activate bash)"  # or zsh, fish
  ```

## Real-World Examples

### Multi-Language Project

```bash
# Navigate to project
cd myproject

# Set tool versions
rtx use node@20.11.0
rtx use python@3.11.7
rtx use ruby@3.3.0

# Creates .tool-versions file
cat .tool-versions
# node 20.11.0
# python 3.11.7
# ruby 3.3.0

# Versions automatically activate when entering directory
```

### CI/CD Pipeline

```yaml
# Install rtx and tools
- preset: rtx

- name: Install project tools
  shell: rtx install
  # Reads .tool-versions and installs all tools

- name: Run tests with correct versions
  shell: |
    eval "$(rtx activate bash)"
    npm test
    python -m pytest
```

### Global Tool Setup

```bash
# Install commonly used tools
rtx install node@lts
rtx install python@3.11
rtx install terraform@latest
rtx install kubectl@1.29

# Set as global defaults
rtx global node@lts python@3.11 terraform@latest kubectl@1.29

# Verify
node --version
python --version
terraform --version
kubectl version --client
```

### Per-Project Node Versions

```bash
# Project A (older Next.js)
cd ~/projects/legacy-app
rtx local node@16.20.2
npm install
npm run dev

# Project B (latest React)
cd ~/projects/new-app
rtx local node@20.11.0
npm install
npm run dev

# Each project uses its specified version automatically
```

### Managing Python Environments

```bash
# Install multiple Python versions
rtx install python@3.11.7
rtx install python@3.10.13
rtx install python@3.9.18

# Use for different projects
cd project-a && rtx local python@3.11.7
cd project-b && rtx local python@3.10.13

# Works with venv
python -m venv .venv
source .venv/bin/activate
```

## Agent Use

- Manage runtime versions across development environments
- Ensure consistent tool versions in CI/CD pipelines
- Switch between project-specific tool versions automatically
- Test applications against multiple language versions
- Simplify onboarding with `.tool-versions` files

## Troubleshooting

### rtx: command not found

```bash
# Add to shell configuration
echo 'eval "$(rtx activate bash)"' >> ~/.bashrc
source ~/.bashrc

# For zsh
echo 'eval "$(rtx activate zsh)"' >> ~/.zshrc
source ~/.zshrc
```

### Tool not activating

```bash
# Check if rtx is activated
rtx doctor

# Verify .tool-versions file
cat .tool-versions

# Check current versions
rtx current

# Reinstall tool
rtx install --force node@20
```

### Plugin not found

```bash
# Update plugin list
rtx plugins update

# Install plugin manually
rtx plugin add nodejs https://github.com/rtx-plugins/rtx-nodejs

# List installed plugins
rtx plugins list
```

### Slow shell startup

```bash
# Use lazy loading
# In ~/.bashrc or ~/.zshrc:
eval "$(rtx activate bash --lazy)"

# Or use direnv integration
eval "$(rtx activate bash --direnv)"
```

## Uninstall

```yaml
- preset: rtx
  with:
    state: absent
```

**Note**: This removes rtx but preserves installed tools. To remove everything:

```bash
rm -rf ~/.local/share/rtx
rm -rf ~/.config/rtx
```

## Resources

- Official docs: https://mise.jdx.dev/ (rtx was renamed to mise)
- GitHub: https://github.com/jdx/mise
- Plugin list: https://mise.jdx.dev/plugins.html
- Search: "mise tool versions", "rtx vs asdf", "mise runtime manager"

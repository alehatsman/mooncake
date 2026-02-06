# navi - Interactive Cheatsheet Tool for Command-Line

Navi is an interactive cheatsheet tool that allows you to browse, search, and execute commands from your terminal without leaving the command line. Perfect for remembering complex command syntax or discovering new command patterns.

## Quick Start

```yaml
- preset: navi
```

## Features

- **Interactive Search**: Fast fuzzy-search through thousands of cheatsheets
- **Built-in Cheatsheets**: Extensive library of common commands and tools
- **Custom Cheatsheets**: Create personal cheatsheets for your own commands
- **Variable Substitution**: Interactive prompts for command parameters
- **Copy-Paste Ready**: Get commands ready to execute with one keystroke
- **Community Driven**: Access crowdsourced command collections
- **Cross-platform**: Works on Linux, macOS, and Windows

## Basic Usage

```bash
# Check version and verify installation
navi --version

# Start interactive mode - browse all cheatsheets
navi

# Search for a specific command
navi --query "git"

# Browse a specific category
navi --cheatsheet git

# Display help
navi --help

# List available cheatsheets
navi --list

# Show your custom cheatsheets directory
navi --edit

# Execute a command directly
navi --preview
```

## Advanced Configuration

```yaml
# Basic installation
- preset: navi

# Prepare for uninstallation
- preset: navi
  with:
    state: absent
```

## Parameters

| Parameter | Type   | Default | Description            |
|-----------|--------|---------|------------------------|
| state     | string | present | Install or remove tool |

## Platform Support

- ✅ macOS (Homebrew)
- ✅ Linux (GitHub releases and package managers)
- ✅ Windows (via manual installation)

## Configuration

- **Cheatsheets directory**: `~/.config/navi/` (Linux), `~/Library/Application Support/navi/` (macOS)
- **Custom cheatsheets**: Place your `.cheat` files in the cheatsheets directory
- **Config file**: `~/.config/navi/config.yml` (optional)
- **Editor**: Uses `$EDITOR` environment variable for editing
- **Fuzzy finder**: Uses `fzf` for interactive search (installed separately if not present)

## Real-World Examples

### DevOps Engineer Workflow

```bash
# Quick access to Docker commands
navi --query "docker"

# Find Kubernetes kubectl commands
navi --query "kubectl"

# Get systemctl commands for service management
navi --query "systemctl"
```

### Development Workflow

```bash
# Find git commands for branching and merging
navi --query "git merge"

# Remember complex npm commands
navi --query "npm"

# Get JavaScript/Node.js patterns
navi --cheatsheet javascript
```

### Creating Custom Cheatsheets

```bash
# Edit personal cheatsheets
navi --edit

# Create a file: ~/.config/navi/cheats/my-custom.cheat
# Format:
# % my-tools
# # My custom commands
#
# ; List files with details
# ls -lah
#
# ; Search for process
# ps aux | grep <process_name>
#
# ; Kill process by name
# pkill -f <process_name>
```

### Terminal Productivity

```bash
# Use with fzf for faster search
navi | fzf --preview

# Integrate with shell aliases
alias ch='navi'
alias chgit='navi --query "git"'
alias chdocker='navi --query "docker"'

# Use in scripts to find command examples
./deploy.sh && navi --query "deployment verification"
```

## Agent Use

- Automated command discovery for AI agents learning new tools
- Infrastructure-as-code documentation through cheatsheets
- Training systems with command examples and patterns
- Debugging assistance by searching cheatsheets for common issues
- Validation of command syntax before execution
- Command pattern matching for intelligent automation

## Troubleshooting

### Command Not Found

**Problem**: `navi: command not found`

**Solution**: Verify installation:
```bash
command -v navi
which navi

# If not found, reinstall:
brew install navi  # macOS
# For Linux, download from: https://github.com/denisidoro/navi/releases
```

### Cheatsheets Won't Load

**Problem**: No cheatsheets found or error loading

**Solution**: Check cheatsheets directory:
```bash
# Verify directory exists
ls -la ~/.config/navi/

# Initialize cheatsheets if missing
mkdir -p ~/.config/navi/cheats
cd ~/.config/navi/cheats

# Clone community cheatsheets
git clone https://github.com/denisidoro/navi-cheats.git
```

### Search Not Working / Fuzzy Finder Issues

**Problem**: Interactive search fails or returns no results

**Solution**: Ensure fzf is installed:
```bash
# Check if fzf is available
which fzf

# Install fzf if missing
brew install fzf  # macOS
sudo apt-get install fzf  # Ubuntu/Debian

# Try navi again
navi
```

### Permission Issues on Linux

**Problem**: `permission denied` when trying to run downloaded binary

**Solution**: Make the binary executable:
```bash
# Find navi location
which navi

# Add execute permission
chmod +x ~/.local/bin/navi
# or wherever you installed it

# Verify it works
navi --version
```

### Custom Cheatsheets Not Appearing

**Problem**: Created cheatsheet files but they don't show up in navi

**Solution**: Verify file format and location:
```bash
# Check cheatsheets are in correct directory
ls -la ~/.config/navi/cheats/

# Verify syntax - each line must start with '%' for category or ';' for command
# Incorrect:
echo "my command" > ~/.config/navi/cheats/test.cheat

# Correct:
cat > ~/.config/navi/cheats/test.cheat << 'EOF'
% my-category
; My command
echo "Hello"
EOF

# Restart navi to reload
navi
```

## Uninstall

```yaml
- preset: navi
  with:
    state: absent
```

## Resources

- Official docs: https://github.com/denisidoro/navi
- Community cheatsheets: https://github.com/denisidoro/navi-cheats
- Getting started: https://github.com/denisidoro/navi#install
- Cheatsheet format: https://github.com/denisidoro/navi#cheatsheet-syntax
- Search: "navi tutorial", "navi custom cheatsheet", "navi integration shell"

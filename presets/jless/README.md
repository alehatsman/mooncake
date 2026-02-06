# jless - JSON Viewer

Terminal JSON viewer with vim-like keybindings. Collapsible structure, search, copy, and YAML support.

## Quick Start
```yaml
- preset: jless
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage
```bash
# View JSON file
jless data.json

# From stdin
cat data.json | jless
curl -s https://api.github.com/users/octocat | jless

# View YAML
jless data.yaml

# Multiple files
jless file1.json file2.json
```


## Advanced Configuration
```yaml
- preset: jless
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove jless |
## Navigation
```
Movement:
  j/↓       - Move down
  k/↑       - Move up
  h/←       - Collapse current level
  l/→       - Expand current level
  g         - Jump to top
  G         - Jump to bottom
  Ctrl+d    - Page down
  Ctrl+u    - Page up
  Ctrl+f    - Page down
  Ctrl+b    - Page up

Node Navigation:
  J         - Next sibling
  K         - Previous sibling
  Ctrl+n    - Next node (depth-first)
  Ctrl+p    - Previous node
```

## Collapsing/Expanding
```
Collapse/Expand:
  Space     - Toggle current node
  c         - Collapse node
  e         - Expand node
  C         - Collapse all
  E         - Expand all

Depth Control:
  0-9       - Collapse to depth N
  1         - Show only top level
  2         - Expand 2 levels deep
  3         - Expand 3 levels deep
```

## Search
```
Search Commands:
  /         - Search forward
  ?         - Search backward
  n         - Next match
  N         - Previous match
  *         - Search for word under cursor
  #         - Search backward for word under cursor

Search Modes:
  /pattern  - Case-sensitive search
  /\cpattern - Case-insensitive search
  /"email"  - Search in quoted strings
```

## Copy Operations
```
Copy:
  y         - Copy current value
  Y         - Copy current path
  Ctrl+c    - Copy to clipboard (if supported)

Path Display:
  .         - Show current path
  p         - Show path in status bar
```

## Data Modes
```
View Modes:
  m         - Toggle data mode (auto/line/data)

Data Mode Options:
  - auto: Auto-detect structure
  - line: Show line numbers
  - data: Pure data view
```

## Filtering
```bash
# Search within jless
# Press / then type pattern

# Examples:
/email       # Find "email" anywhere
/user.*id    # Regex pattern
/\cERROR     # Case-insensitive

# Navigate results with n/N
```

## Configuration
```bash
# Config file: ~/.config/jless/config.yaml
style:
  line-numbers: true
  theme: monokai

keybindings:
  quit: 'q'
  search: '/'

# Or via environment
export JLESS_THEME=monokai
```

## Themes
```bash
# Available themes
jless --theme monokai data.json
jless --theme dracula data.json
jless --theme gruvbox data.json

# List themes
jless --help | grep -A 20 'THEMES'
```

## Command Line Options
```bash
# Start collapsed
jless --collapse data.json

# Line numbers
jless --line-numbers data.json

# No colors
jless --no-color data.json

# Expand to depth
jless --expand-depth 2 data.json

# Read from stdin
echo '{"key":"value"}' | jless
```

## API Response Viewing
```bash
# GitHub API
curl -s https://api.github.com/repos/owner/repo | jless

# Pretty print and explore
curl -s https://api.example.com/users | jless

# With authentication
curl -s -H "Authorization: Bearer $TOKEN" \
  https://api.example.com/data | jless

# Save and view
curl -s https://api.example.com/data > response.json
jless response.json
```

## Large Files
```bash
# Streaming support
cat large-file.jsonl | jless

# Start collapsed for better performance
jless --collapse huge-data.json

# Expand only what you need
# Open file, press 1 to collapse to depth 1
# Navigate to section of interest, press e to expand
```

## Comparison with Less
```bash
# Regular less
cat data.json | less

# jless (structured navigation)
cat data.json | jless

# Benefits over less:
# - Collapsible structure
# - Path display
# - Type-aware display
# - Easy navigation
# - Copy values
```

## Keyboard Shortcuts Summary
```
Essential:
  j/k       - Up/down
  h/l       - Collapse/expand
  Space     - Toggle
  /         - Search
  n/N       - Next/prev search
  y         - Copy value
  Y         - Copy path
  q         - Quit

Advanced:
  J/K       - Next/prev sibling
  1-9       - Collapse to depth
  C/E       - Collapse/expand all
  .         - Show current path
  m         - Toggle mode
```

## Real-World Examples
```bash
# Explore package.json
jless package.json

# View API response
curl -s https://api.github.com/users/octocat | jless

# Kubernetes resource
kubectl get pods -o json | jless

# Docker inspect
docker inspect container_name | jless

# Terraform state
terraform show -json | jless

# AWS CLI output
aws ec2 describe-instances | jless

# NPM package info
npm view express --json | jless

# Log files (JSON format)
tail -f app.log | grep '{' | jless
```

## YAML Support
```bash
# View YAML files
jless config.yaml

# Convert and view
yq eval -o json config.yaml | jless

# Kubernetes YAML
kubectl get deployment myapp -o yaml | jless
```

## CI/CD Integration
```bash
# Inspect test results
jless test-results.json

# Review build artifacts
jless build-manifest.json

# Check deployment config
jless deployment-config.json

# Verify environment variables
env | jq -R 'split("=") | {(.[0]): .[1]}' | jq -s 'add' | jless
```

## Tips for Large JSON
```bash
# Start collapsed
jless --collapse data.json

# Collapse to depth 1, then navigate
jless data.json
# Press 1 (collapse all to depth 1)
# Navigate to section with j/k
# Press e to expand section

# Use search to jump
jless data.json
# Press / and type field name
# Press n to find next occurrence
```

## Comparison
| Feature | jless | less | fx | jq |
|---------|-------|------|-----|-----|
| Structured view | Yes | No | Yes | No |
| Vim keys | Yes | Partial | No | N/A |
| Collapsible | Yes | No | Yes | No |
| Search | Yes | Yes | Yes | No |
| Copy path | Yes | No | No | No |

## Best Practices
- **Start collapsed** for large files (`--collapse`)
- **Use depth** to control detail (press 1-9)
- **Search to navigate** large structures (`/`)
- **Copy paths** for documentation (`Y`)
- **Use with curl** for API exploration
- **Configure theme** for better readability
- **Leverage vim keys** if familiar with vim

## Tips
- Much faster than `less` for JSON
- Vim-like navigation is intuitive
- Great for exploring unknown structures
- Copy path feature saves time
- Works with YAML too
- Collapsing improves performance
- Perfect for log file exploration

## Agent Use
- JSON response inspection
- Configuration file review
- API debugging
- Log file exploration (JSON logs)
- Kubernetes resource viewing
- Terraform state inspection

## Uninstall
```yaml
- preset: jless
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/PaulJuliusMartinez/jless
- Search: "jless json viewer", "jless vim"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

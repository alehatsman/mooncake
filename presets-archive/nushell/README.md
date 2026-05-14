# Nushell - Modern Shell with Structured Data

Modern shell with structured data pipelines, written in Rust for speed and reliability.

## Quick Start

```yaml
- preset: nushell
```

## Features

- **Structured data**: Everything is a table, not text
- **Type system**: Strong typing for data manipulation
- **Plugins**: Extend with custom commands
- **Cross-platform**: Linux, macOS, Windows
- **Fast**: Written in Rust for performance
- **Interactive**: Syntax highlighting and completions
- **Composable**: Unix-style pipelines with structured data

## Basic Usage

```bash
# Start Nushell
nu

# List files as table
ls | where size > 1MB

# Sort and filter
ls | sort-by modified | reverse | first 10

# Work with JSON
fetch https://api.example.com/data | get items | where status == "active"

# Convert between formats
open data.json | to yaml

# Create custom commands
def greet [name] {
  $"Hello, ($name)!"
}

# Environment variables
$env.PATH | split row ":"
```

## Advanced Configuration

```yaml
# Install Nushell
- preset: nushell

# Create configuration
- name: Deploy Nushell config
  template:
    src_template: config.nu.j2
    dest: ~/.config/nushell/config.nu

# Set as default shell (optional)
- name: Set Nushell as default shell
  shell: chsh -s $(which nu)
  when: set_default_shell
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Nushell |

## Platform Support

- ✅ Linux (apt, dnf, pacman, Homebrew, binary)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (winget, chocolatey, binary)

## Configuration

- **Config file**: `~/.config/nushell/config.nu`
- **Environment**: `~/.config/nushell/env.nu`
- **Custom commands**: Add to config.nu or separate files

## Real-World Examples

### Data Processing
```nushell
# Parse CSV and aggregate
open sales.csv
  | where region == "US"
  | group-by product
  | each { |it| { product: $it.0, total: ($it.1 | get amount | math sum) }}
  | sort-by total
  | reverse

# Convert JSON to table
open api-response.json
  | get results
  | select name email created_at
  | to md  # Markdown table
```

### System Administration
```nushell
# Find large files
ls **/* | where size > 100MB | sort-by size | reverse

# Monitor processes
ps | where cpu > 50 | select name pid cpu mem

# Network connections
netstat | where state == "ESTABLISHED" | group-by remote_address | length
```

### Automation Scripts
```nushell
# Deploy script
def deploy [env: string] {
  print $"Deploying to ($env)..."

  # Build
  cargo build --release

  # Upload
  scp target/release/myapp $"deploy@($env).example.com:/opt/myapp/"

  # Restart service
  ssh $"deploy@($env).example.com" "systemctl restart myapp"

  print "Deployment complete!"
}

# Usage: deploy production
```

### CI/CD Integration
```yaml
# Use Nushell for pipeline scripts
- name: Install Nushell
  preset: nushell

- name: Run deployment script
  shell: nu deploy.nu --env production --version {{ version }}
  args:
    executable: /usr/bin/nu
```

## Custom Commands

```nushell
# ~/.config/nushell/config.nu

# Git shortcuts
def gs [] { git status }
def gco [branch] { git checkout $branch }
def gp [] { git push origin (git branch --show-current) }

# Docker helpers
def dps [] { docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" }
def dlogs [container] { docker logs -f $container }

# System info
def sysinfo [] {
  {
    os: (sys | get host.name),
    kernel: (sys | get host.kernel_version),
    memory: (sys | get mem.total),
    cpu: (sys | get cpu | first | get name)
  }
}
```

## Structured Data Pipelines

```nushell
# Input → Transform → Output
ls
  | where type == file
  | where size > 1MB
  | sort-by modified
  | reverse
  | first 10
  | select name size modified
  | to json

# HTTP API processing
fetch https://api.github.com/repos/nushell/nushell
  | get [stargazers_count, forks_count, open_issues_count]
  | transpose key value

# File format conversion
open data.json
  | to yaml
  | save data.yaml
```

## Agent Use

- Process structured data (JSON, CSV, YAML) in automation scripts
- Build type-safe deployment and orchestration scripts
- Parse and transform API responses
- Create interactive system administration tools
- Analyze logs and metrics with structured queries
- Generate reports from multiple data sources

## Troubleshooting

### Config not loading
```bash
# Check config file location
config nu

# Verify syntax
nu --config ~/.config/nushell/config.nu --commands "exit"
```

### Command not found
```bash
# Add to PATH in env.nu
$env.PATH = ($env.PATH | split row ":" | append "/custom/path")

# Reload config
source ~/.config/nushell/config.nu
```

## Uninstall

```yaml
- preset: nushell
  with:
    state: absent
```

## Resources

- Official docs: https://www.nushell.sh/book/
- Cookbook: https://www.nushell.sh/cookbook/
- GitHub: https://github.com/nushell/nushell
- Search: "nushell tutorial", "nushell examples", "nushell commands"

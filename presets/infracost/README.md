# infracost - Cloud Cost Estimates for IaC

Infracost shows cloud cost estimates for Terraform, allowing you to see cost breakdowns and compare infrastructure changes before deployment.

## Quick Start

```yaml
- preset: infracost
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage

```bash
# Basic usage
infracost --help

# Common operations
infracost --version
```


## Advanced Configuration
```yaml
- preset: infracost
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove infracost |
## Agent Use

Automation-friendly CLI tool with:
- Exit codes for error handling  
- JSON/YAML output support (where applicable)
- Scriptable interface
- Idempotent operations

## Uninstall
```yaml
- preset: infracost
  with:
    state: absent
```

## Resources

Search: "infracost documentation" or "infracost github"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

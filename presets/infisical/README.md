# infisical - Secrets Management Platform

Infisical is an open-source secrets management platform for managing application secrets, environment variables, and credentials across teams and infrastructure.

## Quick Start

```yaml
- preset: infisical
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage

```bash
# Basic usage
infisical --help

# Common operations
infisical --version
```


## Advanced Configuration
```yaml
- preset: infisical
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove infisical |
## Agent Use

Automation-friendly CLI tool with:
- Exit codes for error handling  
- JSON/YAML output support (where applicable)
- Scriptable interface
- Idempotent operations

## Uninstall
```yaml
- preset: infisical
  with:
    state: absent
```

## Resources

Search: "infisical documentation" or "infisical github"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

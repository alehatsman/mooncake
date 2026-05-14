# influxdb3

Time-series / analytics database

## Quick Start

```yaml
- preset: influxdb3
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage

```bash
# Basic usage
influxdb3 --help

# Common operations
influxdb3 --version
```


## Advanced Configuration
```yaml
- preset: influxdb3
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove influxdb3 |
## Agent Use

Automation-friendly CLI tool with:
- Exit codes for error handling  
- JSON/YAML output support (where applicable)
- Scriptable interface
- Idempotent operations

## Uninstall
```yaml
- preset: influxdb3
  with:
    state: absent
```

## Resources

Search: "influxdb3 documentation" or "influxdb3 github"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

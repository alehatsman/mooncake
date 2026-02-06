# iperf3

Network bandwidth measurement

## Quick Start

```yaml
- preset: iperf3
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage

```bash
# Basic usage
iperf3 --help

# Common operations
iperf3 --version
```


## Advanced Configuration
```yaml
- preset: iperf3
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove iperf3 |
## Agent Use

Automation-friendly CLI tool with:
- Exit codes for error handling  
- JSON/YAML output support (where applicable)
- Scriptable interface
- Idempotent operations

## Uninstall
```yaml
- preset: iperf3
  with:
    state: absent
```

## Resources

Search: "iperf3 documentation" or "iperf3 github"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

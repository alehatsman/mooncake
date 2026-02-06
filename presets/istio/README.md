# istio - Service Mesh Control Plane

Istio is an open-source service mesh that provides traffic management, security, and observability for microservices running on Kubernetes.

## Quick Start

```yaml
- preset: istio
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage

```bash
# Basic usage
istio --help

# Common operations
istio --version
```


## Advanced Configuration
```yaml
- preset: istio
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove istio |
## Agent Use

Automation-friendly CLI tool with:
- Exit codes for error handling  
- JSON/YAML output support (where applicable)
- Scriptable interface
- Idempotent operations

## Uninstall
```yaml
- preset: istio
  with:
    state: absent
```

## Resources

Search: "istio documentation" or "istio github"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

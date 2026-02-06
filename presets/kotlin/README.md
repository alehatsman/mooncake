# kotlin - Modern JVM Language

Kotlin is a modern, statically-typed programming language for the JVM that combines object-oriented and functional programming features.

## Quick Start

```yaml
- preset: kotlin
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage

```bash
# Basic usage
kotlin --help

# Common operations
kotlin --version
```


## Advanced Configuration
```yaml
- preset: kotlin
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove kotlin |
## Agent Use

Automation-friendly CLI tool with:
- Exit codes for error handling  
- JSON/YAML output support (where applicable)
- Scriptable interface
- Idempotent operations

## Uninstall
```yaml
- preset: kotlin
  with:
    state: absent
```

## Resources

Search: "kotlin documentation" or "kotlin github"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

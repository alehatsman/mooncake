# Ollama Preset Examples

This directory contains examples demonstrating the Ollama preset for managing Ollama installations, service configuration, and LLM model management.

## Quick Start

**New to Ollama?** Start here:

### `ollama-quick-start.yml`
Simple 5-minute demo that installs Ollama and runs test queries:
```bash
mooncake run -c examples/ollama/ollama-quick-start.yml --ask-become-pass
```
- Installs Ollama
- Pulls tinyllama (smallest model, ~637MB)
- Starts server
- Runs test queries (math, geography)

## Examples

### `ollama-example.yml` (Comprehensive)
Complete example demonstrating all Ollama preset capabilities:
- Installation variations (basic, with service, via specific method)
- Model management (single, multiple, force re-pull)
- Service configuration (custom host, models directory, environment variables)
- Complete deployment workflow
- Uninstallation scenarios
- Platform-specific examples (Linux/macOS)
- Integration with other actions

```bash
# Dry-run mode (shows what would happen)
mooncake run -c examples/ollama/ollama-example.yml --dry-run

# Actual execution (requires sudo)
mooncake run -c examples/ollama/ollama-example.yml --ask-become-pass
```

### `ollama-quick-start.yml` (Beginner-Friendly)
Fast introduction to Ollama preset with minimal configuration:
- Quick installation
- Single model download
- Simple test queries
- Good for first-time users

## Basic Usage

### 1. Basic Installation
```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
  become: true
```

### 2. Install with Service
```yaml
- name: Install Ollama with service
  preset: ollama
  with:
    state: present
    service: true
  become: true
```

### 3. Install and Pull Models
```yaml
- name: Install Ollama and pull models
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - "llama3.1:8b"
      - "mistral:latest"
  become: true
```

### 4. Complete Configuration
```yaml
- name: Full Ollama deployment
  preset: ollama
  with:
    state: present
    service: true
    method: auto
    host: "0.0.0.0:11434"
    models_dir: "/data/ollama"
    pull:
      - "llama3.1:8b"
    env:
      OLLAMA_DEBUG: "1"
  become: true
```

## Features Demonstrated

- Installation management (auto, script, package methods)
- Service configuration (systemd on Linux, launchd on macOS)
- Model pulling (single, multiple, with force flag)
- Custom configuration (host, models directory, environment variables)
- Uninstallation (with optional model removal)
- Facts integration (automatic detection)
- Idempotency (won't reinstall if present)
- Platform support (Linux, macOS)

## Supported Platforms

- **Linux** (Ubuntu, Debian, Fedora, Arch, etc.)
  - systemd service management
  - Package managers: apt, dnf, yum, pacman, zypper, apk

- **macOS**
  - launchd service management
  - Homebrew integration

## Tips

1. **Start with dry-run**: Use `--dry-run` to see what will happen
2. **Use facts**: Check `{{ ollama_version }}` before installation
3. **Idempotency**: The preset won't reinstall if Ollama is already present
4. **Model size**: Consider starting with tinyllama (~637MB) for testing
5. **Service management**: Use `service: true` for production deployments
6. **Sudo required**: Most operations need `become: true` or `--ask-become-pass`

## Documentation

For complete documentation, see:
- [Preset Reference](../../docs/guide/presets.md) - Full preset documentation
- [Configuration Reference](../../docs/guide/config/reference.md) - Property tables
- [Core Concepts](../../docs/guide/core-concepts.md) - Overview

For questions or issues, see the main [Mooncake documentation](../../docs/).

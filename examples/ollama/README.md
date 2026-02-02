# Ollama Preset Examples

This directory contains comprehensive examples demonstrating the Ollama preset for managing Ollama installations, service configuration, and LLM model management.

## 🚀 Quick Start

**New to Ollama?** Start here:

#### `ollama-quick-start.yml`
Simple 5-minute demo that installs Ollama and runs test queries:
```bash
mooncake run -c examples/ollama/ollama-quick-start.yml
```
- Installs Ollama
- Pulls tinyllama (smallest model, ~637MB)
- Starts server
- Runs test queries (math, geography)

---

## 📚 Examples Overview

### Main Example

#### `ollama-example.yml` (Recommended)
Complete comprehensive example with 11 basic examples and 8 practical use cases:
- Installation variations (basic, with service, via specific method)
- Model management (single, multiple, force re-pull)
- Service configuration (custom host, models directory, environment variables)
- Complete deployment workflow
- Uninstallation scenarios
- Platform-specific examples (Linux/macOS)
- Integration with other actions

**Start here for a complete overview of all Ollama preset capabilities.**

---

### Demo Examples

#### `ollama-dry-run-demo.yml`
Demonstrates dry-run mode showing what the Ollama preset would do without executing:
- Shows installation plan
- Service configuration preview
- Model pulling preview
- Useful for understanding action behavior

#### `ollama-facts-demo.yml`
Shows system facts integration:
- Automatic Ollama detection
- Version information
- Installed models
- Conditional execution based on facts

#### `ollama-simple-facts.yml`
Simple demonstration of facts detection:
- Ollama version from facts
- Endpoint information
- Model listing
- API health check

---

### Docker/Ubuntu Examples

#### `ollama-ubuntu-working.yml`
Complete working example for Ubuntu/Docker:
- Official script installation
- Server startup
- Model pulling (tinyllama)
- LLM inference demonstration
- Full workflow from install to running queries

#### `ollama-ubuntu-final.yml`
Simplified Ubuntu demo with clear output:
- Installation verification
- Model management
- Multiple inference examples (math, geography, code)

#### `ollama-docker-demo.yml`
Original Docker demo (reference):
- Shows Ollama preset in Docker context
- Historical reference

#### `ollama-docker-simple.yml`
Simplified Docker installation approach:
- Manual installation steps
- Server management
- Model downloading

---

## 🚀 Quick Start

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

---

## 🔧 Running the Examples

### Run with Mooncake:
```bash
# Dry-run mode (shows what would happen)
mooncake run --config examples/ollama/ollama-example.yml --dry-run

# Actual execution (requires sudo)
mooncake run --config examples/ollama/ollama-example.yml --ask-become-pass
```

### Docker Ubuntu Demo:
```bash
# Build demo image
docker build -f Dockerfile.demo -t mooncake-ollama-demo .

# Run demo (installs Ollama + runs LLM inference)
docker run --rm mooncake-ollama-demo mooncake run --config /workspace/demo.yml
```

---

## 📖 Documentation

For complete documentation, see:
- [Ollama Action Reference](../../docs/guide/config/actions.md#ollama) - Full property reference
- [Configuration Reference](../../docs/guide/config/reference.md#ollama) - Property tables
- [Core Concepts](../../docs/guide/core-concepts.md) - Overview

---

## 🎯 Features Demonstrated

- ✅ Installation management (auto, script, package methods)
- ✅ Service configuration (systemd on Linux, launchd on macOS)
- ✅ Model pulling (single, multiple, with force flag)
- ✅ Custom configuration (host, models directory, environment variables)
- ✅ Uninstallation (with optional model removal)
- ✅ Facts integration (automatic detection)
- ✅ Idempotency (won't reinstall if present)
- ✅ Platform support (Linux, macOS, Docker)
- ✅ Real LLM inference (tested with tinyllama)

---

## 🧪 Tested Platforms

- ✅ **Linux** (Ubuntu 22.04 in Docker)
  - systemd service management
  - Package managers: apt, dnf, yum, pacman, zypper, apk

- ✅ **macOS**
  - launchd service management
  - Homebrew integration

- ✅ **Docker**
  - Ubuntu 22.04 base image
  - Full installation + LLM inference verified

---

## 💡 Tips

1. **Start with dry-run**: Use `--dry-run` to see what will happen
2. **Use facts**: Check `{{ ollama_version }}` before installation
3. **Idempotency**: The action won't reinstall if Ollama is already present
4. **Model size**: Consider starting with tinyllama (~637MB) for testing
5. **Service management**: Use `service: true` for production deployments
6. **Sudo required**: Most operations need `become: true` or `--ask-become-pass`

---

## 📝 Example Output

```
▶ Install Ollama with full configuration
Ollama already installed              ← Idempotency working!
Starting Ollama via Homebrew services ← Service management
Pulling model: llama3.1:8b            ← Model pulling
✓ Install Ollama with full configuration

Duration: 180518ms (3 minutes)
```

---

For questions or issues, see the main [Mooncake documentation](../../docs/).

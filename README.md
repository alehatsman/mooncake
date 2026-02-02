# Mooncake

[![CI](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml)
[![Security](https://github.com/alehatsman/mooncake/actions/workflows/security.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/security.yml)
[![codecov](https://codecov.io/gh/alehatsman/mooncake/branch/master/graph/badge.svg)](https://codecov.io/gh/alehatsman/mooncake)

Lightweight automation tool for dotfiles and system configuration. Single binary, no dependencies. **Chookity!**

```yaml
- name: Hello Mooncake
  shell: echo "Running on {{os}}/{{arch}}"

- name: Create file
  file:
    path: /tmp/hello.txt
    state: file
    content: "Hello from Mooncake!"
```

## Quick Start

```bash
# Install
go install github.com/alehatsman/mooncake@latest

# Create config.yml (example above)

# Preview changes (safe!)
mooncake run --config config.yml --dry-run

# Run it
mooncake run --config config.yml
```

## What You Can Do

| Action | Purpose | Example |
|--------|---------|---------|
| **shell** | Run commands | `shell: echo "hello"` |
| **file** | Create files/directories | `file: {path: /tmp/test, state: directory}` |
| **template** | Render configs | `template: {src: app.j2, dest: /etc/app.conf}` |
| **copy** | Copy with checksums | `copy: {src: ./file, dest: /tmp/file}` |
| **download** | Fetch from URLs | `download: {url: https://..., dest: /tmp/file}` |
| **service** | Manage services | `service: {name: nginx, state: started}` |
| **assert** | Verify state | `assert: {command: {cmd: docker --version}}` |
| **preset** | Reusable workflows | `preset: ollama` |

**Variables & Facts**: Auto-detected system info - `{{os}}`, `{{arch}}`, `{{cpu_cores}}`, `{{memory_total_mb}}`, `{{distribution}}`, `{{package_manager}}`

```bash
mooncake facts  # See all available facts
```

**Control Flow**: Conditionals (`when`), loops (`with_items`, `with_filetree`), tags, sudo

## Features

- 🚀 **Single Binary** - No dependencies, written in Go
- 🎨 **Animated TUI** - Real-time progress with beautiful output
- 🔍 **Dry-run Mode** - Preview all changes before applying
- 📝 **Simple YAML** - No complex DSL to learn
- 🌍 **Cross-Platform** - Linux, macOS, and Windows
- 🔧 **Powerful** - Variables, conditionals, loops, templates, system facts
- ⚡ **Robust** - Timeouts, retries, custom environments, failure handling

## Why Mooncake?

| Feature | Mooncake | Ansible | Shell Scripts |
|---------|----------|---------|---------------|
| **Setup** | Single binary | Python + modules | Text editor |
| **Dependencies** | None | Python, modules | System tools |
| **Learning Curve** | Minutes | Hours/Days | Varies |
| **Cross-platform** | ✅ Built-in | ⚠️ Limited | ❌ OS-specific |
| **Dry-run** | ✅ Native | ✅ Check mode | ❌ Manual |
| **Best For** | Personal configs, dotfiles | Enterprise automation | Quick tasks |

## Documentation

**📚 [Full Documentation](https://mooncake.alehatsman.com)** - Complete guide with examples

Quick links:
- 🚀 **[Quick Start](https://mooncake.alehatsman.com/getting-started/quick-start/)** - 30 second tutorial
- 📚 **[Examples](https://mooncake.alehatsman.com/examples/)** - Learn by doing (beginner → advanced)
- 📖 **[Actions Guide](https://mooncake.alehatsman.com/guide/config/actions/)** - What you can do
- 📋 **[Complete Reference](https://mooncake.alehatsman.com/guide/config/reference/)** - All properties
- 🎯 **[Presets](https://mooncake.alehatsman.com/guide/presets/)** - Reusable workflows

### Local Examples

Try the examples in the [`examples/`](examples/) directory:

```bash
# Clone and try
git clone https://github.com/alehatsman/mooncake.git
cd mooncake

# Run Hello World
mooncake run --config examples/01-hello-world/config.yml

# Browse all examples
ls examples/
```

## Common Use Cases

**Dotfiles Management**
```yaml
- name: Deploy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Development Environment Setup**
```yaml
- vars:
    packages: [neovim, ripgrep, fzf, tmux]

- name: Install dev tools
  shell: brew install {{item}}
  with_items: "{{packages}}"
  when: os == "darwin"
```

**Multi-OS Configuration**
```yaml
- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"
```

## Testing

Thoroughly tested across multiple platforms:
- **Linux**: Ubuntu, Debian, Alpine, Fedora
- **macOS**: Intel and Apple Silicon
- **Windows**: Windows Server

See [Testing Documentation](docs/testing/README.md) for details.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

- 🐛 [Report bugs](https://github.com/alehatsman/mooncake/issues)
- 💡 [Request features](https://github.com/alehatsman/mooncake/issues)
- 🗺️ [Roadmap](docs/development/roadmap.md)

## License

MIT License - Copyright (c) 2026 Aleh Atsman

See [LICENSE](LICENSE) file for details.

---

**[📚 Read the Full Documentation](https://mooncake.alehatsman.com)** for detailed guides, examples, and reference materials.

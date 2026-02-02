# Mooncake

[![CI](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml)
[![Security](https://github.com/alehatsman/mooncake/actions/workflows/security.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/security.yml)
[![codecov](https://codecov.io/gh/alehatsman/mooncake/branch/master/graph/badge.svg)](https://codecov.io/gh/alehatsman/mooncake)

Space fighters provisioning tool for managing dotfiles and system configuration. **Chookity!**

```yaml
- name: Hello Mooncake
  shell: echo "Running on {{os}}/{{arch}}"

- name: Deploy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

## Features

- 🚀 **Single Binary** - No dependencies, written in Go
- 🎨 **Animated TUI** - Real-time progress with beautiful output
- 🔍 **Dry-run Mode** - Preview all changes before applying
- 📝 **Simple YAML** - No complex DSL to learn
- 🌍 **Cross-Platform** - Linux, macOS, and Windows
- 🔧 **Powerful** - Variables, conditionals, loops, templates, system facts
- ⚡ **Robust** - Timeouts, retries, custom environments, failure handling
- 📁 **Advanced File Operations** - Create, remove, link, copy, download with checksums, extract archives, ownership management
- ⚙️ **Service Management** - Manage systemd (Linux) and launchd (macOS) services with full lifecycle control

## Quick Start

```bash
# Install
go install github.com/alehatsman/mooncake@latest

# Create a configuration
cat > config.yml <<EOF
- name: Install packages
  shell: "{{package_manager}} install neovim ripgrep"
  become: true
  when: os == "linux"

- name: Setup dotfiles
  template:
    src: ./templates/vimrc.j2
    dest: ~/.vimrc
    mode: "0644"
EOF

# Preview changes
mooncake run --config config.yml --dry-run

# Apply configuration
mooncake run --config config.yml
```

## Documentation

📚 **[Full Documentation](https://mooncake.alehatsman.com)** - Complete guide with examples

Quick links:
- [Installation Guide](https://mooncake.alehatsman.com#installation)
- [Quick Start Tutorial](https://mooncake.alehatsman.com#quick-start)
- [Configuration Reference](https://mooncake.alehatsman.com/guide/config/actions/)
- [Examples](https://mooncake.alehatsman.com/examples/)
- [Best Practices](https://mooncake.alehatsman.com#best-practices)

### Local Examples

Try the examples in the [`examples/`](examples/) directory:

```bash
# Run Hello World example
mooncake run --config examples/01-hello-world/config.yml

# Browse all examples
ls examples/
```

## Common Use Cases

### Dotfiles Management
```yaml
- name: Backup existing dotfiles
  shell: |
    for file in .bashrc .vimrc .gitconfig; do
      [ -f ~/$file ] && cp ~/$file ~/.backup/$file
    done

- name: Deploy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
```

### Development Environment Setup
```yaml
- vars:
    packages: [neovim, ripgrep, fzf, tmux]

- name: Install dev tools
  shell: brew install {{item}}
  with_items: "{{packages}}"
  when: os == "darwin"
```

### Multi-OS Configuration
```yaml
- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"
```

## Key Features

### Variables & System Facts
```yaml
- name: OS-specific command
  shell: echo "Running on {{distribution}} {{distribution_version}}"

- name: Show system info
  shell: echo "{{cpu_cores}} cores, {{memory_total_mb}}MB RAM"
```

Run `mooncake facts` to see all available system facts.

### Execution Control
```yaml
- name: Robust command with retry
  shell: curl -O https://example.com/file.tar.gz
  timeout: 5m
  retries: 3
  retry_delay: 10s
  env:
    HTTP_PROXY: "{{proxy_url}}"
  cwd: /tmp/downloads
  changed_when: "result.rc == 0"
  failed_when: "result.rc not in [0, 18]"  # 18 = partial transfer
```

### Templates (pongo2)
```yaml
- name: Render nginx config
  template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    vars:
      port: 8080
      ssl_enabled: true
```

### Tags for Workflow Control
```yaml
- name: Development setup
  shell: install-dev-tools
  tags: [dev]

- name: Production deployment
  shell: deploy-to-prod
  tags: [prod]
```

Run with: `mooncake run --config config.yml --tags dev`

### Loops
```yaml
# Iterate over lists
- name: Create directories
  file:
    path: "/opt/{{item}}"
    state: directory
  with_items: [app1, app2, app3]

# Iterate over files
- name: Copy configs
  shell: cp "{{item.src}}" "/backup/{{item.name}}"
  with_filetree: ./configs
```

## Commands

```bash
# Run configuration
mooncake run --config config.yml

# Preview without executing
mooncake run --config config.yml --dry-run

# Filter by tags
mooncake run --config config.yml --tags dev,test

# With sudo
mooncake run --config config.yml --sudo-pass <password>

# Debug mode
mooncake run --config config.yml --log-level debug

# Show system information
mooncake facts
```

## Why Mooncake?

| Feature | Mooncake | Ansible | Shell Scripts |
|---------|----------|---------|---------------|
| **Setup** | Single binary | Python + modules | Text editor |
| **Dependencies** | None | Python, modules | System tools |
| **Learning Curve** | Minutes | Hours/Days | Varies |
| **Cross-platform** | ✅ Built-in | ⚠️ Limited | ❌ OS-specific |
| **Dry-run** | ✅ Native | ✅ Check mode | ❌ Manual |
| **Best For** | Personal configs, dotfiles | Enterprise automation | Quick tasks |

## Testing

Mooncake is thoroughly tested across multiple platforms:

- **Linux**: Ubuntu 22.04/20.04, Debian 12, Alpine 3.19, Fedora 39 (Docker)
- **macOS**: Intel (macos-13) and Apple Silicon (macos-latest) - native + GitHub Actions
- **Windows**: Windows Server (GitHub Actions)

### Quick Testing Commands

```bash
# Run unit tests on current platform
make test

# Quick smoke test (Linux via Docker, ~2 minutes)
make test-quick

# Test on specific Linux distro
make test-docker-ubuntu
make test-docker-alpine

# Test all Linux distros (~10 minutes)
make test-smoke

# Run complete local test suite (native + Docker)
make test-all-platforms
```

### Documentation

- 📖 **[Testing Documentation Index](docs/testing/README.md)** - Complete testing docs
- ⚡ **[Quick Reference](docs/testing/quick-reference.md)** - Common commands
- 📚 **[Testing Guide](docs/testing/guide.md)** - Detailed guide
- 🏗️ **[Architecture](docs/testing/architecture.md)** - How it works

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

- 🐛 [Report bugs](https://github.com/alehatsman/mooncake/issues)
- 💡 [Request features](https://github.com/alehatsman/mooncake/issues)
- 📖 [Documentation](https://mooncake.alehatsman.com)
- 🗺️ [Roadmap](https://github.com/alehatsman/mooncake/blob/master/ROADMAP.md)

## License

MIT License - Copyright (c) 2026 Aleh Atsman

See [LICENSE](LICENSE) file for details.

---

**[📚 Read the Full Documentation](https://mooncake.alehatsman.com)** for detailed guides, examples, and reference materials.

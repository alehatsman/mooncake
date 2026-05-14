# Cargo Make - Rust Task Runner

Task runner and build tool for Rust projects with Makefile-like functionality and cross-platform support.

## Quick Start
```yaml
- preset: cargo-make
```

## Features
- **Task automation**: Define custom build tasks in Makefile.toml
- **Cross-platform**: Works on Linux, macOS, Windows
- **Conditional logic**: Platform-specific tasks and conditions
- **Task dependencies**: Chain tasks together
- **Built-in tasks**: Common Rust tasks predefined
- **Workspace support**: Manage multi-crate workspaces

## Basic Usage
```bash
# Run default task
cargo make

# Run specific task
cargo make build

# List all tasks
cargo make --list-all-steps

# Run task with args
cargo make test -- --nocapture

# Run in release mode
cargo make --profile production
```

## Makefile.toml Example
```toml
[tasks.build]
command = "cargo"
args = ["build", "--release"]

[tasks.test]
command = "cargo"
args = ["test"]
dependencies = ["build"]

[tasks.clean]
command = "cargo"
args = ["clean"]

[tasks.dev]
watch = true
command = "cargo"
args = ["run"]
```

## Advanced Configuration
```toml
# Platform-specific tasks
[tasks.install-deps.linux]
command = "apt-get"
args = ["install", "-y", "libssl-dev"]

[tasks.install-deps.mac]
command = "brew"
args = ["install", "openssl"]

# Conditional execution
[tasks.deploy]
condition = { env_set = ["DEPLOY_KEY"] }
command = "kubectl"
args = ["apply", "-f", "k8s/"]

# Multi-step workflow
[tasks.ci]
dependencies = [
    "format-check",
    "lint",
    "test",
    "build"
]
```

## Real-World Examples

### Development Workflow
```yaml
- name: Install cargo-make
  preset: cargo-make

- name: Create Makefile.toml
  template:
    dest: Makefile.toml
    content: |
      [tasks.dev]
      watch = true
      command = "cargo"
      args = ["run"]

      [tasks.test-watch]
      watch = true
      command = "cargo"
      args = ["test"]

- name: Run development server
  shell: cargo make dev
  cwd: /app
```

### CI/CD Pipeline
```yaml
- name: Run CI tasks
  shell: cargo make ci
  cwd: /app

- name: Build release
  shell: cargo make build --profile production
  cwd: /app

- name: Run benchmarks
  shell: cargo make bench
  cwd: /app
```

## Platform Support
- ✅ Linux (via cargo)
- ✅ macOS (via cargo)
- ✅ Windows (via cargo)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automate Rust build workflows
- Define consistent build steps across projects
- Create platform-specific build tasks
- Run complex CI/CD pipelines
- Manage multi-crate workspace builds

## Uninstall
```yaml
- preset: cargo-make
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/sagiegurari/cargo-make
- Documentation: https://sagiegurari.github.io/cargo-make/
- Search: "cargo make tutorial", "rust task runner", "cargo make examples"

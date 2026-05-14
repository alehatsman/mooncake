# Cargo Watch - Auto-rebuild

Watches your Rust project and automatically rebuilds, re-runs, or re-tests on file changes.

## Quick Start
```yaml
- preset: cargo-watch
```

## Features
- **Auto-rebuild**: Recompiles on file changes
- **Watch tests**: Auto-run tests on save
- **Custom commands**: Run any command on changes
- **Fast**: Only rebuilds what changed
- **Ignore patterns**: Exclude files/directories
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Watch and build
cargo watch

# Watch and run
cargo watch -x run

# Watch and test
cargo watch -x test

# Watch with multiple commands
cargo watch -x build -x test

# Watch specific files
cargo watch -w src/ -x build

# Clear screen on rebuild
cargo watch -c -x run

# Ignore target directory
cargo watch -i target/ -x build
```

## Development Workflow
```bash
# Development mode (build + run on change)
cargo watch -x 'run --bin myapp'

# Test-driven development
cargo watch -x 'test --lib'

# Watch and check (faster than build)
cargo watch -x check

# Watch with custom delay
cargo watch -d 2 -x build

# Execute shell command
cargo watch -s 'cargo build && ./target/debug/myapp'
```

## Real-World Examples

### Development Environment
```yaml
- name: Install cargo-watch
  preset: cargo-watch

- name: Start development server
  shell: cargo watch -x 'run --bin api-server'
  cwd: /app
```

### TDD Workflow
```yaml
- name: Watch and test
  shell: cargo watch -c -x 'test -- --nocapture'
  cwd: /app
```

### Multi-command Watching
```bash
# Format, check, test on every change
cargo watch -x fmt -x clippy -x test
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
- Enable hot-reload during development
- Auto-run tests in TDD workflows
- Continuously check code during development
- Monitor and rebuild on file changes
- Reduce manual rebuild cycles


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install cargo-watch
  preset: cargo-watch

- name: Use cargo-watch in automation
  shell: |
    # Custom configuration here
    echo "cargo-watch configured"
```
## Uninstall
```yaml
- preset: cargo-watch
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/watchexec/cargo-watch
- Search: "cargo watch tutorial", "rust auto build", "cargo watch examples"

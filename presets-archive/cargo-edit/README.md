# Cargo Edit - Cargo Subcommand Extensions

Cargo subcommands for modifying Cargo.toml dependencies from the command line.

## Quick Start
```yaml
- preset: cargo-edit
```

## Features
- **Add dependencies**: Add crates from command line
- **Remove dependencies**: Remove dependencies easily
- **Upgrade dependencies**: Update dependencies to latest versions
- **Set versions**: Pin specific versions
- **Automatic formatting**: Keeps Cargo.toml clean
- **Feature management**: Add/remove features for dependencies

## Basic Usage
```bash
# Add a dependency
cargo add serde

# Add with version
cargo add serde@1.0

# Add with features
cargo add serde --features derive

# Add dev dependency
cargo add --dev proptest

# Add build dependency
cargo add --build cc

# Remove dependency
cargo rm serde

# Upgrade dependencies
cargo upgrade

# Upgrade specific dependency
cargo upgrade serde

# Upgrade to latest compatible version
cargo upgrade --compatible
```

## Advanced Usage
```bash
# Add from git
cargo add serde --git https://github.com/serde-rs/serde

# Add from path
cargo add mylib --path ../mylib

# Add optional dependency
cargo add feature-x --optional

# Add with renamed dependency
cargo add serde --rename serde_crate

# Set specific version
cargo set-version 1.0.0

# Add multiple dependencies
cargo add serde tokio reqwest
```

## Real-World Examples

### Quick Project Setup
```bash
# Start new project
cargo new myapp
cd myapp

# Add common dependencies
cargo add tokio --features full
cargo add serde --features derive
cargo add serde_json
cargo add reqwest --features json
cargo add anyhow

# Add dev dependencies
cargo add --dev proptest
cargo add --dev criterion
```

### CI/CD Dependency Management
```yaml
- name: Install cargo-edit
  preset: cargo-edit

- name: Upgrade dependencies
  shell: cargo upgrade --workspace
  cwd: /app

- name: Check for outdated dependencies
  shell: cargo outdated
  cwd: /app
  register: outdated
  failed_when: false

- name: Build with updated dependencies
  shell: cargo build --release
  cwd: /app
```

### Development Workflow
```yaml
- name: Add new feature dependency
  shell: cargo add async-trait
  cwd: /project

- name: Add with specific feature
  shell: cargo add sqlx --features "postgres runtime-tokio-native-tls"
  cwd: /project

- name: Build and test
  shell: cargo test
  cwd: /project
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
- Automate Rust dependency management
- Update project dependencies in CI/CD
- Quickly add dependencies during development
- Upgrade dependencies across workspaces
- Script dependency modifications


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install cargo-edit
  preset: cargo-edit

- name: Use cargo-edit in automation
  shell: |
    # Custom configuration here
    echo "cargo-edit configured"
```
## Uninstall
```yaml
- preset: cargo-edit
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/killercup/cargo-edit
- Documentation: https://github.com/killercup/cargo-edit/blob/master/README.md
- Search: "cargo edit tutorial", "cargo add", "rust dependency management"

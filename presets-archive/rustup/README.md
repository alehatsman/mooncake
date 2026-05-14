# rustup - Rust Toolchain Installer

rustup is the official Rust toolchain installer for managing Rust versions and components.

## Quick Start

```yaml
- preset: rustup
```

## Features

- **Official installer**: Recommended way to install Rust
- **Multiple toolchains**: Install stable, beta, nightly simultaneously
- **Cross-compilation**: Install targets for different platforms
- **Component management**: rustfmt, clippy, rust-analyzer, and more
- **Override system**: Per-project toolchain selection
- **Update management**: Easy updates to latest Rust versions

## Basic Usage

```bash
# Install Rust (stable)
rustup default stable

# Install nightly
rustup install nightly

# Switch default toolchain
rustup default nightly

# Update Rust
rustup update

# Show installed toolchains
rustup show

# Check version
rustc --version
cargo --version

# Install component
rustup component add clippy
rustup component add rustfmt
```

## Advanced Configuration

```yaml
# Simple installation
- preset: rustup

# Remove installation
- preset: rustup
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (official installer script)
- ✅ macOS (Homebrew or official installer)
- ❌ Windows (not yet supported by this preset)

## Configuration

- **Home directory**: `~/.rustup/` (toolchains and settings)
- **Cargo home**: `~/.cargo/` (installed binaries and registry)
- **Toolchain override**: `rust-toolchain.toml` (per-project)
- **Shell integration**: Automatically added to PATH

## Real-World Examples

### Basic Rust Setup

```bash
# Install rustup (done by preset)
# Default stable toolchain installed automatically

# Install additional components
rustup component add clippy
rustup component add rustfmt
rustup component add rust-src

# Verify installation
cargo --version
rustc --version
rustfmt --version
clippy-driver --version
```

### Multi-Toolchain Setup

```bash
# Install multiple toolchains
rustup install stable
rustup install beta
rustup install nightly

# Set default
rustup default stable

# Use nightly for current shell
rustup run nightly cargo build

# Use nightly for specific project
cd myproject
rustup override set nightly
```

### Cross-Compilation

```bash
# Install cross-compilation targets
rustup target add x86_64-unknown-linux-musl
rustup target add aarch64-unknown-linux-gnu
rustup target add wasm32-unknown-unknown

# Build for specific target
cargo build --target x86_64-unknown-linux-musl
cargo build --target wasm32-unknown-unknown

# List installed targets
rustup target list --installed
```

### Per-Project Toolchain

```toml
# rust-toolchain.toml in project root
[toolchain]
channel = "1.75.0"
components = ["rustfmt", "clippy"]
targets = ["wasm32-unknown-unknown"]
```

```bash
# When you cd into project, toolchain activates automatically
cd myproject
cargo build  # Uses specified toolchain
```

### CI/CD Setup

```yaml
# Install Rust in CI pipeline
- preset: rustup

- name: Install stable toolchain
  shell: rustup default stable

- name: Install components
  shell: |
    rustup component add clippy
    rustup component add rustfmt

- name: Cache cargo registry
  # Cache ~/.cargo/registry for faster builds

- name: Run tests
  shell: cargo test --all-features

- name: Run clippy
  shell: cargo clippy -- -D warnings

- name: Check formatting
  shell: cargo fmt -- --check
```

### Development Workflow

```bash
# Format code
cargo fmt

# Lint code
cargo clippy

# Run tests
cargo test

# Build release
cargo build --release

# Run benchmarks
cargo bench

# Generate documentation
cargo doc --open
```

## Common Commands

```bash
# Toolchain management
rustup install 1.75.0          # Install specific version
rustup uninstall nightly       # Remove toolchain
rustup default stable          # Set default
rustup override set nightly    # Project override
rustup override unset          # Remove override

# Component management
rustup component add rust-analyzer
rustup component remove rustfmt
rustup component list          # List available
rustup component list --installed

# Target management
rustup target add wasm32-unknown-unknown
rustup target remove x86_64-pc-windows-gnu
rustup target list            # List available

# Updates
rustup update                 # Update all toolchains
rustup update stable          # Update specific toolchain
rustup self update           # Update rustup itself

# Information
rustup show                   # Show active toolchain
rustup which cargo           # Show cargo path
rustup doc                   # Open local docs
```

## Agent Use

- Install and manage Rust toolchains in automated environments
- Configure cross-compilation targets for deployment
- Ensure consistent Rust versions across development team
- Install specific Rust versions for CI/CD pipelines
- Manage Rust components (clippy, rustfmt) for code quality

## Troubleshooting

### rustup: command not found

```bash
# Source cargo environment
source ~/.cargo/env

# Or add to shell config
echo 'source ~/.cargo/env' >> ~/.bashrc
source ~/.bashrc

# For zsh
echo 'source ~/.cargo/env' >> ~/.zshrc
source ~/.zshrc
```

### Toolchain not switching

```bash
# Check current toolchain
rustup show

# Remove override
rustup override unset

# Verify rust-toolchain.toml
cat rust-toolchain.toml

# Force reinstall
rustup toolchain uninstall stable
rustup toolchain install stable
```

### Compilation errors after update

```bash
# Clean build artifacts
cargo clean

# Update dependencies
cargo update

# Check for breaking changes
rustup doc --release-notes
```

### Disk space issues

```bash
# Remove unused toolchains
rustup toolchain list
rustup toolchain uninstall nightly-2023-01-01

# Clean cargo cache
cargo cache --autoclean

# Remove old target builds
cargo clean
```

## Uninstall

```yaml
- preset: rustup
  with:
    state: absent
```

**Note**: This removes rustup and all installed Rust toolchains. To manually uninstall:

```bash
rustup self uninstall
rm -rf ~/.rustup ~/.cargo
```

## Resources

- Official docs: https://rust-lang.github.io/rustup/
- Rust book: https://doc.rust-lang.org/book/
- GitHub: https://github.com/rust-lang/rustup
- Search: "rustup tutorial", "rust installation", "rustup toolchain management"

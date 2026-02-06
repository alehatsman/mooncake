# Rust Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check version
rustc --version
cargo --version

# Create new project
cargo new hello-world
cd hello-world
cargo run

# Build release
cargo build --release
```

## Configuration

- **Install directory:** `~/.cargo/`, `~/.rustup/`
- **Binaries:** `~/.cargo/bin/`
- **Toolchains:** Managed via rustup

## Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `toolchain` | `stable` | stable/beta/nightly/1.75.0 |
| `profile` | `default` | minimal/default/complete |
| `components` | `[]` | clippy, rustfmt, rust-analyzer, etc. |
| `targets` | `[]` | wasm32, musl, cross-compile targets |

## Common Operations

```bash
# Manage toolchains
rustup update
rustup toolchain install nightly
rustup default nightly

# Components
rustup component add clippy rustfmt rust-analyzer

# Targets
rustup target add wasm32-unknown-unknown

# Build & test
cargo build
cargo test
cargo run

# Format & lint
cargo fmt
cargo clippy

# Documentation
cargo doc --open
rustup doc
```

## Usage Examples

**Basic install:**
```yaml
- preset: rust
```

**With dev tools:**
```yaml
- preset: rust
  with:
    components: [clippy, rustfmt, rust-analyzer]
```

**WASM development:**
```yaml
- preset: rust
  with:
    targets: [wasm32-unknown-unknown]
    components: [clippy, rustfmt]
```

**Minimal (CI):**
```yaml
- preset: rust
  with:
    profile: minimal
```

## Common Components

- `clippy` - Linter
- `rustfmt` - Formatter
- `rust-analyzer` - LSP
- `rust-src` - Source code
- `llvm-tools-preview` - Profiling

## Common Targets

- `wasm32-unknown-unknown` - WebAssembly
- `x86_64-unknown-linux-musl` - Static binaries
- `aarch64-unknown-linux-gnu` - ARM64 Linux
- `x86_64-pc-windows-gnu` - Windows cross-compile

## Project Structure

```
project/
├── Cargo.toml      # Package metadata
├── Cargo.lock      # Dependency versions
├── src/
│   └── main.rs     # Source code
└── target/         # Build output
```

## Cargo.toml Example

```toml
[package]
name = "myapp"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
tokio = { version = "1", features = ["full"] }

[profile.release]
opt-level = 3
lto = true
```

## Uninstall

```yaml
- preset: rust
  with:
    state: absent
```

**Note:** Projects in `~/.cargo/` preserved after uninstall.

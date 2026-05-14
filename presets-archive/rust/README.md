# Rust Preset

**Status:** ✓ Installed successfully

## Features

- **Toolchain management** - Stable, beta, nightly via rustup
- **Multiple targets** - Cross-compilation support
- **Components** - clippy, rustfmt, rust-analyzer
- **Zero-cost abstractions** - High performance
- **Memory safety** - No GC, no data races
- **Package manager** - Cargo built-in
- **WebAssembly** - First-class WASM support
- **Cross-platform** - Linux, macOS, Windows

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

## Basic Usage

After installation:
```bash
# Create binary project
cargo new myapp
cd myapp

# Create library project
cargo new --lib mylib

# Run project
cargo run

# Run with arguments
cargo run -- arg1 arg2

# Build (debug)
cargo build

# Build (release)
cargo build --release

# Check without building
cargo check

# Run tests
cargo test

# Format code
cargo fmt

# Lint code
cargo clippy

# Update dependencies
cargo update

# Show documentation
cargo doc --open
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

## Advanced Configuration

### Custom Installation
```yaml
- preset: rust
  with:
    toolchain: nightly
    profile: complete
    components: [clippy, rustfmt, rust-analyzer, rust-src]
    targets: [wasm32-unknown-unknown, x86_64-unknown-linux-musl]
```

### Toolchain Management
```bash
# Install toolchains
rustup toolchain install stable
rustup toolchain install nightly
rustup toolchain install 1.75.0

# Set default
rustup default stable
rustup default nightly

# Per-directory override
rustup override set nightly

# Show active toolchain
rustup show

# Update
rustup update
```

### Components
```bash
# Add components
rustup component add clippy
rustup component add rustfmt
rustup component add rust-analyzer
rustup component add rust-src
rustup component add llvm-tools-preview

# List components
rustup component list
```

### Cross-Compilation
```bash
# Add targets
rustup target add wasm32-unknown-unknown
rustup target add x86_64-unknown-linux-musl
rustup target add aarch64-unknown-linux-gnu

# Build for target
cargo build --target wasm32-unknown-unknown
cargo build --target x86_64-unknown-linux-musl

# Static binary (musl)
cargo build --release --target x86_64-unknown-linux-musl
```

### Performance Tuning
```toml
# Cargo.toml
[profile.release]
opt-level = 3           # Maximum optimization
lto = "fat"            # Link-time optimization
codegen-units = 1      # Better optimization
strip = true           # Remove debug symbols
panic = "abort"        # Smaller binary

[profile.dev]
opt-level = 1          # Fast debug builds
```

### Build Caching
```bash
# Use sccache
cargo install sccache
export RUSTC_WRAPPER=sccache

# Show cache stats
sccache --show-stats
```

## Platform Support

- ✅ **Linux** - x86_64, ARM64, musl
- ✅ **macOS** - Intel, Apple Silicon
- ✅ **Windows** - MSVC, GNU
- ✅ **WebAssembly** - wasm32 target
- ✅ **Embedded** - ARM Cortex-M, RISC-V

**Tier 1 Targets** (guaranteed to work):
- x86_64-unknown-linux-gnu
- x86_64-apple-darwin
- aarch64-apple-darwin
- x86_64-pc-windows-msvc

## Agent Use

Rust is ideal for high-performance agent systems:

### System Agents
```rust
use std::process::Command;

fn agent_execute(cmd: &str) -> String {
    let output = Command::new("sh")
        .arg("-c")
        .arg(cmd)
        .output()
        .expect("Failed to execute");
    String::from_utf8_lossy(&output.stdout).to_string()
}
```

### Async Agents
```rust
use tokio::runtime::Runtime;

#[tokio::main]
async fn main() {
    // Concurrent agent tasks
    let handle1 = tokio::spawn(async { task1().await });
    let handle2 = tokio::spawn(async { task2().await });

    let (r1, r2) = tokio::join!(handle1, handle2);
}
```

### CLI Tools
```rust
use clap::Parser;

#[derive(Parser)]
struct Args {
    #[arg(short, long)]
    config: String,
}

fn main() {
    let args = Args::parse();
    // Agent logic
}
```

### WebAssembly Agents
```yaml
# Build WASM agent
- preset: rust
  with:
    targets: [wasm32-unknown-unknown]

- name: Build WASM
  shell: |
    cargo build --release --target wasm32-unknown-unknown
```

### HTTP Agents
```rust
use reqwest;

async fn fetch_data(url: &str) -> Result<String, Box<dyn std::error::Error>> {
    let body = reqwest::get(url).await?.text().await?;
    Ok(body)
}
```

### Parallel Processing
```rust
use rayon::prelude::*;

fn process_parallel(items: Vec<String>) -> Vec<String> {
    items.par_iter()
        .map(|item| process(item))
        .collect()
}
```

Benefits for agents:
- **Performance** - Near C/C++ speed
- **Safety** - No segfaults, no data races
- **Concurrency** - Fearless async/await
- **Small binaries** - Single executable
- **Cross-compile** - Build anywhere, run anywhere
- **WebAssembly** - Run in browsers/edge
- **Zero cost** - No runtime overhead

## Uninstall

```yaml
- preset: rust
  with:
    state: absent
```

**Note:** Projects in `~/.cargo/` preserved after uninstall.

## Resources
- Official site: https://www.rust-lang.org/
- The Rust Book: https://doc.rust-lang.org/book/
- Rust by Example: https://doc.rust-lang.org/rust-by-example/
- Cargo Book: https://doc.rust-lang.org/cargo/
- Crates.io: https://crates.io/
- Rust Playground: https://play.rust-lang.org/
- Search: "rust tutorial", "rust book", "cargo guide", "rust async programming"

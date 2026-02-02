# Rust/rustup Preset

Install Rust programming language via rustup (the official Rust toolchain installer).

## Features

- ✅ Installs rustup (Rust toolchain manager)
- ✅ Installs specified Rust toolchain (stable, beta, nightly)
- ✅ Configures default toolchain
- ✅ Installs additional components (clippy, rustfmt, rust-analyzer)
- ✅ Adds compilation targets (wasm, cross-compile)
- ✅ Configures shell profiles automatically
- ✅ Cross-platform (Linux, macOS, Windows)

## Usage

### Install stable Rust
```yaml
- name: Install Rust
  preset: rust
```

### Install with dev tools
```yaml
- name: Install Rust with common tools
  preset: rust
  with:
    toolchain: stable
    components:
      - clippy        # Linter
      - rustfmt       # Code formatter
      - rust-analyzer # LSP server
      - rust-src      # Source code (for goto definition)
```

### Install nightly with WASM support
```yaml
- name: Install Rust nightly for WebAssembly
  preset: rust
  with:
    toolchain: nightly
    components:
      - rustfmt
      - clippy
    targets:
      - wasm32-unknown-unknown
      - wasm32-wasi
```

### Install minimal Rust
```yaml
- name: Install minimal Rust (CI/small containers)
  preset: rust
  with:
    profile: minimal   # Just rustc, rust-std, cargo
    toolchain: stable
```

### Uninstall
```yaml
- name: Remove Rust and rustup
  preset: rust
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `toolchain` | string | `stable` | Toolchain: "stable", "beta", "nightly", "1.75.0" |
| `set_default` | bool | `true` | Set as default toolchain |
| `profile` | string | `default` | Profile: "minimal", "default", "complete" |
| `components` | array | `[]` | Components to install |
| `targets` | array | `[]` | Compilation targets to add |

## Profiles

| Profile | Includes |
|---------|----------|
| **minimal** | rustc, rust-std, cargo |
| **default** | minimal + rust-docs, rustfmt, clippy |
| **complete** | All components |

## Common Components

- `clippy` - Linter for catching common mistakes
- `rustfmt` - Code formatter
- `rust-analyzer` - LSP server for IDE support
- `rust-src` - Rust source code (for IDE goto definition)
- `rust-docs` - Offline documentation
- `llvm-tools-preview` - LLVM tools for profiling/coverage

## Common Targets

- `wasm32-unknown-unknown` - WebAssembly
- `wasm32-wasi` - WebAssembly with WASI
- `x86_64-pc-windows-gnu` - Windows (from Linux)
- `x86_64-unknown-linux-musl` - Static Linux binaries
- `aarch64-unknown-linux-gnu` - ARM64 Linux
- `x86_64-apple-darwin` - macOS Intel
- `aarch64-apple-darwin` - macOS Apple Silicon

## Platform Support

- ✅ Linux (all distributions)
- ✅ macOS
- ✅ Windows

## What Gets Installed

1. **rustup** - Toolchain manager
   - Installed to `~/.rustup/`
   - Manages multiple Rust versions

2. **Rust toolchain** - Compiler and tools
   - Installed to `~/.cargo/`
   - Includes rustc (compiler) and cargo (build tool)

3. **PATH configuration**
   - `~/.cargo/bin` added to PATH
   - Configured in `~/.bashrc`, `~/.zshrc`, `~/.profile`

## Common Use Cases

### Web Development
```yaml
- name: Rust for web development
  preset: rust
  with:
    components:
      - clippy
      - rustfmt
      - rust-analyzer
    targets:
      - wasm32-unknown-unknown  # For WebAssembly
```

### Systems Programming
```yaml
- name: Rust for systems programming
  preset: rust
  with:
    profile: complete
    components:
      - rust-src
      - llvm-tools-preview
```

### Cross-compilation
```yaml
- name: Rust for cross-compilation
  preset: rust
  with:
    targets:
      - x86_64-unknown-linux-musl    # Static Linux binaries
      - aarch64-unknown-linux-gnu    # ARM64
      - x86_64-pc-windows-gnu        # Windows
```

## Post-Installation

After installation, restart your terminal or run:
```bash
source ~/.cargo/env
```

### Using rustup
```bash
# List installed toolchains
rustup toolchain list

# Install another toolchain
rustup toolchain install nightly

# Switch toolchain
rustup default nightly

# Update toolchains
rustup update

# Install component
rustup component add rust-analyzer

# Install target
rustup target add wasm32-unknown-unknown

# Show docs
rustup doc

# Check for updates
rustup check
```

### Verify Installation
```bash
rustc --version
cargo --version
rustup --version
```

### Create First Project
```bash
cargo new hello-world
cd hello-world
cargo run
```

## Learn More

- [Rust Book](https://doc.rust-lang.org/book/)
- [rustup Documentation](https://rust-lang.github.io/rustup/)
- [Cargo Book](https://doc.rust-lang.org/cargo/)
- [Rust by Example](https://doc.rust-lang.org/rust-by-example/)

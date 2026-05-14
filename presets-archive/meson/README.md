# Meson - Fast, Developer-Friendly Build System

A next-generation build system designed for speed and simplicity. Meson compiles projects faster than traditional build systems like CMake or Autotools while providing a clean, intuitive configuration syntax. Built on Python, Meson supports cross-compilation, multiple languages (C, C++, Rust, Java, D), and seamless integration with modern development workflows.

## Quick Start

```yaml
- preset: meson
```

## Features

- **Fast compilation**: Ninja backend for parallel builds with minimal overhead
- **Cross-compilation**: Built-in support for targeting different architectures and platforms
- **Python-based**: Clean, readable configuration syntax that's easy to learn
- **Multi-language**: Compile C, C++, Rust, Java, D, and other languages in one project
- **Modern tooling**: First-class support for pkg-config, CMake, and other dependency management
- **Easy dependency management**: Automatic dependency resolution and version checking

## Basic Usage

```bash
# Initialize project (create build directory and Meson configuration)
meson setup builddir

# Build with Ninja backend
meson compile -C builddir

# Run tests
meson test -C builddir

# Install built artifacts
meson install -C builddir

# Check version
meson --version
```

## Advanced Configuration

```yaml
# Basic installation
- preset: meson

# Install with specific version
- preset: meson
  with:
    version: "1.3.0"

# Install from source with pip
- preset: meson
  with:
    method: pip

# Install via package manager only (fail if unavailable)
- preset: meson
  with:
    method: package
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Meson (present/absent) |
| version | string | latest | Specific version to install (e.g., "1.3.0", "latest") |
| method | string | auto | Installation method: auto (try package manager first), package (package manager only), pip (pip only) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported, but available via WSL)

## Configuration

- **Default installation**: `/usr/bin/meson` (Linux/macOS via package manager), `~/.local/bin/meson` (pip)
- **Python requirement**: Python 3.7+
- **Ninja backend**: Automatically installed as dependency
- **Project config**: `meson.build` files (read from project root)

## Real-World Examples

### Building a C Project

```bash
# Setup build environment
meson setup builddir --prefix=/usr/local

# Build application
meson compile -C builddir

# Run unit tests before install
meson test -C builddir

# Install to specified prefix
meson install -C builddir
```

### Cross-Compilation for ARM

```bash
# Create cross-compilation file
cat > armv7.txt << 'EOF'
[properties]
c_args = ['-march=armv7-a']
cpp_args = ['-march=armv7-a']

[binaries]
c = '/usr/bin/arm-linux-gnueabihf-gcc'
cpp = '/usr/bin/arm-linux-gnueabihf-g++'
ar = '/usr/bin/arm-linux-gnueabihf-ar'
EOF

# Setup with cross-compilation
meson setup builddir --cross-file armv7.txt
```

### CI/CD Integration

```yaml
# Example: Build and test in CI pipeline
- name: Setup Meson build
  preset: meson

- name: Initialize project
  shell: meson setup builddir --prefix=/tmp/install
  cwd: /path/to/project

- name: Compile
  shell: meson compile -C builddir
  cwd: /path/to/project

- name: Run tests
  shell: meson test -C builddir
  cwd: /path/to/project

- name: Generate coverage report
  shell: meson compile -C builddir coverage
  cwd: /path/to/project
```

### Multi-Language Project

```bash
# Compile C/C++ executable with Rust library
meson setup builddir

# Specify build options for mixed language support
meson configure builddir -Dcpp_std=c++17

# Build everything
meson compile -C builddir
```

## Agent Use

- Automate cross-platform C/C++ project builds in deployment pipelines
- Perform dependency resolution and version compatibility checks
- Execute unit tests and generate coverage reports in CI/CD workflows
- Manage multi-language compilation and linking in monorepo environments
- Verify build compatibility across different target platforms and architectures
- Parse Meson configuration files to analyze project structure and dependencies

## Troubleshooting

### Meson not found in PATH

If `meson` command is not found after installation via pip:
```bash
# Check Python user site-packages path
python3 -m site --user-site

# Add to PATH if needed
export PATH="$PATH:$(python3 -m site --user-site)/bin"

# Or use full path
python3 -m pip install --user meson
```

### Build failures due to missing dependencies

```bash
# Check which dependencies are missing
meson setup builddir --cleandeps

# Review detailed error messages
meson compile -C builddir -v  # Verbose mode
```

### Ninja backend not found

If Meson can't find Ninja:
```bash
# Manually specify backend
meson setup builddir -Dbackend=vs  # Visual Studio on Windows

# Or ensure Ninja is installed first
apt-get install ninja-build  # Linux
brew install ninja            # macOS
```

### Version conflicts between Meson and project

```bash
# Check installed version
meson --version

# Check project requirements in meson.build
grep -E "project\(|version:" meson.build

# Reinstall specific version if needed
pip install meson==1.3.0
```

## Uninstall

```yaml
- preset: meson
  with:
    state: absent
```

## Resources

- Official docs: https://mesonbuild.com/
- GitHub: https://github.com/mesonbuild/meson
- Manual: https://mesonbuild.com/Manual.html
- Meson quick guide: https://mesonbuild.com/Quick-guide.html
- Search: "meson build system tutorial", "meson cross-compilation", "meson best practices"

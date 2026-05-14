# Zig - General-Purpose Programming Language

A general-purpose programming language and toolchain for maintaining robust, optimal, and reusable software with compile-time code execution and manual memory management.

## Quick Start
```yaml
- preset: zig
```

## Features
- **Simple language**: No hidden control flow, no hidden memory allocations
- **C interoperability**: Import .h files directly, cross-compile to any target
- **Compile-time execution**: Run arbitrary code at compile time
- **Memory safety**: Optional runtime checks, clear undefined behavior
- **Fast builds**: Incremental compilation and caching
- **Cross-platform**: Windows, Linux, macOS, bare metal

## Basic Usage
```bash
# Create new project
zig init-exe

# Build and run
zig build run

# Build for release
zig build -Doptimize=ReleaseFast

# Run tests
zig build test

# Format code
zig fmt src/

# Check syntax without building
zig ast-check src/main.zig

# Show version
zig version

# Get help
zig --help
```

## Common Commands

### Compilation
```bash
# Compile single file
zig build-exe src/main.zig

# Compile library
zig build-lib src/mylib.zig

# Cross-compile for different target
zig build-exe src/main.zig -target x86_64-linux

# Enable optimizations
zig build-exe src/main.zig -O ReleaseFast
```

### Testing
```bash
# Run all tests
zig test src/main.zig

# Run tests with coverage
zig test src/main.zig --test-coverage

# Test specific function
zig test src/main.zig --test-filter "my_function"
```

### C Integration
```bash
# Compile C code with Zig
zig cc main.c -o main

# Use as drop-in C compiler
zig c++ main.cpp -o main

# Translate C to Zig
zig translate-c input.h > output.zig
```

## Advanced Configuration
```yaml
# Install specific version
- preset: zig
  with:
    version: "0.12.0"

# Install latest version
- preset: zig
  with:
    version: "latest"
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| version | string | latest | Zig version to install (e.g., "0.12.0", "latest") |

## Platform Support
- ✅ Linux (tarball download)
- ✅ macOS (Homebrew, tarball download)
- ✅ Windows (tarball download)

## Configuration
- **Binary location**: `/usr/local/bin/zig` (manual), `/opt/homebrew/bin/zig` (Homebrew)
- **Standard library**: Bundled with Zig installation
- **Global cache**: `~/.cache/zig/` (Linux), `~/Library/Caches/zig/` (macOS)
- **Project config**: `build.zig` (build script), `build.zig.zon` (package manifest)

### Sample build.zig
```zig
const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "myapp",
        .root_source_file = .{ .path = "src/main.zig" },
        .target = target,
        .optimize = optimize,
    });

    b.installArtifact(exe);

    const run_cmd = b.addRunArtifact(exe);
    const run_step = b.step("run", "Run the app");
    run_step.dependOn(&run_cmd.step);
}
```

## Real-World Examples

### Hello World Project
```bash
# Create new project
mkdir myproject && cd myproject
zig init-exe

# Edit src/main.zig
cat > src/main.zig <<'EOF'
const std = @import("std");

pub fn main() !void {
    const stdout = std.io.getStdOut().writer();
    try stdout.print("Hello, {s}!\n", .{"World"});
}
EOF

# Build and run
zig build run
```

### Cross-Compilation for Embedded
```bash
# Compile for ARM Cortex-M4
zig build-exe src/firmware.zig \
  -target thumb-freestanding-eabi \
  -mcpu cortex_m4

# Compile for Raspberry Pi
zig build-exe src/app.zig \
  -target aarch64-linux
```

### Using Zig as C Compiler in CI/CD
```yaml
# In mooncake playbook
- name: Install Zig
  preset: zig

- name: Build C project with Zig
  shell: zig cc -o myapp main.c lib.c
  cwd: /path/to/c/project

- name: Run tests
  shell: ./myapp --test
```

## Agent Use
- Compile Zig projects in CI/CD pipelines
- Cross-compile binaries for multiple platforms
- Use as drop-in C/C++ compiler for faster builds
- Generate optimized binaries for deployment
- Build embedded firmware in automated workflows

## Troubleshooting

### zig command not found
Ensure Zig is in PATH:
```bash
export PATH="/usr/local/bin:$PATH"
```

### Build fails with "FileNotFound"
Check that build.zig exists:
```bash
ls build.zig
# If missing, create with: zig init-exe
```

### Linking errors with C libraries
Specify library paths explicitly:
```bash
zig build-exe src/main.zig -L/usr/local/lib -lmylib
```

### Slow incremental builds
Clear build cache:
```bash
rm -rf zig-cache/ zig-out/
zig build
```

## Uninstall
```yaml
- preset: zig
  with:
    state: absent
```

**Manual cleanup:**
```bash
# Remove cache
rm -rf ~/.cache/zig/  # Linux
rm -rf ~/Library/Caches/zig/  # macOS
```

## Resources
- Official site: https://ziglang.org
- Documentation: https://ziglang.org/documentation/master/
- GitHub: https://github.com/ziglang/zig
- Learn: https://ziglearn.org
- Search: "zig language tutorial", "zig vs rust", "zig cross compilation"

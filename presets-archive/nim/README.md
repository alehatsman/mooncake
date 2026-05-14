# Nim - Efficient Systems Programming Language

Statically typed compiled systems programming language combining Python-like syntax with C-like performance.

## Quick Start

```yaml
- preset: nim
```

## Features

- **Python-like syntax**: Easy to learn and read
- **C performance**: Compiles to C, C++, or JavaScript
- **Memory safety**: Optional garbage collection or manual memory management
- **Macro system**: Powerful compile-time metaprogramming
- **Cross-compilation**: Target multiple platforms from one machine
- **Small binaries**: Minimal runtime overhead
- **FFI**: Easy C/C++ interop

## Basic Usage

```bash
# Check version
nim --version

# Compile and run
nim c -r hello.nim

# Compile for release (optimized)
nim c -d:release myapp.nim

# Compile to JavaScript
nim js myapp.nim

# Run without compiling (interpreter mode)
nim e script.nim

# Generate documentation
nim doc mymodule.nim
```

## Advanced Configuration

```yaml
# Install Nim
- preset: nim

# Create Nim project
- name: Create project directory
  file:
    path: /opt/myapp
    state: directory

- name: Compile Nim application
  shell: nim c -d:release --opt:size myapp.nim
  cwd: /opt/myapp

- name: Install binary
  copy:
    src: /opt/myapp/myapp
    dest: /usr/local/bin/myapp
    mode: "0755"
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Nim |

## Platform Support

- ✅ Linux (apt, dnf, binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Windows (installer, chocolatey)

## Configuration

- **Config file**: `nim.cfg` or `config.nims` (project-specific)
- **Standard library**: `~/.nimble/pkgs/`
- **Package manager**: Nimble (included with Nim)
- **Compiler cache**: `~/.cache/nim/`

## Real-World Examples

### Hello World
```nim
# hello.nim
echo "Hello, World!"

# Compile and run
# nim c -r hello.nim
```

### HTTP Server
```nim
# server.nim
import asynchttpserver, asyncdispatch

var server = newAsyncHttpServer()

proc handleRequest(req: Request) {.async.} =
  await req.respond(Http200, "Hello from Nim!")

waitFor server.serve(Port(8080), handleRequest)
```

### CLI Application
```nim
# cli.nim
import parseopt, strutils

var p = initOptParser()
var filename = ""
var verbose = false

while true:
  p.next()
  case p.kind
  of cmdEnd: break
  of cmdShortOption, cmdLongOption:
    if p.key == "v" or p.key == "verbose":
      verbose = true
    if p.key == "f" or p.key == "file":
      filename = p.val
  of cmdArgument:
    filename = p.key

if verbose:
  echo "Processing file: ", filename
```

### JSON Processing
```nim
# json_example.nim
import json, httpclient

var client = newHttpClient()
var response = client.getContent("https://api.example.com/data")
var data = parseJson(response)

for item in data["items"]:
  echo item["name"].getStr()
```

### CI/CD Compilation
```yaml
# Build Nim application in CI
- name: Install Nim
  preset: nim

- name: Install dependencies
  shell: nimble install -y
  cwd: /app

- name: Run tests
  shell: nim c -r tests/test_all.nim
  cwd: /app

- name: Build release binary
  shell: |
    nim c -d:release \
      --opt:size \
      --threads:on \
      --app:console \
      src/main.nim
  cwd: /app

- name: Strip binary (reduce size)
  shell: strip main
  cwd: /app/src
```

## Package Management with Nimble

```bash
# Initialize new package
nimble init mypackage

# Install package
nimble install package_name

# Install dependencies from .nimble file
nimble install -y

# Search for packages
nimble search json

# List installed packages
nimble list -i

# Update package
nimble install package_name@#head
```

## Common Compilation Flags

```bash
# Release build (optimized)
nim c -d:release main.nim

# Debug build with symbols
nim c --debugger:native main.nim

# Optimize for size
nim c -d:release --opt:size main.nim

# Enable threads
nim c --threads:on main.nim

# Static linking
nim c --passL:-static main.nim

# Cross-compile for Windows (from Linux)
nim c -d:mingw main.nim
```

## Project Structure

```
myproject/
├── myproject.nimble      # Package definition
├── src/
│   └── main.nim          # Entry point
├── tests/
│   └── test_all.nim      # Tests
├── config.nims           # Build configuration
└── README.md
```

## Testing

```nim
# tests/test_math.nim
import unittest
import ../src/math

suite "Math operations":
  test "addition":
    check add(2, 3) == 5

  test "subtraction":
    check sub(5, 3) == 2

# Run tests
# nim c -r tests/test_math.nim
```

## Agent Use

- Build high-performance CLI tools with Python-like syntax
- Create system utilities with minimal resource usage
- Compile cross-platform binaries from single codebase
- Develop web backends with async I/O
- Write performance-critical components for larger systems
- Generate optimized binaries for embedded systems

## Troubleshooting

### Compiler not found
```bash
# Verify installation
which nim
nim --version

# Add to PATH
export PATH=$PATH:$HOME/.nimble/bin
```

### Dependencies not found
```bash
# Install nimble dependencies
nimble install -y

# Clear nimble cache
rm -rf ~/.nimble/pkgs_temp
```

### Compilation errors
```bash
# Use verbose output
nim c --verbosity:2 main.nim

# Check hints and warnings
nim c --hints:on --warnings:on main.nim
```

## Uninstall

```yaml
- preset: nim
  with:
    state: absent
```

## Resources

- Official docs: https://nim-lang.org/documentation.html
- Tutorial: https://nim-lang.org/docs/tut1.html
- Package directory: https://nimble.directory/
- GitHub: https://github.com/nim-lang/Nim
- Search: "nim programming tutorial", "nim examples", "nimble packages"

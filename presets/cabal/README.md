# Cabal - Haskell Build Tool

Build system and package manager for Haskell projects, part of the Haskell toolchain.

## Quick Start
```yaml
- preset: cabal
```

## Features
- **Package management**: Install and manage Haskell libraries
- **Build system**: Compile Haskell projects with dependencies
- **Project structure**: Standard layout for Haskell applications
- **Version resolution**: Automatic dependency resolution
- **Sandboxing**: Isolated build environments
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Check version
cabal --version

# Initialize new project
cabal init

# Update package list
cabal update

# Build project
cabal build

# Run executable
cabal run

# Install dependencies
cabal install --only-dependencies

# Clean build artifacts
cabal clean
```

## Project Management
```bash
# Create new project
mkdir myproject && cd myproject
cabal init --interactive

# Build and run
cabal build
cabal run myproject

# Run tests
cabal test

# Generate documentation
cabal haddock

# Install package globally
cabal install mypackage
```

## Configuration
```bash
# Global config
~/.cabal/config

# Project config
cabal.project

# Package definition
mypackage.cabal
```

## Real-World Examples

### CI/CD Pipeline
```yaml
- name: Install Cabal
  preset: cabal

- name: Update package list
  shell: cabal update

- name: Install dependencies
  shell: cabal install --only-dependencies
  cwd: /app

- name: Build project
  shell: cabal build
  cwd: /app

- name: Run tests
  shell: cabal test
  cwd: /app
```

### Development Environment
```yaml
- name: Setup Haskell development
  preset: cabal

- name: Install common tools
  shell: cabal install hlint stylish-haskell haskell-language-server
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ✅ Windows (chocolatey)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Build and test Haskell projects in CI/CD
- Manage Haskell dependencies automatically
- Set up development environments
- Package Haskell applications for distribution
- Run automated Haskell project builds


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install cabal
  preset: cabal

- name: Use cabal in automation
  shell: |
    # Custom configuration here
    echo "cabal configured"
```
## Uninstall
```yaml
- preset: cabal
  with:
    state: absent
```

## Resources
- Official site: https://www.haskell.org/cabal/
- User Guide: https://cabal.readthedocs.io/
- Hackage (packages): https://hackage.haskell.org/
- Search: "cabal tutorial", "cabal haskell", "cabal project setup"

# opam - OCaml Package Manager

Source-based package manager for OCaml. Install libraries, manage compiler versions, and create isolated development environments.

## Quick Start
```yaml
- preset: opam
```

## Features
- **Compiler management**: Install and switch between OCaml versions
- **Package installation**: Access 3,000+ OCaml packages
- **Switch isolation**: Separate environments for different projects
- **Source-based**: Builds packages from source with full control
- **Dependency resolution**: Automatic handling of package dependencies
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# Initialize opam
opam init

# List available OCaml versions
opam switch list-available

# Create new switch with specific OCaml version
opam switch create 5.1.0

# Install packages
opam install dune utop merlin

# Search for packages
opam search <keyword>

# Show package info
opam show <package>

# List installed packages
opam list

# Update package list
opam update

# Upgrade installed packages
opam upgrade
```

## Advanced Configuration
```yaml
# Install opam (default)
- preset: opam

# Uninstall opam
- preset: opam
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ BSD (pkg)

## Configuration
- **Root directory**: `~/.opam/`
- **Config file**: `~/.opam/config`
- **Switches**: `~/.opam/<switch-name>/`
- **Repository**: Default package repository at opam.ocaml.org

## Compiler Management
```bash
# List available compilers
opam switch list-available

# Create switch with specific compiler
opam switch create myproject 5.1.0

# Create local switch in project directory
cd myproject && opam switch create . 5.0.0

# Switch to different compiler
opam switch 5.1.0

# Show current switch
opam switch show

# Remove a switch
opam switch remove myproject
```

## Package Management
```bash
# Install package
opam install core async lwt

# Install package with version
opam install dune.3.10.0

# Remove package
opam remove <package>

# Pin package to specific version
opam pin add <package> <version>

# Show package dependencies
opam show -f depends <package>

# Show reverse dependencies
opam show -f depopts <package>
```

## Project Workflow
```bash
# Initialize new OCaml project
mkdir myproject && cd myproject

# Create local switch
opam switch create . 5.1.0

# Install development tools
opam install dune utop merlin ocaml-lsp-server

# Install project dependencies
opam install core async yojson

# Activate switch in current shell
eval $(opam env)

# Install dependencies from dune-project
opam install . --deps-only
```

## Repository Management
```bash
# List repositories
opam repository list

# Add custom repository
opam repository add <name> <url>

# Remove repository
opam repository remove <name>

# Update repository indexes
opam update
```

## Real-World Examples

### Development Environment Setup
```yaml
- name: Install OCaml development environment
  preset: opam
  become: true

- name: Initialize opam
  shell: opam init -y --bare

- name: Create project switch
  shell: opam switch create myproject 5.1.0

- name: Install dev tools
  shell: opam install -y dune utop merlin ocaml-lsp-server
```

### CI/CD Pipeline
```bash
# Cache opam root for faster builds
export OPAMROOT=$CI_PROJECT_DIR/.opam
opam init -y --bare --disable-sandboxing
opam switch create . 5.1.0
opam install . --deps-only -y
eval $(opam env)
dune build
dune runtest
```

### Multi-Version Testing
```bash
# Test package across OCaml versions
for version in 4.14.0 5.0.0 5.1.0; do
  opam switch create test-$version $version
  opam install . --deps-only -y
  eval $(opam env --switch=test-$version)
  dune test
done
```

## Agent Use
- Automated OCaml environment setup
- CI/CD pipeline configuration
- Multi-version compatibility testing
- Dependency management in build scripts
- Package release automation
- Development environment provisioning

## Troubleshooting

### Installation fails
Update opam repository:
```bash
opam update
opam upgrade
```

### Switch activation issues
Ensure environment is loaded:
```bash
eval $(opam env)
# Add to shell profile for persistence
echo 'eval $(opam env)' >> ~/.bashrc
```

### Build failures
Update system dependencies:
```bash
opam depext <package>  # Install system dependencies
```

### Permission errors
Run opam commands as user, not root. Only use sudo for system package installation.

## Uninstall
```yaml
- preset: opam
  with:
    state: absent
```

## Resources
- Official docs: https://opam.ocaml.org/doc/
- Package repository: https://opam.ocaml.org/packages/
- GitHub: https://github.com/ocaml/opam
- Search: "opam tutorial", "opam getting started", "ocaml opam guide"

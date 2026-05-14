# Hatch - Modern Python Project Manager

Modern, standards-based Python project manager with built-in versioning, environment management, and publishing.

## Quick Start
```yaml
- preset: hatch
```

## Features
- **Project scaffolding**: Create new Python projects with best practices
- **Environment management**: Isolated virtual environments per project
- **Build backend**: PEP 517/518 compliant build system
- **Version management**: Semantic versioning with automatic bumping
- **Testing**: Integrated test runner with matrix testing
- **Publishing**: Seamless PyPI publishing workflow
- **Cross-platform**: Linux, macOS, and Windows support

## Basic Usage
```bash
# Create new project
hatch new my-project
cd my-project

# Create and enter environment
hatch shell

# Run command in environment
hatch run python script.py
hatch run pytest

# Run tests
hatch test

# Build package
hatch build

# Publish to PyPI
hatch publish

# Version management
hatch version patch  # 1.0.0 -> 1.0.1
hatch version minor  # 1.0.1 -> 1.1.0
hatch version major  # 1.1.0 -> 2.0.0
```

## Configuration
- **Config file**: `pyproject.toml` (project root)
- **Global config**: `~/.config/hatch/config.toml`
- **Environments**: `.hatch/` (project directory)

## Real-World Examples

### Initialize New Python Package
```bash
# Create package with CLI
hatch new --cli my-cli-tool
cd my-cli-tool

# Project structure created:
# my-cli-tool/
#   ├── pyproject.toml
#   ├── README.md
#   ├── src/
#   │   └── my_cli_tool/
#   │       ├── __init__.py
#   │       └── cli.py
#   └── tests/
#       └── __init__.py
```

### Multi-Environment Testing
```toml
# pyproject.toml
[tool.hatch.envs.test]
dependencies = [
  "pytest",
  "pytest-cov",
]

[[tool.hatch.envs.test.matrix]]
python = ["3.9", "3.10", "3.11", "3.12"]
```

```bash
# Run tests across all Python versions
hatch run test:pytest
```

### Development Workflow
```bash
# Enter development environment
hatch shell

# Install dependencies
hatch env create

# Run linters
hatch run lint:check

# Run formatters
hatch run lint:format

# Run type checking
hatch run lint:typing

# Run tests with coverage
hatch run test:cov
```

### CI/CD Publishing
```yaml
- name: Build Python package
  shell: hatch build

- name: Verify build artifacts
  shell: ls -lh dist/

- name: Publish to PyPI
  shell: hatch publish --user __token__ --auth ${{ pypi_token }}
  when: tag_created
```

### Semantic Version Bumping
```bash
# Bump patch version and create git tag
hatch version patch
git tag v$(hatch version)
git push origin v$(hatch version)

# Pre-release versions
hatch version rc  # 1.0.0 -> 1.0.0rc0
hatch version alpha  # 1.0.0 -> 1.0.0a0
```

## Project Configuration

### Basic pyproject.toml
```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "my-package"
version = "0.1.0"
description = "My awesome package"
authors = [
  {name = "Your Name", email = "you@example.com"},
]
dependencies = [
  "requests>=2.28.0",
]
requires-python = ">=3.9"

[project.optional-dependencies]
dev = [
  "pytest>=7.0.0",
  "black>=23.0.0",
  "ruff>=0.1.0",
]

[tool.hatch.envs.default]
dependencies = [
  "pytest",
  "pytest-cov",
]

[tool.hatch.envs.default.scripts]
test = "pytest"
cov = "pytest --cov-report=term-missing --cov=my_package"
```

## Agent Use
- Automate Python package creation and scaffolding
- Manage multi-version testing in CI/CD pipelines
- Standardize project structure across teams
- Automate version bumping and release workflows
- Build and publish packages to PyPI
- Manage development environments programmatically

## Advanced Configuration
```yaml
- preset: hatch
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Hatch |

## Troubleshooting

### Environment Issues
```bash
# Remove all environments
hatch env prune

# Recreate default environment
hatch env create

# List all environments
hatch env show
```

### Build Failures
```bash
# Clean build artifacts
hatch clean

# Rebuild package
hatch build --clean

# Verbose build output
hatch build -v
```

### Version Conflicts
```bash
# Show current version
hatch version

# Show dependency tree
hatch dep show tree

# Check for outdated dependencies
hatch dep show updates
```

## Platform Support
- ✅ Linux (pip, pipx)
- ✅ macOS (Homebrew, pip)
- ✅ Windows (pip, pipx)

## Uninstall
```yaml
- preset: hatch
  with:
    state: absent
```

## Resources
- Official docs: https://hatch.pypa.io/
- GitHub: https://github.com/pypa/hatch
- PyPA: https://www.pypa.io/
- Search: "hatch python tutorial", "hatch project management", "hatch vs poetry"

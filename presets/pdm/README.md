# PDM - Python Development Master

Modern Python package and dependency manager with PEP 582 support. Fast, reliable, standards-compliant alternative to pip and virtualenv.

## Quick Start
```yaml
- preset: pdm
```

## Features
- **PEP 582**: No virtualenv needed, packages in `__pypackages__`
- **Fast**: Parallel installation with caching
- **Lock file**: Reproducible builds with pdm.lock
- **Standards-compliant**: Uses pyproject.toml (PEP 621)
- **Plugin system**: Extensible architecture
- **Python version management**: Install and switch Python versions

## Basic Usage
```bash
# Initialize project
pdm init

# Add dependencies
pdm add requests
pdm add -d pytest  # Dev dependency

# Install dependencies
pdm install

# Run commands
pdm run python script.py
pdm run pytest

# Update dependencies
pdm update

# Show dependencies
pdm list
pdm list --tree

# Remove package
pdm remove requests

# Export requirements.txt
pdm export -o requirements.txt
```

## Advanced Configuration

### Initialize with template
```yaml
- preset: pdm
  become: true

- name: Create new project
  shell: pdm init --non-interactive
  cwd: /opt/myapp

- name: Add dependencies
  shell: |
    pdm add flask gunicorn
    pdm add -d pytest black mypy
  cwd: /opt/myapp
```

### Production build
```yaml
- name: Install PDM
  preset: pdm

- name: Install production dependencies
  shell: pdm install --prod --no-editable
  cwd: /opt/myapp

- name: Export requirements
  shell: pdm export -o requirements.txt --without-hashes
  cwd: /opt/myapp
```

### Multi-environment setup
```yaml
- name: Setup dev environment
  shell: |
    pdm add -G test pytest pytest-cov
    pdm add -G docs sphinx sphinx-rtd-theme
    pdm add -G lint black mypy ruff
  cwd: /opt/myapp

- name: Install with specific group
  shell: pdm install -G test
  cwd: /opt/myapp
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove PDM |

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, pip)
- ✅ Windows (pip, scoop)
- ✅ Cross-platform via pipx

## Configuration
- **Config file**: `~/.config/pdm/config.toml` (Linux), `~/Library/Application Support/pdm/config.toml` (macOS)
- **Cache**: `~/.cache/pdm/` (Linux), `~/Library/Caches/pdm/` (macOS)
- **Project config**: `pyproject.toml`, `pdm.lock`, `pdm.toml`
- **Packages**: `__pypackages__/` (PEP 582 mode)

## Real-World Examples

### Flask application
```bash
# Initialize project
pdm init --non-interactive --python 3.11

# Add dependencies
pdm add flask gunicorn redis celery

# Add dev tools
pdm add -d pytest black mypy flask-testing

# Run development server
pdm run flask run

# Run tests
pdm run pytest
```

### CI/CD pipeline
```yaml
- name: Install PDM
  preset: pdm

- name: Cache PDM packages
  shell: mkdir -p ~/.cache/pdm

- name: Install dependencies
  shell: pdm install --prod
  cwd: /app

- name: Run tests
  shell: pdm run pytest --cov
  cwd: /app

- name: Build package
  shell: pdm build
  cwd: /app
```

### Monorepo setup
```bash
# Project structure
# monorepo/
# ├── packages/
# │   ├── api/
# │   │   ├── pyproject.toml
# │   │   └── pdm.lock
# │   └── workers/
# │       ├── pyproject.toml
# │       └── pdm.lock

# Install all projects
for dir in packages/*/; do
  cd "$dir"
  pdm install
  cd ../..
done
```

## Project Structure
```
myproject/
├── pyproject.toml       # Project config and dependencies
├── pdm.lock            # Locked versions
├── pdm.toml            # PDM-specific settings (optional)
├── src/
│   └── mypackage/
│       └── __init__.py
├── tests/
│   └── test_main.py
└── __pypackages__/     # PEP 582 packages (if enabled)
    └── 3.11/
        └── lib/
```

## Configuration Options
```toml
# ~/.config/pdm/config.toml

[global]
# Use PEP 582 instead of virtualenv
use_venv = false

# Parallel downloads
parallel_install = true

# Cache directory
cache_dir = "~/.cache/pdm"

[pypi]
# Custom index
url = "https://pypi.org/simple"
verify_ssl = true

[install]
# Default dependency groups
default_groups = ["default", "dev"]
```

## Python Version Management
```bash
# Install Python version
pdm python install 3.11

# List installed Pythons
pdm python list

# Use specific Python
pdm use 3.11
pdm use python3.10

# Show current Python
pdm info --python
```

## Scripts in pyproject.toml
```toml
[tool.pdm.scripts]
start = "flask run"
test = "pytest tests/"
lint = "black . && mypy src/"
dev = "flask run --debug"
prod = "gunicorn 'app:create_app()'"
```

```bash
# Run scripts
pdm run start
pdm run test
pdm run lint
```

## Dependency Groups
```toml
[project.optional-dependencies]
test = ["pytest>=7.0", "pytest-cov>=4.0"]
docs = ["sphinx>=5.0", "sphinx-rtd-theme"]
dev = ["black", "mypy", "ruff"]
```

```bash
# Install specific group
pdm install -G test
pdm install -G docs

# Install all groups
pdm install --dev

# Install production only
pdm install --prod
```

## Lock File Management
```bash
# Update lock file
pdm lock

# Update specific package
pdm update requests

# Update all packages
pdm update

# Show outdated packages
pdm outdated

# Sync environment with lock file
pdm sync
```

## Environment Variables
```bash
# Disable PEP 582
export PDM_USE_VENV=1

# Custom cache directory
export PDM_CACHE_DIR=/tmp/pdm-cache

# Skip SSL verification (not recommended)
export PDM_NO_VERIFY_SSL=1

# Parallel downloads
export PDM_PARALLEL_INSTALL=4
```

## Agent Use
- Automated Python dependency management
- CI/CD pipeline integration
- Reproducible builds with lock files
- Multi-project workspace management
- Python version management
- Generate requirements.txt for legacy systems
- Package publishing to PyPI

## Troubleshooting

### PEP 582 not working
```bash
# Enable PEP 582 support
pdm --pep582

# Add to shell profile
eval "$(pdm --pep582)"

# Or manually add to PATH
export PYTHONPATH="__pypackages__/3.11/lib:$PYTHONPATH"
```

### Slow installation
```bash
# Enable parallel installation
pdm config install.parallel true

# Clear cache
pdm cache clear
```

### Lock file conflicts
```bash
# Regenerate lock file
pdm lock --refresh

# Update specific package
pdm lock --update-reuse requests
```

### Dependency resolution errors
```bash
# Show dependency tree
pdm list --tree

# Resolve conflicts
pdm update --unconstrained
```

## Migration

### From pip
```bash
# Import from requirements.txt
pdm import requirements.txt
```

### From Poetry
```bash
# Convert pyproject.toml
pdm import pyproject.toml
```

### From Pipenv
```bash
# Import Pipfile
pdm import Pipfile
```

## Best Practices
- **Use lock files**: Commit pdm.lock for reproducibility
- **Dependency groups**: Separate dev, test, docs dependencies
- **PEP 582**: Use `__pypackages__` instead of virtualenvs
- **Scripts**: Define common tasks in pyproject.toml
- **Cache**: Enable parallel installation for speed
- **Version pins**: Use `~=` for compatible updates
- **Security**: Regularly run `pdm update` for patches
- **CI/CD**: Use `pdm install --prod` for deployments

## Comparison

| Feature | PDM | Poetry | Pipenv | pip |
|---------|-----|--------|--------|-----|
| Lock file | ✅ | ✅ | ✅ | ❌ |
| PEP 582 | ✅ | ❌ | ❌ | ❌ |
| Speed | Fast | Medium | Slow | Fast |
| Standards | PEP 621 | Custom | Custom | Basic |
| Python mgmt | ✅ | ❌ | ❌ | ❌ |

## Uninstall
```yaml
- preset: pdm
  with:
    state: absent
```

**Note**: This removes PDM but keeps your projects and `__pypackages__` directories.

## Resources
- Official docs: https://pdm.fming.dev/
- GitHub: https://github.com/pdm-project/pdm
- PEP 582: https://www.python.org/dev/peps/pep-0582/
- Search: "pdm python tutorial", "pdm vs poetry", "pdm pep 582"

# Rye - Rust-Powered Python Package Manager

Modern Python package and project manager written in Rust. Fast, reliable dependency management with built-in Python version management.

## Quick Start
```yaml
- preset: rye
```

## Features
- **Built-in Python installer**: No system Python needed
- **Fast dependency resolution**: Rust-based solver, 10-100x faster than pip
- **Lockfiles**: Reproducible builds with `requirements.lock`
- **Workspace support**: Monorepo with multiple packages
- **Tool management**: Install CLI tools in isolated environments
- **Cross-platform**: Linux, macOS, Windows
- **pyproject.toml native**: Modern Python packaging standard

## Basic Usage
```bash
# Create new project
rye init myproject
cd myproject

# Add dependency
rye add requests

# Install dependencies
rye sync

# Run Python
rye run python script.py

# Run scripts defined in pyproject.toml
rye run dev
```

## Advanced Configuration
```yaml
# Install Rye (default)
- preset: rye

# Uninstall Rye
- preset: rye
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Intel and Apple Silicon)
- ✅ Windows
- ✅ BSD

## Configuration
- **Rye home**: `~/.rye/` (Linux/macOS), `%USERPROFILE%\.rye\` (Windows)
- **Python installations**: `~/.rye/py/`
- **Tools**: `~/.rye/tools/`
- **Shims**: `~/.rye/shims/` (added to PATH)
- **Config**: `~/.rye/config.toml`
- **Project config**: `pyproject.toml`

## Project Management
```bash
# Create new project
rye init myproject
rye init --py 3.12 myproject  # Specific Python version

# Create library project
rye init --lib mylib

# Add dependencies
rye add requests
rye add pytest --dev  # Development dependency
rye add "flask>=2.0,<3.0"  # Version constraint

# Remove dependency
rye remove requests

# Sync project (install all dependencies)
rye sync

# Sync without dev dependencies
rye sync --no-dev
```

## Python Version Management
```bash
# List available Python versions
rye toolchain list

# Install Python version
rye toolchain install 3.12
rye toolchain install 3.11.5

# Pin Python version for project
rye pin 3.12

# Use specific Python
rye run python --version

# Fetch Python (download without installing)
rye fetch 3.12
```

## Dependency Management
```bash
# Add package
rye add numpy
rye add pandas scipy matplotlib

# Add from Git
rye add mypackage --git https://github.com/user/repo

# Add with extras
rye add requests[security]

# Add development dependencies
rye add pytest pytest-cov --dev

# Add optional dependencies
rye add sphinx --optional docs

# Update dependencies
rye sync --update-all

# Update specific package
rye add requests --sync
```

## Running Code
```bash
# Run Python
rye run python script.py

# Run module
rye run python -m mymodule

# Run custom scripts (from pyproject.toml)
rye run dev
rye run test
rye run lint

# Run with environment
rye run --env-file .env python app.py
```

## Tool Management
```bash
# Install global tool
rye install black
rye install ruff
rye install mypy

# List installed tools
rye toolchain list

# Uninstall tool
rye uninstall black

# Run tool directly
black .
ruff check .
```

## Workspaces
```bash
# Create workspace
mkdir myworkspace
cd myworkspace
rye init --workspace

# Add member packages
rye init --lib packages/core
rye init --lib packages/utils

# Configure workspace in pyproject.toml
[tool.rye.workspace]
members = ["packages/*"]

# Sync workspace
rye sync
```

## Build and Publish
```bash
# Build package
rye build

# Build wheel only
rye build --wheel

# Build sdist only
rye build --sdist

# Publish to PyPI
rye publish

# Publish to custom index
rye publish --repository-url https://test.pypi.org/legacy/
```

## Lockfiles
```bash
# Generate lockfile
rye lock

# Update lockfile
rye lock --update-all

# Update specific package in lock
rye add requests --sync

# Install from lockfile
rye sync

# Export requirements
rye lock --export requirements.txt
```

## Configuration
```toml
# ~/.rye/config.toml
[default]
# Use specific PyPI index
index-url = "https://pypi.org/simple/"

# Add extra index
extra-index-urls = ["https://my-index.com/simple/"]

# Configure behavior
autosync = true
generate-lockfile = true

# Default Python
default-toolchain = "cpython@3.12"
```

## Project Configuration
```toml
# pyproject.toml
[project]
name = "myapp"
version = "0.1.0"
description = "My application"
requires-python = ">= 3.11"

dependencies = [
    "requests>=2.31.0",
    "flask>=3.0.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.4.0",
    "black>=23.7.0",
]

[project.scripts]
myapp = "myapp.cli:main"

[tool.rye]
managed = true
dev-dependencies = [
    "pytest>=7.4.0",
]

[tool.rye.scripts]
dev = "flask --app myapp run --debug"
test = "pytest"
lint = "ruff check ."
```

## Real-World Examples

### New Web Application
```bash
# Initialize project
rye init myapp
cd myapp

# Pin Python version
rye pin 3.12

# Add dependencies
rye add flask sqlalchemy
rye add pytest pytest-cov --dev

# Configure scripts
cat >> pyproject.toml << 'EOF'
[tool.rye.scripts]
dev = "flask --app app run --debug"
test = "pytest tests/"
EOF

# Run development server
rye sync
rye run dev
```

### Data Science Project
```bash
# Create project
rye init data-analysis
cd data-analysis

# Add data science stack
rye add numpy pandas matplotlib seaborn
rye add jupyter scikit-learn
rye add pytest --dev

# Launch Jupyter
rye run jupyter notebook

# Run analysis
rye run python analysis.py
```

### CLI Tool Development
```bash
# Create library project
rye init --lib mytool
cd mytool

# Add dependencies
rye add click rich
rye add pytest --dev

# Configure CLI entry point
cat >> pyproject.toml << 'EOF'
[project.scripts]
mytool = "mytool.cli:main"
EOF

# Build and install
rye build
rye install .

# Use tool
mytool --help
```

### Monorepo Workspace
```bash
# Create workspace
mkdir myworkspace
cd myworkspace

# Initialize packages
rye init --lib packages/core
rye init --lib packages/api
rye init packages/app

# Configure workspace
cat > pyproject.toml << 'EOF'
[tool.rye.workspace]
members = ["packages/*"]
EOF

# Add inter-package dependencies
cd packages/api
rye add core --path ../core

# Sync entire workspace
cd ../..
rye sync
```

## CI/CD Integration
```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Rye
        preset: rye

      - name: Sync dependencies
        shell: rye sync

      - name: Run tests
        shell: rye run pytest

      - name: Check formatting
        shell: rye run black --check .
```

## Troubleshooting

### Rye command not found
Add Rye shims to PATH:
```bash
# Linux/macOS
echo 'source "$HOME/.rye/env"' >> ~/.bashrc
source ~/.bashrc

# Or manually
export PATH="$HOME/.rye/shims:$PATH"
```

### Python version not found
Install Python version:
```bash
# List available versions
rye toolchain list

# Install specific version
rye toolchain install 3.12
rye toolchain install cpython@3.11.5
```

### Dependency resolution fails
Clear cache and retry:
```bash
rm -rf ~/.rye/cache
rye sync
```

### Slow dependency resolution
Use pre-resolved lockfile:
```bash
# Commit requirements.lock to version control
rye lock
git add requirements.lock

# CI uses lockfile (fast)
rye sync
```

## Migration

### From pip + virtualenv
```bash
# Instead of:
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Use:
rye sync  # One command
```

### From Poetry
```bash
# Rye can read pyproject.toml
# Dependencies in [project] section work
# Convert Poetry-specific fields if needed
rye sync
```

### From Pipenv
```bash
# Export Pipenv dependencies
pipenv requirements > requirements.txt

# Import to Rye
rye init
# Add dependencies from requirements.txt to pyproject.toml
rye sync
```

## Best Practices
- Commit `requirements.lock` for reproducible builds
- Use `rye pin` to lock Python version per project
- Define common tasks in `[tool.rye.scripts]`
- Use `--dev` for testing/linting dependencies
- Keep `pyproject.toml` minimal, let Rye manage versions
- Use workspaces for monorepos
- Run `rye sync` after pulling changes

## Agent Use
- Set up reproducible Python environments for CI/CD
- Automate dependency updates and security patches
- Manage Python versions across multiple projects
- Create isolated environments for testing
- Build and publish Python packages automatically
- Provision development environments consistently
- Validate dependency compatibility across Python versions

## Uninstall
```yaml
- preset: rye
  with:
    state: absent
```

Manual uninstall:
```bash
rye self uninstall
# Remove from shell config:
# Remove 'source "$HOME/.rye/env"' line
```

## Resources
- Official site: https://rye-up.com/
- Documentation: https://rye-up.com/guide/
- GitHub: https://github.com/mitsuhiko/rye
- Discord: https://discord.gg/drbkcdtSbg
- Search: "rye python", "rye vs poetry", "rye tutorial"

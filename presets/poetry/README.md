# Poetry - Python Dependency Management

Python packaging and dependency management made easy. Manages virtual environments, dependencies, and publishing with a single pyproject.toml file.

## Quick Start
```yaml
- preset: poetry
```

## Features
- **Dependency resolution**: Deterministic dependency resolution
- **Virtual environments**: Automatic virtual environment management
- **Lock files**: `poetry.lock` for reproducible installs
- **pyproject.toml**: Single configuration file (PEP 518)
- **Publishing**: Built-in package publishing to PyPI
- **Version management**: Automatic semantic versioning
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Create new project
poetry new myproject

# Initialize existing project
poetry init

# Add dependency
poetry add requests

# Add dev dependency
poetry add --group dev pytest

# Install dependencies
poetry install

# Update dependencies
poetry update

# Show installed packages
poetry show

# Run command in virtual environment
poetry run python script.py

# Activate virtual environment
poetry shell
```

## Advanced Configuration
```yaml
# Install Poetry (default)
- preset: poetry

# Uninstall Poetry
- preset: poetry
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (installer script)
- ✅ macOS (installer script, Homebrew)
- ✅ Windows (installer script)

## Configuration
- **Config file**: `~/.config/pypoetry/config.toml`
- **Cache**: `~/.cache/pypoetry/` (Linux), `~/Library/Caches/pypoetry/` (macOS)
- **Virtual envs**: `.venv/` in project (default) or centralized
- **Project config**: `pyproject.toml` in project root

## Project Setup
```bash
# Create new project with structure
poetry new my-package

# Creates:
# my-package/
# ├── my_package/
# │   └── __init__.py
# ├── tests/
# │   └── __init__.py
# ├── pyproject.toml
# └── README.md

# Initialize in existing directory
cd existing-project
poetry init

# Follow interactive prompts
```

## Dependency Management
```bash
# Add package
poetry add requests

# Add specific version
poetry add requests==2.28.0
poetry add "requests>=2.28,<3.0"

# Add dev dependency
poetry add --group dev pytest black flake8

# Add optional dependency
poetry add --optional redis

# Remove package
poetry remove requests

# Update specific package
poetry update requests

# Update all packages
poetry update

# Show outdated packages
poetry show --outdated

# Show dependency tree
poetry show --tree
```

## Virtual Environments
```bash
# Create/use virtual environment
poetry install

# Activate shell
poetry shell

# Run command without activating
poetry run python script.py
poetry run pytest

# Show virtual environment info
poetry env info

# List virtual environments
poetry env list

# Remove virtual environment
poetry env remove python3.11

# Use specific Python version
poetry env use python3.11
poetry env use /usr/bin/python3.11
```

## Lock Files
```bash
# Generate lock file
poetry lock

# Install from lock file
poetry install

# Update lock file
poetry lock --no-update

# Export requirements.txt
poetry export -f requirements.txt -o requirements.txt

# Export without hashes
poetry export -f requirements.txt -o requirements.txt --without-hashes

# Export dev dependencies
poetry export --with dev -o requirements-dev.txt
```

## Building and Publishing
```bash
# Build package
poetry build

# Creates:
# dist/
# ├── my_package-0.1.0-py3-none-any.whl
# └── my_package-0.1.0.tar.gz

# Configure PyPI credentials
poetry config pypi-token.pypi my-token

# Publish to PyPI
poetry publish

# Build and publish
poetry publish --build

# Publish to test PyPI
poetry publish -r testpypi
```

## Configuration Management
```bash
# List configuration
poetry config --list

# Set config value
poetry config virtualenvs.in-project true

# Get config value
poetry config virtualenvs.in-project

# Unset config value
poetry config virtualenvs.in-project --unset

# Configure repository
poetry config repositories.private https://pypi.example.com/simple/
```

## Real-World Examples

### Django Project
```yaml
- name: Install Poetry
  preset: poetry

- name: Create Django project
  shell: |
    poetry new mysite
    cd mysite
    poetry add django psycopg2-binary
    poetry add --group dev pytest-django black
    poetry run django-admin startproject config .
```

### FastAPI Microservice
```bash
# Initialize project
poetry init

# Add dependencies
poetry add fastapi uvicorn[standard] pydantic

# Add dev dependencies
poetry add --group dev pytest pytest-cov httpx

# Create main.py
cat > main.py << 'EOF'
from fastapi import FastAPI
app = FastAPI()

@app.get("/")
def read_root():
    return {"Hello": "World"}
EOF

# Run server
poetry run uvicorn main:app --reload
```

### CLI Tool Distribution
```toml
# pyproject.toml
[tool.poetry]
name = "my-cli"
version = "1.0.0"

[tool.poetry.scripts]
mycli = "my_cli.main:cli"

[tool.poetry.dependencies]
python = "^3.8"
click = "^8.0"
```

### Data Science Project
```bash
# Add data science stack
poetry add pandas numpy matplotlib scikit-learn jupyter

# Add specific versions for reproducibility
poetry add "pandas==1.5.3" "numpy==1.24.2"

# Export for Conda users
poetry export -f requirements.txt -o requirements.txt
```

## pyproject.toml Configuration
```toml
[tool.poetry]
name = "myproject"
version = "0.1.0"
description = "My awesome project"
authors = ["Your Name <you@example.com>"]
readme = "README.md"
homepage = "https://github.com/user/repo"
repository = "https://github.com/user/repo"
keywords = ["python", "package"]
classifiers = [
    "Programming Language :: Python :: 3",
    "License :: OSI Approved :: MIT License",
]

[tool.poetry.dependencies]
python = "^3.8"
requests = "^2.28.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.0"
black = "^23.0"

[tool.poetry.scripts]
myapp = "myproject.cli:main"

[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"
```

## CI/CD Integration
```yaml
# GitHub Actions
- name: Install dependencies
  run: |
    curl -sSL https://install.python-poetry.org | python3 -
    poetry install

- name: Run tests
  run: poetry run pytest

- name: Build package
  run: poetry build
```

## Agent Use
- Python project bootstrapping
- Dependency management automation
- CI/CD pipeline integration
- Package distribution
- Virtual environment management
- Multi-project workspace setup
- Development environment standardization

## Troubleshooting

### Virtual environment not found
Create virtual environment:
```bash
poetry install
# Or explicitly:
poetry env use python3.11
```

### Dependency resolution conflicts
Update lock file:
```bash
poetry lock --no-update
# Or force update:
poetry update
```

### Slow dependency resolution
Clear cache:
```bash
poetry cache clear pypi --all
```

### Poetry command not found
Add to PATH:
```bash
export PATH="$HOME/.local/bin:$PATH"
# Add to ~/.bashrc or ~/.zshrc
```

## Uninstall
```yaml
- preset: poetry
  with:
    state: absent
```

## Resources
- Official docs: https://python-poetry.org/docs/
- GitHub: https://github.com/python-poetry/poetry
- PyPI: https://pypi.org/project/poetry/
- Search: "poetry python tutorial", "poetry dependency management", "poetry vs pipenv"

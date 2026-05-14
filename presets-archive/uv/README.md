# uv - Ultra-Fast Python Package Manager

Python package and project manager written in Rust. 10-100x faster than pip and pip-tools.

## Quick Start
```yaml
- preset: uv
```

## Features
- **Blazing Fast**: 10-100x faster than pip
- **Drop-in Replacement**: Compatible with pip interface
- **Dependency Resolution**: Fast and reliable solver
- **Virtual Environments**: Create venvs in milliseconds
- **Lockfile Support**: Reproducible installations with uv.lock
- **Cross-platform**: Linux, macOS, and Windows support

## Basic Usage
```bash
# Install package
uv pip install requests

# Install from requirements.txt
uv pip install -r requirements.txt

# Uninstall package
uv pip uninstall requests

# List installed packages
uv pip list

# Freeze dependencies
uv pip freeze > requirements.txt
```

## Virtual Environments
```bash
# Create virtual environment (instant)
uv venv

# Create with specific Python
uv venv --python 3.11

# Create in custom directory
uv venv .venv

# Activate (same as venv)
source .venv/bin/activate  # Linux/macOS
.venv\Scripts\activate     # Windows
```

## Project Management
```bash
# Initialize new project
uv init

# Add dependency
uv add requests
uv add "fastapi[all]"

# Add dev dependency
uv add --dev pytest

# Remove dependency
uv remove requests

# Install all dependencies
uv sync

# Update dependencies
uv sync --upgrade
```

## Real-World Examples

### Initialize New Project
```bash
# Create project
mkdir myproject && cd myproject
uv init
uv venv
source .venv/bin/activate

# Add dependencies
uv add fastapi uvicorn

# Run
uvicorn main:app --reload
```

### Docker Integration
```dockerfile
FROM python:3.11-slim

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Copy project files
COPY pyproject.toml uv.lock ./
COPY . .

# Install dependencies
RUN uv sync --frozen

CMD ["uv", "run", "python", "main.py"]
```

### CI/CD Pipeline
```yaml
# GitHub Actions
- name: Setup uv
  uses: astral-sh/setup-uv@v1

- name: Install dependencies
  run: uv sync

- name: Run tests
  run: uv run pytest
```

## Advanced Configuration
```yaml
- preset: uv
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove uv |

## Platform Support
- ✅ Linux (standalone installer or cargo)
- ✅ macOS (Homebrew or standalone installer)
- ✅ Windows (standalone installer or cargo)

## Performance Comparison
```bash
# pip install numpy (20s)
time pip install numpy

# uv pip install numpy (2s)
time uv pip install numpy

# 10x faster!
```

## Agent Use
- Accelerate CI/CD pipelines with 10-100x faster installs
- Reproducible builds with lockfiles
- Virtual environment creation in milliseconds
- Dependency resolution for complex requirements
- Docker image builds with faster layer caching
- Development workflow automation

## Uninstall
```yaml
- preset: uv
  with:
    state: absent
```

## Resources
- Official docs: https://docs.astral.sh/uv/
- GitHub: https://github.com/astral-sh/uv
- Search: "uv python package manager", "uv vs pip", "uv migration guide"

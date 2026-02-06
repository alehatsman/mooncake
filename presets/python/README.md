# Python/pyenv Preset

Install Python via pyenv (Python version manager) with support for multiple versions and virtual environments.

## Features

- ✅ Installs build dependencies automatically
- ✅ Installs pyenv (Python version manager)
- ✅ Installs specified Python version(s)
- ✅ Sets global Python version
- ✅ Supports multiple Python versions side-by-side
- ✅ Optional pyenv-virtualenv plugin for virtual environments
- ✅ Configures shell profiles automatically
- ✅ Cross-platform (Linux, macOS)

## Quick Start

```yaml
- preset: python
```

Installs Python 3.12.1 via pyenv with build dependencies.

## Basic Usage

After installation:
```bash
# Verify installation
python --version
pip --version

# Install packages
pip install requests pandas numpy

# Run Python
python
>>> import sys
>>> print(sys.version)
>>> exit()

# Run script
echo 'print("Hello, Python!")' > hello.py
python hello.py

# Check installed versions
pyenv versions

# Show current version
pyenv version
```

## Usage

### Install latest Python 3.12
```yaml
- name: Install Python
  preset: python
```

### Install specific Python version
```yaml
- name: Install Python 3.11
  preset: python
  with:
    version: "3.11.7"
```

### Install multiple Python versions
```yaml
- name: Install Python with multiple versions
  preset: python
  with:
    version: "3.12.1"
    additional_versions:
      - "3.11.7"
      - "3.10.13"
    install_virtualenv: true
```

### Uninstall
```yaml
- name: Remove Python and pyenv
  preset: python
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `"3.12.1"` | Python version to install |
| `set_global` | bool | `true` | Set as global Python version |
| `additional_versions` | array | `[]` | Other Python versions to install |
| `install_virtualenv` | bool | `true` | Install pyenv-virtualenv plugin |

## Platform Support

- ✅ Linux (apt, dnf, yum)
- ✅ macOS (Homebrew)
- ❌ Windows (use pyenv-win separately)

## What Gets Installed

1. **Build dependencies** - Required for compiling Python
   - Linux (apt): gcc, make, libssl-dev, zlib1g-dev, etc.
   - Linux (dnf/yum): gcc, make, zlib-devel, openssl-devel, etc.
   - macOS: openssl, readline, sqlite3, xz, zlib

2. **pyenv** - Python version manager
   - Installed to `~/.pyenv/`
   - Added to `~/.bashrc`, `~/.zshrc`, `~/.profile`

3. **Python** - Compiled from source
   - Installed to `~/.pyenv/versions/`
   - Multiple versions can coexist

4. **pyenv-virtualenv** (optional) - Virtual environment support
   - Plugin for creating isolated environments

## Common Use Cases

### Data Science Environment
```yaml
- name: Setup Python for data science
  preset: python
  with:
    version: "3.11.7"
    install_virtualenv: true
```

Then install packages:
```bash
pip install numpy pandas scikit-learn jupyter
```

### Web Development
```yaml
- name: Setup Python for Django
  preset: python
  with:
    version: "3.12.1"
    install_virtualenv: true
```

### Testing Multiple Versions
```yaml
- name: Install Python for CI testing
  preset: python
  with:
    version: "3.12.1"
    additional_versions:
      - "3.11.7"
      - "3.10.13"
      - "3.9.18"
```

## Post-Installation

After installation, restart your terminal or run:
```bash
source ~/.bashrc  # or ~/.zshrc
```

### Using pyenv
```bash
# List installed versions
pyenv versions

# Install another version
pyenv install 3.10.13

# Switch global version
pyenv global 3.11.7

# Set local version (creates .python-version)
pyenv local 3.12.1

# Show current version
pyenv version

# List available versions
pyenv install --list
```

### Using pyenv-virtualenv
```bash
# Create virtual environment
pyenv virtualenv 3.12.1 myproject

# Activate virtual environment
pyenv activate myproject

# Deactivate
pyenv deactivate

# List virtual environments
pyenv virtualenvs

# Delete virtual environment
pyenv uninstall myproject
```

### Verify Installation
```bash
python --version
pip --version
pyenv --version
```

### Project Setup Example
```bash
# Create project directory
mkdir myproject
cd myproject

# Set Python version for this directory
pyenv local 3.12.1

# Create virtual environment
pyenv virtualenv 3.12.1 myproject-env

# Auto-activate in this directory
echo "myproject-env" > .python-version

# Install dependencies
pip install -r requirements.txt
```

## Troubleshooting

### Python compilation fails
- Ensure build dependencies are installed
- On macOS with Apple Silicon, you may need:
  ```bash
  CFLAGS="-I$(brew --prefix openssl)/include" \
  LDFLAGS="-L$(brew --prefix openssl)/lib" \
  pyenv install 3.12.1
  ```

### Command not found: pyenv
- Restart your terminal or run: `source ~/.bashrc`
- Check that `~/.pyenv/bin` is in your PATH

### pip install fails
- Upgrade pip: `pip install --upgrade pip`
- Check Python version: `python --version`

## Advanced Configuration

### Performance Optimizations
```bash
# Use optimized Python build
PYTHON_CONFIGURE_OPTS="--enable-optimizations --with-lto" \
PYTHON_CFLAGS="-march=native -O3" \
pyenv install 3.12.1
```

### Custom Build Options
```bash
# With specific features
PYTHON_CONFIGURE_OPTS="--enable-shared --with-computed-gotos" \
pyenv install 3.12.1

# macOS with Homebrew OpenSSL
CFLAGS="-I$(brew --prefix openssl)/include" \
LDFLAGS="-L$(brew --prefix openssl)/lib" \
pyenv install 3.12.1
```

### Shell Optimization (Lazy Loading)
```bash
# Add to .zshrc for faster startup
pyenv() {
  unset -f pyenv
  export PYENV_ROOT="$HOME/.pyenv"
  export PATH="$PYENV_ROOT/bin:$PATH"
  eval "$(pyenv init -)"
  pyenv "$@"
}
```

### Global Python Tools
```bash
# Install tools once for all projects
pyenv global 3.12.1
pip install black isort pylint mypy pytest
```

### Project Isolation
```bash
# Each project has isolated environment
cd project-a
pyenv local 3.11.7
pyenv virtualenv 3.11.7 project-a-env
pyenv activate project-a-env

cd ../project-b
pyenv local 3.12.1
pyenv virtualenv 3.12.1 project-b-env
pyenv activate project-b-env
```

## Agent Use

Python + pyenv is essential for AI agent development:

### Agent Environment Setup
```yaml
# Install Python with common ML/AI packages
- preset: python
  with:
    version: "3.11.7"
    install_virtualenv: true

- name: Install AI libraries
  shell: |
    eval "$(pyenv init -)"
    pip install openai anthropic langchain transformers
```

### Multi-Version Testing
```yaml
# Test agent code on Python 3.9-3.12
- preset: python
  with:
    version: "3.12.1"
    additional_versions: ["3.11.7", "3.10.13", "3.9.18"]
```

### Isolated Agent Environments
```python
# Create isolated env for each agent
import subprocess

def setup_agent_env(agent_name, python_version):
    subprocess.run([
        "pyenv", "virtualenv", python_version, f"agent-{agent_name}"
    ])
    subprocess.run([
        "pyenv", "activate", f"agent-{agent_name}"
    ])
```

### Data Science Pipeline
```bash
# Setup for ML agents
pyenv virtualenv 3.11.7 ml-agent
pyenv activate ml-agent
pip install numpy pandas scikit-learn torch transformers
pip install jupyter notebook ipython
```

### Reproducible Environments
```bash
# Lock dependencies for agent deployment
pip freeze > requirements.txt

# Recreate exact environment
pyenv virtualenv 3.11.7 agent-prod
pyenv activate agent-prod
pip install -r requirements.txt
```

Benefits for agents:
- **Isolation** - Each agent has own environment
- **Reproducibility** - Lock exact Python + package versions
- **Multi-version** - Test agents on different Python versions
- **Local control** - No system Python conflicts
- **Fast switching** - Change Python version per directory

## Learn More

- [pyenv GitHub](https://github.com/pyenv/pyenv)
- [pyenv-virtualenv](https://github.com/pyenv/pyenv-virtualenv)
- [Python Documentation](https://docs.python.org/)
- [Real Python - pyenv Tutorial](https://realpython.com/intro-to-pyenv/)

## Resources

- **Python.org**: https://www.python.org
- **PyPI (Package Index)**: https://pypi.org
- **pyenv Releases**: https://github.com/pyenv/pyenv/releases
- **Build Dependencies**: https://github.com/pyenv/pyenv/wiki#suggested-build-environment
- **Virtual Environments**: https://docs.python.org/3/tutorial/venv.html
- **PEP 668 (System Python)**: https://peps.python.org/pep-0668/

**Search Terms:**
- "pyenv install python", "pyenv virtualenv"
- "python version manager", "multiple python versions"
- "pyenv commands", "pyenv local global"

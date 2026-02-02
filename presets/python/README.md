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

## Learn More

- [pyenv GitHub](https://github.com/pyenv/pyenv)
- [pyenv-virtualenv](https://github.com/pyenv/pyenv-virtualenv)
- [Python Documentation](https://docs.python.org/)
- [Real Python - pyenv Tutorial](https://realpython.com/intro-to-pyenv/)

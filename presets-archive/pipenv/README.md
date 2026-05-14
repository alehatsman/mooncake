# pipenv - Python Dependency Manager

Python development workflow tool that combines pip and virtualenv for package management.

## Quick Start
```yaml
- preset: pipenv
```

## Features
- **Automatic virtualenv**: Creates and manages virtual environments
- **Dependency resolution**: Deterministic builds with Pipfile.lock
- **Security**: Checks for known vulnerabilities
- **Simple workflow**: Unified tool for package and environment management
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Install dependencies from Pipfile
pipenv install

# Install dev dependencies
pipenv install --dev

# Activate virtual environment
pipenv shell

# Run command in virtualenv
pipenv run python script.py

# Install specific package
pipenv install requests

# Uninstall package
pipenv uninstall requests

# Check for security vulnerabilities
pipenv check
```

## Advanced Configuration
```yaml
- preset: pipenv
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove pipenv |

## Platform Support
- ✅ Linux (pip3)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Pipfile**: Project dependency specification
- **Pipfile.lock**: Locked dependency versions
- **Virtualenv location**: `~/.local/share/virtualenvs/` (default)
- **Environment variable**: `PIPENV_VENV_IN_PROJECT=1` (create .venv in project)

## Real-World Examples

### New Project Setup
```bash
# Initialize project
cd myproject
pipenv install

# Install dependencies
pipenv install flask sqlalchemy

# Install dev dependencies
pipenv install --dev pytest black

# Generate Pipfile.lock
pipenv lock
```

### Existing Project
```bash
# Clone repository
git clone https://github.com/org/project.git
cd project

# Install from Pipfile.lock
pipenv install --deploy

# Run application
pipenv run python app.py
```

### CI/CD Pipeline
```yaml
- preset: pipenv

- name: Install dependencies
  shell: pipenv install --deploy --ignore-pipfile
  cwd: /app

- name: Run tests
  shell: pipenv run pytest
  cwd: /app

- name: Security check
  shell: pipenv check
  cwd: /app
```

### Flask Development
```bash
# Create Flask project
mkdir myapp && cd myapp
pipenv install flask

# Create app
cat > app.py <<'EOF'
from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello World!'

if __name__ == '__main__':
    app.run(debug=True)
EOF

# Run development server
pipenv run python app.py
```

## Agent Use
- Set up reproducible Python development environments
- Install project dependencies in CI/CD pipelines
- Security vulnerability scanning in deployment workflows
- Manage multiple Python projects with isolated dependencies
- Generate lockfiles for deterministic builds

## Common Commands
```bash
# Show dependency graph
pipenv graph

# Update all dependencies
pipenv update

# Remove virtualenv
pipenv --rm

# Show virtualenv path
pipenv --venv

# Run script
pipenv run python manage.py migrate

# Generate requirements.txt
pipenv lock -r > requirements.txt
pipenv lock -r --dev > requirements-dev.txt
```

## Troubleshooting

### Lock file out of sync
```bash
# Regenerate lock file
pipenv lock --clear
```

### Virtualenv in wrong location
```bash
# Use project directory
export PIPENV_VENV_IN_PROJECT=1
pipenv install
```

### Dependency conflicts
```bash
# Show dependency tree
pipenv graph

# Update specific package
pipenv update package-name
```

## Uninstall
```yaml
- preset: pipenv
  with:
    state: absent
```

## Resources
- Official docs: https://pipenv.pypa.io/
- GitHub: https://github.com/pypa/pipenv
- Search: "pipenv tutorial", "pipenv best practices"

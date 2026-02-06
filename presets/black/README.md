# Black - Python Code Formatter

Uncompromising Python code formatter that automatically formats code to a consistent style.

## Quick Start
```yaml
- preset: black
```

## Features
- **Zero configuration**: Opinionated formatting, no bike-shedding
- **Consistent style**: Deterministic output across all projects
- **Fast**: Written in Python, optimized for speed
- **Editor integration**: Supports VS Code, PyCharm, Vim, Emacs, Sublime
- **CI/CD friendly**: Check mode for validating formatting
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Format a file
black script.py

# Format a directory
black src/

# Format entire project
black .

# Check without formatting (CI mode)
black --check .

# Show diff without formatting
black --diff src/

# Verbose output
black --verbose src/

# Quiet mode (only errors)
black --quiet src/
```

## Advanced Configuration

```yaml
# Install Black
- preset: black
  register: black_result

# Format Python code
- name: Format Python files
  shell: black .
  cwd: /path/to/project
  register: format_result

# Check formatting in CI
- name: Check code formatting
  shell: black --check --diff .
  cwd: /path/to/project
  register: check_result

- name: Verify formatting passed
  assert:
    command:
      cmd: echo {{ check_result.rc }}
      exit_code: 0
```

## Configuration File

### pyproject.toml
```toml
[tool.black]
line-length = 88
target-version = ['py38', 'py39', 'py310', 'py311']
include = '\.pyi?$'
exclude = '''
/(
    \.git
  | \.hg
  | \.mypy_cache
  | \.tox
  | \.venv
  | _build
  | buck-out
  | build
  | dist
)/
'''
```

### Custom Line Length
```bash
# Format with custom line length
black --line-length 100 src/

# Or in pyproject.toml
# [tool.black]
# line-length = 100
```

## Editor Integration

### VS Code
```json
// settings.json
{
  "python.formatting.provider": "black",
  "python.formatting.blackArgs": ["--line-length", "100"],
  "editor.formatOnSave": true
}
```

### PyCharm
```
Settings → Tools → Black
Enable: "On save"
Arguments: --line-length 100
```

### Vim/Neovim
```vim
" Format on save
autocmd BufWritePre *.py execute ':Black'

" Or use with ALE
let g:ale_fixers = {'python': ['black']}
let g:ale_fix_on_save = 1
```

## Command Line Options

```bash
# Line length
black --line-length 100 src/

# Python version target
black --target-version py311 src/

# Skip string normalization
black --skip-string-normalization src/

# Skip magic trailing comma
black --skip-magic-trailing-comma src/

# Fast mode (skip AST safety checks)
black --fast src/

# Include/exclude patterns
black --include '\.pyi?$' --exclude '/tests/' src/

# Color output
black --color src/

# No color
black --no-color src/
```

## Real-World Examples

### CI/CD Pipeline Formatting Check
```yaml
# Enforce Black formatting in CI
- preset: black

- name: Check code formatting
  shell: black --check --diff .
  cwd: /workspace
  register: black_check

- name: Fail if not formatted
  assert:
    command:
      cmd: echo {{ black_check.rc }}
      exit_code: 0
  failed_when: black_check.rc != 0
```

### Pre-commit Hook
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/psf/black
    rev: 24.1.1
    hooks:
      - id: black
        language_version: python3.11
```

```bash
# Install pre-commit
pip install pre-commit
pre-commit install

# Now Black runs automatically on git commit
git commit -m "Add feature"  # Formats code automatically
```

### Format Changed Files Only
```bash
# In CI, format only changed files
git diff --name-only --diff-filter=ACM origin/main | \
  grep '\.py$' | \
  xargs black --check

# Or format them
git diff --name-only --diff-filter=ACM origin/main | \
  grep '\.py$' | \
  xargs black
```

### Docker Integration
```dockerfile
# Dockerfile
FROM python:3.11-slim
RUN pip install black
COPY . /app
WORKDIR /app
RUN black --check .
```

```bash
# Format in container
docker run --rm -v $(pwd):/app python:3.11 \
  sh -c "pip install black && black /app"
```

## Integration with Other Tools

### With isort (import sorting)
```bash
# Run both formatters
isort .
black .

# Or use isort's Black profile
isort --profile black .
black .
```

### With flake8 (linting)
```toml
# pyproject.toml
[tool.black]
line-length = 88

# setup.cfg or .flake8
[flake8]
max-line-length = 88
extend-ignore = E203, E501, W503
```

### With mypy (type checking)
```bash
# Format, then type check
black .
mypy src/
```

## Jupyter Notebook Support

```bash
# Format Jupyter notebooks
black --ipynb notebook.ipynb

# Format all notebooks in directory
black --ipynb notebooks/

# Skip cells
# Add # fmt: off and # fmt: on in cells to skip
```

## Troubleshooting

### File Not Formatted
```bash
# Check if file is excluded
black --verbose --check file.py

# Force include
black --force-exclude file.py
```

### Syntax Errors
```bash
# Black won't format files with syntax errors
# Fix syntax errors first
python -m py_compile file.py

# Then format
black file.py
```

### Conflicts with Other Formatters
```bash
# Disable autopep8, yapf in editor
# Use only Black for formatting
# Configure flake8 to be Black-compatible (ignore E203, E501, W503)
```

### Performance Issues
```bash
# Use fast mode (skips AST checks)
black --fast .

# Process fewer files at once
find . -name '*.py' -print0 | xargs -0 -n 10 black
```

## Black Philosophy

Black is opinionated by design:
- **One style to rule them all**: No configuration options for style
- **Readability over brevity**: Prefers clarity
- **Stability**: Format is stable and deterministic
- **No surprises**: Same input always produces same output

## Comparison with Other Formatters

| Feature | Black | autopep8 | yapf |
|---------|-------|----------|------|
| Configuration | Minimal | Many options | Many options |
| Line length | Configurable | Configurable | Configurable |
| Deterministic | Yes | No | No |
| Speed | Fast | Fast | Slower |
| Opinionated | Very | No | Somewhat |

## Platform Support
- ✅ Linux (pip, apt, dnf, Homebrew)
- ✅ macOS (pip, Homebrew)
- ✅ Windows (pip, Chocolatey)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Enforce consistent code style across team
- Automate code formatting in CI/CD
- Validate formatting in pull requests
- Format codebases during migrations
- Integrate with pre-commit hooks
- Ensure PEP 8 compliance without manual work
- Reduce code review friction about style

## Uninstall
```yaml
- preset: black
  with:
    state: absent
```

## Resources
- Official docs: https://black.readthedocs.io/
- GitHub: https://github.com/psf/black
- Playground: https://black.vercel.app/
- VS Code extension: https://marketplace.visualstudio.com/items?itemName=ms-python.black-formatter
- Search: "black python formatter", "black configuration", "black vs autopep8"

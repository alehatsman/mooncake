# ruff - Extremely Fast Python Linter

ruff is an extremely fast Python linter and code formatter, written in Rust.

## Quick Start

```yaml
- preset: ruff
```

## Features

- **Blazingly fast**: 10-100x faster than Flake8 and Black
- **Drop-in replacement**: Compatible with Flake8, isort, and Black
- **Comprehensive**: 700+ lint rules from popular tools
- **Auto-fix**: Automatically fix many lint errors
- **Format**: Built-in code formatter (compatible with Black)
- **No dependencies**: Single binary written in Rust

## Basic Usage

```bash
# Check code
ruff check .

# Check with auto-fix
ruff check . --fix

# Format code
ruff format .

# Check specific files
ruff check src/

# Show rule violations
ruff check --output-format=grouped .

# List all rules
ruff rule --all
```

## Advanced Configuration

```yaml
# Simple installation
- preset: ruff

# Remove installation
- preset: ruff
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, pip)
- ✅ macOS (Homebrew, pip)
- ❌ Windows (not yet supported by this preset)

## Configuration

- **Config file**: `pyproject.toml` or `ruff.toml`
- **Per-directory config**: `.ruff.toml` in project root
- **VSCode extension**: `charliermarsh.ruff`

### Example pyproject.toml

```toml
[tool.ruff]
line-length = 88
target-version = "py311"

# Enable specific rule sets
select = [
    "E",   # pycodestyle errors
    "W",   # pycodestyle warnings
    "F",   # pyflakes
    "I",   # isort
    "B",   # flake8-bugbear
    "C4",  # flake8-comprehensions
    "UP",  # pyupgrade
]

# Ignore specific rules
ignore = [
    "E501",  # line too long (handled by formatter)
]

# Exclude directories
exclude = [
    ".git",
    ".venv",
    "build",
    "dist",
]

[tool.ruff.format]
quote-style = "double"
indent-style = "space"
```

## Real-World Examples

### Pre-Commit Integration

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.2.0
    hooks:
      # Run linter
      - id: ruff
        args: [--fix]
      # Run formatter
      - id: ruff-format
```

### CI/CD Pipeline

```yaml
# Install and run ruff in CI
- preset: ruff

- name: Lint Python code
  shell: ruff check . --output-format=github
  register: lint_result

- name: Check formatting
  shell: ruff format --check .

- name: Fail on errors
  assert:
    command:
      cmd: test {{ lint_result.rc }} -eq 0
      exit_code: 0
```

### Replace Multiple Tools

```bash
# Before: Multiple tools
flake8 src/
isort src/
black src/
pylint src/

# After: Just ruff
ruff check src/ --fix
ruff format src/
```

### VSCode Integration

```json
// settings.json
{
  "[python]": {
    "editor.formatOnSave": true,
    "editor.defaultFormatter": "charliermarsh.ruff",
    "editor.codeActionsOnSave": {
      "source.organizeImports": true,
      "source.fixAll": true
    }
  },
  "ruff.lint.args": [
    "--config=pyproject.toml"
  ]
}
```

### Gradual Adoption

```toml
# Start with minimal rules
[tool.ruff]
select = ["E", "F"]  # Just pycodestyle and pyflakes

# Gradually add more
select = ["E", "F", "I", "N"]  # Add isort and naming

# Eventually enable all
select = ["ALL"]
ignore = ["D"]  # Except docstrings (for now)
```

### Custom Rule Configuration

```toml
[tool.ruff]
select = ["ALL"]
ignore = [
    "D",     # pydocstyle (no docstrings required)
    "ANN",   # flake8-annotations (no type hints required)
    "COM812" # trailing commas (handled by formatter)
]

# Per-file ignores
[tool.ruff.per-file-ignores]
"tests/*" = ["S101"]  # Allow assert in tests
"__init__.py" = ["F401"]  # Allow unused imports

# isort settings
[tool.ruff.isort]
known-first-party = ["myapp"]

# pyupgrade settings
[tool.ruff.pyupgrade]
keep-runtime-typing = true
```

## Rule Categories

```bash
# Pyflakes (F)
E, W   # pycodestyle (errors and warnings)
F      # Pyflakes
I      # isort
N      # pep8-naming
D      # pydocstyle
UP     # pyupgrade
B      # flake8-bugbear
A      # flake8-builtins
C4     # flake8-comprehensions
T10    # flake8-debugger
S      # flake8-bandit (security)
RUF    # Ruff-specific rules
```

## Agent Use

- Enforce code quality standards in automated workflows
- Auto-fix common Python issues in CI/CD
- Format code before committing
- Replace multiple linting tools with single fast tool
- Validate Python code syntax and style

## Troubleshooting

### Too many errors

```bash
# Start with minimal rules
ruff check . --select E,F

# Show only fixable errors
ruff check . --fix-only

# Generate baseline config
ruff check . --add-noqa
```

### Rule conflicts

```bash
# Check rule details
ruff rule E501

# See which rules overlap
ruff check . --show-source

# Disable conflicting rule
# In pyproject.toml:
ignore = ["E501"]
```

### Format conflicts with Black

```toml
# Use Black-compatible settings
[tool.ruff.format]
quote-style = "double"
line-ending = "auto"

# Or just use ruff format (Black-compatible by default)
```

### Slow on large codebases

```bash
# Use --no-cache for one-off runs
ruff check . --no-cache

# Clean cache
rm -rf ~/.cache/ruff

# Exclude large directories
# In pyproject.toml:
exclude = ["build", "dist", "node_modules"]
```

## Uninstall

```yaml
- preset: ruff
  with:
    state: absent
```

## Resources

- Official docs: https://docs.astral.sh/ruff/
- GitHub: https://github.com/astral-sh/ruff
- Rule index: https://docs.astral.sh/ruff/rules/
- Search: "ruff python linter", "ruff vs flake8", "ruff format vs black"

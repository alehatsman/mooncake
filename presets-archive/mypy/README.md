# mypy - Static Type Checker for Python

A static type checker that helps find bugs in Python code by verifying type annotations. Integrates seamlessly with IDEs, CI/CD pipelines, and development workflows.

## Quick Start

```yaml
- preset: mypy
```

## Features

- **Type checking**: Verify type annotations in Python code
- **Gradual typing**: Adopt types incrementally in existing codebases
- **Plugin system**: Extend with custom type checkers
- **IDE integration**: Works with PyCharm, VS Code, and other editors
- **CI/CD ready**: Exit codes for automation
- **Flexible configuration**: Fine-tune checking via config files
- **Cross-platform**: Linux and macOS support

## Basic Usage

```bash
# Check single file
mypy script.py

# Check entire package
mypy mypackage/

# Check with specific Python version
mypy --python-version 3.10 src/

# Strict mode (recommended for new code)
mypy --strict mycode.py

# Generate report
mypy --html report/ src/

# View version
mypy --version
```

## Advanced Configuration

```yaml
# Basic installation
- preset: mypy

# With uninstall
- preset: mypy
  with:
    state: absent
```

### Configuration File

Create `mypy.ini` in project root:

```ini
[mypy]
python_version = 3.10
warn_return_any = True
warn_unused_configs = True
disallow_untyped_defs = True
disallow_any_unimported = True
```

Or `pyproject.toml`:

```toml
[tool.mypy]
python_version = "3.10"
warn_return_any = true
disallow_untyped_defs = true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, pip3)
- ✅ macOS (Homebrew, pip3)
- ❌ Windows (not supported)

## Configuration

- **Config file**: `mypy.ini` or `pyproject.toml` (project root)
- **Cache directory**: `.mypy_cache/` (created in current directory)
- **Report output**: `htmlcov/` or custom location
- **Python version**: Defaults to current Python version

## Real-World Examples

### Development Workflow

```bash
# Check code before committing
mypy src/

# Watch mode with integration
while true; do mypy src/ && echo "✓ Types OK"; sleep 2; done

# Strict mode for new modules
mypy --strict src/new_module.py
```

### Type Annotation Examples

```python
# Before mypy (no type hints)
def add(x, y):
    return x + y

result = add("hello", 5)  # Type error not caught at runtime
```

```python
# After mypy (with type hints)
def add(x: int, y: int) -> int:
    return x + y

result = add("hello", 5)  # mypy catches this error: str cannot be int
```

### CI/CD Integration

```bash
#!/bin/bash
# Pre-commit hook or CI pipeline

mypy src/ --junit-xml mypy-report.xml || {
  echo "Type checking failed"
  exit 1
}

echo "✓ All type checks passed"
```

### Strict Mode Adoption

```bash
# Gradually adopt strict typing
mypy --strict src/module1.py  # Start with core modules
mypy src/module2.py            # Others with standard checking
mypy src/legacy.py --ignore-missing-imports  # Pragmatic approach
```

## Agent Use

- Validate Python type annotations in CI/CD pipelines
- Detect type mismatches before runtime errors occur
- Generate type compliance reports for code quality metrics
- Enforce typing standards across team codebases
- Identify untyped function arguments and return values
- Validate type stub files for third-party libraries

## Troubleshooting

### Missing type stubs

Install type stubs for third-party packages:

```bash
# For popular packages, install typeshed stubs
pip install types-requests types-PyYAML types-dateutil

# Check what's missing
mypy --install-types
```

### Django or FastAPI types not working

Install framework-specific stubs:

```bash
pip install django-stubs fastapi-stubs sqlalchemy-stubs
```

### Too many errors to fix at once

Use allowlist and incremental checking:

```bash
# Show summary only
mypy src/ --summary-only

# Incremental builds
mypy src/ --incremental

# Disable checks for specific files
mypy src/ --ignore-missing-imports
```

### Cache issues

Clear mypy cache when encountering stale data:

```bash
# Remove all caches
rm -rf .mypy_cache/

# Then re-run
mypy src/
```

## Uninstall

```yaml
- preset: mypy
  with:
    state: absent
```

## Resources

- Official docs: https://mypy.readthedocs.io/
- GitHub: https://github.com/python/mypy
- Type hints guide: https://docs.python.org/3/library/typing.html
- Search: "mypy tutorial", "Python type hints", "mypy strict mode"

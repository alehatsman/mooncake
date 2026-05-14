# Pylint - Python Code Analyzer

Industry-standard Python linter that checks for errors, enforces coding standards, and suggests improvements.

## Quick Start
```yaml
- preset: pylint
```

## Features
- **Error detection**: Find bugs and code smells
- **Style enforcement**: PEP 8 compliance checking
- **Code quality metrics**: Complexity analysis and scoring
- **Customizable**: Extensive configuration options
- **Plugin support**: Extensible with custom checkers
- **CI/CD ready**: Exit codes and report formats for automation

## Basic Usage
```bash
# Check single file
pylint myfile.py

# Check directory
pylint mypackage/

# Check multiple files
pylint file1.py file2.py

# Generate report
pylint --output-format=text myfile.py

# Show only errors
pylint --errors-only myfile.py

# Set minimum score
pylint --fail-under=8.0 mypackage/

# Generate config
pylint --generate-rcfile > .pylintrc
```

## Advanced Configuration

### CI/CD integration
```yaml
- name: Install Pylint
  preset: pylint

- name: Run linting
  shell: pylint --output-format=parseable src/
  cwd: /app
  register: lint_result
  failed_when: lint_result.rc != 0

- name: Generate report
  shell: pylint --output-format=json src/ > pylint-report.json
  cwd: /app
```

### Pre-commit hook
```yaml
- name: Install Pylint
  preset: pylint

- name: Create pre-commit hook
  copy:
    dest: .git/hooks/pre-commit
    mode: "0755"
    content: |
      #!/bin/bash
      pylint --fail-under=8.0 $(git diff --cached --name-only --diff-filter=d | grep '\.py$')
```

### Quality gate
```yaml
- name: Check code quality
  shell: |
    pylint --output-format=json mypackage/ > pylint.json
    python -c "import json; score = json.load(open('pylint.json'))['statistics']['score']; exit(0 if score >= 8.0 else 1)"
  register: quality_check
  failed_when: quality_check.rc != 0
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Pylint |

## Platform Support
- ✅ Linux (pip, apt python3-pylint)
- ✅ macOS (pip, Homebrew)
- ✅ Windows (pip)
- ✅ All platforms via pip/pipx

## Configuration
- **Config file**: `.pylintrc`, `pylintrc`, `pyproject.toml`, `setup.cfg`
- **Global config**: `~/.pylintrc`, `~/.config/pylintrc`
- **Output**: Console, JSON, parseable, colorized, HTML

## Real-World Examples

### Django project
```bash
# Create config for Django
cat > .pylintrc <<EOF
[MASTER]
load-plugins=pylint_django

[MESSAGES CONTROL]
disable=C0111,C0103,W0212

[FORMAT]
max-line-length=120
EOF

# Run pylint
pylint --load-plugins=pylint_django myapp/
```

### Flask API
```bash
# Check API code
pylint --disable=C0111,C0103 --max-line-length=120 api/

# Focus on errors only
pylint --errors-only api/
```

### GitHub Actions
```yaml
- name: Install Pylint
  preset: pylint

- name: Lint code
  shell: |
    pylint --output-format=parseable --reports=no src/ || exit_code=$?
    echo "Pylint exit code: $exit_code"
    if [ $exit_code -eq 32 ]; then
      echo "Usage error"
      exit 1
    elif [ $exit_code -eq 16 ]; then
      echo "Fatal errors found"
      exit 1
    elif [ $exit_code -eq 1 ]; then
      echo "Convention or refactor messages"
      # Don't fail on these
    fi
```

## Configuration File

### .pylintrc
```ini
[MASTER]
# Parallel jobs for speed
jobs=4

# Plugins
load-plugins=pylint.extensions.docparams,pylint.extensions.check_elif

# Python path
init-hook='import sys; sys.path.append("src")'

[MESSAGES CONTROL]
# Disable specific warnings
disable=C0111,  # missing-docstring
        C0103,  # invalid-name
        R0903,  # too-few-public-methods
        R0913,  # too-many-arguments
        W0212   # protected-access

[FORMAT]
# Line length
max-line-length=100

# Indentation
indent-string='    '

[BASIC]
# Naming conventions
good-names=i,j,k,x,y,z,id,pk,db,_

# Regex patterns
variable-rgx=[a-z_][a-z0-9_]{2,30}$
const-rgx=(([A-Z_][A-Z0-9_]*)|(__.*__))$

[DESIGN]
# Complexity limits
max-args=7
max-locals=20
max-returns=8
max-branches=15
max-statements=60
```

### pyproject.toml
```toml
[tool.pylint.main]
jobs = 4
load-plugins = ["pylint.extensions.docparams"]

[tool.pylint.messages_control]
disable = ["C0111", "C0103", "R0903"]

[tool.pylint.format]
max-line-length = 100

[tool.pylint.design]
max-args = 7
max-locals = 20
```

## Message Categories
- **C**: Convention - coding standard violation
- **R**: Refactor - code smell
- **W**: Warning - potential issues
- **E**: Error - probable bugs
- **F**: Fatal - errors preventing further analysis

## Exit Codes
- `0`: No errors
- `1`: Fatal or error messages
- `2`: Warning messages
- `4`: Refactor messages
- `8`: Convention messages
- `16`: Usage error
- `32`: Fatal error

## Common Disables
```python
# Disable for whole file
# pylint: disable=invalid-name

# Disable for line
x = 1  # pylint: disable=invalid-name

# Disable for block
# pylint: disable=missing-docstring
def my_function():
    pass
# pylint: enable=missing-docstring

# Disable multiple
# pylint: disable=invalid-name,missing-docstring
```

## Integration with Tools

### Black (code formatter)
```ini
[FORMAT]
max-line-length=88
disable=C0330  # Wrong hanging indentation
```

### MyPy (type checker)
```bash
# Run both
pylint mypackage/ && mypy mypackage/
```

### pytest
```bash
# Run tests then lint
pytest && pylint tests/ src/
```

## Plugins
```bash
# Django
pip install pylint-django
pylint --load-plugins=pylint_django myapp/

# Flask
pip install pylint-flask
pylint --load-plugins=pylint_flask api/

# Celery
pip install pylint-celery
pylint --load-plugins=pylint_celery tasks/
```

## Output Formats
```bash
# Text (default)
pylint myfile.py

# Parseable (for tools)
pylint --output-format=parseable myfile.py

# JSON
pylint --output-format=json myfile.py

# Colorized
pylint --output-format=colorized myfile.py

# HTML report
pylint --output-format=html myfile.py > report.html
```

## Score Interpretation
- **10.0**: Perfect code (rarely achieved)
- **8.0-9.9**: Excellent quality
- **7.0-7.9**: Good quality
- **6.0-6.9**: Acceptable
- **< 6.0**: Needs improvement

## Agent Use
- Enforce code quality standards in CI/CD
- Block PRs with score below threshold
- Generate quality reports for dashboards
- Pre-commit hooks for instant feedback
- Automated refactoring suggestions
- Track quality metrics over time
- Identify technical debt

## Troubleshooting

### False positives
```python
# Disable specific check
# pylint: disable=no-member
obj.dynamic_attribute

# Configure in .pylintrc
[TYPECHECK]
ignored-modules=dynamic_module
```

### Import errors
```ini
[MASTER]
init-hook='import sys; sys.path.append("src")'
```

### Too slow
```bash
# Parallel execution
pylint --jobs=4 mypackage/

# Disable reports
pylint --reports=no mypackage/
```

### Conflicting with Black
```ini
[FORMAT]
max-line-length=88
disable=C0330,C0326
```

## Best Practices
- **Run early**: Integrate in pre-commit hooks
- **Set thresholds**: Enforce minimum scores
- **Customize rules**: Disable irrelevant checks
- **Use plugins**: Django, Flask support
- **CI integration**: Block low-quality code
- **Track trends**: Monitor scores over time
- **Team agreement**: Align on coding standards
- **Gradual adoption**: Start lenient, tighten over time

## Comparison

| Tool | Focus | Speed | Strictness |
|------|-------|-------|------------|
| Pylint | Comprehensive | Slow | Very strict |
| Flake8 | Style + errors | Fast | Moderate |
| Ruff | Style + errors | Very fast | Configurable |
| MyPy | Type checking | Medium | Type-focused |
| Black | Formatting | Fast | Opinionated |

## Uninstall
```yaml
- preset: pylint
  with:
    state: absent
```

## Resources
- Official docs: https://pylint.pycqa.org/
- GitHub: https://github.com/pylint-dev/pylint
- Message catalog: https://pylint.pycqa.org/en/latest/user_guide/messages/messages_overview.html
- Search: "pylint tutorial", "pylint configuration", "pylint ci/cd"

# yamllint - YAML Linter

Linter for YAML files that checks syntax and enforces style conventions.

## Quick Start
```yaml
- preset: yamllint
```

## Features
- **Syntax Validation**: Catch YAML syntax errors
- **Style Enforcement**: Consistent formatting across files
- **Configurable Rules**: Customize checks via .yamllint config
- **CI/CD Integration**: Exit codes for automated validation
- **Multiple Output Formats**: parsable, github, colored
- **Cross-platform**: Works on Linux, macOS, Windows

## Basic Usage
```bash
# Lint single file
yamllint file.yml

# Lint directory
yamllint .
yamllint config/

# Lint with specific config
yamllint -c .yamllint file.yml

# Strict mode (warnings as errors)
yamllint -s file.yml

# Output formats
yamllint -f parsable file.yml
yamllint -f github file.yml
yamllint -f colored file.yml
```

## Advanced Configuration
```yaml
- preset: yamllint
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove yamllint |

## Platform Support
- ✅ Linux (pip, apt, dnf, pacman)
- ✅ macOS (pip, Homebrew)
- ✅ Windows (pip)

## Configuration
- **Config file**: `.yamllint`, `.yamllint.yaml`, or `.yamllint.yml`
- **System config**: `/etc/yamllint/config`
- **User config**: `~/.config/yamllint/config`

## Configuration File

`.yamllint`:
```yaml
extends: default

rules:
  line-length:
    max: 120
    level: warning
  indentation:
    spaces: 2
  comments:
    min-spaces-from-content: 1
  braces:
    max-spaces-inside: 1
  brackets:
    max-spaces-inside: 1
```

## Real-World Examples

### CI/CD Validation
```yaml
- name: Install yamllint
  preset: yamllint

- name: Lint YAML files
  shell: yamllint .
  cwd: /app

- name: Lint with specific config
  shell: yamllint -c .yamllint.yaml k8s/
  cwd: /app
```

### Pre-commit Hook
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/adrienverge/yamllint
    rev: v1.32.0
    hooks:
      - id: yamllint
        args: [-c=.yamllint]
```

### Kubernetes Manifests
```bash
# Lint all manifests
yamllint k8s/*.yaml

# Strict mode for production
yamllint -s deployment.yaml

# Check specific patterns
yamllint -f parsable k8s/ | grep error
```

## Common Rules

```yaml
# .yamllint
extends: default

rules:
  # Line length
  line-length:
    max: 120
    allow-non-breakable-words: true

  # Indentation
  indentation:
    spaces: 2
    indent-sequences: true

  # Comments
  comments:
    min-spaces-from-content: 1
    require-starting-space: true

  # Quotes
  quoted-strings:
    quote-type: single
    required: only-when-needed

  # Trailing spaces
  trailing-spaces: enable

  # Document start (---)
  document-start: disable

  # Empty lines
  empty-lines:
    max: 2
```

## Agent Use
- Automated YAML syntax validation
- CI/CD pipeline quality gates
- Configuration file validation
- Kubernetes manifest linting
- Pre-commit hooks for code quality
- Style enforcement across teams

## Troubleshooting

### False positives
```yaml
# Disable specific rule for file
# yamllint disable-file

# Disable rule for line
key: value  # yamllint disable-line rule:line-length

# Disable rule for block
# yamllint disable rule:line-length
long_line: very long value here
# yamllint enable rule:line-length
```

### Custom config not found
```bash
# Specify config explicitly
yamllint -c /path/to/.yamllint file.yml

# Check config location
yamllint --print-config file.yml
```

## Uninstall
```yaml
- preset: yamllint
  with:
    state: absent
```

## Resources
- Official docs: https://yamllint.readthedocs.io/
- GitHub: https://github.com/adrienverge/yamllint
- Search: "yamllint configuration", "yamllint rules"

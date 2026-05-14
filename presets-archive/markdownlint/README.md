# markdownlint - Markdown Linter and Style Checker

A Node.js-based style checker and linter for Markdown files that helps maintain consistent documentation quality.

## Quick Start
```yaml
- preset: markdownlint
```

## Features
- **Style enforcement**: Ensures consistent Markdown formatting
- **Configurable rules**: 50+ rules for various Markdown patterns
- **CI/CD integration**: Exit codes for automated checks
- **Fast execution**: Scans hundreds of files in seconds
- **Auto-fix**: Automatically fixes many common issues
- **Cross-platform**: Works on Linux, macOS, Windows

## Basic Usage
```bash
# Check all Markdown files in current directory
markdownlint '**/*.md'

# Check specific files
markdownlint README.md docs/*.md

# Show version
markdownlint --version

# Auto-fix issues
markdownlint --fix '**/*.md'

# Use custom config
markdownlint --config .markdownlint.json '**/*.md'
```

## Advanced Configuration
```yaml
- preset: markdownlint
  with:
    state: present              # Install or remove (present/absent)
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether markdownlint should be installed (present) or removed (absent) |

## Platform Support
- ✅ Linux (npm)
- ✅ macOS (Homebrew, npm)
- ✅ Windows (npm)

## Configuration

**Config file**: `.markdownlint.json` or `.markdownlint.yaml` in project root

Example `.markdownlint.json`:
```json
{
  "default": true,
  "MD013": false,
  "MD033": {
    "allowed_elements": ["br", "img"]
  }
}
```

**Common rules**:
- `MD001`: Heading levels increment by one
- `MD003`: Heading style (atx, setext)
- `MD007`: Unordered list indentation
- `MD013`: Line length (default 80 chars)
- `MD033`: Inline HTML
- `MD041`: First line in file should be top-level heading

## Real-World Examples

### CI/CD Pipeline
```bash
# Fail build if Markdown has issues
markdownlint '**/*.md' || exit 1
```

### Pre-commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit
markdownlint $(git diff --cached --name-only --diff-filter=ACM "*.md")
```

### GitHub Actions
```yaml
- name: Lint Markdown files
  run: |
    npm install -g markdownlint-cli
    markdownlint '**/*.md'
```

### Documentation Quality Check
```bash
# Check docs with auto-fix
markdownlint --fix docs/**/*.md

# Verify README
markdownlint README.md CONTRIBUTING.md
```

## Agent Use
- Validate Markdown documentation in automated pipelines
- Enforce consistent documentation style across repositories
- Auto-fix common Markdown issues before commits
- Generate documentation quality reports
- Integrate with PR review workflows to ensure standards

## Troubleshooting

### Installation fails
Install Node.js first, then install markdownlint globally:
```bash
npm install -g markdownlint-cli
```

### Rule violations
Check which rules are failing:
```bash
markdownlint --help | grep MD
```

Disable specific rules in `.markdownlint.json`.

## Uninstall
```yaml
- preset: markdownlint
  with:
    state: absent
```

## Resources
- Official docs: https://github.com/igorshubovych/markdownlint-cli
- Rules reference: https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md
- Search: "markdownlint rules", "markdownlint configuration"

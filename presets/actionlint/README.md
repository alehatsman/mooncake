# actionlint - GitHub Actions Linter

Static checker for GitHub Actions workflow files. Catch errors before pushing, enforce best practices, validate syntax.

## Quick Start
```yaml
- preset: actionlint
```

## Basic Usage
```bash
# Lint all workflows
actionlint

# Lint specific file
actionlint .github/workflows/ci.yml

# Multiple files
actionlint .github/workflows/*.yml

# With color output
actionlint -color
```

## Output Formats
```bash
# Default format (human-readable)
actionlint

# JSON output
actionlint -format '{{json .}}'

# Custom format
actionlint -format '{{range $err := .}}{{$err.Filepath}}:{{$err.Line}}:{{$err.Column}}: {{$err.Message}}{{"\n"}}{{end}}'

# Sarif format (for GitHub)
actionlint -format sarif
```

## Validation Types
```bash
# Syntax errors
# - Invalid YAML
# - Malformed expressions
# - Unknown keys

# Type checking
# - Wrong input types
# - Invalid outputs
# - Type mismatches in expressions

# Best practices
# - Deprecated features
# - Security issues
# - Performance problems
```

## Common Errors Detected
```yaml
# Shell injection vulnerability
- run: echo "${{ github.event.issue.title }}"
# Error: Potential shell injection

# Undefined step output
- run: echo "${{ steps.missing.outputs.value }}"
# Error: Step 'missing' not found

# Invalid action version
- uses: actions/checkout@v999
# Warning: Tag not found

# Type mismatch
if: steps.test.outputs.result == true
# Error: Comparing string with boolean

# Undefined secret
env:
  TOKEN: ${{ secrets.MISSING_TOKEN }}
# Warning: Secret not defined in repository
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Lint workflows
  run: |
    actionlint

# With specific exit codes
- name: Lint workflows
  run: actionlint || exit 1

# Save results
- name: Lint workflows
  run: actionlint -format '{{json .}}' > lint-results.json
```

## Pre-commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit

if git diff --cached --name-only | grep -q '\.github/workflows/'; then
  echo "Linting GitHub Actions workflows..."
  actionlint
  if [ $? -ne 0 ]; then
    echo "Workflow linting failed. Fix errors and try again."
    exit 1
  fi
fi
```

## Configuration
```yaml
# .github/actionlint.yaml
self-hosted-runner:
  labels:
    - linux-custom
    - gpu-enabled

config-variables:
  - MY_VAR
  - DEPLOY_ENV
```

## Ignore Rules
```bash
# Ignore specific errors with comments
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      # actionlint: ignore[shellcheck]
      - run: echo $UNSAFE_VAR
```

## Shell Check Integration
```bash
# Validates shell scripts in 'run' steps
- run: |
    echo "Testing"
    if [ $STATUS = "success" ]; then
      echo "Done"
    fi
# Error: SC2086 - Double quote to prevent globbing

# Disable for specific step
- run: echo $VAR
  # actionlint: disable=shellcheck
```

## Expression Validation
```yaml
# Invalid expressions caught
- name: Check status
  if: ${{ steps.test.result == 'success' }}
  # Error: 'result' should be 'outcome' or 'conclusion'

- name: Matrix value
  run: echo "${{ matrix.missing }}"
  # Error: 'missing' not defined in matrix

- name: Context usage
  run: echo "${{ github.event.unknown }}"
  # Warning: Unknown property in github.event
```

## CODEOWNERS Validation
```bash
# Check CODEOWNERS syntax
actionlint --validate-codeowners

# Validate file permissions
actionlint --check-permissions
```

## Comparing Workflows
```bash
# Before/after validation
actionlint .github/workflows/before.yml
# Fix issues
actionlint .github/workflows/after.yml

# Diff output
diff <(actionlint before.yml 2>&1) <(actionlint after.yml 2>&1)
```

## Real-World Examples
```bash
# Lint all workflows in CI
name: Lint
on: [push, pull_request]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: actionlint

# Local development
actionlint && act -n  # Lint then dry-run

# Pre-merge validation
git diff main...HEAD --name-only | \
  grep '.github/workflows/' | \
  xargs actionlint
```

## Security Checks
```bash
# Detects security issues:

# 1. Shell injection
run: echo "${{ github.event.comment.body }}"
# Fix: Use environment variable
env:
  COMMENT: ${{ github.event.comment.body }}
run: echo "$COMMENT"

# 2. Script injection in pull_request_target
on: pull_request_target
run: ${{ github.event.pull_request.title }}
# Fix: Use pull_request or validate input

# 3. Unvalidated inputs
run: npm install ${{ github.event.inputs.package }}
# Fix: Validate package name first
```

## Best Practices Enforced
```yaml
# Pinned action versions
- uses: actions/checkout@v4  # Good
- uses: actions/checkout@main  # Warning: use SHA or tag

# Explicit permissions
permissions:
  contents: read
  pull-requests: write

# Timeout settings
jobs:
  test:
    timeout-minutes: 30

# Concurrency control
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

## Editor Integration
```bash
# VS Code (via extension)
# Install: GitHub Actions extension

# Vim/Neovim (via ALE)
let g:ale_linters = {'yaml': ['actionlint']}

# CI/CD (automated)
actionlint -format '{{json .}}' | jq
```

## Troubleshooting
```bash
# Show all errors
actionlint -verbose

# Ignore specific checks
actionlint -ignore 'SC2086'

# Check specific runner
actionlint -shellcheck=/usr/bin/shellcheck

# Debug mode
actionlint -debug
```

## Exit Codes
```
0 - No errors found
1 - Errors found
2 - Fatal error (invalid args, file not found)
```

## Comparison
| Feature | actionlint | yamllint | shellcheck |
|---------|------------|----------|------------|
| Actions-specific | Yes | No | No |
| Expression checking | Yes | No | No |
| Type validation | Yes | No | No |
| Shell validation | Via shellcheck | No | Yes |
| Security checks | Yes | No | Limited |

## Best Practices
- **Lint before committing** to catch errors early
- **Use in CI** to enforce quality
- **Enable shellcheck** for shell script validation
- **Configure self-hosted runners** in config file
- **Fix, don't ignore** issues when possible
- **Use with act** for local testing
- **Check security warnings** carefully

## Tips
- Catches 90% of workflow errors before push
- Type-checks expressions and contexts
- Validates action versions exist
- Integrates with shellcheck for scripts
- Fast (< 1 second for most repos)
- Zero configuration needed
- Works offline (after initial setup)

## Agent Use
- Automated workflow validation
- Pre-commit quality gates
- CI/CD syntax checking
- Security vulnerability detection
- Best practice enforcement
- Pull request validation

## Uninstall
```yaml
- preset: actionlint
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/rhysd/actionlint
- Docs: https://github.com/rhysd/actionlint/blob/main/docs/usage.md
- Search: "actionlint examples", "github actions linting"

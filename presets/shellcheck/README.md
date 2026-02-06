# shellcheck - Shell Script Linter

Static analysis tool for shell scripts. Catch bugs, enforce best practices, suggest improvements for bash/sh scripts.

## Quick Start
```yaml
- preset: shellcheck
```

## Basic Usage
```bash
# Check single script
shellcheck script.sh

# Multiple scripts
shellcheck *.sh

# Specific shell
shellcheck -s bash script.sh
shellcheck -s sh script.sh

# Follow sourced files
shellcheck -x script.sh
```

## Common Issues Detected
```bash
# SC2086: Double quote to prevent globbing
echo $var  # Bad
echo "$var"  # Good

# SC2046: Quote to prevent word splitting
for file in $(ls); do  # Bad
for file in *; do  # Good
# Or
while IFS= read -r file; do  # Good

# SC2034: Variable appears unused
unused="value"  # Warning

# SC2162: read without -r mangles backslashes
read line  # Bad
read -r line  # Good

# SC2155: Declare and assign separately
local var=$(cmd)  # Bad - masks return value
local var
var=$(cmd)  # Good

# SC2164: Use cd ... || exit in case cd fails
cd /tmp  # Bad
cd /tmp || exit  # Good
```

## Output Formats
```bash
# Default (TTY)
shellcheck script.sh

# JSON
shellcheck -f json script.sh

# GCC style
shellcheck -f gcc script.sh

# Checkstyle XML
shellcheck -f checkstyle script.sh

# Diff (show fixes)
shellcheck -f diff script.sh | git apply

# Quiet (only errors)
shellcheck -f quiet script.sh
```

## Severity Levels
```bash
# Show all (default)
shellcheck script.sh

# Only errors
shellcheck -S error script.sh

# Errors and warnings
shellcheck -S warning script.sh

# Include info
shellcheck -S info script.sh

# Include style suggestions
shellcheck -S style script.sh
```

## Ignoring Warnings
```bash
# Ignore specific code
shellcheck -e SC2086 script.sh

# Multiple codes
shellcheck -e SC2086,SC2046 script.sh

# Inline ignore
# shellcheck disable=SC2086
echo $var

# Inline ignore (next line only)
# shellcheck disable=SC2086
echo $var
echo $other  # Still checked

# Ignore for whole file
# shellcheck disable=SC2086,SC2046
#!/bin/bash
```

## Shell Specification
```bash
# Auto-detect from shebang
shellcheck script.sh

# Force bash
shellcheck -s bash script.sh

# Force sh (POSIX)
shellcheck -s sh script.sh

# dash
shellcheck -s dash script.sh

# ksh
shellcheck -s ksh script.sh
```

## Following Sources
```bash
# Follow source/. statements
shellcheck -x script.sh

# Add source paths
shellcheck -P /path/to/lib script.sh

# Multiple paths
shellcheck -P /lib1:/lib2 script.sh
```

## CI/CD Integration
```bash
# GitHub Actions
- name: ShellCheck
  run: shellcheck **/*.sh

# With specific options
- name: ShellCheck
  run: |
    shellcheck -f json *.sh > shellcheck.json
    if [ -s shellcheck.json ]; then
      cat shellcheck.json
      exit 1
    fi

# GitLab CI
shellcheck:
  image: koalaman/shellcheck-alpine:latest
  script:
    - shellcheck **/*.sh
  allow_failure: false

# With error threshold
- shellcheck -S error **/*.sh
```

## Pre-commit Hook
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/shellcheck-py/shellcheck-py
    rev: v0.9.0.6
    hooks:
      - id: shellcheck
        args: [-e, SC2086]
```

## Configuration File
```bash
# .shellcheckrc
disable=SC2086,SC2046
shell=bash
source-path=SCRIPTDIR
```

## Fixing Issues
```bash
# Get diff of fixes
shellcheck -f diff script.sh

# Apply fixes
shellcheck -f diff script.sh | patch

# Or with git
shellcheck -f diff script.sh | git apply
```

## Advanced Usage
```bash
# Check stdin
echo 'echo $var' | shellcheck -

# Color output
shellcheck --color=always script.sh

# No color
shellcheck --color=never script.sh

# Wiki links for errors
shellcheck --wiki-link-count=3 script.sh

# External sources
shellcheck -x -P ./lib script.sh
```

## Real-World Examples
```bash
# Check all scripts
find . -name '*.sh' -type f -exec shellcheck {} +

# Check scripts in CI
#!/bin/bash
failed=0
for script in $(git ls-files '*.sh'); do
  if ! shellcheck "$script"; then
    failed=1
  fi
done
exit $failed

# With colored output in CI
shellcheck --color=always **/*.sh || exit 1

# Only check changed files
git diff --name-only --diff-filter=AM master... | \
  grep '\.sh$' | \
  xargs shellcheck
```

## Integration with Editors
```bash
# VS Code
# Install: ShellCheck extension

# Vim/Neovim (via ALE)
let g:ale_linters = {'sh': ['shellcheck']}

# Emacs (via flycheck)
(add-hook 'sh-mode-hook 'flycheck-mode)

# Sublime Text
# Install: SublimeLinter-shellcheck
```

## Common Patterns
```bash
# Safe variable expansion
var="some value"
echo "$var"  # Quote variables

# Safe command substitution
result=$(command)  # Use $() over backticks

# Safe loops
while IFS= read -r line; do
  echo "$line"
done < file.txt

# Safe cd
cd /tmp || exit 1

# Check command existence
if command -v docker > /dev/null; then
  docker ps
fi

# Array iteration
files=("file1" "file2")
for file in "${files[@]}"; do
  echo "$file"
done
```

## Error Code Examples
```bash
# SC2086 - Quoting
myvar="hello world"
echo $myvar  # SC2086
echo "$myvar"  # OK

# SC2046 - Word splitting
for f in $(ls); do  # SC2046
for f in *; do  # OK

# SC2006 - Backticks deprecated
result=`cmd`  # SC2006
result=$(cmd)  # OK

# SC2164 - cd without error check
cd /some/path  # SC2164
cd /some/path || exit  # OK

# SC2115 - Dangerous rm -rf
rm -rf "$dir/"  # SC2115 if $dir is empty
[ -n "$dir" ] && rm -rf "${dir}/"  # OK
```

## Best Practices Enforced
- Quote variables to prevent word splitting
- Use `[[ ]]` over `[ ]` in bash
- Check command exit codes
- Use `$()` over backticks
- Declare functions before use
- Use `read -r` to preserve backslashes
- Check if variables are set before use
- Use arrays for file lists

## Tips
- Catches 95% of common shell script bugs
- Works with bash, sh, ksh, dash
- Fast (< 1 second for most scripts)
- No runtime dependencies
- Offline documentation (wiki codes)
- Integrates with most editors
- Used by GitHub for Actions validation

## Agent Use
- Automated script validation
- CI/CD quality gates
- Pre-commit hooks
- Security baseline checks
- Best practices enforcement
- Deployment script validation

## Uninstall
```yaml
- preset: shellcheck
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/koalaman/shellcheck
- Wiki: https://www.shellcheck.net/wiki/
- Online: https://www.shellcheck.net/
- Search: "shellcheck errors", "shellcheck SC2086"

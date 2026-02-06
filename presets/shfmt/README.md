# shfmt - Shell Script Formatter

Format shell scripts with consistent style. Auto-format bash/sh/mksh scripts like prettier for JavaScript.

## Features
- **Multiple shells**: bash, POSIX sh, mksh, bats support
- **Configurable indentation**: Tabs or spaces (2, 4, 8)
- **Binary operators**: Control line break placement
- **Switch case indentation**: Optional case indentation
- **Fast**: Single-pass formatting
- **Safe**: Never changes script logic
- **Editor integration**: VS Code, Vim, Emacs plugins

## Quick Start
```yaml
- preset: shfmt
```

## Basic Usage
```bash
# Format and print to stdout
shfmt script.sh

# Format in-place
shfmt -w script.sh

# Format multiple files
shfmt -w *.sh

# Format directory recursively
shfmt -w .

# Check if files are formatted
shfmt -d script.sh
```

## Indentation Styles
```bash
# Tabs (default)
shfmt script.sh

# Spaces (2)
shfmt -i 2 script.sh

# Spaces (4)
shfmt -i 4 script.sh

# Example output with -i 2:
if [ -f file.txt ]; then
  echo "exists"
fi

# Example with tabs:
if [ -f file.txt ]; then
	echo "exists"
fi
```

## Shell Language
```bash
# Auto-detect from shebang
shfmt script.sh

# Force bash
shfmt -ln bash script.sh

# Force POSIX sh
shfmt -ln posix script.sh

# mksh
shfmt -ln mksh script.sh

# bats (Bash Automated Testing)
shfmt -ln bats test.bats
```

## Binary Operators
```bash
# Default (next line)
if [ "$var" = "value" ] ||
   [ "$var2" = "other" ]; then

# Keep on same line (-bn)
shfmt -bn script.sh
if [ "$var" = "value" ] ||
   [ "$var2" = "other" ]; then
```

## Switch Cases
```bash
# Indent switch cases (-ci)
shfmt -ci script.sh

case "$var" in
  opt1)
    echo "option 1"
    ;;
  opt2)
    echo "option 2"
    ;;
esac

# Without -ci:
case "$var" in
opt1)
  echo "option 1"
  ;;
opt2)
  echo "option 2"
  ;;
esac
```

## Redirects
```bash
# Space after redirect (-sr)
shfmt -sr script.sh

# Before: cmd >file
# After:  cmd > file

# Before: cmd 2>&1
# After:  cmd 2> &1
```

## Function Braces
```bash
# Keep function braces (-fn)
shfmt -fn script.sh

# Before:
foo() {
  echo "bar"
}

# After (with -fn):
foo()
{
  echo "bar"
}
```

## Write/Diff Modes
```bash
# Print formatted (default)
shfmt script.sh

# Write in-place
shfmt -w script.sh

# Show diff
shfmt -d script.sh

# Find files that need formatting
shfmt -l .

# List and format
shfmt -l -w .
```

## Common Configurations
```bash
# Google shell style
shfmt -i 2 -bn -ci -sr script.sh

# Standard bash
shfmt -i 4 -bn script.sh

# POSIX sh
shfmt -ln posix -i 2 script.sh

# My preferred style
shfmt -i 2 -ci -bn script.sh
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Check shell scripts formatting
  run: |
    shfmt -d .
    if [ $? -ne 0 ]; then
      echo "Scripts need formatting. Run: shfmt -w ."
      exit 1
    fi

# Auto-format
- name: Format scripts
  run: shfmt -w -i 2 -ci -bn .

# GitLab CI
format:check:
  image: mvdan/shfmt:latest
  script:
    - shfmt -d .
  allow_failure: false

format:fix:
  image: mvdan/shfmt:latest
  script:
    - shfmt -w -i 2 .
  when: manual
```

## Pre-commit Hook
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/scop/pre-commit-shfmt
    rev: v3.8.0-1
    hooks:
      - id: shfmt
        args: [-w, -i, '2', -ci]
```

## Git Hook Script
```bash
#!/bin/bash
# .git/hooks/pre-commit

files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.sh$')

if [ -n "$files" ]; then
  shfmt -l -d -i 2 -ci $files
  if [ $? -ne 0 ]; then
    echo "Shell scripts need formatting. Run: shfmt -w -i 2 -ci ."
    exit 1
  fi
fi
```

## Editor Integration
```bash
# VS Code
# Install: shell-format extension
# Settings.json:
{
  "shellformat.flag": "-i 2 -ci -bn"
}

# Vim/Neovim
autocmd FileType sh setlocal formatprg=shfmt\ -i\ 2\ -ci

# Format with gq
:!shfmt -w %

# Emacs
(add-hook 'sh-mode-hook
  (lambda ()
    (add-hook 'before-save-hook
      (lambda ()
        (when (eq major-mode 'sh-mode)
          (shell-command-on-region
            (point-min) (point-max)
            "shfmt -i 2 -ci" t t))))))
```

## Batch Processing
```bash
# Format all scripts
find . -name '*.sh' -exec shfmt -w -i 2 -ci {} \;

# Format git tracked files
git ls-files '*.sh' | xargs shfmt -w -i 2

# Format changed files only
git diff --name-only | grep '\.sh$' | xargs shfmt -w

# Format and commit
shfmt -w . && git add -u && git commit -m "format shell scripts"
```

## Comparison
| Feature | shfmt | beautysh | bashfmt |
|---------|-------|----------|---------|
| Speed | Fast | Slow | Moderate |
| Styles | Configurable | Limited | Limited |
| Languages | sh/bash/mksh | bash | bash |
| Active | Yes | No | No |
| Install | Single binary | pip | npm |

## Before/After Example
```bash
# Before
if   [    "$var"=="value"   ];  then
echo   "hello"
   fi

# After (shfmt -i 2 -ci)
if [ "$var" == "value" ]; then
  echo "hello"
fi

# Before
case  $var  in
a  )  echo  "a" ;;
b) echo "b";;
esac

# After (shfmt -i 2 -ci)
case $var in
  a)
    echo "a"
    ;;
  b)
    echo "b"
    ;;
esac
```

## Advanced Usage
```bash
# Keep padding
shfmt -kp script.sh

# Simplify code
shfmt -s script.sh

# Minify (single line)
shfmt -mn script.sh

# Parse but don't format
shfmt --filename script.sh < /dev/stdin

# Output as JSON
shfmt --tojson script.sh

# Custom parser
shfmt --from-json < input.json
```

## Best Practices
- **Run in CI** to enforce consistent style
- **Use .editorconfig** for team settings
- **Format before commit** via pre-commit hook
- **Use -i 2 or -i 4** (spaces more portable than tabs)
- **Add -ci** for case indentation
- **Use -bn** for binary operators
- **Check with -d** before committing

## Tips
- Fastest shell formatter
- Single binary (no dependencies)
- Works with stdin/stdout
- Handles complex syntax
- Preserves comments
- Safe (doesn't change logic)
- Integrates with most editors

## Advanced Configuration

### Configuration File
```bash
# .editorconfig
[*.sh]
indent_style = space
indent_size = 2
binary_next_line = true
switch_case_indent = true
space_redirects = true
```

### GitHub Actions Workflow
```yaml
name: Shell Format
on: [pull_request]
jobs:
  shfmt:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run shfmt
        run: |
          curl -L https://github.com/mvdan/sh/releases/latest/download/shfmt_linux_amd64 -o shfmt
          chmod +x shfmt
          ./shfmt -d -i 2 -ci .
```

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (binary, Scoop)
- ✅ BSD systems
- ✅ Docker (mvdan/shfmt image)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove shfmt |

## Agent Use
- Automated code formatting
- CI/CD quality checks
- Pre-commit validation
- Code review automation
- Style enforcement
- Repository cleanup

## Uninstall
```yaml
- preset: shfmt
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/mvdan/sh
- Playground: https://github.com/mvdan/sh#shfmt
- Search: "shfmt options", "shell script formatter"

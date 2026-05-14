# entr - Run Commands When Files Change

File watcher that executes commands when files are modified.

## Quick Start
```yaml
- preset: entr
```

## Features
- **Simple**: One command, powerful results
- **Fast**: Minimal overhead and resource usage
- **Cross-platform**: Linux, macOS, BSD
- **Non-interactive**: Works in CI/CD and scripts
- **Clear output**: Shows what changed and what ran
- **Shell-free**: Direct command execution

## Basic Usage
```bash
# Watch files and run command
ls *.c | entr make

# Clear screen before running
ls *.py | entr -c pytest

# Restart server on change
ls *.go | entr -r go run main.go

# Interactive shell mode
ls *.sh | entr -s 'shellcheck $0 && bash $0'

# Watch directory recursively
find . -name '*.js' | entr npm test

# Pass changed file to command
ls *.md | entr -p pandoc /_ -o output.pdf
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ BSD

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Options
```bash
-c  # Clear screen before running command
-r  # Restart command if it's still running
-s  # Invoke shell (allows pipes, redirection)
-p  # Postpone first execution until file changes
-n  # Run once and exit after first change
-d  # Track directories and new files
-z  # Exit after command returns zero
```

## Real-World Examples

### Web Development
```bash
# Auto-rebuild CSS
ls *.scss | entr -c sass style.scss style.css

# Restart Node.js server
ls *.js | entr -r node server.js

# Auto-reload browser (with live-server)
ls *.html *.css | entr -c browser-sync reload
```

### Testing
```bash
# Run tests on change
find . -name '*.py' | entr -c pytest

# Run specific test
ls app.py test_app.py | entr pytest test_app.py

# Lint and test
ls *.js | entr -c sh -c 'eslint $0 && jest'
```

### Build Automation
```bash
# Rebuild Go binary
ls *.go | entr -c go build

# Compile C program
ls *.c *.h | entr make

# Build Docker image
ls Dockerfile *.go | entr docker build -t myapp .
```

### Documentation
```bash
# Regenerate docs
ls *.md | entr -c mkdocs build

# Convert markdown to PDF
ls README.md | entr -p pandoc /_ -o README.pdf

# Rebuild API docs
find ./src -name '*.py' | entr sphinx-build -b html docs build
```

### Static Site Generation
```bash
# Rebuild Jekyll site
find . -name '*.md' -o -name '*.html' | entr -c jekyll build

# Regenerate static site
ls content/* | entr hugo
```

## Agent Use
- Auto-run tests during development
- Rebuild applications on code changes
- Regenerate documentation
- Restart services automatically
- Continuous integration workflows
- Hot reloading development environments

## Troubleshooting

### Too many files
```bash
# Use find with reasonable depth
find . -maxdepth 3 -name '*.py' | entr pytest

# Or be more specific
find src -name '*.py' | entr pytest
```

### Command not restarting
```bash
# Use -r flag to restart
ls *.js | entr -r node server.js
```


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install entr
  preset: entr

- name: Use entr in automation
  shell: |
    # Custom configuration here
    echo "entr configured"
```
## Uninstall
```yaml
- preset: entr
  with:
    state: absent
```

## Resources
- Official site: https://eradman.com/entrproject/
- GitHub: https://github.com/eradman/entr
- Search: "entr tutorial", "entr examples"

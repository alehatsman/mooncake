# watchexec - Execute Commands When Files Change

Fast, cross-platform file watcher that executes commands in response to filesystem changes.

## Quick Start
```yaml
- preset: watchexec
```

## Features
- **Fast**: Written in Rust for minimal overhead and quick response
- **Smart Filtering**: Gitignore support, custom glob patterns
- **Debouncing**: Configurable delay to batch rapid changes
- **Cross-platform**: Linux, macOS, Windows, BSD
- **Flexible**: Custom commands, environment variables, working directory
- **Signal Handling**: Graceful process termination and restart

## Basic Usage
```bash
# Watch current directory, run command on any change
watchexec echo "File changed"

# Watch specific files
watchexec --exts rs,toml cargo test

# Watch directory and subdirectories
watchexec --watch src npm run build

# Run shell command
watchexec 'cargo check && cargo test'

# Clear screen before each run
watchexec --clear npm test

# Restart long-running process
watchexec --restart --signal SIGTERM npm start

# Multiple commands
watchexec --shell=bash 'npm run lint && npm run test'
```

## Advanced Configuration
```yaml
- preset: watchexec
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove watchexec |

## Platform Support
- ✅ Linux (apt, dnf, pacman, binary)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (binary, Scoop)
- ✅ BSD (pkg, binary)

## Configuration
- **No config file**: All configuration via CLI flags
- **Gitignore**: Respects .gitignore by default
- **Debounce**: Default 50ms delay

## Real-World Examples

### Frontend Development
```bash
# Watch source files, rebuild on change
watchexec --watch src --exts js,jsx,ts,tsx npm run build

# Development server with auto-reload
watchexec --restart --watch src npm run dev

# Run linter and tests
watchexec --clear --watch src 'npm run lint && npm test'
```

### Backend Development
```bash
# Rust development
watchexec --watch src --exts rs cargo check

# Go development with restart
watchexec --restart --signal SIGTERM --watch . --exts go go run main.go

# Python development
watchexec --watch app --exts py pytest

# Node.js with auto-restart
watchexec --restart --watch src --exts js,json node server.js
```

### Documentation
```bash
# Build docs on change
watchexec --watch docs --exts md mkdocs build

# Live preview
watchexec --restart --watch docs 'mkdocs serve --dev-addr 0.0.0.0:8000'

# Markdown to PDF
watchexec --watch content --exts md pandoc input.md -o output.pdf
```

### CI/CD Development
```bash
# Test CI configuration
watchexec --watch .github/workflows --exts yml act

# Validate Kubernetes manifests
watchexec --watch k8s --exts yaml kubeval

# Terraform validation
watchexec --watch infra --exts tf terraform validate
```

### Multiple Watch Paths
```bash
# Watch multiple directories
watchexec --watch src --watch tests --watch config cargo test

# Ignore specific paths
watchexec --ignore target --ignore node_modules npm test

# Complex filtering
watchexec \
  --watch src \
  --watch tests \
  --ignore '*.tmp' \
  --ignore 'target/*' \
  --exts rs \
  cargo test
```

## Agent Use
- Automated testing during development
- Continuous build and validation
- Documentation regeneration
- Configuration validation on change
- Development server auto-restart
- Code quality checks on save

## Advanced Options

```bash
# Debounce (wait before executing)
watchexec --debounce 500ms cargo check

# Execute on startup
watchexec --on-busy-update=restart npm test

# Custom signal for process termination
watchexec --restart --signal SIGUSR1 ./myapp

# Change working directory
watchexec --workdir /app npm test

# Set environment variables
watchexec --env FOO=bar --env BAZ=qux ./script.sh

# Postpone first execution
watchexec --postpone npm build

# Watch for creation/deletion only
watchexec --no-vcs-ignore --watch . echo "Changed"
```

## Filtering

```bash
# File extensions
watchexec --exts rs,toml cargo test

# Glob patterns
watchexec --filter '*.js' --filter '*.json' npm test

# Ignore patterns
watchexec --ignore '*.log' --ignore 'tmp/*' npm build

# Disable gitignore
watchexec --no-vcs-ignore npm test

# Disable default ignores
watchexec --no-default-ignore npm test
```

## Troubleshooting

### Too many file events
```bash
# Increase debounce time
watchexec --debounce 1s command

# Use more specific filters
watchexec --exts rs --watch src cargo check
```

### Process not restarting properly
```bash
# Use different signal
watchexec --restart --signal SIGKILL command

# Add delay before restart
watchexec --restart --delay-run 500ms command
```

### Missing file changes
```bash
# Check gitignore
cat .gitignore

# Disable gitignore
watchexec --no-vcs-ignore command

# Increase verbosity to debug
watchexec --verbose command
```

## Comparison with Other Tools

| Feature | watchexec | nodemon | entr | inotifywait |
|---------|-----------|---------|------|-------------|
| Speed | ⚡ Fastest | Medium | Fast | Fast |
| Cross-platform | ✅ Yes | ✅ Yes | ❌ Unix only | ❌ Linux only |
| Filtering | ✅ Advanced | ✅ Good | ❌ Basic | ❌ Basic |
| Restart | ✅ Yes | ✅ Yes | ❌ No | ❌ No |
| Gitignore | ✅ Yes | ✅ Yes | ❌ No | ❌ No |

## Uninstall
```yaml
- preset: watchexec
  with:
    state: absent
```

## Resources
- Official docs: https://watchexec.github.io/
- GitHub: https://github.com/watchexec/watchexec
- Search: "watchexec examples", "watchexec tutorial"

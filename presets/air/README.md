# air - Live Reload for Go

Hot reload for Go applications. Watch for file changes, rebuild and restart automatically during development.

## Quick Start
```yaml
- preset: air
```

## Basic Usage
```bash
# Initialize config
air init

# Run with live reload
air

# Custom config
air -c .air.toml

# With build arguments
air -d
```

## Configuration File
```toml
# .air.toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  # Binary name
  bin = "./tmp/main"

  # Command to build
  cmd = "go build -o ./tmp/main ."

  # Delay after build (ms)
  delay = 1000

  # Exclude directories
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]

  # Exclude files
  exclude_file = []

  # Exclude regex
  exclude_regex = ["_test.go"]

  # Exclude unchanged files
  exclude_unchanged = false

  # Follow symlinks
  follow_symlink = false

  # Full build path
  full_bin = ""

  # Include directories (watched)
  include_dir = []

  # Include extensions
  include_ext = ["go", "tpl", "tmpl", "html"]

  # Include files
  include_file = []

  # Kill delay (ms)
  kill_delay = "0s"

  # Log
  log = "build-errors.log"

  # Poll interval (ms)
  poll = false
  poll_interval = 0

  # Rerun
  rerun = false
  rerun_delay = 500

  # Arguments for binary
  args_bin = []

  # Send interrupt before kill
  send_interrupt = false

  # Stop on error
  stop_on_error = false

[color]
  # Customize colors
  main = "magenta"
  watcher = "cyan"
  build = "yellow"
  runner = "green"

[log]
  # Log time
  time = false

  # Main log
  main_only = false

[misc]
  # Clean tmp on exit
  clean_on_exit = false
```

## Quick Config Examples
```toml
# Minimal
[build]
  cmd = "go build -o ./tmp/main ."
  bin = "./tmp/main"
  include_ext = ["go"]
  exclude_dir = ["tmp"]

# Web app with templates
[build]
  cmd = "go build -o ./tmp/main ."
  bin = "./tmp/main"
  include_ext = ["go", "html", "css", "js"]
  exclude_dir = ["tmp", "node_modules", "vendor"]

# API server
[build]
  cmd = "go build -o ./tmp/main ./cmd/api"
  bin = "./tmp/main"
  args_bin = ["-port", "8080"]
  include_ext = ["go"]
  exclude_dir = ["tmp", "vendor"]
```

## Command Line Options
```bash
# Custom config
air -c custom.toml

# Debug mode
air -d

# Build only (no run)
air --build

# Version
air -v
```

## Watch Patterns
```toml
# Watch specific directories
[build]
  include_dir = ["cmd", "internal", "pkg"]

# Watch specific extensions
[build]
  include_ext = ["go", "mod", "sum"]

# Exclude test files
[build]
  exclude_regex = [".*_test\\.go$"]

# Exclude directories
[build]
  exclude_dir = ["vendor", "tmp", "testdata", ".git"]
```

## Build Commands
```toml
# Standard build
[build]
  cmd = "go build -o ./tmp/main ."

# With tags
[build]
  cmd = "go build -tags=dev -o ./tmp/main ."

# With flags
[build]
  cmd = "go build -ldflags='-X main.Version=dev' -o ./tmp/main ."

# Multiple commands
[build]
  cmd = "make build"

# Generate + build
[build]
  cmd = "go generate && go build -o ./tmp/main ."
```

## Binary Arguments
```toml
# Pass args to binary
[build]
  bin = "./tmp/main"
  args_bin = [
    "-port", "3000",
    "-env", "development",
    "-debug"
  ]
```

## Development Workflows
```bash
# Start with air
air

# In one terminal
air

# In another terminal
curl http://localhost:8080

# Edit code -> auto rebuild & restart
vim main.go
```

## Integration Examples
```bash
# With Docker
docker run -it --rm \
  -v $(pwd):/app \
  -w /app \
  -p 8080:8080 \
  golang:1.21 \
  sh -c "go install github.com/cosmtrek/air@latest && air"

# With docker-compose
services:
  app:
    image: golang:1.21
    volumes:
      - .:/app
    working_dir: /app
    ports:
      - "8080:8080"
    command: air

# With Makefile
dev:
    @air

# With npm scripts (monorepo)
"scripts": {
  "dev:api": "cd api && air",
  "dev": "concurrently 'npm:dev:*'"
}
```

## Project Structure
```
myproject/
├── .air.toml           # Air configuration
├── tmp/                # Build output (excluded from git)
│   └── main           # Binary
├── cmd/
│   └── main.go
├── internal/
│   ├── handlers/
│   └── models/
└── templates/          # Watched for changes
    └── index.html
```

## Multiple Configurations
```bash
# Development
air -c .air.dev.toml

# Production-like
air -c .air.prod.toml

# Testing
air -c .air.test.toml
```

## Performance Tuning
```toml
[build]
  # Faster builds
  poll = false

  # Increase delay for slower machines
  delay = 2000

  # Exclude more directories
  exclude_dir = ["vendor", "tmp", "node_modules", ".git", "testdata"]

  # Kill faster
  kill_delay = "100ms"
```

## Debugging
```toml
[build]
  # Log build output
  log = "build-errors.log"

  # Stop on errors
  stop_on_error = true

  # Send interrupt for graceful shutdown
  send_interrupt = true
```

## Common Issues
```bash
# Port already in use
# Solution: Kill process or use different port in args_bin

# Changes not detected
# Solution: Check include_ext and exclude_dir

# Build too slow
# Solution: Exclude more dirs, use poll=false

# Binary not starting
# Solution: Check bin path and permissions
```

## CI/CD
```yaml
# Not typically used in CI (for dev only)
# But can be used for integration tests

- name: Run with air
  run: |
    air &
    sleep 5
    curl http://localhost:8080/health
    pkill -f "tmp/main"
```

## Comparison
| Feature | air | nodemon | watchexec | entr |
|---------|-----|---------|-----------|------|
| Go-specific | Yes | No | No | No |
| Auto-restart | Yes | Yes | Yes | No |
| Config file | Yes | Yes | No | No |
| Speed | Fast | Moderate | Fast | Fast |

## Best Practices
- **Add tmp/ to .gitignore** (build output)
- **Use .air.toml** for project-specific config
- **Exclude vendor/** for faster reloads
- **Set appropriate delay** (1000ms usually good)
- **Use args_bin** for consistent flags
- **Enable stop_on_error** during development
- **Exclude test files** unless testing

## Tips
- Faster than manually rebuilding
- Works with any Go project structure
- Watches templates, configs, and Go files
- Graceful shutdown support
- Cross-platform
- Zero dependencies (single binary)
- Great for rapid iteration

## Agent Use
- Automated development environments
- Local testing workflows
- Rapid prototyping
- Integration test setups
- Demo environments
- Development automation

## Uninstall
```yaml
- preset: air
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/cosmtrek/air
- Search: "air golang live reload", "air config examples"

# just - Command Runner

Modern command runner with a clean syntax. Save and run project commands without shell script boilerplate.

## Quick Start
```yaml
- preset: just
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage
```bash
# List available recipes
just --list
just -l

# Run a recipe
just build
just test
just deploy

# Run recipe with arguments
just serve 8080
just test unit integration

# Run from subdirectory (finds justfile in parent dirs)
just build
```


## Advanced Configuration
```yaml
- preset: just
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove just |
## Justfile Syntax
```justfile
# Simple recipe
build:
    cargo build --release

# Recipe with parameters
serve PORT:
    python -m http.server {{PORT}}

# Recipe with default parameters
test FILTER="":
    cargo test {{FILTER}}

# Multi-line recipe
deploy ENV:
    docker build -t myapp .
    docker tag myapp myapp:{{ENV}}
    docker push myapp:{{ENV}}

# Recipe that calls other recipes
all: build test deploy

# Recipe with dependencies
deploy: build test
    kubectl apply -f k8s/

# Private recipe (doesn't show in --list)
_prepare:
    mkdir -p build/

# Recipe with environment variables
docker-build:
    #!/usr/bin/env bash
    set -euxo pipefail
    docker build -t myapp:$(git rev-parse --short HEAD) .
```

## Common Patterns
```justfile
# Default recipe (runs when just called with no args)
default:
    @just --list

# Variables
image_name := "myapp"
version := `git rev-parse --short HEAD`

build:
    docker build -t {{image_name}}:{{version}} .

# Conditional execution
test:
    @if [ -f "go.mod" ]; then go test ./...; fi
    @if [ -f "package.json" ]; then npm test; fi

# Working directory
@package:
    cd dist && tar -czf app.tar.gz *

# Error suppression
@clean:
    -rm -rf build/
    -docker rmi myapp 2>/dev/null

# Print recipe before running
install:
    npm install
    pip install -r requirements.txt

# Recipe aliases
alias b := build
alias t := test
alias d := deploy
```

## Real-World Examples

### Development Workflow
```justfile
# Development commands
dev:
    cargo watch -x run

fmt:
    cargo fmt
    prettier --write .

lint:
    cargo clippy
    eslint .

fix: fmt lint
```

### CI/CD Pipeline
```justfile
ci: lint test build

lint:
    golangci-lint run

test:
    go test -v -race -coverprofile=coverage.out ./...

build:
    go build -o bin/app cmd/main.go

coverage:
    go tool cover -html=coverage.out
```

### Docker Workflow
```justfile
image := "myapp"
tag := `git describe --tags --always`

build:
    docker build -t {{image}}:{{tag}} .

push: build
    docker push {{image}}:{{tag}}

run:
    docker run -p 8080:8080 {{image}}:{{tag}}

shell:
    docker run -it {{image}}:{{tag}} /bin/bash
```

## Advanced Features
```justfile
# Set shell
set shell := ["bash", "-c"]

# Set dotenv file
set dotenv-load

# Allow positional arguments
set positional-arguments

# Recipe with positional args
test *ARGS:
    pytest $@

# Multiline strings
readme := """
This is a multiline
string value
"""

# Functions
test:
    echo "Current directory: {{justfile_directory()}}"
    echo "OS: {{os()}}"
    echo "Arch: {{arch()}}"
```

## Tips
- **Place at project root**: `justfile` or `.justfile`
- **Tab or spaces**: Use your preference (unlike Make)
- **@-prefix**: Suppress command echo
- **-prefix**: Continue on error
- **Variables**: Use `{{var}}` for interpolation
- **Backticks**: Use for command substitution `` `cmd` ``
- **Recipes run independently**: Each line is a separate shell invocation

## vs Make
| Feature | just | Make |
|---------|------|------|
| Syntax | Clean, intuitive | Terse, cryptic |
| Tabs | Optional | Required |
| Default shell | User's shell | sh |
| Error messages | Clear | Confusing |
| Purpose | Command runner | Build automation |

## Justfile Example (Complete)
```justfile
# Default recipe
default:
    @just --list

# Variables
app_name := "myapp"
version := `git describe --tags --always`

# Development
dev:
    cargo watch -x "run -- --debug"

# Testing
test *ARGS:
    cargo test {{ARGS}}

# Building
build:
    cargo build --release

# Docker
docker-build: build
    docker build -t {{app_name}}:{{version}} .

docker-run: docker-build
    docker run -p 8080:8080 {{app_name}}:{{version}}

# Deployment
deploy ENV: test build
    kubectl set image deployment/{{app_name}} app={{app_name}}:{{version}} -n {{ENV}}

# Cleanup
clean:
    cargo clean
    rm -rf target/

# Aliases
alias b := build
alias t := test
alias r := dev
```

## Configuration
- **No config file** - Settings in justfile
- **Shell**: Set per justfile with `set shell := [...]`
- **Environment**: Load from `.env` with `set dotenv-load`

## Agent Use
- Project-specific command standardization
- Cross-platform development workflows
- CI/CD pipeline definitions
- Automated deployment procedures

## Uninstall
```yaml
- preset: just
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/casey/just
- Docs: https://just.systems/man/en/
- Search: "just command runner examples"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

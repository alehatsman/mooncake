# Go - Systems Programming Language

Modern, fast, compiled language with built-in concurrency. Build reliable and efficient software that scales. From Google, powering Kubernetes, Docker, and Mooncake itself.

## Quick Start
```yaml
- preset: go
```

## Features
- **Fast compilation**: Build millions of lines in seconds
- **Static binaries**: Single executable, no dependencies
- **Cross-compilation**: Build for any platform from anywhere
- **Built-in concurrency**: Goroutines and channels for parallelism
- **Garbage collected**: Memory safety without manual management
- **Standard library**: Batteries included (HTTP, JSON, crypto, testing)
- **Go modules**: Dependency management built-in
- **Simple syntax**: Easy to learn, readable code

## Basic Usage
```bash
# Check version
go version

# Initialize a new module
go mod init github.com/username/myproject

# Run code directly
go run main.go

# Build binary
go build

# Build with custom output
go build -o myapp

# Install binary to $GOPATH/bin
go install

# Format code
go fmt ./...

# Run tests
go test ./...

# Get dependencies
go get github.com/pkg/errors
```

## Project Structure
```
myproject/
├── go.mod              # Module definition
├── go.sum              # Dependency checksums
├── main.go             # Main package
├── internal/           # Private code
│   └── pkg/
├── pkg/                # Public libraries
│   └── api/
├── cmd/                # Command-line tools
│   └── myapp/
└── testdata/           # Test fixtures
```

## Advanced Configuration

### Install with version verification
```yaml
- preset: go
  with:
    state: present
```

### Development environment
```yaml
- name: Setup Go development environment
  preset: go

- name: Install Go tools
  shell: |
    go install golang.org/x/tools/gopls@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install github.com/go-delve/delve/cmd/dlv@latest
```

### CI/CD environment
```yaml
- name: Install Go for CI
  preset: go
  become: true

- name: Cache Go modules
  file:
    path: /home/runner/go/pkg/mod
    state: directory
    mode: '0755'
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Go |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (use official installer)

## Go Modules

### Initialize module
```bash
# Create new module
go mod init github.com/user/project

# Add dependency
go get github.com/gin-gonic/gin@v1.9.1

# Update dependencies
go get -u ./...

# Clean up unused dependencies
go mod tidy

# Verify dependencies
go mod verify

# Download dependencies
go mod download
```

### go.mod example
```go
module github.com/user/myproject

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/stretchr/testify v1.8.4
)
```

### Replace directive (local development)
```go
replace github.com/user/dep => ../dep
```

## Building

### Basic build
```bash
# Build current directory
go build

# Build specific package
go build ./cmd/myapp

# Build with output name
go build -o myapp ./cmd/myapp
```

### Optimized builds
```bash
# Smaller binaries (strip debug info)
go build -ldflags="-s -w" -o myapp

# Inject version info
go build -ldflags="-X main.version=1.0.0" -o myapp

# Remove file paths (reproducible builds)
go build -trimpath -o myapp

# Static binary (no CGO)
CGO_ENABLED=0 go build -o myapp

# Full optimization
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o myapp
```

### Cross-compilation
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o myapp-linux-amd64

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o myapp-linux-arm64

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o myapp-darwin-amd64

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o myapp-darwin-arm64

# Windows
GOOS=windows GOARCH=amd64 go build -o myapp.exe

# All platforms at once
GOOS=linux GOARCH=amd64 go build -o dist/myapp-linux-amd64 && \
GOOS=darwin GOARCH=arm64 go build -o dist/myapp-darwin-arm64 && \
GOOS=windows GOARCH=amd64 go build -o dist/myapp-windows.exe
```

## Testing

### Run tests
```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# With coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# Run specific test
go test -run TestMyFunction

# Test with race detector
go test -race ./...

# Benchmark tests
go test -bench=. ./...

# Parallel tests
go test -parallel 4 ./...
```

### Test flags
```bash
# Short mode (skip slow tests)
go test -short ./...

# Timeout
go test -timeout 30s ./...

# Fail fast
go test -failfast ./...

# Cache disable
go test -count=1 ./...
```

## Development Tools

### Essential tools
```bash
# Language server (IDE support)
go install golang.org/x/tools/gopls@latest

# Linter (comprehensive checks)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Debugger
go install github.com/go-delve/delve/cmd/dlv@latest

# Code generator
go install github.com/golang/mock/mockgen@latest

# Live reload
go install github.com/cosmtrek/air@latest
```

### Formatting and linting
```bash
# Format code (modifies files)
go fmt ./...
gofmt -w .

# Format with stricter rules
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .

# Vet code (suspicious constructs)
go vet ./...

# Lint with golangci-lint
golangci-lint run

# Lint with auto-fix
golangci-lint run --fix
```

## Environment Variables

### Important variables
```bash
# View all Go environment
go env

# Module cache
go env GOMODCACHE  # ~/.go/pkg/mod by default

# Binary install location
go env GOPATH      # ~/go by default

# Go installation
go env GOROOT      # /usr/local/go or similar
```

### Custom configuration
```bash
# Set module proxy
go env -w GOPROXY=https://proxy.golang.org,direct

# Private modules (skip proxy)
go env -w GOPRIVATE=github.com/myorg/*

# Disable CGO
go env -w CGO_ENABLED=0

# Add $GOPATH/bin to PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

## Use Cases

### Local Development
```yaml
- name: Setup Go development environment
  preset: go

- name: Install development tools
  shell: |
    go install golang.org/x/tools/gopls@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install github.com/go-delve/delve/cmd/dlv@latest
    go install github.com/cosmtrek/air@latest

- name: Clone and build project
  shell: |
    git clone https://github.com/user/project
    cd project
    go mod download
    go build -o myapp
```

### CI/CD Pipeline (GitHub Actions)
```yaml
- name: Setup Go for CI
  preset: go
  become: true

- name: Build and test
  shell: |
    go mod download
    go test -race -coverprofile=coverage.out ./...
    go build -trimpath -ldflags="-s -w" -o dist/myapp

- name: Upload coverage
  shell: |
    go tool cover -html=coverage.out -o coverage.html
```

### Cross-Platform Release Build
```yaml
- name: Install Go
  preset: go

- name: Build for all platforms
  shell: |
    mkdir -p dist

    # Linux AMD64
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/myapp-linux-amd64

    # Linux ARM64
    GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/myapp-linux-arm64

    # macOS Intel
    GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/myapp-darwin-amd64

    # macOS Apple Silicon
    GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/myapp-darwin-arm64

    # Windows
    GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/myapp-windows.exe

- name: Create checksums
  shell: |
    cd dist
    sha256sum * > checksums.txt
```

## Configuration Files

### .golangci.yml (linter config)
```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - typecheck
    - unused
    - gofmt
    - goimports

issues:
  exclude-use-default: false
```

### .air.toml (live reload)
```toml
[build]
  cmd = "go build -o ./tmp/main ."
  bin = "tmp/main"
  include_ext = ["go", "tpl", "tmpl", "html"]
  exclude_dir = ["assets", "tmp", "vendor"]
```

## Workspaces (Multi-Module)

### go.work example
```go
go 1.21

use (
    ./api
    ./worker
    ./shared
)
```

### Commands
```bash
# Initialize workspace
go work init ./module1 ./module2

# Add module to workspace
go work use ./module3

# Sync workspace
go work sync
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Go
  preset: go
```

### With development tools
```yaml
- name: Setup Go development
  preset: go

- name: Install Go tools
  shell: |
    go install golang.org/x/tools/gopls@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  register: tools_result

- name: Verify installation
  shell: go version && gopls version
```

### Production build environment
```yaml
- name: Install Go
  preset: go
  become: true

- name: Build application
  shell: |
    CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-s -w -X main.version={{ version }}" \
      -o /usr/local/bin/myapp
  become: true
```

## Agent Use
- **Development setup**: Install Go + tools for local development
- **CI/CD**: Automated testing and building in pipelines
- **Cross-compilation**: Build binaries for multiple platforms
- **Static analysis**: Integrate linters and security scanners
- **Dependency management**: Module vendoring and verification
- **Performance profiling**: CPU and memory profiling in production
- **Microservices deployment**: Build and deploy Go services

## Troubleshooting

### Command not found after install
```bash
# Add GOPATH/bin to PATH
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# Or for zsh
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
source ~/.zshrc
```

### Module checksum mismatch
```bash
# Clean module cache
go clean -modcache

# Re-download modules
go mod download

# Verify checksums
go mod verify
```

### Cannot find package
```bash
# Ensure module mode is enabled
go env GO111MODULE  # Should be "on" or ""

# Sync dependencies
go mod tidy

# Download missing dependencies
go mod download
```

### CGO errors
```bash
# Disable CGO if not needed
CGO_ENABLED=0 go build

# Or set permanently
go env -w CGO_ENABLED=0

# Install CGO dependencies (Ubuntu)
sudo apt-get install build-essential
```

### Import cycle detected
```bash
# Refactor code to break circular dependencies
# Move shared code to a new package
# Use interfaces to invert dependencies
```

### Build is slow
```bash
# Use build cache (automatic in Go 1.10+)
go env GOCACHE

# Parallel compilation (automatic)
# Go uses all CPU cores by default

# Skip test cache for fresh run
go test -count=1 ./...
```

## Best Practices
- **Use Go modules**: Always initialize with `go mod init`
- **Vendor dependencies**: `go mod vendor` for reproducibility
- **Pin versions**: Specify exact versions in go.mod for production
- **Small interfaces**: Prefer small, focused interfaces
- **Error handling**: Always check errors, don't ignore them
- **Code formatting**: Run `go fmt` and `goimports` before commit
- **Static analysis**: Use `golangci-lint` in CI pipeline
- **Table-driven tests**: Use test tables for multiple cases
- **Race detector**: Run tests with `-race` flag
- **Semantic import versioning**: Use `/v2`, `/v3` for major versions

## Common Patterns

### HTTP server
```go
package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Graceful shutdown
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server error: %v", err)
    }
}()

<-ctx.Done()
log.Println("Shutting down gracefully...")
srv.Shutdown(context.Background())
```

## Uninstall
```yaml
- preset: go
  with:
    state: absent
```

**Note**: This removes Go but preserves `~/go` (GOPATH) and installed binaries.

## Resources
- Official: https://go.dev/
- Documentation: https://go.dev/doc/
- Tour: https://go.dev/tour/
- Effective Go: https://go.dev/doc/effective_go
- Go by Example: https://gobyexample.com/
- Standard Library: https://pkg.go.dev/std
- Search: "golang tutorial", "go programming", "golang best practices"

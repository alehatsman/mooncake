# Go Preset

**Status:** ✓ Installed successfully

## Quick Start

```yaml
- preset: go
```

```bash
# Check version
go version

# Create a new project
mkdir myproject && cd myproject
go mod init github.com/user/myproject

# Run Go code
go run main.go

# Build binary
go build -o myapp
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`

## Basic Usage
```bash
# Check Go version
go version

# Initialize a new module
go mod init example.com/myproject

# Build your program
go build

# Run your program
go run main.go

# Format code
go fmt ./...

# Run tests
go test ./...

# Install dependencies
go mod download
go mod tidy
```

## Environment

```bash
# View Go environment
go env

# Important paths
go env GOPATH  # ~/go by default
go env GOROOT  # Go installation path
```

## GOPATH Structure

```
$GOPATH/
├── bin/     # Compiled executables
├── pkg/     # Package objects
└── src/     # Source code
```

## Common Operations

```bash
# Install a package
go get github.com/user/package

# Install binary to $GOPATH/bin
go install github.com/user/package@latest

# Update dependencies
go get -u ./...
go mod tidy

# Run tests
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...
```

## Add to PATH

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

## Agent Use
- Automated environment setup
- CI/CD pipeline integration
- Development environment provisioning
- Infrastructure automation

## Uninstall

```yaml
- preset: go
  with:
    state: absent
```

**Note:** GOPATH directory is preserved after uninstall.

## Advanced Configuration
```yaml
- preset: go
  with:
    state: present
```


## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove go |
## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Resources
- Search: "go documentation", "go tutorial"

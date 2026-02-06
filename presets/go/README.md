# Go Preset

**Status:** ✓ Installed successfully

## Quick Start

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

## Uninstall

```yaml
- preset: go
  with:
    state: absent
```

**Note:** GOPATH directory is preserved after uninstall.

# gox - Go Cross Compilation Tool

Dead simple, no frills Go cross compile tool. Build Go binaries for multiple platforms in parallel.

## Quick Start
```yaml
- preset: gox
```

## Features
- **Parallel builds**: Compile for multiple platforms simultaneously
- **Simple interface**: Easier than standard Go cross compilation
- **All platforms**: Build for any Go-supported OS/architecture
- **Custom output**: Control output paths and filenames
- **Fast**: Parallel compilation speeds up multi-platform builds
- **Zero config**: Works out of the box for most projects

## Basic Usage
```bash
# Build for all platforms
gox

# Build for specific platforms
gox -osarch="linux/amd64 darwin/amd64 windows/amd64"

# Build for specific OS
gox -os="linux darwin windows"

# Build for specific architecture
gox -arch="amd64 arm64"

# Custom output template
gox -output="dist/{{.OS}}_{{.Arch}}/{{.Dir}}"

# Parallel builds (default: number of CPUs)
gox -parallel=4

# Verbose output
gox -verbose
```

## Advanced Configuration
```yaml
- preset: gox
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove gox |

## Platform Support
- ✅ Linux (go install, binary download)
- ✅ macOS (Homebrew, go install)
- ✅ Windows (go install, Scoop)

## Configuration
- **No config file**: All options via CLI flags
- **Environment**: Uses Go build environment
- **Output**: `./{{.OS}}_{{.Arch}}/{{.Dir}}` (default)

## Real-World Examples

### CLI Tool Release
```bash
# Build release binaries
gox \
  -osarch="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64" \
  -output="dist/{{.Dir}}_{{.OS}}_{{.Arch}}" \
  -ldflags="-s -w -X main.version=1.0.0"

# Result:
# dist/myapp_linux_amd64
# dist/myapp_linux_arm64
# dist/myapp_darwin_amd64
# dist/myapp_darwin_arm64
# dist/myapp_windows_amd64.exe
```

### Multi-Architecture Docker
```bash
# Build for Docker platforms
gox -osarch="linux/amd64 linux/arm64" -output="bin/{{.OS}}_{{.Arch}}/app"

# Create multi-arch Docker images
docker buildx build --platform linux/amd64,linux/arm64 -t myapp:latest .
```

### GitHub Release Workflow
```bash
#!/bin/bash
VERSION=$1

# Build all platforms
gox \
  -osarch="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64" \
  -output="release/{{.Dir}}_${VERSION}_{{.OS}}_{{.Arch}}" \
  -ldflags="-X main.Version=${VERSION}"

# Create archives
cd release
for binary in *; do
  if [[ "$binary" == *.exe ]]; then
    zip "${binary%.exe}.zip" "$binary"
  else
    tar czf "${binary}.tar.gz" "$binary"
  fi
done

# Upload to GitHub
gh release create "v${VERSION}" *.{zip,tar.gz}
```

### CI/CD Build Matrix
```yaml
# .github/workflows/build.yml
name: Build
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'

      - name: Install gox
        run: go install github.com/mitchellh/gox@latest

      - name: Build binaries
        run: |
          gox \
            -osarch="linux/amd64 darwin/amd64 windows/amd64" \
            -output="dist/{{.Dir}}_{{.OS}}_{{.Arch}}" \
            -ldflags="-s -w"

      - name: Upload artifacts
        uses: actions/upload-artifact@v2
        with:
          name: binaries
          path: dist/
```

### Custom Build Tags
```bash
# Build with tags
gox -tags="production" -osarch="linux/amd64"

# Multiple tags
gox -tags="production ssl" -osarch="linux/amd64 darwin/amd64"

# CGO builds (slower, not all platforms)
CGO_ENABLED=1 gox -osarch="linux/amd64"
```

### Rebuild Toolchain
```bash
# First time setup for cross compilation
gox -build-toolchain

# List available platforms
gox -osarch-list
```

## Agent Use
- Build release binaries for multiple platforms
- Create distribution packages in CI/CD pipelines
- Generate platform-specific builds for testing
- Automate cross-compilation for Go projects
- Build Docker images for multiple architectures
- Prepare artifacts for GitHub releases

## Troubleshooting

### Build fails for specific platform
```bash
# Check if platform is supported
gox -osarch-list

# Skip failing platforms
gox -osarch="!freebsd/arm"

# Build with verbose output
gox -verbose

# Check CGO requirements
# CGO must be disabled for cross-compilation
CGO_ENABLED=0 gox
```

### Output directory issues
```bash
# Create output directory
mkdir -p dist

# Use absolute path
gox -output="$PWD/dist/{{.Dir}}_{{.OS}}_{{.Arch}}"

# Clean previous builds
rm -rf dist/
gox -output="dist/{{.Dir}}_{{.OS}}_{{.Arch}}"
```

### Slow builds
```bash
# Increase parallelism
gox -parallel=8

# Build fewer platforms
gox -osarch="linux/amd64 darwin/amd64"

# Use go build for single platform (faster)
GOOS=linux GOARCH=amd64 go build
```

### Missing dependencies
```bash
# Ensure Go modules are downloaded
go mod download

# Vendor dependencies
go mod vendor
gox -mod=vendor
```

## Uninstall
```yaml
- preset: gox
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/mitchellh/gox
- Go cross compilation: https://golang.org/doc/install/source#environment
- Search: "gox cross compile", "go build multiple platforms"

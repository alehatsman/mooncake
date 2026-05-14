# GoReleaser - Release Automation for Go

GoReleaser builds, packages, and publishes Go binaries for multiple platforms with support for Docker, Homebrew, Snapcraft, and more—all from a single YAML configuration.

## Quick Start

```yaml
- preset: goreleaser
```

```bash
# Initialize goreleaser in your Go project
goreleaser init

# Test release without publishing
goreleaser release --snapshot --clean

# Create a release
git tag v1.0.0
git push origin v1.0.0
goreleaser release --clean
```

## Features

- **Multi-platform builds**: Build for Linux, macOS, Windows, FreeBSD
- **Multiple formats**: Generate binaries, archives, Docker images, Homebrew formulas
- **Changelog generation**: Automatically create release notes from commits
- **Signing**: Code signing and checksum generation
- **CI-friendly**: Integrates with GitHub Actions, GitLab CI, CircleCI
- **Extensible**: Hooks for custom pre/post-release actions

## Basic Usage

```bash
# Initialize configuration
goreleaser init

# Build without releasing (test locally)
goreleaser build --snapshot --clean

# Create release (requires Git tag)
goreleaser release --clean

# Skip publishing
goreleaser release --snapshot --clean

# Check configuration
goreleaser check

# Generate shell completions
goreleaser completion bash > /etc/bash_completion.d/goreleaser
```

## Advanced Configuration

```yaml
- preset: goreleaser
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove goreleaser |

## Platform Support

- ✅ Linux (binary install)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

Create `.goreleaser.yml` in project root:

```yaml
# .goreleaser.yml
project_name: myapp

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - README.md
      - LICENSE

checksum:
  name_template: 'checksums.txt'

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
```

## Real-World Examples

### GitHub Actions Integration

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v5
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Docker Image Generation

```yaml
# .goreleaser.yml
dockers:
  - image_templates:
      - "ghcr.io/myorg/{{ .ProjectName }}:{{ .Version }}"
      - "ghcr.io/myorg/{{ .ProjectName }}:latest"
    dockerfile: Dockerfile
    build_flag_templates:
      - "--label=org.opencontainers.image.created={{.Date}}"
      - "--label=org.opencontainers.image.revision={{.FullCommit}}"
```

### Homebrew Tap

```yaml
# .goreleaser.yml
brews:
  - name: myapp
    homepage: "https://github.com/myorg/myapp"
    description: "My awesome application"
    license: "MIT"
    repository:
      owner: myorg
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
```

### Multi-Binary Project

```yaml
# .goreleaser.yml - build multiple binaries
builds:
  - id: server
    main: ./cmd/server
    binary: myapp-server
    goos: [linux, darwin]
    goarch: [amd64, arm64]

  - id: client
    main: ./cmd/client
    binary: myapp-client
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
```

## Agent Use

- Automate binary releases in CI/CD pipelines
- Generate multi-platform distributions automatically
- Publish to package managers (Homebrew, Snapcraft)
- Create Docker images alongside binaries
- Generate consistent release artifacts
- Sign binaries and generate checksums

## Troubleshooting

### "Tag is required" error

GoReleaser requires a Git tag:
```bash
git tag v1.0.0
git push origin v1.0.0
```

### Cross-compilation errors

Ensure CGO is disabled for pure Go:
```yaml
builds:
  - env:
      - CGO_ENABLED=0
```

### Missing GITHUB_TOKEN

Set token for GitHub releases:
```bash
export GITHUB_TOKEN=your_github_token
goreleaser release --clean
```

### Configuration validation

```bash
goreleaser check
```

## Uninstall

```yaml
- preset: goreleaser
  with:
    state: absent
```

## Resources

- Official docs: https://goreleaser.com/
- GitHub: https://github.com/goreleaser/goreleaser
- Examples: https://goreleaser.com/cookbooks/
- Search: "goreleaser tutorial", "goreleaser github actions", "goreleaser docker"

# skopeo - Container Image Tool

Inspect, copy, delete, and sign container images without Docker daemon. Advanced registry operations.

## Features
- **Daemon-free**: No Docker daemon required
- **Multi-format**: Docker, OCI, archives, directories
- **Registry operations**: Copy, inspect, delete, sync
- **Multi-architecture**: Full support for multi-arch images
- **Signing**: Built-in image signing and verification
- **Fast**: Optimized for registry operations
- **Sync**: Advanced multi-image synchronization

## Quick Start
```yaml
- preset: skopeo
```

## Basic Usage
```bash
# Inspect remote image
skopeo inspect docker://nginx:latest

# Copy image between registries
skopeo copy docker://nginx:latest docker://registry.local/nginx:latest

# Delete remote image
skopeo delete docker://registry.local/image:tag

# List tags
skopeo list-tags docker://nginx
```

## Image Operations
```bash
# Copy with different tag
skopeo copy docker://nginx:1.21 docker://nginx:stable

# Copy to/from archive
skopeo copy docker://nginx:latest docker-archive:nginx.tar
skopeo copy docker-archive:nginx.tar docker://registry.local/nginx:v1

# Copy to OCI layout
skopeo copy docker://nginx:latest oci:nginx-oci

# Copy from local Docker
skopeo copy docker-daemon:myapp:latest docker://registry.io/myapp:latest
```

## Inspect Images
```bash
# Basic inspection
skopeo inspect docker://nginx:latest

# Raw manifest
skopeo inspect --raw docker://nginx:latest

# Configuration
skopeo inspect --config docker://nginx:latest

# Specific platform
skopeo inspect --override-os=linux --override-arch=arm64 docker://nginx:latest

# Get digest
skopeo inspect docker://nginx:latest | jq -r '.Digest'

# Get layers
skopeo inspect docker://nginx:latest | jq '.Layers'
```

## Authentication
```bash
# Login to registry
skopeo login registry.io -u username -p password

# Use credentials file
skopeo copy --authfile ~/.docker/config.json \
  docker://source/image docker://dest/image

# Per-command credentials
skopeo copy \
  --src-creds user:pass \
  --dest-creds user:pass \
  docker://source/image docker://dest/image

# Logout
skopeo logout registry.io
```

## Registry Synchronization
```bash
# Mirror single image
skopeo sync --src docker --dest docker \
  nginx:latest registry.local/

# Sync all tags
skopeo sync --src docker --dest docker \
  --all nginx registry.local/

# Sync from YAML
cat > sync.yaml <<EOF
nginx:
  - latest
  - stable
  - 1.21
redis:
  - alpine
  - 6.2
EOF

skopeo sync --src yaml --dest docker \
  sync.yaml registry.local/
```

## Multi-Architecture Images
```bash
# Inspect manifest list
skopeo inspect --raw docker://nginx:latest | jq

# Copy preserving all architectures
skopeo copy --all docker://nginx:latest docker://registry.local/nginx:latest

# Copy specific architecture
skopeo copy --override-arch=arm64 \
  docker://nginx:latest docker://registry.local/nginx:arm64

# List available architectures
skopeo inspect docker://nginx:latest | jq -r '.RepoTags[]'
```

## Signing and Verification
```bash
# Sign image
skopeo copy --sign-by fingerprint \
  docker://source/image docker://dest/image

# Copy with signature
skopeo copy --sign-by-sigstore \
  docker://source/image docker://dest/image
```

## CI/CD Integration
```bash
# Promote image
skopeo copy \
  docker://registry.io/app:staging \
  docker://registry.io/app:production

# Multi-registry deployment
for registry in us-east eu-west asia-pacific; do
  skopeo copy docker://build/app:v1 docker://$registry.io/app:v1
done

# Validation before deploy
if skopeo inspect docker://registry.io/app:v1 >/dev/null 2>&1; then
  kubectl set image deployment/app app=registry.io/app:v1
fi
```

## Batch Operations
```bash
# Copy all tags
for tag in $(skopeo list-tags docker://source/repo | jq -r '.Tags[]'); do
  skopeo copy \
    docker://source/repo:$tag \
    docker://dest/repo:$tag
done

# Cleanup old images
skopeo list-tags docker://registry.io/app | jq -r '.Tags[]' | \
  grep -v 'latest\|stable' | sort -V | head -n -10 | \
  xargs -I {} skopeo delete docker://registry.io/app:{}
```

## Format Conversion
```bash
# Docker to OCI
skopeo copy docker://nginx:latest oci:nginx-oci

# OCI to Docker archive
skopeo copy oci:nginx-oci docker-archive:nginx.tar

# Docker daemon to registry
skopeo copy docker-daemon:myapp:latest docker://registry.io/myapp:latest

# Directory to registry
skopeo copy dir:/path/to/image docker://registry.io/image:tag
```

## Offline Operations
```bash
# Download for offline use
skopeo copy docker://nginx:latest docker-archive:nginx.tar

# Load on air-gapped system
skopeo copy docker-archive:nginx.tar docker-daemon:nginx:latest

# Sync to local directory
skopeo sync --src docker --dest dir \
  nginx:latest /offline-images/
```

## Advanced Features
```bash
# Remove signatures
skopeo delete --signature docker://registry.io/image:tag

# Standalone signatures
skopeo standalone-sign manifest.json \
  registry.io/image:tag \
  fingerprint \
  --output signature.json

# Custom user agent
skopeo inspect --user-agent "MyApp/1.0" docker://nginx:latest

# Debug mode
skopeo --debug copy docker://source docker://dest
```

## Comparison with Other Tools
| Feature | skopeo | crane | docker |
|---------|--------|-------|--------|
| Daemon-free | Yes | Yes | No |
| Multi-arch | Full support | Good | Limited |
| Signing | Built-in | No | Separate |
| Sync | Advanced | Basic | No |
| Format support | Most formats | OCI focus | Docker only |

## Policy Examples
```bash
# Pre-deployment verification
if skopeo inspect docker://registry.io/app:$TAG >/dev/null 2>&1; then
  echo "Image exists, proceeding"
else
  echo "Image not found"
  exit 1
fi

# Tag validation
DIGEST=$(skopeo inspect docker://registry.io/app:latest | jq -r '.Digest')
if skopeo inspect docker://registry.io/app:$TAG | grep -q "$DIGEST"; then
  echo "Tag matches latest"
fi
```

## Troubleshooting
```bash
# Verify registry connectivity
skopeo inspect docker://registry.io/test

# Check credentials
skopeo login --get-login registry.io

# Detailed error info
skopeo --debug copy docker://source docker://dest

# Timeout issues
skopeo --command-timeout=5m copy docker://large-image docker://dest
```

## Best Practices
- Use `--authfile` for consistent credentials
- Run `sync` for bulk operations
- Preserve signatures with `--sign-by`
- Use `inspect` before operations
- Set timeouts for large images
- Leverage `--all` for multi-arch

## Tips
- No daemon needed (unlike Docker)
- Faster than Docker for registry ops
- Built-in signature support
- Advanced sync capabilities
- Works in minimal environments
- Good for CI/CD pipelines

## Advanced Configuration

### Authentication Configuration
```yaml
# ~/.docker/config.json
{
  "auths": {
    "registry.io": {
      "auth": "base64encodedcredentials"
    }
  }
}
```

### Registry Mirror Setup
```bash
# Mirror images nightly
#!/bin/bash
IMAGES=(
  "nginx:latest"
  "redis:alpine"
  "postgres:15"
)

for img in "${IMAGES[@]}"; do
  skopeo copy \
    --dest-creds user:pass \
    docker://docker.io/$img \
    docker://mirror.local/$img
done
```

### CI/CD Integration Script
```bash
# Promote image through environments
skopeo copy \
  docker://registry.io/app:dev-$CI_COMMIT_SHA \
  docker://registry.io/app:staging-$CI_COMMIT_SHA

# Wait for approval
# ...

skopeo copy \
  docker://registry.io/app:staging-$CI_COMMIT_SHA \
  docker://registry.io/app:production-$CI_COMMIT_SHA
```

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (WSL, binary)
- ✅ BSD systems
- ✅ Docker container

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove skopeo |

## Agent Use
- Registry synchronization
- Image promotion pipelines
- Multi-region deployments
- Offline image management
- Cross-registry migration

## Uninstall
```yaml
- preset: skopeo
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/containers/skopeo
- Docs: https://github.com/containers/skopeo/blob/main/docs/skopeo.1.md
- Search: "skopeo examples", "skopeo vs crane"

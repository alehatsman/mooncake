# crane - Container Registry Tool

Fast container image manipulation without Docker daemon. Part of Google's go-containerregistry tools.

## Quick Start
```yaml
- preset: crane
```

## Basic Commands
```bash
# Pull image to tarball
crane pull nginx:latest nginx.tar

# Push tarball to registry
crane push nginx.tar registry.io/nginx:v1

# Copy image between registries
crane copy source.io/app:v1 dest.io/app:v1

# Get image digest
crane digest nginx:latest

# List tags
crane ls nginx

# Get manifest
crane manifest nginx:latest

# Delete image
crane delete registry.io/image:tag
```

## Image Operations
```bash
# Copy with different tag
crane cp nginx:1.21 nginx:stable

# Copy to different registry
crane cp docker.io/nginx:latest ghcr.io/myorg/nginx:latest

# Copy all tags
for tag in $(crane ls nginx); do
  crane cp nginx:$tag backup.io/nginx:$tag
done

# Multi-arch images
crane copy --all-tags nginx:latest registry.io/nginx:latest
```

## Registry Management
```bash
# Authentication
crane auth login registry.io -u username -p password

# Use credentials from Docker config
crane --use-docker-auth copy source:tag dest:tag

# Environment variable auth
export CRANE_USERNAME=user
export CRANE_PASSWORD=pass

# List repository tags
crane ls gcr.io/my-project/my-image

# Get image config
crane config nginx:latest | jq

# Get image layers
crane manifest nginx:latest | jq '.layers'
```

## Image Analysis
```bash
# Get digest (sha256)
crane digest nginx:latest

# Get image size
crane manifest nginx:latest | jq '.config.size'

# Inspect layers
crane manifest nginx:latest | jq '.layers[] | {digest, size}'

# Validate image exists
crane digest registry.io/image:tag && echo "exists"

# Compare digests
if [ "$(crane digest img1:tag)" = "$(crane digest img2:tag)" ]; then
  echo "Images are identical"
fi
```

## CI/CD Workflows
```bash
# Promote staging to production
crane cp registry.io/app:staging registry.io/app:production

# Copy build to multiple registries
for registry in gcr.io ghcr.io docker.io; do
  crane cp local-registry/app:$TAG $registry/myorg/app:$TAG
done

# Retag latest
crane tag registry.io/app:v1.2.3 latest

# Cleanup old tags (keep last 10)
crane ls registry.io/app | sort -V | head -n -10 | \
  xargs -I {} crane delete registry.io/app:{}
```

## Multi-Architecture Images
```bash
# Copy preserving all platforms
crane cp --platform=all source:tag dest:tag

# Get available platforms
crane manifest source:tag | jq '.manifests[].platform'

# Copy specific platform
crane cp --platform=linux/amd64 source:tag dest:tag
crane cp --platform=linux/arm64 source:tag dest:tag
```

## Working Without Docker
```bash
# crane doesn't need Docker daemon

# Push OCI layout directory
crane push oci-layout/ registry.io/image:tag

# Pull to OCI layout
crane pull nginx:latest --format=oci nginx-oci/

# Export as tarball
crane export nginx:latest > nginx.tar
```

## Batch Operations
```bash
# Backup all tags to file
crane ls source.io/app > tags.txt

# Restore from backup
while read tag; do
  crane cp backup.io/app:$tag source.io/app:$tag
done < tags.txt

# Mirror repository
crane catalog source.io | while read repo; do
  crane ls source.io/$repo | while read tag; do
    crane cp source.io/$repo:$tag mirror.io/$repo:$tag
  done
done
```

## Debugging
```bash
# Verbose output
crane --verbose pull nginx:latest nginx.tar

# Debug mode
crane --debug manifest nginx:latest

# Check registry accessibility
crane catalog registry.io

# Validate credentials
crane auth get registry.io
```

## Comparison with Other Tools
| Task | crane | docker | skopeo |
|------|-------|--------|--------|
| Daemon required | No | Yes | No |
| Speed | Fast | Slow | Fast |
| Size | Small (~20MB) | Large (~500MB) | Medium (~50MB) |
| Auth | Simple | Docker config | Multiple methods |

## Tips
- **No daemon**: Runs anywhere, even minimal containers
- **Fast**: Optimized for registry operations
- **Lightweight**: Small binary, no dependencies
- **Auth**: Respects Docker credentials by default
- **Streaming**: Efficient for large images

## Agent Use
- Registry synchronization
- Image promotion pipelines
- Multi-registry deployments
- Tag management automation
- Mirror maintenance

## Uninstall
```yaml
- preset: crane
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/google/go-containerregistry/tree/main/cmd/crane
- Search: "crane registry tool", "crane vs skopeo"

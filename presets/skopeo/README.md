# skopeo - Container Image Tool

Work with container images without a daemon. Copy, inspect, delete images.

## Quick Start
```yaml
- preset: skopeo
```

## Usage
```bash
# Copy image
skopeo copy docker://nginx:latest docker://registry.local/nginx:latest

# Inspect remote image
skopeo inspect docker://redis:alpine

# Delete image
skopeo delete docker://registry.local/old-image:v1
```

**Agent Use**: Automated image management, cross-registry sync

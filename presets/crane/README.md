# crane - Image Manipulation

Fast container image manipulation without Docker daemon.

## Quick Start
```yaml
- preset: crane
```

## Usage
```bash
# Push/pull
crane pull nginx:latest image.tar
crane push image.tar registry.local/nginx:latest

# Copy with tag
crane cp source:tag dest:tag

# Digest
crane digest nginx:latest
```

**Agent Use**: Efficient image operations, CI/CD pipelines

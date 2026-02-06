# dive - Image Layer Explorer

Explore Docker image layers and find ways to shrink image size.

## Quick Start
```yaml
- preset: dive
```

## Usage
```bash
dive nginx:latest
dive build -t myimage .       # Build and analyze
```

**Agent Use**: Optimize images, analyze layer efficiency, find bloat

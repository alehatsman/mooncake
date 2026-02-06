# syft - SBOM Generator

Generate Software Bill of Materials (SBOM) for containers and filesystems.

## Quick Start
```yaml
- preset: syft
```

## Usage
```bash
# Generate SBOM
syft nginx:latest

# Export formats
syft nginx:latest -o json
syft nginx:latest -o cyclonedx
syft nginx:latest -o spdx

# Scan directory
syft dir:/path/to/project
```

**Agent Use**: Supply chain security, compliance, dependency tracking

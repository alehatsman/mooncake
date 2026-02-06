# grype - Vulnerability Scanner

Find vulnerabilities in container images and filesystems.

## Quick Start
```yaml
- preset: grype
```

## Usage
```bash
# Scan image
grype nginx:latest

# Scan directory
grype dir:/path/to/project

# Filter by severity
grype nginx:latest --fail-on=high

# JSON output
grype nginx:latest -o json
```

**Agent Use**: Automated vulnerability detection, security pipelines

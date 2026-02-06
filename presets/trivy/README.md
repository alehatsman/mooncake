# trivy - Vulnerability Scanner

Comprehensive vulnerability scanner for containers, filesystems, and Git repos.

## Quick Start
```yaml
- preset: trivy
```

## Usage
```bash
# Scan image
trivy image nginx:latest

# Scan filesystem
trivy fs /path/to/project

# Scan Git repo
trivy repo https://github.com/user/repo

# JSON output
trivy image -f json nginx:latest
```

**Agent Use**: Automated security scanning, CI/CD gates

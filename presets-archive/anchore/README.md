# Anchore - Container Security

Container image security and compliance scanning. Detect vulnerabilities, enforce policies, and generate SBOMs.

## Quick Start
```yaml
- preset: anchore
```

## Features
- **Vulnerability scanning**: CVE detection in container images
- **Policy enforcement**: Custom security and compliance policies
- **SBOM generation**: Software Bill of Materials creation
- **CI/CD integration**: Scan in build pipelines
- **Multi-registry**: Scan images from any registry
- **Continuous monitoring**: Track vulnerabilities over time
- **Compliance reports**: Generate compliance documentation

## Basic Usage
```bash
# Scan image
anchore-cli image add docker.io/library/nginx:latest
anchore-cli image wait docker.io/library/nginx:latest
anchore-cli image vuln docker.io/library/nginx:latest all

# List images
anchore-cli image list

# Policy check
anchore-cli evaluate check docker.io/library/nginx:latest

# Generate SBOM
anchore-cli image content docker.io/library/nginx:latest os
```

## Advanced Configuration
```yaml
- preset: anchore
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: anchore
  with:
    state: absent
```

## Resources
- Search: "anchore documentation", "anchore tutorial"

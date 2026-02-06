# Aqua Security - Cloud Native Security

Comprehensive security platform for containers and Kubernetes. Runtime protection, vulnerability management, and compliance.

## Quick Start
```yaml
- preset: aqua
```

## Features
- **Image scanning**: Vulnerability and malware detection
- **Runtime protection**: Behavioral monitoring and threat detection
- **Compliance**: CIS, PCI-DSS, HIPAA compliance checks
- **Network security**: Micro-segmentation and firewall
- **Secrets management**: Secure injection of secrets
- **SBOM**: Software Bill of Materials generation
- **CI/CD integration**: Security gates in pipelines

## Basic Usage
```bash
# Scan image
aqua scan image nginx:latest

# Check vulnerabilities
aqua vuln list

# Runtime protection
aqua runtime policy

# Compliance scan
aqua compliance scan

# Generate SBOM
aqua sbom generate nginx:latest
```

## Advanced Configuration
```yaml
- preset: aqua
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
- preset: aqua
  with:
    state: absent
```

## Resources
- Search: "aqua documentation", "aqua tutorial"

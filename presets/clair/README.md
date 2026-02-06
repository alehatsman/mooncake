# Clair - Container Security Scanner

Vulnerability scanner for container images that finds known security issues in application dependencies.

## Quick Start
```yaml
- preset: clair
```

## Features
- **Vulnerability detection**: Scans for CVEs in container layers
- **Multi-distro**: Supports Alpine, Debian, Ubuntu, RHEL, and more
- **API-driven**: RESTful API for integration
- **Continuous scanning**: Monitors images for new vulnerabilities
- **Language support**: Detects issues in Python, Ruby, Java, Go packages
- **Database updates**: Regular vulnerability database updates

## Basic Usage
```bash
# Start Clair server
clair -config config.yaml

# Scan image (using clairctl)
clairctl analyze myimage:latest

# Get vulnerability report
clairctl report myimage:latest

# Check for specific CVE
clairctl vulnerabilities myimage:latest | grep CVE-2023-1234
```

## Configuration
```yaml
# config.yaml
http:
  addr: ":6060"
indexer:
  connstring: "host=postgres port=5432 user=clair dbname=clair sslmode=disable"
  scanlock_retry: 10
matcher:
  connstring: "host=postgres port=5432 user=clair dbname=clair sslmode=disable"
notifier:
  connstring: "host=postgres port=5432 user=clair dbname=clair sslmode=disable"
  webhook:
    target: "http://webhook-receiver/notify"
```

## Real-World Examples

### CI/CD Integration
```yaml
- name: Start Clair
  service:
    name: clair
    state: started
  become: true

- name: Scan container image
  shell: clairctl analyze {{ image_name }}:{{ image_tag }}
  register: scan

- name: Check for critical vulnerabilities
  shell: clairctl report {{ image_name }}:{{ image_tag }} --threshold High
  register: vulnerabilities
  failed_when: vulnerabilities.rc != 0
```

### Registry Scanning
```yaml
- name: Scan all images in registry
  shell: |
    for image in $(docker images --format "{{.Repository}}:{{.Tag}}"); do
      clairctl analyze $image
      clairctl report $image --threshold Medium
    done
```

## Platform Support
- ✅ Linux (binary, Docker)
- ✅ macOS (Docker)
- ✅ Windows (Docker)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Scan container images for vulnerabilities in CI/CD
- Monitor production images for new CVEs
- Block deployment of vulnerable images
- Generate security reports for compliance
- Integrate with container registries


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install clair
  preset: clair

- name: Use clair in automation
  shell: |
    # Custom configuration here
    echo "clair configured"
```
## Uninstall
```yaml
- preset: clair
  with:
    state: absent
```

## Resources
- Official site: https://quay.github.io/clair/
- GitHub: https://github.com/quay/clair
- Documentation: https://quay.github.io/clair/concepts/
- Search: "clair tutorial", "container vulnerability scanning", "clair docker"

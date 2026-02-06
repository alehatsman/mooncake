# Checkov - Infrastructure Security Scanner

Static analysis tool for scanning infrastructure as code files for security and compliance issues.

## Quick Start
```yaml
- preset: checkov
```

## Features
- **Multi-format**: Terraform, CloudFormation, Kubernetes, Dockerfiles, ARM templates
- **2000+ checks**: Built-in security and compliance policies
- **CI/CD integration**: Fail builds on policy violations
- **Custom policies**: Write your own checks
- **Fix suggestions**: Automatically suggest fixes
- **Multiple outputs**: JSON, JUnit XML, SARIF, CLI

## Basic Usage
```bash
# Scan Terraform
checkov -d /path/to/terraform

# Scan specific file
checkov -f main.tf

# Scan Kubernetes
checkov --framework kubernetes -d k8s/

# Scan Dockerfile
checkov --framework dockerfile -f Dockerfile

# Output as JSON
checkov -d . --output json

# Skip specific checks
checkov -d . --skip-check CKV_AWS_20,CKV_AWS_21

# Only run specific checks
checkov -d . --check CKV_AWS_20
```

## Advanced Usage
```bash
# Scan with custom policies
checkov -d . --external-checks-dir ./custom-policies

# Baseline scan (only show new issues)
checkov -d . --baseline checkov-baseline.json

# Soft fail (don't exit with error)
checkov -d . --soft-fail

# Set severity threshold
checkov -d . --compact --quiet --threshold critical

# Generate baseline
checkov -d . --create-baseline

# Scan and output to file
checkov -d . --output json > checkov-report.json
```

## Real-World Examples

### CI/CD Pipeline
```yaml
- name: Install Checkov
  preset: checkov

- name: Scan Terraform files
  shell: checkov -d terraform/ --output junitxml --output-file results.xml
  cwd: /app
  register: scan
  failed_when: false

- name: Upload scan results
  shell: aws s3 cp results.xml s3://security-scans/
  when: scan.rc != 0
```

### Pre-commit Hook
```yaml
- name: Run security scan
  shell: checkov -d . --quiet --compact
  cwd: /project
```

### Kubernetes Security
```bash
# Scan Kubernetes manifests
checkov --framework kubernetes -d k8s/

# Scan Helm charts
checkov --framework helm -d charts/
```

## Platform Support
- ✅ Linux (pip, binary)
- ✅ macOS (pip, Homebrew)
- ✅ Windows (pip)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Scan infrastructure code for security issues in CI/CD
- Enforce security policies before deployment
- Audit existing infrastructure for compliance
- Identify misconfigurations early in development
- Generate security reports for compliance


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install checkov
  preset: checkov

- name: Use checkov in automation
  shell: |
    # Custom configuration here
    echo "checkov configured"
```
## Uninstall
```yaml
- preset: checkov
  with:
    state: absent
```

## Resources
- Official site: https://www.checkov.io
- GitHub: https://github.com/bridgecrewio/checkov
- Documentation: https://www.checkov.io/documentation/
- Search: "checkov tutorial", "terraform security scan", "checkov examples"

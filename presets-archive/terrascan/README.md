# Terrascan - Infrastructure as Code Security Scanner

Static code analyzer for Infrastructure as Code. Detect compliance and security violations in Terraform, Kubernetes, Helm, Dockerfile, and more.

## Quick Start
```yaml
- preset: terrascan
```

## Features
- **Multi-IaC support**: Terraform, Kubernetes, Helm, Dockerfile, Kustomize, ARM templates
- **500+ policies**: Built-in security and compliance policies
- **Compliance standards**: CIS, PCI-DSS, NIST, HIPAA, SOC2, GDPR
- **Custom policies**: Write your own Rego policies
- **CI/CD integration**: Block deployments with policy violations
- **JSON/SARIF output**: For automated processing
- **Remote scanning**: Scan git repos, registries, S3

## Basic Usage
```bash
# Check version
terrascan version

# Scan current directory (auto-detects IaC type)
terrascan scan

# Scan Terraform
terrascan scan -t terraform

# Scan Kubernetes manifests
terrascan scan -t k8s

# Scan with specific policy
terrascan scan -p aws

# Output JSON
terrascan scan -o json
```

## Scanning IaC

### Terraform
```bash
# Scan terraform directory
terrascan scan -t terraform -d /path/to/terraform

# Scan specific file
terrascan scan -t terraform -f main.tf

# Scan terraform plan
terraform plan -out=plan.out
terraform show -json plan.out > plan.json
terrascan scan -t terraform --iac-file plan.json
```

### Kubernetes
```bash
# Scan K8s manifests
terrascan scan -t k8s -d ./manifests

# Scan Helm chart
terrascan scan -t helm -d ./mychart

# Scan Kustomize
terrascan scan -t kustomize -d ./overlays/production
```

### Docker
```bash
# Scan Dockerfile
terrascan scan -t docker -f Dockerfile

# Scan with context
terrascan scan -t docker -d ./docker/app
```

## Policy Management

### List Policies
```bash
# List all policies
terrascan init  # Download policies first
ls ~/.terrascan/pkg/policies/opa/rego

# Show policy types
terrascan scan --policy-type aws
terrascan scan --policy-type azure
terrascan scan --policy-type gcp
terrascan scan --policy-type k8s
```

### Severity Levels
```bash
# Scan for HIGH and CRITICAL only
terrascan scan --severity high,critical

# Show all severities
terrascan scan --severity low,medium,high,critical
```

### Skip Rules
```bash
# Skip specific rules
terrascan scan --skip-rules AC_AWS_0001,AC_AWS_0002

# Skip rules via config
cat > terrascan-config.toml <<EOF
[rules]
  skip-rules = [
    "AC_AWS_0001",
    "AC_K8S_0001"
  ]
EOF

terrascan scan --config-path terrascan-config.toml
```

### Custom Policies
```bash
# Create custom policy (Rego)
mkdir -p policies
cat > policies/my-policy.rego <<EOF
package accurics

deny[msg] {
  input.kind == "Deployment"
  not input.spec.replicas >= 2
  msg := "Deployment must have at least 2 replicas"
}
EOF

# Scan with custom policies
terrascan scan -t k8s --policy-path policies/
```

## Remote Scanning

### Git Repositories
```bash
# Scan remote git repo
terrascan scan -r git \
  -u https://github.com/example/terraform-infra

# Scan specific branch
terrascan scan -r git \
  -u https://github.com/example/terraform-infra \
  -b develop

# Private repo with token
terrascan scan -r git \
  -u https://github.com/example/private-repo \
  --scan-rules AC_AWS_*
```

### Container Registries
```bash
# Scan Docker image
terrascan scan -t docker \
  -r gcr \
  -u gcr.io/my-project/my-image:v1.0

# Scan with credentials
terrascan scan -t docker \
  -r gcr \
  -u gcr.io/my-project/my-image:v1.0 \
  --rego-subcommand "eval -i json"
```

### S3 Buckets
```bash
# Scan from S3
terrascan scan -r s3 \
  -u s3://my-bucket/terraform/
```

## Output Formats

### Human-Readable (Default)
```bash
terrascan scan
```

### JSON
```bash
# JSON output
terrascan scan -o json

# Save to file
terrascan scan -o json > scan-results.json

# Pretty print
terrascan scan -o json | jq .
```

### YAML
```bash
terrascan scan -o yaml
```

### SARIF (for GitHub)
```bash
# SARIF output
terrascan scan -o sarif > terrascan.sarif

# Upload to GitHub Code Scanning
gh api -X POST /repos/owner/repo/code-scanning/sarifs \
  -f sarif=@terrascan.sarif
```

### JUnit XML
```bash
# For CI/CD test reporting
terrascan scan -o junit-xml > test-results.xml
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install Terrascan
  run: |
    curl -L https://github.com/tenable/terrascan/releases/latest/download/terrascan_Linux_x86_64.tar.gz | tar -xz
    sudo mv terrascan /usr/local/bin/

- name: Initialize Terrascan
  run: terrascan init

- name: Scan Terraform
  run: |
    terrascan scan -t terraform -d terraform/ -o sarif > terrascan.sarif

- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v2
  with:
    sarif_file: terrascan.sarif

- name: Fail on violations
  run: |
    terrascan scan -t terraform --severity high,critical
```

### GitLab CI
```yaml
terrascan:
  image: tenable/terrascan:latest
  script:
    - terrascan init
    - terrascan scan -t terraform -d . -o json > gl-sast-report.json
  artifacts:
    reports:
      sast: gl-sast-report.json
```

### Jenkins
```groovy
pipeline {
  agent any
  stages {
    stage('Terrascan') {
      steps {
        sh 'terrascan init'
        sh 'terrascan scan -t terraform --severity high,critical'
      }
    }
  }
}
```

### Pre-Commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running Terrascan security scan..."

terrascan scan -t terraform --severity high,critical

if [ $? -ne 0 ]; then
  echo "❌ Security violations found!"
  echo "Run 'terrascan scan' for details"
  exit 1
fi

echo "✅ No critical security violations"
```

## Real-World Examples

### Complete Terraform Validation
```yaml
- name: Install Terrascan
  shell: |
    curl -L https://github.com/tenable/terrascan/releases/latest/download/terrascan_Linux_x86_64.tar.gz | tar -xz
    sudo mv terrascan /usr/local/bin/
  become: true

- name: Initialize policies
  shell: terrascan init

- name: Validate terraform syntax
  shell: terraform validate
  cwd: /infrastructure

- name: Run security scan
  shell: |
    terrascan scan -t terraform \
      -d /infrastructure \
      --severity high,critical \
      --policy-type aws \
      -o json > /tmp/scan-results.json
  register: scan
  failed_when: scan.rc != 0

- name: Generate report
  shell: |
    echo "Security Scan Results" > /tmp/report.txt
    jq '.results.violations[]' /tmp/scan-results.json >> /tmp/report.txt
  when: scan.rc != 0
```

### Kubernetes Admission Controller
```yaml
- name: Scan K8s manifests before apply
  shell: |
    terrascan scan -t k8s -d {{ manifests_dir }} --severity high,critical
  register: k8s_scan

- name: Apply manifests
  shell: kubectl apply -f {{ manifests_dir }}
  when: k8s_scan.rc == 0

- name: Send alert on violations
  when: k8s_scan.rc != 0
  shell: |
    curl -X POST {{ webhook_url }} \
      -d "Security violations detected in K8s manifests"
```

### Multi-Cloud Compliance
```bash
# AWS resources
terrascan scan -t terraform --policy-type aws \
  --config-path aws-compliance.toml

# Azure resources
terrascan scan -t terraform --policy-type azure \
  --config-path azure-compliance.toml

# GCP resources
terrascan scan -t terraform --policy-type gcp \
  --config-path gcp-compliance.toml
```

## Configuration File

### terrascan-config.toml
```toml
[rules]
  skip-rules = [
    "AC_AWS_0001",  # S3 encryption - handled elsewhere
    "AC_K8S_0080"   # Privileged containers - known requirement
  ]

[severity]
  level = "high"

[notifications]
  webhook-url = "https://hooks.slack.com/..."
  webhook-token = "xxx"

[category]
  list = ["Infrastructure Security", "Compliance"]
```

## Compliance Frameworks

### Scan for Specific Compliance
```bash
# CIS Benchmark
terrascan scan --compliance cis

# PCI-DSS
terrascan scan --compliance pci

# HIPAA
terrascan scan --compliance hipaa

# NIST
terrascan scan --compliance nist

# SOC 2
terrascan scan --compliance soc2

# GDPR
terrascan scan --compliance gdpr
```

## Troubleshooting

### Policy Update
```bash
# Reinitialize policies
rm -rf ~/.terrascan
terrascan init

# Verify policies downloaded
ls ~/.terrascan/pkg/policies/opa/rego
```

### Debug Mode
```bash
# Enable debug output
terrascan scan -t terraform --verbose

# Show policy evaluation
terrascan scan -t terraform --log-level debug
```

### Common Issues
```bash
# "No policies found"
terrascan init

# "Unsupported IaC type"
terrascan scan -t terraform  # Specify type explicitly

# False positives
terrascan scan --skip-rules AC_XXX_XXXX
```

## Best Practices
- Run `terrascan init` to download latest policies
- Use `--severity high,critical` in CI/CD to reduce noise
- Store exemptions in `terrascan-config.toml` version-controlled
- Combine with other tools (tfsec, checkov) for comprehensive coverage
- Scan early in development (pre-commit hooks)
- Review and document skipped rules
- Use SARIF output for GitHub/GitLab integration
- Scan both plan and code for full coverage

## Platform Support
- ✅ Linux (amd64, arm64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (amd64)
- ✅ Docker container

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated security scanning in CI/CD
- Compliance validation automation
- Infrastructure code review
- Policy enforcement
- Multi-cloud security posture management
- Vulnerability detection before deployment

## Advanced Configuration
```yaml
- preset: terrascan
  with:
    state: present
```

## Uninstall
```yaml
- preset: terrascan
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/tenable/terrascan
- Documentation: https://runterrascan.io/docs/
- Policies: https://runterrascan.io/docs/policies/
- Integrations: https://runterrascan.io/docs/integrations/
- Search: "terrascan policies", "terrascan ci cd", "terrascan custom policies"

# Trivy - Comprehensive Security Scanner

All-in-one security scanner for containers, filesystems, Git repositories, and Kubernetes. Detects vulnerabilities, misconfigurations, secrets, and license issues.

## Quick Start
```yaml
- preset: trivy
```

## Features

- **Vulnerability Detection**: CVE scanning for OS packages and application dependencies
- **Misconfiguration Detection**: IaC scanning (Terraform, CloudFormation, Kubernetes)
- **Secret Detection**: API keys, tokens, passwords in code and images
- **SBOM Generation**: CycloneDX and SPDX format support
- **License Compliance**: Detect incompatible or risky licenses
- **Multi-Target**: Scan images, filesystems, repositories, Kubernetes clusters
- **CI/CD Friendly**: Exit codes, JSON output, GitHub Actions integration
- **Fast**: Offline mode, caching, parallel scanning

## Basic Scanning
```bash
# Scan container image
trivy image nginx:latest
trivy image alpine:3.18

# Scan local filesystem
trivy fs /path/to/project
trivy fs .

# Scan Git repository
trivy repo https://github.com/user/repo
trivy repo .

# Scan Kubernetes cluster
trivy k8s --report=summary cluster
```

## Output Formats
```bash
# Table (default)
trivy image nginx:latest

# JSON
trivy image -f json nginx:latest

# SARIF (for GitHub)
trivy image -f sarif -o results.sarif nginx:latest

# Template
trivy image -f template --template "@contrib/html.tpl" -o report.html nginx:latest

# CycloneDX SBOM
trivy image -f cyclonedx nginx:latest

# SPDX SBOM
trivy image -f spdx-json nginx:latest
```

## Severity Filtering
```bash
# Only CRITICAL and HIGH
trivy image --severity CRITICAL,HIGH nginx:latest

# Ignore unfixed vulnerabilities
trivy image --ignore-unfixed nginx:latest

# Exit code on severity
trivy image --exit-code 1 --severity CRITICAL nginx:latest

# Filter by vulnerability IDs
trivy image --skip-ids CVE-2019-1234,CVE-2020-5678 nginx:latest
```

## Scan Types
```bash
# Vulnerabilities only (default)
trivy image --scanners vuln nginx:latest

# Misconfigurations
trivy config ./kubernetes/
trivy image --scanners config nginx:latest

# Secrets
trivy fs --scanners secret .
trivy image --scanners secret nginx:latest

# License compliance
trivy image --scanners license nginx:latest

# All scanners
trivy image --scanners vuln,config,secret,license nginx:latest
```

## CI/CD Integration
```bash
# Fail build on CRITICAL
trivy image --exit-code 1 --severity CRITICAL myapp:latest

# GitHub Actions
- name: Scan image
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.IMAGE }}:${{ github.sha }}
    severity: 'CRITICAL,HIGH'
    exit-code: '1'

# GitLab CI
trivy-scan:
  script:
    - trivy image --exit-code 1 --severity HIGH,CRITICAL $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA

# Generate SARIF for GitHub Security tab
trivy image -f sarif -o trivy-results.sarif myapp:latest
```

## Configuration Files
```bash
# Scan IaC
trivy config ./terraform/
trivy config ./kubernetes/
trivy config ./cloudformation/
trivy config ./dockerfile

# Misconfigurations
trivy config --severity HIGH,CRITICAL .

# Policy as code
trivy config --policy ./policy .

# Skip checks
trivy config --skip-policy-update .
```

## Kubernetes Scanning
```bash
# Cluster scan
trivy k8s --report summary cluster

# Specific namespace
trivy k8s -n kube-system all

# Specific resources
trivy k8s deployment/myapp
trivy k8s pod/nginx-12345

# Infrastructure as Code
trivy config ./k8s-manifests/
```

## Database Management
```bash
# Update vulnerability database
trivy image --download-db-only

# Skip database update
trivy image --skip-db-update nginx:latest

# Offline mode
trivy image --offline-scan nginx:latest

# Custom database location
trivy image --cache-dir /path/to/cache nginx:latest
```

## Filtering and Ignoring
```bash
# .trivyignore file
cat > .trivyignore <<EOF
# Ignore specific CVEs
CVE-2019-1234
CVE-2020-5678

# Ignore by path
vendor/
node_modules/

# Ignore by package
pkg:golang/example.com/vulnerable@1.0.0
EOF

# Ignore policy
trivy image --ignorefile .trivyignore nginx:latest

# Ignore file patterns
trivy fs --skip-files "**/*.test.js" .
trivy fs --skip-dirs "vendor/,node_modules/" .
```

## Advanced Features
```bash
# Custom policies
trivy config --config-policy ./policies/ .

# Generate SBOM
trivy image -f cyclonedx nginx:latest > sbom.json

# Scan SBOM
trivy sbom ./sbom.json

# Remote scanning (client-server)
trivy server --listen 0.0.0.0:8080
trivy client --remote http://trivy-server:8080 nginx:latest

# Scan with timeout
trivy image --timeout 10m nginx:latest
```

## Secrets Detection
```bash
# Scan for secrets
trivy fs --scanners secret .

# Skip secret scanning
trivy fs --skip-files "**/*.env" --scanners secret .

# Custom secret patterns
cat > .trivy-secret.yaml <<EOF
rules:
  - id: custom-token
    category: general
    title: Custom Token
    severity: HIGH
    regex: "token-[a-z0-9]{32}"
EOF

trivy fs --secret-config .trivy-secret.yaml .
```

## Performance Optimization
```bash
# Parallel scanning
trivy image --parallel 4 nginx:latest

# Skip slow scanners
trivy image --scanners vuln --skip-files "**/*.md" nginx:latest

# Light mode (skip update)
trivy image --skip-db-update --skip-java-db-update nginx:latest

# Cached results
trivy image --cache-ttl 24h nginx:latest
```

## Reporting
```bash
# HTML report
trivy image -f template --template "@contrib/html.tpl" -o report.html nginx:latest

# Compare scans
trivy image nginx:1.21 -f json > v1.json
trivy image nginx:1.22 -f json > v2.json
diff <(jq -S . v1.json) <(jq -S . v2.json)

# Metrics
trivy image -f json nginx:latest | jq '.Results[].Vulnerabilities | length'
```

## Policy Examples
```bash
# Block deployment if HIGH+ vulns
if trivy image --exit-code 1 --severity HIGH,CRITICAL myapp:latest; then
  kubectl apply -f deployment.yaml
else
  echo "Security scan failed"
  exit 1
fi

# Require SBOM
trivy image -f cyclonedx myapp:latest > sbom.json
if [ ! -s sbom.json ]; then
  echo "SBOM generation failed"
  exit 1
fi
```

## Basic Usage
```bash
# Scan Docker image
trivy image myimage:latest

# Scan filesystem
trivy fs /path/to/project

# Scan Git repository
trivy repo https://github.com/user/repo
```

## Advanced Configuration

### Basic Installation
```yaml
- preset: trivy
```

### With Specific Version
```yaml
- preset: trivy
  with:
    version: "0.48.0"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove trivy |
| version | string | latest | Specific version to install |

## Platform Support

- ✅ Linux (apt, dnf, yum, rpm, binary)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (Chocolatey, binary)
- ✅ Docker (official images)

## Configuration

- **Database**: `~/.cache/trivy/db/` (vulnerability database)
- **Java DB**: `~/.cache/trivy/java-db/` (Java artifact vulnerabilities)
- **Cache**: `~/.cache/trivy/` (scan results cache)
- **Ignore file**: `.trivyignore` (project root)
- **Config file**: `trivy.yaml` (optional)

## Best Practices

- **Update database regularly** in CI pipelines
- **Use .trivyignore** for accepted vulnerabilities with justification
- **Scan early** in the build process before deployment
- **Set severity thresholds** appropriate to environment (stricter for prod)
- **Enable multiple scanners** (vuln + config + secret) for complete coverage
- **Cache results** to speed up repeated scans
- **Combine with admission controllers** (OPA, Kyverno) in Kubernetes
- **Archive scan results** for compliance and trend analysis
- **Use SBOM** for supply chain security
- **Integrate with security tools** (SIEM, vulnerability management)

## Tips

- Fast database updates (typically < 10s)
- Offline mode for air-gapped environments
- Low false positive rate compared to alternatives
- Supports private registries with authentication
- Can scan without pulling images (--image-src remote)
- Works with podman, containerd, and other runtimes
- Plugin system for extensibility
- Parallel scanning for multiple targets

## Agent Use

- Automated vulnerability scanning in CI/CD
- Pre-deployment security gates
- SBOM generation for supply chain security
- Compliance reporting and audit trails
- Container registry scanning
- Security drift detection
- Policy enforcement automation
- Vulnerability trend analysis

## Uninstall

```yaml
- preset: trivy
  with:
    state: absent
```

## Resources

- Official docs: https://aquasecurity.github.io/trivy/
- GitHub: https://github.com/aquasecurity/trivy
- Community: https://slack.aquasec.com/
- Tutorials: https://aquasecurity.github.io/trivy/latest/tutorials/
- Search: "trivy scan examples", "trivy ci/cd", "trivy kubernetes"

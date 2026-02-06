# grype - Vulnerability Scanner

Find vulnerabilities in container images and filesystems. Fast, accurate CVE detection powered by vulnerability databases.

## Quick Start
```yaml
- preset: grype
```

## Basic Usage
```bash
# Scan container image
grype nginx:latest
grype alpine:3.18

# Scan directory
grype dir:.
grype dir:/path/to/project

# Scan SBOM
grype sbom:./sbom.json

# Scan archive
grype docker-archive:image.tar
```

## Severity Filtering
```bash
# Show only CRITICAL and HIGH
grype nginx:latest --fail-on=high

# Exit with error on specific severity
grype nginx:latest --fail-on=medium
grype nginx:latest --fail-on=critical

# Show all severities
grype nginx:latest --fail-on=negligible
```

## Output Formats
```bash
# Table (default)
grype nginx:latest

# JSON
grype nginx:latest -o json

# CycloneDX
grype nginx:latest -o cyclonedx-json

# SARIF (for GitHub)
grype nginx:latest -o sarif

# Template
grype nginx:latest -o template -t custom.tmpl
```

## Source Types
```bash
# Docker image
grype docker:nginx:latest
grype registry:ghcr.io/org/image:tag

# Directory scan
grype dir:.
grype dir:/usr/local/app

# SBOM (from syft)
syft nginx:latest -o json | grype

# Archive
grype docker-archive:image.tar
grype oci-archive:image.tar
```

## CI/CD Integration
```bash
# Fail build on HIGH+
grype nginx:latest --fail-on=high

# GitHub Actions
- name: Scan for vulnerabilities
  run: |
    grype ${{ env.IMAGE }}:${{ github.sha }} --fail-on=critical

# GitLab CI
security-scan:
  script:
    - grype ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA} --fail-on=high
  allow_failure: false

# Generate report
grype nginx:latest -o json > vulns.json
```

## Filtering
```bash
# Ignore specific CVEs
grype nginx:latest --ignore CVE-2019-1234,CVE-2020-5678

# Filter by package
grype nginx:latest -o json | jq '.matches[] | select(.artifact.name=="openssl")'

# Count by severity
grype nginx:latest -o json | jq '.matches | group_by(.vulnerability.severity) | map({severity: .[0].vulnerability.severity, count: length})'

# Only fixable vulnerabilities
grype nginx:latest -o json | jq '.matches[] | select(.vulnerability.fix.state=="fixed")'
```

## Configuration
```yaml
# .grype.yaml
fail-on-severity: "high"
output: ["json"]
ignore:
  - vulnerability: CVE-2019-1234
    reason: "false positive"
  - vulnerability: CVE-2020-5678
    package:
      name: "openssl"
      version: "1.1.1"
```

## Database Management
```bash
# Update vulnerability database
grype db update

# Check database status
grype db status

# Database location
~/.cache/grype/db/

# Skip update
grype --db-update-skip nginx:latest
```

## SBOM Workflow
```bash
# Generate SBOM with syft
syft nginx:latest -o json > sbom.json

# Scan SBOM with grype
grype sbom:sbom.json

# Pipeline
syft nginx:latest -o json | grype --fail-on=high
```

## Comparison
```bash
# Scan two versions
grype nginx:1.21 -o json > v1-vulns.json
grype nginx:1.22 -o json > v2-vulns.json

# Compare vulnerability counts
echo "v1.21: $(jq '.matches | length' v1-vulns.json)"
echo "v1.22: $(jq '.matches | length' v2-vulns.json)"

# Find new vulnerabilities
comm -13 \
  <(jq -r '.matches[].vulnerability.id' v1-vulns.json | sort) \
  <(jq -r '.matches[].vulnerability.id' v2-vulns.json | sort)
```

## Reporting
```bash
# Summary report
grype nginx:latest | grep -A 20 "Vulnerability Summary"

# Critical vulnerabilities only
grype nginx:latest -o json | jq '.matches[] | select(.vulnerability.severity=="Critical")'

# Packages with vulnerabilities
grype nginx:latest -o json | jq -r '.matches[].artifact.name' | sort -u

# Generate HTML report
grype nginx:latest -o template -t html.tmpl > report.html
```

## Policy Examples
```bash
# Block deployment on critical
if grype nginx:latest --fail-on=critical; then
  echo "Security scan passed"
  kubectl apply -f deployment.yaml
else
  echo "CRITICAL vulnerabilities found"
  exit 1
fi

# Require fix available
fixable=$(grype nginx:latest -o json | jq '[.matches[] | select(.vulnerability.fix.state=="fixed")] | length')
if [ $fixable -gt 0 ]; then
  echo "Fixable vulnerabilities found: $fixable"
  exit 1
fi
```

## Integration Examples
```bash
# With trivy for comparison
grype nginx:latest > grype-report.txt
trivy image nginx:latest > trivy-report.txt

# With Docker build
docker build -t myapp:latest .
grype myapp:latest --fail-on=high

# Multi-stage verification
docker build --target=builder -t myapp:builder .
grype myapp:builder --fail-on=medium

docker build -t myapp:final .
grype myapp:final --fail-on=high
```

## Advanced Usage
```bash
# Scan specific platform
grype --platform=linux/amd64 nginx:latest

# Verbose output
grype -v nginx:latest

# Quiet mode
grype -q nginx:latest

# Custom database location
grype --cache-dir=/custom/path nginx:latest

# Skip Java indexing (faster)
grype --skip-java-bins-scanner nginx:latest
```

## Vulnerability Details
```json
{
  "matches": [{
    "vulnerability": {
      "id": "CVE-2021-12345",
      "severity": "High",
      "description": "...",
      "fix": {
        "state": "fixed",
        "versions": ["1.2.3"]
      }
    },
    "artifact": {
      "name": "openssl",
      "version": "1.1.1",
      "type": "deb"
    }
  }]
}
```

## Best Practices
- **Scan early**: Check during build
- **Update database**: Run `grype db update` daily
- **Set thresholds**: Use `--fail-on` appropriately
- **Ignore wisely**: Document ignored CVEs
- **Compare versions**: Track vulnerability trends
- **Combine with SBOM**: Use syft + grype workflow

## Tips
- Faster than trivy for large images
- Works offline after DB download
- Supports multiple SBOM formats
- Integrates with syft naturally
- Lower false positive rate
- Good for CI/CD gates

## Agent Use
- Automated vulnerability scanning
- Security quality gates
- Compliance verification
- Risk assessment
- Deployment blockers

## Uninstall
```yaml
- preset: grype
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/anchore/grype
- Docs: https://github.com/anchore/grype#readme
- Search: "grype vs trivy", "grype ci/cd"

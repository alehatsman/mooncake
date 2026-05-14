# Snyk - Developer Security Platform

Find and fix vulnerabilities in code, dependencies, containers, and IaC. Integrate security scanning into development workflow and CI/CD pipelines.

## Quick Start
```yaml
- preset: snyk
```

## Features
- **Dependency scanning**: Find vulnerabilities in npm, pip, Maven, Go modules, etc.
- **Container scanning**: Scan Docker images for security issues
- **Code analysis**: Static application security testing (SAST)
- **IaC scanning**: Check Terraform, Kubernetes, CloudFormation for misconfigurations
- **License compliance**: Track open source license issues
- **Auto-fix**: Generate PRs with patches
- **CI/CD integration**: Block builds with vulnerabilities

## Basic Usage
```bash
# Check version
snyk --version

# Authenticate
snyk auth

# Test for vulnerabilities
snyk test

# Monitor project (send snapshot to Snyk)
snyk monitor

# Test Docker image
snyk container test nginx:latest

# Test IaC files
snyk iac test terraform/
```

## Authentication
```bash
# Interactive auth (opens browser)
snyk auth

# Using token
export SNYK_TOKEN=your-token-here
snyk test

# Verify authentication
snyk auth --version
snyk whoami
```

## Dependency Scanning

### Scan Project
```bash
# Auto-detect project type
snyk test

# Specific package manager
snyk test --file=package-lock.json  # npm
snyk test --file=requirements.txt    # pip
snyk test --file=pom.xml             # Maven
snyk test --file=go.mod              # Go
snyk test --file=Gemfile.lock        # Ruby
```

### Monitor Project
```bash
# Send snapshot to Snyk dashboard
snyk monitor

# With project name
snyk monitor --project-name="my-app"

# For specific environment
snyk monitor --project-environment=production
```

### Fix Vulnerabilities
```bash
# Show fix advice
snyk test

# Auto-fix (updates dependencies)
snyk fix

# Generate fix PR (GitHub/GitLab)
snyk test --fix-pr
```

## Container Scanning

### Docker Images
```bash
# Scan image
snyk container test nginx:latest

# Scan local image
snyk container test myapp:v1

# Scan with Dockerfile
snyk container test myapp:v1 --file=Dockerfile

# Monitor image
snyk container monitor nginx:latest
```

### Show Recommendations
```bash
# Get base image recommendations
snyk container test myapp:v1 --app-vulns

# Show alternative images
snyk container test ubuntu:20.04 --print-deps
```

## Infrastructure as Code

### Scan IaC Files
```bash
# Scan Terraform
snyk iac test terraform/

# Scan Kubernetes manifests
snyk iac test k8s/

# Scan CloudFormation
snyk iac test cloudformation.yaml

# Scan Helm charts
snyk iac test helm-chart/
```

### IaC Test Options
```bash
# Specific severity
snyk iac test --severity-threshold=high

# JSON output
snyk iac test --json

# SARIF output (for GitHub)
snyk iac test --sarif-file-output=results.sarif
```

## Code Scanning (Snyk Code)

### SAST Analysis
```bash
# Scan source code
snyk code test

# Specific file
snyk code test src/app.js

# JSON output
snyk code test --json
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install Snyk
  run: npm install -g snyk

- name: Authenticate
  env:
    SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
  run: snyk auth $SNYK_TOKEN

- name: Test dependencies
  run: snyk test --severity-threshold=high

- name: Test Docker image
  run: snyk container test myapp:${{ github.sha }}

- name: Test IaC
  run: snyk iac test terraform/

- name: Monitor
  run: snyk monitor
```

### GitLab CI
```yaml
snyk-test:
  image: snyk/snyk:node
  script:
    - snyk auth $SNYK_TOKEN
    - snyk test --severity-threshold=high
    - snyk monitor
  only:
    - main
```

### Jenkins
```groovy
pipeline {
  agent any
  environment {
    SNYK_TOKEN = credentials('snyk-token')
  }
  stages {
    stage('Security Scan') {
      steps {
        sh 'snyk auth $SNYK_TOKEN'
        sh 'snyk test --severity-threshold=high'
      }
    }
  }
}
```

### Docker Build
```dockerfile
# Install Snyk
RUN npm install -g snyk

# Scan during build
RUN snyk auth ${SNYK_TOKEN} && \
    snyk test --severity-threshold=high
```

## Advanced Configuration

### .snyk Policy File
```yaml
# .snyk file
version: v1.25.0

# Ignore specific vulnerabilities
ignore:
  'SNYK-JS-LODASH-590103':
    - '*':
        reason: False positive, not exploitable in our use case
        expires: 2024-12-31T23:59:59.999Z

# Patch rules
patch:
  'SNYK-JS-MINIMIST-559764':
    - '*':
        patched: '2024-01-01T00:00:00.000Z'
```

### snyk.config
```yaml
# Organization
org: my-org-id

# Severity threshold
severity-threshold: high

# Fail on issues
fail-on: all

# Project settings
project-name: my-app
project-environment: production
```

### Environment Variables
```bash
# Authentication
export SNYK_TOKEN=token

# Organization
export SNYK_ORG=my-org

# API endpoint (for on-prem)
export SNYK_API=https://api.snyk.io

# Configuration
export SNYK_CFG_FAIL_ON=upgradable
export SNYK_CFG_SEVERITY_THRESHOLD=medium
```

## Reporting

### JSON Output
```bash
# JSON format
snyk test --json > results.json

# JSON with all details
snyk test --json --all-projects
```

### SARIF Output
```bash
# For GitHub Code Scanning
snyk test --sarif > results.sarif
snyk code test --sarif > code-results.sarif

# Upload to GitHub
gh api -X POST /repos/owner/repo/code-scanning/sarifs \
  -f sarif=@results.sarif
```

### HTML Report
```bash
# Generate HTML report
snyk test --json | snyk-to-html > report.html
```

## Real-World Examples

### Complete Security Pipeline
```yaml
- name: Install Snyk
  shell: npm install -g snyk

- name: Authenticate
  shell: snyk auth {{ snyk_token }}
  no_log: true

- name: Test npm dependencies
  shell: snyk test --severity-threshold=high
  cwd: /app
  register: deps_scan
  failed_when: deps_scan.rc != 0

- name: Test Docker image
  shell: |
    docker build -t myapp:{{ version }} .
    snyk container test myapp:{{ version }} --severity-threshold=high
  register: container_scan

- name: Test Terraform
  shell: snyk iac test terraform/ --severity-threshold=high
  register: iac_scan

- name: Monitor in Snyk dashboard
  shell: |
    snyk monitor --project-name="myapp" --project-environment=production
  when: container_scan.rc == 0
```

### Pre-Commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running Snyk security scan..."

# Test dependencies
snyk test --severity-threshold=high

if [ $? -ne 0 ]; then
  echo "❌ Security vulnerabilities found!"
  echo "Run 'snyk test' for details"
  exit 1
fi

echo "✅ No high/critical vulnerabilities found"
```

### Dependency Update Workflow
```yaml
- name: Check for vulnerabilities
  shell: snyk test --json
  register: vulnerabilities
  failed_when: false

- name: Generate fix PR
  when: vulnerabilities.stdout | from_json | length > 0
  shell: |
    snyk test --fix-pr
  environment:
    GITHUB_TOKEN: "{{ github_token }}"
```

## Policy Management

### Ignore Vulnerabilities
```bash
# Ignore for 30 days
snyk ignore --id=SNYK-JS-LODASH-590103 \
  --expiry=2024-12-31 \
  --reason="Not exploitable in our context"

# Ignore path
snyk ignore --id=SNYK-JS-MINIMIST-559764 \
  --path='dev-dependency > test-framework'
```

### License Policies
```bash
# Check licenses
snyk test --license-policy

# List licenses
snyk test --print-licenses
```

## Troubleshooting

### Authentication Failures
```bash
# Re-authenticate
snyk auth

# Verify token
snyk config get api

# Test connection
snyk test --help
```

### False Positives
```bash
# Ignore specific vulnerability
snyk ignore --id=SNYK-XXX-XXX

# Check vulnerability details
snyk test --json | jq '.vulnerabilities[] | select(.id=="SNYK-XXX-XXX")'
```

### Performance Issues
```bash
# Skip dev dependencies
snyk test --prod

# Limit concurrent scanning
snyk test --max-depth=2

# Use cache
snyk test --use-cache
```

## Best Practices
- Run `snyk test` in CI/CD before deployment
- Set severity threshold (high or critical only)
- Use `snyk monitor` to track projects over time
- Create `.snyk` policy file for exceptions
- Scan Docker images before pushing to registry
- Test IaC files before applying changes
- Enable automated fix PRs for dependencies
- Review and update ignored vulnerabilities regularly

## Platform Support
- ✅ Linux (x64, ARM64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (x64)
- ✅ Docker containers
- ✅ Alpine Linux

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated security scanning in CI/CD
- Vulnerability detection and reporting
- Container image security validation
- Infrastructure as Code security checks
- License compliance enforcement
- Automated dependency updates

## Uninstall
```yaml
- preset: snyk
  with:
    state: absent
```

## Resources
- Website: https://snyk.io/
- Documentation: https://docs.snyk.io/
- CLI Reference: https://docs.snyk.io/snyk-cli
- GitHub: https://github.com/snyk/cli
- Search: "snyk test", "snyk container", "snyk iac"

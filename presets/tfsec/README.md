# tfsec - Terraform Security Scanner

Static analysis security scanner for Terraform. Find security issues before deployment, enforce policies, compliance checks.

## Quick Start
```yaml
- preset: tfsec
```

## Basic Usage
```bash
# Scan current directory
tfsec

# Scan specific directory
tfsec /path/to/terraform

# Scan with custom config
tfsec --config-file tfsec.yml

# Format output
tfsec --format json
tfsec --format sarif
```

## Output Formats
```bash
# Default (colorized)
tfsec

# JSON
tfsec --format json

# CSV
tfsec --format csv

# Checkstyle XML
tfsec --format checkstyle

# JUnit XML
tfsec --format junit

# SARIF (GitHub)
tfsec --format sarif

# GitHub annotations
tfsec --format github

# GitLab SAST
tfsec --format gitlab-sast
```

## Severity Filtering
```bash
# Only CRITICAL and HIGH
tfsec --minimum-severity HIGH

# All severities
tfsec --minimum-severity LOW

# Specific severity
tfsec --severity CRITICAL
tfsec --severity HIGH,MEDIUM
```

## Common Security Checks
```hcl
# Unencrypted S3 bucket
resource "aws_s3_bucket" "bad" {
  bucket = "my-bucket"
  # CRITICAL: Bucket does not have encryption enabled
}

resource "aws_s3_bucket" "good" {
  bucket = "my-bucket"
}

resource "aws_s3_bucket_server_side_encryption_configuration" "good" {
  bucket = aws_s3_bucket.good.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Public S3 bucket
resource "aws_s3_bucket_acl" "bad" {
  bucket = aws_s3_bucket.example.id
  acl    = "public-read"  # HIGH: Bucket is publicly accessible
}

# Security group too open
resource "aws_security_group_rule" "bad" {
  type        = "ingress"
  from_port   = 22
  to_port     = 22
  protocol    = "tcp"
  cidr_blocks = ["0.0.0.0/0"]  # CRITICAL: SSH exposed to internet
}

# Unencrypted RDS
resource "aws_db_instance" "bad" {
  allocated_storage = 10
  storage_encrypted = false  # HIGH: Database storage not encrypted
}
```

## Ignoring Issues
```hcl
# Inline ignore
resource "aws_s3_bucket" "example" {
  #tfsec:ignore:aws-s3-enable-bucket-encryption
  bucket = "my-bucket"
}

# With reason
resource "aws_security_group_rule" "admin" {
  #tfsec:ignore:aws-vpc-no-public-ingress-sgr Approved for admin access
  type        = "ingress"
  cidr_blocks = ["0.0.0.0/0"]
}

# Multiple ignores
resource "aws_instance" "example" {
  #tfsec:ignore:aws-ec2-enable-at-rest-encryption
  #tfsec:ignore:aws-ec2-enforce-http-token-imds
  ami           = "ami-12345678"
  instance_type = "t2.micro"
}
```

## Configuration File
```yaml
# tfsec.yml
severity_overrides:
  aws-s3-enable-bucket-encryption: ERROR
  aws-s3-enable-bucket-logging: WARNING

exclude:
  - aws-s3-enable-versioning  # Not needed for this project

minimum_severity: HIGH

exclude_paths:
  - tests/**
  - examples/**
```

## CI/CD Integration
```bash
# GitHub Actions
- name: tfsec
  uses: aquasecurity/tfsec-action@v1.0.0
  with:
    soft_fail: false
    format: sarif
    sarif_file: tfsec.sarif

- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v2
  with:
    sarif_file: tfsec.sarif

# GitLab CI
tfsec:
  image: aquasec/tfsec:latest
  script:
    - tfsec --format gitlab-sast > gl-sast-report.json
  artifacts:
    reports:
      sast: gl-sast-report.json

# Pre-commit
- repo: https://github.com/aquasecurity/tfsec
  rev: v1.28.0
  hooks:
    - id: tfsec
```

## Policy as Code
```yaml
# Custom checks
checks:
  - code: CUS001
    description: S3 buckets must have tags
    impact: Low
    resolution: Add required tags
    requiredTypes:
      - resource
    requiredLabels:
      - aws_s3_bucket
    severity: LOW
    matchSpec:
      name: tags
      action: requiresPresence
```

## AWS-Specific Checks
```bash
# S3 Security
- S3 bucket encryption
- S3 versioning
- S3 logging
- S3 public access block

# EC2 Security
- IMDSv2 enforcement
- EBS encryption
- Security group rules
- Key pair usage

# RDS Security
- Encryption at rest
- Encryption in transit
- Public accessibility
- Backup retention

# IAM Security
- Password policy
- MFA requirements
- Access key rotation
- Privilege escalation
```

## Multi-Cloud Support
```bash
# AWS
tfsec --include-aws

# Azure
tfsec --include-azure

# GCP
tfsec --include-gcp

# All clouds
tfsec  # Checks all by default
```

## Comparing Results
```bash
# Baseline
tfsec --format json > baseline.json

# Current
tfsec --format json > current.json

# Diff
diff baseline.json current.json
```

## Integration with Trivy
```bash
# Note: tfsec is now part of Trivy
trivy config .

# Still works
tfsec .

# Both scan IaC security issues
```

## Common Workflows
```bash
# Pre-deployment check
tfsec
terraform validate
terraform plan

# CI pipeline
#!/bin/bash
set -e
tfsec --format json > tfsec-report.json
if [ $? -ne 0 ]; then
  cat tfsec-report.json | jq '.results[] | select(.severity=="CRITICAL")'
  exit 1
fi

# Multiple environments
for env in dev staging prod; do
  echo "Scanning $env..."
  tfsec environments/$env --minimum-severity HIGH
done

# Generate report
tfsec --format html > security-report.html
```

## Best Practices
- **Run in CI/CD** before terraform apply
- **Set minimum severity** to HIGH for prod
- **Use inline ignores** with reasons
- **Track ignored issues** in documentation
- **Combine with tflint** for comprehensive checks
- **Export SARIF** for GitHub integration
- **Regular security reviews** of ignored issues

## Tips
- 1000+ built-in checks
- Fast (< 5 seconds for most projects)
- No external dependencies
- Works offline
- Multi-cloud support
- Custom policy support
- Now integrated into Trivy

## Agent Use
- Automated security scanning
- CI/CD security gates
- Compliance checking
- Policy enforcement
- Pre-deployment validation
- Security baseline verification

## Uninstall
```yaml
- preset: tfsec
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/aquasecurity/tfsec
- Docs: https://aquasecurity.github.io/tfsec/
- Checks: https://aquasecurity.github.io/tfsec/latest/checks/aws/
- Search: "tfsec checks", "tfsec ignore"

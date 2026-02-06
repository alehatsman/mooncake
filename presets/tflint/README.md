# tflint - Terraform Linter

Pluggable linter for Terraform. Catch errors, enforce best practices, cloud provider-specific checks.

## Quick Start
```yaml
- preset: tflint
```

## Features
- **Provider-specific validation**: Plugins for AWS, Azure, GCP with resource validation
- **Best practices enforcement**: Naming conventions, deprecations, unused declarations
- **Fast execution**: Less than 1 second for most Terraform projects
- **Pluggable architecture**: Extend with custom rules and provider plugins
- **Deep module inspection**: Validates referenced modules recursively
- **CI/CD friendly**: JSON, SARIF, JUnit output formats for integration
- **Cross-platform**: Linux, macOS, Windows

## Advanced Configuration
```yaml
# Install tflint (default)
- preset: tflint

# Uninstall tflint
- preset: tflint
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Windows (scoop, choco)

## Basic Usage
```bash
# Lint current directory
tflint

# Lint specific directory
tflint --chdir=modules/vpc

# Recursive
tflint --recursive

# Fix auto-fixable issues
tflint --fix
```

## Configuration
```hcl
# .tflint.hcl
plugin "aws" {
  enabled = true
  version = "0.29.0"
  source  = "github.com/terraform-linters/tflint-ruleset-aws"
}

plugin "azurerm" {
  enabled = true
  version = "0.25.0"
  source  = "github.com/terraform-linters/tflint-ruleset-azurerm"
}

rule "terraform_naming_convention" {
  enabled = true
  format  = "snake_case"
}

rule "terraform_deprecated_interpolation" {
  enabled = true
}

rule "terraform_unused_declarations" {
  enabled = true
}
```

## Plugin Management
```bash
# Initialize plugins
tflint --init

# List installed plugins
tflint --version

# Install specific version
# In .tflint.hcl
plugin "aws" {
  enabled = true
  version = "0.29.0"
}
```

## Output Formats
```bash
# Default format
tflint

# Compact
tflint --format compact

# JSON
tflint --format json

# Checkstyle XML
tflint --format checkstyle

# JUnit XML
tflint --format junit

# SARIF (for GitHub)
tflint --format sarif
```

## Common Rules
```hcl
# Naming conventions
rule "terraform_naming_convention" {
  enabled = true
  format  = "snake_case"

  resource {
    format = "snake_case"
  }

  data {
    format = "snake_case"
  }
}

# Deprecated features
rule "terraform_deprecated_interpolation" {
  enabled = true
}

rule "terraform_deprecated_index" {
  enabled = true
}

# Best practices
rule "terraform_unused_declarations" {
  enabled = true
}

rule "terraform_comment_syntax" {
  enabled = true
}

rule "terraform_documented_outputs" {
  enabled = true
}

rule "terraform_documented_variables" {
  enabled = true
}
```

## AWS Plugin Rules
```hcl
plugin "aws" {
  enabled = true
}

# Invalid instance type
rule "aws_instance_invalid_type" {
  enabled = true
}

# Invalid AMI
rule "aws_instance_invalid_ami" {
  enabled = true
}

# Invalid subnet
rule "aws_instance_invalid_subnet" {
  enabled = true
}

# S3 bucket name
rule "aws_s3_bucket_invalid_name" {
  enabled = true
}

# IAM policy
rule "aws_iam_policy_invalid_policy" {
  enabled = true
}
```

## CI/CD Integration
```bash
# GitHub Actions
- name: TFLint
  run: |
    tflint --init
    tflint --format sarif > tflint.sarif

- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v2
  with:
    sarif_file: tflint.sarif

# GitLab CI
tflint:
  image: ghcr.io/terraform-linters/tflint:latest
  script:
    - tflint --init
    - tflint --recursive
  allow_failure: false

# Pre-commit hook
- repo: https://github.com/terraform-linters/tflint
  rev: v0.50.0
  hooks:
    - id: tflint
      args: [--init]
```

## Ignoring Rules
```hcl
# Disable specific rule
rule "aws_instance_invalid_type" {
  enabled = false
}

# Inline ignore
resource "aws_instance" "example" {
  # tflint-ignore: aws_instance_invalid_type
  instance_type = "t2.micro"
}

# Ignore multiple
resource "aws_instance" "example" {
  # tflint-ignore: aws_instance_invalid_type, aws_instance_invalid_ami
  instance_type = "t2.micro"
  ami           = "ami-12345678"
}
```

## Module Inspection
```bash
# Deep check (inspect modules)
tflint --module

# Recursive modules
tflint --recursive --module

# Specific module
tflint --chdir=modules/vpc --module
```

## Variables
```bash
# Set variables
tflint --var="instance_type=t3.micro"

# Variable file
tflint --var-file=prod.tfvars

# Multiple var files
tflint --var-file=common.tfvars --var-file=prod.tfvars
```

## Comparison with Other Tools
| Feature | tflint | terraform validate | tfsec | checkov |
|---------|--------|-------------------|-------|---------|
| Syntax | Yes | Yes | No | No |
| Best practices | Yes | No | No | Limited |
| Security | Plugin | No | Yes | Yes |
| Custom rules | Yes | No | Limited | Yes |
| Speed | Fast | Fastest | Fast | Slow |

## Real-World Examples
```bash
# Full check with AWS plugin
tflint --init
tflint --module --recursive

# CI pipeline
#!/bin/bash
set -e
tflint --init
tflint --format compact
if [ $? -ne 0 ]; then
  echo "TFLint found issues"
  exit 1
fi

# Pre-deployment validation
tflint --var-file=prod.tfvars
terraform validate
terraform plan

# Multi-environment
for env in dev staging prod; do
  echo "Linting $env..."
  tflint --chdir=environments/$env
done
```

## Best Practices
- **Run tflint --init** in CI to install plugins
- **Use plugins** for cloud-specific checks
- **Enable naming conventions**
- **Run with --module** for deep checking
- **Use --recursive** for monorepos
- **Integrate with pre-commit**
- **Output SARIF** for GitHub integration

## Tips
- Catches invalid resource configurations
- Provider-specific validation
- Fast (< 1 second for most projects)
- Pluggable architecture
- Works offline after init
- Integrates with most CI/CD
- Custom rules via plugins

## Agent Use
- Automated Terraform validation
- CI/CD quality gates
- Pre-commit hooks
- Best practices enforcement
- Multi-environment validation
- Configuration drift detection

## Uninstall
```yaml
- preset: tflint
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/terraform-linters/tflint
- Docs: https://github.com/terraform-linters/tflint/tree/master/docs
- Plugins: https://github.com/terraform-linters/tflint/blob/master/docs/user-guide/plugins.md
- Search: "tflint rules", "tflint aws"

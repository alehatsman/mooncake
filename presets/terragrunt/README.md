# terragrunt - Terraform Wrapper

Keep Terraform code DRY. Manage multiple environments, remote state, dependencies, and before/after hooks.

## Quick Start
```yaml
- preset: terragrunt
```

## Basic Usage
```bash
# Run terraform commands through terragrunt
terragrunt init
terragrunt plan
terragrunt apply
terragrunt destroy

# Auto-approve
terragrunt apply -auto-approve

# Run in all modules
terragrunt run-all apply
```

## Configuration
```hcl
# terragrunt.hcl
terraform {
  source = "git::https://github.com/org/modules.git//vpc?ref=v1.0.0"
}

# Remote state
remote_state {
  backend = "s3"
  config = {
    bucket         = "my-terraform-state"
    key            = "${path_relative_to_include()}/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"
  }
}

# Inputs (variables)
inputs = {
  environment = "production"
  vpc_cidr    = "10.0.0.0/16"
}
```

## DRY Configuration
```hcl
# root terragrunt.hcl
remote_state {
  backend = "s3"
  config = {
    bucket = "my-state-bucket"
    key    = "${path_relative_to_include()}/terraform.tfstate"
    region = "us-east-1"
  }
}

# dev/vpc/terragrunt.hcl
include "root" {
  path = find_in_parent_folders()
}

terraform {
  source = "git::https://github.com/org/modules.git//vpc"
}

inputs = {
  environment = "dev"
  vpc_cidr    = "10.1.0.0/16"
}
```

## Directory Structure
```
infrastructure/
├── terragrunt.hcl          # Root config
├── dev/
│   ├── vpc/
│   │   └── terragrunt.hcl  # Inherits root + specific inputs
│   ├── rds/
│   │   └── terragrunt.hcl
│   └── eks/
│       └── terragrunt.hcl
└── prod/
    ├── vpc/
    │   └── terragrunt.hcl
    ├── rds/
    │   └── terragrunt.hcl
    └── eks/
        └── terragrunt.hcl
```

## Dependencies
```hcl
# rds/terragrunt.hcl
dependency "vpc" {
  config_path = "../vpc"
}

inputs = {
  vpc_id     = dependency.vpc.outputs.vpc_id
  subnet_ids = dependency.vpc.outputs.private_subnet_ids
}
```

## Run All Commands
```bash
# Apply all modules
terragrunt run-all apply

# Plan all modules
terragrunt run-all plan

# Destroy all modules
terragrunt run-all destroy

# With dependencies
terragrunt run-all apply --terragrunt-non-interactive

# Specific directories
cd dev/
terragrunt run-all plan
```

## Hooks
```hcl
terraform {
  before_hook "before_init" {
    commands = ["init"]
    execute  = ["echo", "Running init..."]
  }

  after_hook "after_apply" {
    commands     = ["apply"]
    execute      = ["./notify-slack.sh", "deployed"]
    run_on_error = false
  }
}
```

## Remote State Management
```hcl
# Auto-create S3 bucket and DynamoDB table
remote_state {
  backend = "s3"
  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
  config = {
    bucket         = "my-terraform-state"
    key            = "${path_relative_to_include()}/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"

    s3_bucket_tags = {
      Name = "Terraform State"
    }

    dynamodb_table_tags = {
      Name = "Terraform Locks"
    }
  }
}
```

## Generate Files
```hcl
# Generate provider.tf
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region = "us-east-1"
}
EOF
}

# Generate versions.tf
generate "versions" {
  path      = "versions.tf"
  if_exists = "overwrite"
  contents  = <<EOF
terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
EOF
}
```

## Multiple Environments
```hcl
# env.hcl (per environment)
locals {
  environment = "dev"
  region      = "us-east-1"
}

# terragrunt.hcl
locals {
  env_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
}

inputs = {
  environment = local.env_vars.locals.environment
  region      = local.env_vars.locals.region
}
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Terragrunt Plan
  run: |
    cd infrastructure/dev
    terragrunt run-all plan

- name: Terragrunt Apply
  run: |
    cd infrastructure/prod
    terragrunt run-all apply -auto-approve

# GitLab CI
terragrunt:plan:
  script:
    - cd infrastructure/$ENVIRONMENT
    - terragrunt run-all plan

terragrunt:apply:
  script:
    - cd infrastructure/$ENVIRONMENT
    - terragrunt run-all apply -auto-approve
  when: manual
```

## Debugging
```bash
# Verbose output
terragrunt plan --terragrunt-log-level debug

# Show configuration
terragrunt terragrunt-info

# Validate configuration
terragrunt validate-all

# Graph dependencies
terragrunt graph-dependencies | dot -Tpng > graph.png
```

## Working Directory
```bash
# Specify working dir
terragrunt plan --terragrunt-working-dir dev/vpc

# Use environment variable
export TERRAGRUNT_WORKING_DIR=dev/vpc
terragrunt plan
```

## Common Patterns
```hcl
# Read YAML config
locals {
  config = yamldecode(file("config.yaml"))
}

inputs = {
  vpc_cidr = local.config.vpc_cidr
}

# Conditional inputs
inputs = {
  enable_monitoring = get_env("ENVIRONMENT") == "prod" ? true : false
}

# Dynamic source
terraform {
  source = get_env("TF_MODULE_SOURCE", "git::https://github.com/org/modules.git//vpc")
}
```

## Best Practices
- **Use version pinning** for modules
- **Keep state per environment** (separate backends)
- **Use dependencies** instead of data sources where possible
- **Generate common files** (provider, versions)
- **Use run-all** for multi-module changes
- **Enable state locking** (DynamoDB)
- **Tag resources** via inputs

## Tips
- DRY configuration (Don't Repeat Yourself)
- Automatic backend configuration
- Dependency management
- Hooks for custom logic
- Works with any Terraform module
- Multi-environment support
- Parallel execution

## Agent Use
- Multi-environment deployment
- Infrastructure automation
- State management
- Dependency orchestration
- CI/CD pipeline integration
- Configuration templating

## Uninstall
```yaml
- preset: terragrunt
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/gruntwork-io/terragrunt
- Docs: https://terragrunt.gruntwork.io/
- Search: "terragrunt examples", "terragrunt dry"

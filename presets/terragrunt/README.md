# terragrunt - Terraform Wrapper

Keep Terraform code DRY. Orchestrate multiple environments, manage remote state, handle dependencies, and automate workflows with hooks.

## Quick Start
```yaml
- preset: terragrunt
```

## Features
- **DRY configuration**: Share common settings across environments
- **Remote state automation**: Auto-create S3 buckets, DynamoDB tables, GCS buckets
- **Dependency management**: Execute modules in correct order with outputs
- **Multiple environments**: Manage dev/staging/prod with minimal duplication
- **Before/after hooks**: Run custom commands during Terraform lifecycle
- **Run-all commands**: Execute operations across all modules in parallel
- **Generate files**: Auto-create provider, backend, and version files

## Basic Usage
```bash
# Run Terraform commands through Terragrunt
terragrunt init
terragrunt plan
terragrunt apply
terragrunt destroy

# Auto-approve
terragrunt apply -auto-approve

# Run in all modules
terragrunt run-all apply
terragrunt run-all plan
terragrunt run-all destroy
```

## Advanced Configuration
```yaml
# Install terragrunt (default)
- preset: terragrunt

# Uninstall terragrunt
- preset: terragrunt
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

## Configuration Structure
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

## Directory Structure
```
infrastructure/
├── terragrunt.hcl          # Root config (shared settings)
├── dev/
│   ├── vpc/
│   │   └── terragrunt.hcl  # Inherits root + env-specific inputs
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

## DRY Configuration
```hcl
# root/terragrunt.hcl - Shared configuration
remote_state {
  backend = "s3"
  config = {
    bucket = "my-state-bucket"
    key    = "${path_relative_to_include()}/terraform.tfstate"
    region = "us-east-1"
  }
}

# dev/vpc/terragrunt.hcl - Environment-specific
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

# eks/terragrunt.hcl
dependency "vpc" {
  config_path = "../vpc"
}

dependency "rds" {
  config_path = "../rds"
}

inputs = {
  vpc_id       = dependency.vpc.outputs.vpc_id
  subnet_ids   = dependency.vpc.outputs.private_subnet_ids
  db_endpoint  = dependency.rds.outputs.endpoint
}
```

## Run-All Commands
```bash
# Apply all modules (respects dependencies)
terragrunt run-all apply

# Plan all modules
terragrunt run-all plan

# Destroy all modules (reverse order)
terragrunt run-all destroy

# Non-interactive mode
terragrunt run-all apply --terragrunt-non-interactive

# Specific directories
cd dev/
terragrunt run-all plan
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
      Team = "Infrastructure"
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

  default_tags {
    tags = {
      Environment = "production"
      ManagedBy   = "Terragrunt"
    }
  }
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

## Hooks
```hcl
terraform {
  before_hook "before_init" {
    commands = ["init"]
    execute  = ["echo", "Initializing Terraform..."]
  }

  after_hook "after_apply" {
    commands     = ["apply"]
    execute      = ["./notify-slack.sh", "deployed"]
    run_on_error = false
  }

  after_hook "post_destroy" {
    commands     = ["destroy"]
    execute      = ["./cleanup.sh"]
    run_on_error = true
  }
}
```

## Configuration
- **Root config**: `terragrunt.hcl` in project root for shared settings
- **Module config**: `terragrunt.hcl` in each module directory
- **Cache**: `.terragrunt-cache/` in each module (gitignored)
- **Working dir**: Terragrunt downloads modules to cache before running

## Multiple Environments
```hcl
# env.hcl (per environment)
locals {
  environment = "dev"
  region      = "us-east-1"
  account_id  = "123456789012"
}

# terragrunt.hcl
locals {
  env_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
}

inputs = {
  environment = local.env_vars.locals.environment
  region      = local.env_vars.locals.region
  account_id  = local.env_vars.locals.account_id
}
```

## Real-World Examples

### Multi-Environment Infrastructure
```
infrastructure/
├── terragrunt.hcl          # Shared config
├── dev/
│   ├── env.hcl             # Dev-specific vars
│   ├── vpc/terragrunt.hcl
│   ├── eks/terragrunt.hcl
│   └── rds/terragrunt.hcl
├── staging/
│   ├── env.hcl
│   ├── vpc/terragrunt.hcl
│   ├── eks/terragrunt.hcl
│   └── rds/terragrunt.hcl
└── prod/
    ├── env.hcl
    ├── vpc/terragrunt.hcl
    ├── eks/terragrunt.hcl
    └── rds/terragrunt.hcl
```

### CI/CD Integration
```yaml
# GitHub Actions
- name: Terragrunt Plan
  run: |
    cd infrastructure/dev
    terragrunt run-all plan

- name: Terragrunt Apply
  run: |
    cd infrastructure/prod
    terragrunt run-all apply -auto-approve
  if: github.ref == 'refs/heads/main'
```

### GitLab CI
```yaml
terragrunt:plan:
  script:
    - cd infrastructure/$ENVIRONMENT
    - terragrunt run-all plan

terragrunt:apply:
  script:
    - cd infrastructure/$ENVIRONMENT
    - terragrunt run-all apply -auto-approve
  when: manual
  only:
    - main
```

## Debugging
```bash
# Verbose output
terragrunt plan --terragrunt-log-level debug

# Show configuration
terragrunt terragrunt-info

# Validate all configurations
terragrunt validate-all

# Graph dependencies
terragrunt graph-dependencies | dot -Tpng > graph.png

# Show execution plan
terragrunt run-all plan --terragrunt-log-level info
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

### Read YAML Config
```hcl
locals {
  config = yamldecode(file("config.yaml"))
}

inputs = {
  vpc_cidr = local.config.vpc_cidr
  subnets  = local.config.subnets
}
```

### Conditional Inputs
```hcl
inputs = {
  enable_monitoring = get_env("ENVIRONMENT") == "prod" ? true : false
  instance_count    = get_env("ENVIRONMENT") == "prod" ? 5 : 1
}
```

### Dynamic Source
```hcl
terraform {
  source = get_env("TF_MODULE_SOURCE", "git::https://github.com/org/modules.git//vpc")
}
```

### Module Versioning
```hcl
locals {
  module_version = "v2.0.0"
}

terraform {
  source = "git::https://github.com/org/modules.git//vpc?ref=${local.module_version}"
}
```

## Advanced Features

### Mock Outputs
```hcl
# For development/testing
dependency "vpc" {
  config_path = "../vpc"

  mock_outputs = {
    vpc_id = "vpc-12345678"
    subnet_ids = ["subnet-1", "subnet-2"]
  }
  mock_outputs_allowed_terraform_commands = ["validate", "plan"]
}
```

### Skip Dependencies
```bash
# Skip dependency checks
terragrunt apply --terragrunt-ignore-dependency-errors

# Exclude specific modules
terragrunt run-all apply --terragrunt-exclude-dir dev/eks
```

### Parallel Execution
```bash
# Control parallelism
terragrunt run-all apply --terragrunt-parallelism 5

# Disable parallel execution
terragrunt run-all apply --terragrunt-parallelism 1
```

## Best Practices
- **Version pinning**: Pin module versions in source URLs
- **Separate backends**: Use different S3 buckets/prefixes per environment
- **Dependencies over data sources**: Use `dependency` blocks for module outputs
- **Generate common files**: Auto-generate provider and version files
- **Use run-all**: Leverage parallel execution for multi-module changes
- **State locking**: Enable DynamoDB locking to prevent concurrent modifications
- **Tag resources**: Include environment tags via inputs

## Tips
- DRY configuration eliminates duplicate code
- Automatic backend setup and state management
- Dependency resolution with output passing
- Hooks enable custom workflows (notifications, cleanup, validation)
- Works with any Terraform module
- Multi-environment support with minimal duplication
- Parallel execution speeds up large deployments

## Agent Use
- Multi-environment infrastructure deployment
- Automated infrastructure provisioning
- Remote state management
- Dependency orchestration
- CI/CD pipeline integration
- Configuration templating and generation

## Uninstall
```yaml
- preset: terragrunt
  with:
    state: absent
```

**Note:** Terraform state files and infrastructure remain after uninstall. Only the Terragrunt CLI is removed.

## Resources
- GitHub: https://github.com/gruntwork-io/terragrunt
- Docs: https://terragrunt.gruntwork.io/
- Examples: https://github.com/gruntwork-io/terragrunt-infrastructure-live-example
- Search: "terragrunt examples", "terragrunt dry configuration"

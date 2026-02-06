# Terraform Preset

Infrastructure as Code tool. Provision and manage cloud resources declaratively across AWS, Azure, GCP, and 3000+ providers.

## Features
- **Multi-cloud**: AWS, Azure, GCP, and 3000+ providers
- **Declarative**: Define desired state, Terraform handles changes
- **State management**: Track infrastructure in state files
- **Modules**: Reusable infrastructure components
- **Plan & Apply**: Preview changes before applying
- **Resource graph**: Understand dependencies
- **Workspaces**: Manage multiple environments
- **Version control**: Store configurations in git

## Quick Start

```bash
# Check version
terraform version

# Initialize project
terraform init

# Validate configuration
terraform validate

# Plan changes
terraform plan

# Apply changes
terraform apply

# Destroy infrastructure
terraform destroy
```

## Configuration

- **Config files:** `*.tf` in current directory
- **State file:** `terraform.tfstate`
- **Variables:** `terraform.tfvars`
- **Modules:** `.terraform/` directory

## Project Structure

```
project/
├── main.tf           # Main configuration
├── variables.tf      # Input variables
├── outputs.tf        # Output values
├── terraform.tfvars  # Variable values
└── .terraform/       # Plugins and modules
```

## Basic Configuration

```hcl
# main.tf
terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

resource "aws_instance" "example" {
  ami           = var.ami_id
  instance_type = var.instance_type

  tags = {
    Name = "example-instance"
  }
}
```

## Variables

```hcl
# variables.tf
variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t2.micro"
}

# terraform.tfvars
aws_region    = "us-west-2"
instance_type = "t3.small"
```

## Outputs

```hcl
# outputs.tf
output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.example.id
}

output "public_ip" {
  description = "Public IP of instance"
  value       = aws_instance.example.public_ip
}
```

## Common Commands

```bash
# Initialize
terraform init
terraform init -upgrade  # Upgrade providers

# Validate
terraform validate
terraform fmt  # Format code

# Plan
terraform plan
terraform plan -out=plan.tfplan

# Apply
terraform apply
terraform apply plan.tfplan
terraform apply -auto-approve  # Skip confirmation

# Destroy
terraform destroy
terraform destroy -target=resource.name

# State management
terraform state list
terraform state show resource.name
terraform state rm resource.name

# Workspace
terraform workspace list
terraform workspace new dev
terraform workspace select dev

# Import existing resource
terraform import aws_instance.example i-1234567890

# Output values
terraform output
terraform output instance_id
```

## Remote State

```hcl
# backend.tf
terraform {
  backend "s3" {
    bucket = "my-terraform-state"
    key    = "project/terraform.tfstate"
    region = "us-east-1"
  }
}
```

## Modules

```hcl
# Using a module
module "vpc" {
  source = "./modules/vpc"

  vpc_cidr = "10.0.0.0/16"
  name     = "my-vpc"
}

# Using from registry
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "my-vpc"
  cidr = "10.0.0.0/16"
}
```

## Data Sources

```hcl
# Query existing resources
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]  # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
}

# Use in resource
resource "aws_instance" "example" {
  ami = data.aws_ami.ubuntu.id
}
```

## Provisioners

```hcl
resource "aws_instance" "example" {
  ami           = var.ami_id
  instance_type = var.instance_type

  provisioner "remote-exec" {
    inline = [
      "sudo apt-get update",
      "sudo apt-get install -y nginx"
    ]

    connection {
      type        = "ssh"
      user        = "ubuntu"
      private_key = file("~/.ssh/id_rsa")
      host        = self.public_ip
    }
  }
}
```

## Best Practices

- Use version constraints for providers
- Store state remotely (S3, Terraform Cloud)
- Use workspaces for environments
- Module composition
- Use data sources for lookups
- Tag all resources
- Use `.gitignore` for sensitive files

## .gitignore

```
# Terraform
.terraform/
*.tfstate
*.tfstate.*
*.tfvars
crash.log
override.tf
override.tf.json
*_override.tf
*_override.tf.json
.terraformrc
terraform.rc
```

## Basic Usage
```bash
# Initialize Terraform
terraform init

# Plan changes
terraform plan

# Apply changes
terraform apply

# Destroy infrastructure
terraform destroy
```

## Advanced Configuration

### Backend Configuration
```hcl
# Remote state with locking
terraform {
  backend "s3" {
    bucket         = "terraform-state"
    key            = "prod/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"
  }
}
```

### Module Composition
```hcl
# modules/vpc/main.tf
module "network" {
  source = "./modules/vpc"
  cidr   = var.vpc_cidr
}

module "compute" {
  source = "./modules/ec2"
  vpc_id = module.network.vpc_id
  subnet_ids = module.network.subnet_ids
}
```

### Automated Testing
```bash
# Terraform validate and plan in CI
terraform init -backend=false
terraform validate
terraform fmt -check
terraform plan -out=plan.tfplan
```

## Platform Support
- ✅ Linux (amd64, arm64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (amd64)
- ✅ BSD systems
- ✅ Docker container

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove terraform |

## Uninstall

```yaml
- preset: terraform
  with:
    state: absent
```

**Note:** Terraform state files preserved after uninstall.

## Agent Use
- Automate infrastructure provisioning in CI/CD pipelines
- Generate Terraform configurations from templates
- Validate and plan infrastructure changes
- Manage state and workspace operations
- Deploy multi-environment infrastructure

## Resources
- Official docs: https://www.terraform.io/docs
- Registry: https://registry.terraform.io/
- Tutorials: https://learn.hashicorp.com/terraform
- Search: "terraform tutorial", "terraform best practices", "terraform modules"

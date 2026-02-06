# Pulumi - Modern Infrastructure as Code

Universal infrastructure as code using real programming languages. Deploy to AWS, Azure, GCP, Kubernetes, and 100+ providers.

## Quick Start
```yaml
- preset: pulumi
```

## Features
- **Real languages**: TypeScript, Python, Go, C#, Java, YAML
- **Multi-cloud**: AWS, Azure, GCP, Kubernetes, 100+ providers
- **State management**: Built-in state backend with encryption
- **Preview changes**: See what will change before deploying
- **Policy as code**: Enforce compliance and best practices
- **Secrets management**: Encrypted configuration values

## Basic Usage
```bash
# Initialize new project
pulumi new aws-typescript
pulumi new azure-python
pulumi new kubernetes-go

# Install dependencies
npm install  # TypeScript/JavaScript
pip install -r requirements.txt  # Python

# Preview changes
pulumi preview

# Deploy infrastructure
pulumi up

# View outputs
pulumi stack output

# Destroy infrastructure
pulumi destroy

# View stack
pulumi stack

# Export/import state
pulumi stack export > backup.json
pulumi stack import < backup.json
```

## Advanced Configuration

### CI/CD deployment
```yaml
- name: Install Pulumi
  preset: pulumi
  become: true

- name: Configure Pulumi token
  shell: pulumi login --access-token {{ pulumi_token }}
  environment:
    PULUMI_ACCESS_TOKEN: "{{ pulumi_token }}"

- name: Select stack
  shell: pulumi stack select production
  cwd: /app/infrastructure

- name: Deploy infrastructure
  shell: pulumi up --yes --skip-preview
  cwd: /app/infrastructure
  register: deploy_result
```

### Multi-environment setup
```yaml
- name: Deploy to staging
  shell: |
    pulumi stack select staging
    pulumi config set aws:region us-east-1
    pulumi up --yes
  cwd: /infra

- name: Deploy to production
  shell: |
    pulumi stack select production
    pulumi config set aws:region us-west-2
    pulumi up --yes
  cwd: /infra
```

### Preview before merge
```yaml
- name: Install Pulumi
  preset: pulumi

- name: Preview changes
  shell: pulumi preview --diff
  cwd: /app/infra
  register: preview_result

- name: Post preview to PR
  shell: |
    echo "Infrastructure Changes:"
    echo "{{ preview_result.stdout }}"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Pulumi |

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, script)
- ✅ Windows (Chocolatey, script)
- ✅ Cross-platform binary releases

## Configuration
- **Config file**: `Pulumi.yaml` (project), `Pulumi.<stack>.yaml` (stack)
- **State backend**: Pulumi Cloud (default), S3, Azure Blob, local file
- **Credentials**: `~/.pulumi/credentials.json`
- **Plugins**: `~/.pulumi/plugins/`
- **Cache**: `~/.pulumi/cache/`

## Real-World Examples

### AWS infrastructure
```typescript
// index.ts
import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

// Create VPC
const vpc = new aws.ec2.Vpc("main", {
    cidrBlock: "10.0.0.0/16",
    enableDnsHostnames: true,
});

// Create subnet
const subnet = new aws.ec2.Subnet("public", {
    vpcId: vpc.id,
    cidrBlock: "10.0.1.0/24",
    availabilityZone: "us-east-1a",
});

// Export values
export const vpcId = vpc.id;
export const subnetId = subnet.id;
```

```yaml
# Deploy
- name: Deploy AWS infrastructure
  shell: |
    npm install
    pulumi stack select prod
    pulumi up --yes
  cwd: /infra/aws
```

### Kubernetes cluster
```python
# __main__.py
import pulumi
import pulumi_kubernetes as k8s

# Create deployment
app = k8s.apps.v1.Deployment("nginx",
    spec=k8s.apps.v1.DeploymentSpecArgs(
        replicas=3,
        selector=k8s.meta.v1.LabelSelectorArgs(
            match_labels={"app": "nginx"}
        ),
        template=k8s.core.v1.PodTemplateSpecArgs(
            metadata=k8s.meta.v1.ObjectMetaArgs(
                labels={"app": "nginx"}
            ),
            spec=k8s.core.v1.PodSpecArgs(
                containers=[k8s.core.v1.ContainerArgs(
                    name="nginx",
                    image="nginx:latest",
                    ports=[k8s.core.v1.ContainerPortArgs(
                        container_port=80
                    )]
                )]
            )
        )
    )
)

pulumi.export("deployment_name", app.metadata["name"])
```

### Multi-cloud setup
```yaml
- name: Deploy to AWS
  shell: pulumi up --yes
  cwd: /infra/aws

- name: Deploy to Azure
  shell: pulumi up --yes
  cwd: /infra/azure

- name: Deploy to GCP
  shell: pulumi up --yes
  cwd: /infra/gcp
```

## Project Structure
```
myproject/
├── Pulumi.yaml              # Project definition
├── Pulumi.dev.yaml          # Dev stack config
├── Pulumi.prod.yaml         # Prod stack config
├── index.ts                 # Main infrastructure code
├── package.json             # Dependencies (TypeScript)
├── requirements.txt         # Dependencies (Python)
└── .gitignore               # Ignore node_modules, venv
```

## Stack Management
```bash
# List stacks
pulumi stack ls

# Create stack
pulumi stack init staging

# Select stack
pulumi stack select production

# View stack
pulumi stack

# Rename stack
pulumi stack rename new-name

# Remove stack
pulumi stack rm old-stack

# View outputs
pulumi stack output
pulumi stack output vpcId
```

## Configuration Management
```bash
# Set configuration
pulumi config set aws:region us-west-2
pulumi config set instanceType t2.micro

# Set secret
pulumi config set --secret dbPassword myPassword123

# Get configuration
pulumi config get aws:region

# List configuration
pulumi config

# Remove configuration
pulumi config rm instanceType
```

## State Backends

### Pulumi Cloud (default)
```bash
pulumi login
```

### Self-managed S3
```bash
pulumi login s3://my-pulumi-state-bucket
```

### Azure Blob Storage
```bash
pulumi login azblob://my-container
```

### Local filesystem
```bash
pulumi login file://~
```

## Secrets Management
```bash
# Set secret
pulumi config set --secret apiKey abc123

# Use in code (TypeScript)
import * as pulumi from "@pulumi/pulumi";
const config = new pulumi.Config();
const apiKey = config.requireSecret("apiKey");

# Use in code (Python)
import pulumi
config = pulumi.Config()
api_key = config.require_secret("apiKey")
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install Pulumi
  preset: pulumi

- name: Deploy
  shell: pulumi up --yes
  cwd: ./infra
  environment:
    PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_TOKEN }}
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_KEY }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET }}
```

### GitLab CI
```yaml
- name: Install Pulumi
  preset: pulumi

- name: Deploy infrastructure
  shell: |
    pulumi login --access-token $PULUMI_TOKEN
    pulumi stack select production
    pulumi up --yes
  environment:
    PULUMI_ACCESS_TOKEN: "{{ pulumi_token }}"
```

## Providers

Pulumi supports 100+ cloud providers:
- **AWS**: EC2, S3, RDS, Lambda, EKS, etc.
- **Azure**: VMs, Storage, AKS, Functions, etc.
- **GCP**: Compute, Storage, GKE, Cloud Functions, etc.
- **Kubernetes**: Deployments, Services, Ingress, etc.
- **Docker**: Containers, images, networks, etc.
- **CloudFlare**, **DigitalOcean**, **Datadog**, and more

## Policy as Code
```typescript
// policy.ts
import * as policy from "@pulumi/policy";

new policy.PolicyPack("aws-policies", {
    policies: [
        {
            name: "s3-no-public-read",
            description: "Prohibit public read on S3 buckets",
            enforcementLevel: "mandatory",
            validateResource: policy.validateResourceOfType(
                aws.s3.Bucket,
                (bucket, args, reportViolation) => {
                    if (bucket.acl === "public-read") {
                        reportViolation("S3 buckets cannot be public");
                    }
                }
            ),
        },
    ],
});
```

## Environment Variables
```bash
# Access token
export PULUMI_ACCESS_TOKEN=pul-abc123

# Skip update check
export PULUMI_SKIP_UPDATE_CHECK=true

# Backend URL
export PULUMI_BACKEND_URL=s3://my-state-bucket

# Parallelism
export PULUMI_PARALLEL=10

# Skip confirmations
export PULUMI_SKIP_CONFIRMATIONS=true
```

## Agent Use
- Automated infrastructure provisioning
- Multi-cloud deployments
- GitOps workflows with infrastructure changes
- Environment replication (staging to production)
- Disaster recovery automation
- Compliance enforcement via policies
- Cost optimization through resource management

## Troubleshooting

### State corruption
```bash
# Export state
pulumi stack export > backup.json

# Cancel in-progress update
pulumi cancel

# Refresh state
pulumi refresh
```

### Plugin errors
```bash
# Remove plugins
rm -rf ~/.pulumi/plugins/

# Reinstall plugins
pulumi plugin install
```

### Authentication issues
```bash
# Login again
pulumi logout
pulumi login

# Check credentials
cat ~/.pulumi/credentials.json
```

### Dependency conflicts
```bash
# TypeScript
rm -rf node_modules package-lock.json
npm install

# Python
pip install --upgrade -r requirements.txt
```

## Best Practices
- **Use stacks**: Separate dev, staging, production
- **Store secrets**: Use pulumi config --secret
- **Version control**: Commit Pulumi.yaml and code
- **State backend**: Use remote backend (S3, Pulumi Cloud)
- **Preview first**: Always run pulumi preview
- **Policy as code**: Enforce compliance automatically
- **Organize code**: Split into modules/components
- **CI/CD integration**: Automate deployments
- **Document outputs**: Export important resource IDs

## Comparison

| Tool | Language | State | Preview | Multi-cloud |
|------|----------|-------|---------|-------------|
| Pulumi | TypeScript, Python, Go, C# | Managed | ✅ | ✅ |
| Terraform | HCL | Local/Remote | ✅ | ✅ |
| CloudFormation | YAML/JSON | AWS | ✅ | ❌ |
| Ansible | YAML | Stateless | ❌ | ✅ |

## Uninstall
```yaml
- preset: pulumi
  with:
    state: absent
```

**Note**: This removes the Pulumi CLI but keeps your state and projects.

## Resources
- Official docs: https://www.pulumi.com/docs/
- GitHub: https://github.com/pulumi/pulumi
- Examples: https://github.com/pulumi/examples
- Registry: https://www.pulumi.com/registry/
- Search: "pulumi tutorial", "pulumi aws", "pulumi kubernetes"

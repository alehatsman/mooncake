# AWS CLI - Amazon Web Services Command Line Interface

Official command-line interface for managing AWS services. Control and automate hundreds of AWS services from the terminal.

## Quick Start
```yaml
- preset: awscli
```

## Features
- **Comprehensive**: Manage 200+ AWS services from the command line
- **Multiple versions**: Support for both AWS CLI v1 (Python) and v2 (standalone)
- **Automation-ready**: JSON/YAML output for scripting and CI/CD
- **Credential management**: Supports profiles, roles, SSO, and environment variables
- **S3 transfer acceleration**: High-performance file uploads/downloads
- **CloudShell integration**: Pre-installed in AWS CloudShell
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Check version
aws --version

# Configure credentials (interactive)
aws configure

# List S3 buckets
aws s3 ls

# List EC2 instances
aws ec2 describe-instances

# Get caller identity
aws sts get-caller-identity

# List Lambda functions
aws lambda list-functions
```

## Advanced Configuration
```yaml
# Install AWS CLI v2 with profile configuration
- preset: awscli
  with:
    version: "2"
    configure: true
    access_key_id: "{{ aws_access_key }}"
    secret_access_key: "{{ aws_secret_key }}"
    region: "us-west-2"
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |
| version | string | "2" | AWS CLI version ("1" or "2") |
| configure | bool | false | Run aws configure |
| access_key_id | string | - | AWS Access Key ID |
| secret_access_key | string | - | AWS Secret Access Key |
| region | string | us-east-1 | Default AWS region |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk, install script)
- ✅ macOS (Homebrew, install script)
- ⚠️ Windows (manual installation via MSI)

## Configuration

### Credentials Location
- **Config**: `~/.aws/config`
- **Credentials**: `~/.aws/credentials`
- **Environment variables**: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`

### Multiple Profiles
```bash
# Configure named profile
aws configure --profile production
aws configure --profile staging

# Use profile
aws s3 ls --profile production
export AWS_PROFILE=production

# List profiles
cat ~/.aws/config
```

### Configuration File Format
```ini
# ~/.aws/config
[default]
region = us-east-1
output = json

[profile production]
region = us-west-2
output = json
role_arn = arn:aws:iam::123456789012:role/ProductionRole
source_profile = default

# ~/.aws/credentials
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

[production]
aws_access_key_id = AKIAI44QH8DHBEXAMPLE
aws_secret_access_key = je7MtGbClwBF/2Zp9Utk/h3yCo8nvbEXAMPLEKEY
```

## Real-World Examples

### CI/CD Deployment
```bash
# Deploy Lambda function from CI/CD
aws lambda update-function-code \
    --function-name my-function \
    --zip-file fileb://function.zip \
    --region us-east-1

# Update environment variables
aws lambda update-function-configuration \
    --function-name my-function \
    --environment "Variables={ENV=production,DEBUG=false}"
```

### Infrastructure Management
```bash
# Create S3 bucket with versioning
aws s3 mb s3://my-unique-bucket-name
aws s3api put-bucket-versioning \
    --bucket my-unique-bucket-name \
    --versioning-configuration Status=Enabled

# Launch EC2 instance
aws ec2 run-instances \
    --image-id ami-0c55b159cbfafe1f0 \
    --instance-type t3.micro \
    --key-name my-keypair \
    --security-group-ids sg-0123456789abcdef \
    --subnet-id subnet-0123456789abcdef \
    --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=MyInstance}]'
```

### Backup and Sync
```bash
# Sync local directory to S3
aws s3 sync ./website s3://my-website-bucket --delete

# Download entire bucket
aws s3 sync s3://my-backup-bucket ./backups

# Copy between buckets
aws s3 sync s3://source-bucket s3://destination-bucket
```

### Query and Filter with JMESPath
```bash
# List running instances with specific tag
aws ec2 describe-instances \
    --filters "Name=tag:Environment,Values=production" \
              "Name=instance-state-name,Values=running" \
    --query "Reservations[].Instances[].{ID:InstanceId,Name:Tags[?Key=='Name']|[0].Value}"

# Get RDS endpoints
aws rds describe-db-instances \
    --query "DBInstances[].{Name:DBInstanceIdentifier,Endpoint:Endpoint.Address}" \
    --output table

# List Lambda functions by runtime
aws lambda list-functions \
    --query "Functions[?Runtime=='python3.9'].FunctionName" \
    --output text
```

### Cost Management
```bash
# Get current month costs by service
aws ce get-cost-and-usage \
    --time-period Start=$(date -d "$(date +%Y-%m-01)" +%Y-%m-%d),End=$(date +%Y-%m-%d) \
    --granularity MONTHLY \
    --metrics BlendedCost \
    --group-by Type=DIMENSION,Key=SERVICE

# List unused EBS volumes
aws ec2 describe-volumes \
    --filters "Name=status,Values=available" \
    --query "Volumes[].{ID:VolumeId,Size:Size,Type:VolumeType}"
```

### Security Automation
```bash
# Enable MFA delete on S3 bucket
aws s3api put-bucket-versioning \
    --bucket my-bucket \
    --versioning-configuration Status=Enabled,MFADelete=Enabled \
    --mfa "arn:aws:iam::123456789012:mfa/user 123456"

# List IAM users without MFA
aws iam list-users --query "Users[].UserName" --output text | while read user; do
    mfa=$(aws iam list-mfa-devices --user-name $user --query "MFADevices" --output text)
    if [ -z "$mfa" ]; then
        echo "User $user has no MFA enabled"
    fi
done

# Find public S3 buckets
aws s3api list-buckets --query "Buckets[].Name" --output text | while read bucket; do
    acl=$(aws s3api get-bucket-acl --bucket $bucket)
    if echo "$acl" | grep -q "AllUsers"; then
        echo "Bucket $bucket is publicly accessible"
    fi
done
```

## Agent Use
- Automate AWS infrastructure provisioning in deployment pipelines
- Query resource state for monitoring and reporting
- Implement cost optimization by identifying unused resources
- Enforce security policies across AWS accounts
- Backup and disaster recovery automation
- Multi-account management and governance

## Troubleshooting

### Credential Issues
```bash
# Verify credentials
aws sts get-caller-identity

# Check which profile is active
echo $AWS_PROFILE

# Test with specific profile
aws s3 ls --profile myprofile
```

### Permission Denied
```bash
# Check IAM policy simulator
aws iam simulate-principal-policy \
    --policy-source-arn arn:aws:iam::123456789012:user/myuser \
    --action-names s3:GetObject \
    --resource-arns arn:aws:s3:::mybucket/mykey
```

### Region Configuration
```bash
# Override region
aws ec2 describe-instances --region us-west-2

# Set default region
aws configure set region us-west-2
export AWS_DEFAULT_REGION=us-west-2
```

### Debug Mode
```bash
# Enable debug output
aws s3 ls --debug

# Verbose HTTP logging
aws s3 ls --debug 2>&1 | grep -i "MainThread"
```

## Version Differences

### AWS CLI v1 vs v2
| Feature | v1 | v2 |
|---------|----|----|
| Installation | pip install | Standalone installer |
| Python dependency | Required | Bundled |
| Binary size | Small | ~40MB |
| Auto-prompt | No | Yes (`--cli-auto-prompt`) |
| SSO support | Limited | Full |
| Server-side pagination | Manual | Automatic |
| Release cycle | Frequent | Stable |

### Recommended Version
- **v2**: Production use, CI/CD, new projects
- **v1**: Legacy systems, constrained environments

## Common Commands Reference
```bash
# S3
aws s3 ls                              # List buckets
aws s3 cp file.txt s3://bucket/        # Upload file
aws s3 sync dir/ s3://bucket/prefix/   # Sync directory

# EC2
aws ec2 describe-instances             # List instances
aws ec2 start-instances --instance-ids i-1234567890abcdef0
aws ec2 stop-instances --instance-ids i-1234567890abcdef0

# Lambda
aws lambda list-functions              # List functions
aws lambda invoke --function-name my-function output.json

# CloudFormation
aws cloudformation create-stack --stack-name my-stack --template-body file://template.yaml
aws cloudformation describe-stacks --stack-name my-stack

# ECS
aws ecs list-clusters                  # List clusters
aws ecs list-tasks --cluster my-cluster

# RDS
aws rds describe-db-instances          # List databases
aws rds create-db-snapshot --db-instance-identifier mydb --db-snapshot-identifier mydb-snapshot
```

## Uninstall
```yaml
- preset: awscli
  with:
    state: absent
```

## Resources
- Official docs: https://docs.aws.amazon.com/cli/
- User guide: https://docs.aws.amazon.com/cli/latest/userguide/
- Command reference: https://awscli.amazonaws.com/v2/documentation/api/latest/index.html
- GitHub: https://github.com/aws/aws-cli
- Search: "aws cli examples", "aws cli jmespath", "aws cli best practices"

# AWS CLI Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check version
aws --version

# Configure AWS CLI (interactive)
aws configure

# Test connection
aws sts get-caller-identity
```

## Configuration

- **Config directory:** `~/.aws/`
- **Credentials file:** `~/.aws/credentials`
- **Config file:** `~/.aws/config`

## Manual Configuration

```bash
# Edit credentials
cat > ~/.aws/credentials <<EOF
[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
EOF

# Edit config
cat > ~/.aws/config <<EOF
[default]
region = us-east-1
output = json
EOF
```

## Common Operations

```bash
# List S3 buckets
aws s3 ls

# List EC2 instances
aws ec2 describe-instances

# Get caller identity
aws sts get-caller-identity

# Use specific profile
aws s3 ls --profile myprofile

# Use specific region
aws ec2 describe-instances --region us-west-2
```

## Multiple Profiles

```ini
# ~/.aws/credentials
[default]
aws_access_key_id = KEY1
aws_secret_access_key = SECRET1

[project1]
aws_access_key_id = KEY2
aws_secret_access_key = SECRET2
```

## Uninstall

```yaml
- preset: awscli
  with:
    state: absent
```

**Note:** Configuration in `~/.aws/` is preserved after uninstall.

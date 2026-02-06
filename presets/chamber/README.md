# Chamber - Secrets Management

Store and retrieve secrets from AWS SSM Parameter Store or Secrets Manager with environment variable injection.

## Quick Start
```yaml
- preset: chamber
```

## Features
- **AWS integration**: Works with SSM Parameter Store and Secrets Manager
- **Environment injection**: Load secrets as environment variables
- **Encryption**: Secrets encrypted with KMS
- **Version control**: Track secret versions
- **Namespace support**: Organize secrets by service
- **Export capabilities**: Export secrets to files or shell

## Basic Usage
```bash
# Write secret
chamber write service key value

# Read secret
chamber read service key

# List secrets for service
chamber list service

# Execute command with secrets
chamber exec service -- ./myapp

# Export secrets as env vars
chamber env service

# Delete secret
chamber delete service key
```

## Advanced Usage
```bash
# Write with description
chamber write myapp db_password "secret123" --description "Database password"

# Read specific version
chamber read myapp db_password --version 2

# Export to .env file
chamber env myapp --format dotenv > .env

# Execute with multiple services
chamber exec service1 service2 -- ./app

# List all versions
chamber history myapp db_password
```

## Real-World Examples

### Application Deployment
```yaml
- name: Install Chamber
  preset: chamber

- name: Write application secrets
  shell: |
    chamber write myapp DATABASE_URL "postgresql://localhost/mydb"
    chamber write myapp API_KEY "{{ api_key }}"
    chamber write myapp SECRET_KEY "{{ secret_key }}"
  environment:
    AWS_REGION: us-east-1

- name: Run application with secrets
  shell: chamber exec myapp -- /usr/local/bin/myapp
```

### CI/CD Integration
```yaml
- name: Load secrets and deploy
  shell: chamber exec production -- ./deploy.sh
  environment:
    AWS_REGION: us-west-2
```

## Platform Support
- ✅ Linux (binary, Homebrew)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Inject secrets into applications securely
- Manage application configurations across environments
- Rotate secrets without code changes
- Audit secret access and changes
- Centralize secret storage for microservices


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install chamber
  preset: chamber

- name: Use chamber in automation
  shell: |
    # Custom configuration here
    echo "chamber configured"
```
## Uninstall
```yaml
- preset: chamber
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/segmentio/chamber
- AWS SSM: https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html
- Search: "chamber secrets", "aws ssm chamber", "chamber tutorial"

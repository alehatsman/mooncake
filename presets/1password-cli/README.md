# 1Password CLI - Password and Secrets Management

Command-line interface for 1Password. Access passwords, API keys, SSH keys, and documents from the terminal or CI/CD pipelines.

## Quick Start
```yaml
- preset: 1password-cli
```

## Features
- **Secure CLI access**: Retrieve secrets without opening the GUI
- **Service accounts**: Passwordless authentication for CI/CD
- **Secret references**: Load secrets directly into environment with `op://` URLs
- **SSH integration**: Use 1Password as SSH agent
- **Cross-platform**: Linux, macOS, Windows support
- **Team collaboration**: Share vaults and items with access control
- **Biometric unlock**: Touch ID/Windows Hello support

## Authentication
```bash
# Sign in
op signin

# Sign in to specific account
op signin my.1password.com user@example.com

# Use service account (CI/CD)
export OP_SERVICE_ACCOUNT_TOKEN="ops_xxx"

# Biometric unlock (after first signin)
op signin --account my
```

## Basic Usage
```bash
# List items
op item list

# Get item
op item get "My Login"

# Get specific field
op item get "My Login" --fields password

# Get by UUID
op item get j4jxl7rzncfb3rof7vvhkk64bm
```

## Item Management
```bash
# Create login item
op item create --category=login \
  --title="My Service" \
  --url="https://example.com" \
  username=user@example.com \
  password="$(op generate password)"

# Create secure note
op item create --category="Secure Note" \
  --title="API Keys" \
  notes="Production API keys"

# Create API credential
op item create --category="API Credential" \
  --title="GitHub Token" \
  credential="ghp_xxx"

# Update item
op item edit "My Login" password="newpassword"

# Delete item
op item delete "My Service"
```

## Retrieving Secrets
```bash
# Get password
op item get "My Login" --fields password

# Get username
op item get "My Login" --fields username

# Get specific field
op item get "My Login" --fields "API Key"

# Multiple fields as JSON
op item get "My Login" --format json

# Get OTP/2FA code
op item get "My Login" --otp
```

## Secret References
```bash
# Reference format: op://vault/item/field

# Load secret into environment
export DB_PASSWORD="op://Private/Database/password"
op run -- ./myapp

# In scripts
op run -- sh -c 'echo $DB_PASSWORD'

# Multiple secrets
op run --env-file=.env -- ./app

# .env file with references
DB_PASSWORD=op://Private/Database/password
API_KEY=op://Private/API/credential
```

## Password Generation
```bash
# Generate password
op generate password

# With specific length
op generate password --length 32

# With rules
op generate password --letters 20 --digits 5 --symbols 3

# No symbols
op generate password --no-symbols

# Passphrase
op generate password --words 6 --separator -
```

## Vaults
```bash
# List vaults
op vault list

# Get vault details
op vault get Private

# Create vault
op vault create "Team Secrets"

# Delete vault (dangerous!)
op vault delete "Old Vault"
```

## Documents
```bash
# Upload document
op document create resume.pdf --title "Resume" --vault Private

# Get document
op document get "Resume"

# Download document
op document get "Resume" --output ./resume.pdf

# Delete document
op document delete "Old File"
```

## SSH Keys
```bash
# List SSH keys
op item list --categories "SSH Key"

# Get SSH private key
op item get "GitHub SSH" --fields "private key"

# Use with ssh
op item get "GitHub SSH" --fields "private key" | ssh-add -

# SSH agent integration
op plugin run -- ssh git@github.com
```

## CI/CD Integration
```bash
# Service account auth
export OP_SERVICE_ACCOUNT_TOKEN="ops_xxx"

# Get secret in CI
DB_PASSWORD=$(op item get "Database" --fields password)

# Run command with secrets
op run --env-file=.env -- npm run deploy

# GitHub Actions
- name: Load secrets
  env:
    OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SERVICE_ACCOUNT_TOKEN }}
  run: |
    API_KEY=$(op item get "API Key" --fields credential)
    echo "::add-mask::$API_KEY"
    echo "API_KEY=$API_KEY" >> $GITHUB_ENV
```

## Templates
```bash
# Get item template
op item template get Login

# Create from template
op item template get Login | \
  jq '.fields[0].value = "user@example.com"' | \
  op item create --template -
```

## JSON Output
```bash
# Get as JSON
op item get "My Login" --format json

# Extract with jq
op item list --format json | jq -r '.[].title'

# Get specific field
op item get "Database" --format json | jq -r '.fields[] | select(.label=="password") | .value'

# All items in vault
op item list --vault Private --format json
```

## Sharing
```bash
# Share item
op item share "My Login" --emails user@example.com

# Share with expiry
op item share "Temp Access" \
  --emails user@example.com \
  --expiry 7d

# View shares
op item share list
```

## Groups and Users
```bash
# List users
op user list

# List groups
op group list

# Add user to group
op group user add Developers user@example.com

# Remove user from group
op group user remove Developers user@example.com
```

## Advanced Features
```bash
# Inject secrets into files
op inject -i template.yaml -o output.yaml

# Template with references
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
data:
  password: {{ op://Private/Database/password | base64encode }}

# Plugin integration
op plugin init aws
op plugin run -- aws s3 ls

# Account management
op account list
op account add
op account forget my
```

## Shell Plugins
```bash
# AWS CLI
op plugin run -- aws s3 ls

# Docker
op plugin run -- docker login

# Terraform
op run -- terraform apply

# Any command
op run -- ./deploy.sh
```

## Configuration
- **Config directory**: `~/.config/op/` (Linux), `~/Library/Group Containers/2BUA8C4S2C.com.1password/` (macOS)
- **Service accounts**: Store token in `OP_SERVICE_ACCOUNT_TOKEN` environment variable
- **SSH agent**: Socket at `~/.1password/agent.sock`
- **Cache**: Encrypted session cached locally after signin

## Real-World Examples

### Deployment Script with Secrets
```bash
#!/bin/bash
# Store secrets in 1Password, reference in .env
cat > .env <<EOF
DB_PASSWORD=op://Private/Database/password
API_KEY=op://Private/API/credential
AWS_ACCESS_KEY_ID=op://Private/AWS/access_key_id
AWS_SECRET_ACCESS_KEY=op://Private/AWS/secret_access_key
EOF

# Run deployment with secrets injected
op run -- ./deploy.sh
```

### Rotate Database Passwords
```bash
# Generate new password
NEW_PASSWORD=$(op item get "Production DB" --fields password)

# Update database
mysql -u root -p"$OLD_PASSWORD" -e "ALTER USER 'app'@'%' IDENTIFIED BY '$NEW_PASSWORD';"

# Update 1Password
op item edit "Production DB" password="$NEW_PASSWORD"

# Update application secrets
op item edit "App Config" database_password="$NEW_PASSWORD"
```

### Team Onboarding
```yaml
# Grant new team member access
- preset: 1password-cli

- name: Create developer vault access
  shell: |
    op vault user grant "Development" user@example.com --permissions read
    op group user add "Developers" user@example.com
```

## Troubleshooting

### "no identities match" error
Authentication failed or wrong account selected.
```bash
# List signed-in accounts
op account list

# Sign in to correct account
op signin --account my.1password.com

# Or use specific account
op --account my item list
```

### Service account authentication failing
Token invalid or expired.
```bash
# Verify token format (should start with ops_)
echo $OP_SERVICE_ACCOUNT_TOKEN | cut -c1-4

# Test token
export OP_SERVICE_ACCOUNT_TOKEN="ops_xxx"
op item list

# Regenerate token in 1Password web interface if needed
```

### "command not found: op"
Binary not in PATH after installation.
```bash
# Verify installation
which op || echo "Not installed"

# Add to PATH (if installed to custom location)
export PATH="$PATH:/usr/local/bin"
```

## Best Practices
- **Use service accounts** for CI/CD (not personal accounts)
- **Reference secrets** with op:// URLs, don't hardcode
- **Rotate secrets** regularly using op commands
- **Use vaults** to organize secrets by team/project
- **Enable 2FA** on 1Password account
- **Use `op run`** to inject secrets at runtime
- **Never log** secret values in scripts or CI output
- **Audit access** regularly via web interface

## Platform Support
- ✅ Linux (apt,dnf,yum,Homebrew)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated secret injection
- CI/CD secret management
- Configuration management
- Deployment automation
- Development environment setup
- Secure credential storage


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install 1password-cli
  preset: 1password-cli

- name: Use 1password-cli in automation
  shell: |
    # Custom configuration here
    echo "1password-cli configured"
```
## Uninstall
```yaml
- preset: 1password-cli
  with:
    state: absent
```

## Resources
- Docs: https://developer.1password.com/docs/cli/
- GitHub: https://github.com/1Password/shell-plugins
- Search: "1password cli examples", "op cli secrets"

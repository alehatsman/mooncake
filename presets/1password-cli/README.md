# op - 1Password CLI

Secure secrets management from command line. Access passwords, API keys, SSH keys, and documents stored in 1Password.

## Quick Start
```yaml
- preset: 1password-cli
```

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

## Best Practices
- **Use service accounts** for CI/CD (not personal accounts)
- **Reference secrets** with op:// URLs, don't hardcode
- **Rotate secrets** regularly
- **Use vaults** to organize secrets by team/project
- **Enable 2FA** on 1Password account
- **Use `op run`** to inject secrets at runtime
- **Never log** secret values

## Tips
- Works offline after initial sync
- Biometric unlock on supported platforms
- SSH agent integration for Git
- Browser extension sync
- Team sharing with access controls
- Audit logs for compliance
- Cross-platform (Mac, Linux, Windows)

## Agent Use
- Automated secret injection
- CI/CD secret management
- Configuration management
- Deployment automation
- Development environment setup
- Secure credential storage

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

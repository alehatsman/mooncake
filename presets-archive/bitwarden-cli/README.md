# Bitwarden CLI - Password Manager Command-Line Tool

Official command-line interface for Bitwarden password manager, enabling secure password access and management from terminal.

## Quick Start
```yaml
- preset: bitwarden-cli
```

## Features
- **Password management**: Store, retrieve, and generate passwords
- **Secure vaults**: Access encrypted password vaults
- **Two-factor auth**: TOTP code generation
- **CLI automation**: Script-friendly JSON output
- **Self-hosted support**: Works with Bitwarden or Vaultwarden servers
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Login to Bitwarden
bw login

# Unlock vault (session token)
export BW_SESSION=$(bw unlock --raw)

# List items
bw list items

# Search for item
bw get item github.com

# Get password
bw get password github.com

# Generate password
bw generate --length 20 --uppercase --lowercase --number --special

# Sync vault
bw sync

# Logout
bw logout
```

## Advanced Configuration

```yaml
# Install Bitwarden CLI
- preset: bitwarden-cli
  register: bw_result

# Login and unlock (interactive)
- name: Login to Bitwarden
  shell: bw login
  when: not bw_logged_in

# Get session token (for automation)
- name: Unlock vault
  shell: bw unlock --raw
  register: bw_session
  no_log: true

# Retrieve password
- name: Get database password
  shell: bw get password "Database Production"
  environment:
    BW_SESSION: "{{ bw_session.stdout }}"
  register: db_password
  no_log: true

# Use password in configuration
- name: Configure application
  template:
    dest: /etc/app/config.yml
    content: |
      database:
        password: {{ db_password.stdout }}
  no_log: true
```

## Authentication

### Login
```bash
# Login with email
bw login user@example.com

# Login with API key (for automation)
bw login --apikey

# Login to self-hosted instance
bw config server https://vault.example.com
bw login user@example.com
```

### Session Management
```bash
# Unlock and get session token
export BW_SESSION=$(bw unlock --raw)

# Or enter master password when prompted
bw unlock

# Check session status
bw status

# Lock vault
bw lock
```

## Item Management

### Retrieve Items
```bash
# List all items
bw list items | jq

# Get specific item by name
bw get item "GitHub"

# Get item by ID
bw get item a1b2c3d4-e5f6-7890-abcd-ef1234567890

# Search items
bw list items --search "aws"

# Get password only
bw get password "GitHub"

# Get username
bw get username "GitHub"

# Get TOTP code
bw get totp "GitHub"
```

### Create Items
```bash
# Create login item
bw get template item | jq '.type = 1 | .name = "New Account" | .login.username = "user@example.com" | .login.password = "password123"' | bw encode | bw create item

# Create secure note
bw get template item | jq '.type = 2 | .name = "API Keys" | .notes = "Secret notes here"' | bw encode | bw create item
```

### Update Items
```bash
# Edit item
bw get item "GitHub" | jq '.login.password = "newpassword"' | bw encode | bw edit item a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

### Delete Items
```bash
# Delete item
bw delete item a1b2c3d4-e5f6-7890-abcd-ef1234567890

# Permanently delete (bypass trash)
bw delete item a1b2c3d4-e5f6-7890-abcd-ef1234567890 --permanent
```

## Password Generation

```bash
# Generate strong password
bw generate

# Custom length
bw generate --length 32

# Specific character sets
bw generate --uppercase --lowercase --number --special

# Passphrase (words)
bw generate --passphrase --words 5 --separator -

# No special characters
bw generate --length 20 --uppercase --lowercase --number

# Include ambiguous characters
bw generate --includeNumber --ambiguous
```

## Configuration

### Server Configuration
```bash
# Use self-hosted Bitwarden
bw config server https://vault.example.com

# Use Bitwarden cloud (default)
bw config server https://vault.bitwarden.com

# Check current server
bw config server
```

### Environment Variables
```bash
# Session token (for automation)
export BW_SESSION="session_token_here"

# Master password (use with caution)
export BW_PASSWORD="master_password"

# Client ID and secret (for API key login)
export BW_CLIENTID="client_id"
export BW_CLIENTSECRET="client_secret"

# Server URL
export BW_URL="https://vault.example.com"
```

## Real-World Examples

### CI/CD Secret Injection
```yaml
# Retrieve secrets from Bitwarden in CI
- preset: bitwarden-cli

- name: Login with API key
  shell: |
    bw config server {{ vault_url }}
    bw login --apikey
  environment:
    BW_CLIENTID: "{{ lookup('env', 'BW_CLIENTID') }}"
    BW_CLIENTSECRET: "{{ lookup('env', 'BW_CLIENTSECRET') }}"
  no_log: true

- name: Unlock vault
  shell: bw unlock --passwordenv BW_PASSWORD --raw
  environment:
    BW_PASSWORD: "{{ lookup('env', 'BW_PASSWORD') }}"
  register: session
  no_log: true

- name: Get database credentials
  shell: bw get item "Production Database"
  environment:
    BW_SESSION: "{{ session.stdout }}"
  register: db_creds
  no_log: true

- name: Deploy with secrets
  shell: ./deploy.sh
  environment:
    DB_HOST: "{{ (db_creds.stdout | from_json).fields[0].value }}"
    DB_PASSWORD: "{{ (db_creds.stdout | from_json).login.password }}"
  no_log: true
```

### Ansible Vault Integration
```bash
# Store Ansible vault password in Bitwarden
echo "MyVaultPassword" | bw encode | bw create item --name "Ansible Vault"

# Retrieve for use
export ANSIBLE_VAULT_PASSWORD=$(bw get password "Ansible Vault")
ansible-playbook --vault-password-file <(echo "$ANSIBLE_VAULT_PASSWORD") playbook.yml
```

### Automated Backup
```bash
#!/bin/bash
# Backup Bitwarden vault
export BW_SESSION=$(bw unlock --raw)

# Export vault (encrypted)
bw export --format encrypted_json --password backup_password > vault_backup.json

# Upload to backup location
aws s3 cp vault_backup.json s3://backups/bitwarden/vault_$(date +%Y%m%d).json

bw lock
```

### SSH Key Management
```bash
# Store SSH private key in secure note
cat ~/.ssh/id_rsa | bw encode | bw create item --name "SSH Private Key" --type 2

# Retrieve SSH key
mkdir -p ~/.ssh
bw get item "SSH Private Key" | jq -r '.notes' > ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa
```

## Troubleshooting

### Session Expired
```bash
# Error: "Session key is invalid"
# Solution: Re-unlock vault
export BW_SESSION=$(bw unlock --raw)
```

### Login Failed
```bash
# Check server configuration
bw config server

# Verify credentials
bw login user@example.com

# For 2FA issues, use recovery code
bw login user@example.com --code recovery_code
```

### Items Not Syncing
```bash
# Force sync
bw sync --force

# Check sync status
bw status

# Re-login if needed
bw logout
bw login
```

### Permission Denied
```bash
# Ensure binary is executable
chmod +x $(which bw)

# Check session token is set
echo $BW_SESSION

# Re-unlock if token is empty
export BW_SESSION=$(bw unlock --raw)
```

## Security Best Practices

```bash
# Never log sensitive commands
bw get password "secret" 2>/dev/null

# Clear session when done
bw lock
unset BW_SESSION

# Use short-lived sessions in scripts
trap 'bw lock' EXIT

# Store API credentials securely
# Use GitHub Secrets, AWS Secrets Manager, etc.

# Enable 2FA on Bitwarden account
# Reduces risk of compromised master password
```

## Platform Support
- ✅ Linux (apt, snap, AppImage, Homebrew)
- ✅ macOS (Homebrew, DMG installer)
- ✅ Windows (Chocolatey, Scoop, MSI installer)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Inject secrets into CI/CD pipelines securely
- Automate credential rotation in infrastructure
- Retrieve API keys for deployment scripts
- Manage SSH keys and certificates
- Generate strong passwords for provisioning
- Sync secrets across development environments
- Integrate with configuration management tools

## Uninstall
```yaml
- preset: bitwarden-cli
  with:
    state: absent
```

## Resources
- Official docs: https://bitwarden.com/help/cli/
- GitHub: https://github.com/bitwarden/clients
- Self-hosting: https://github.com/dani-garcia/vaultwarden
- API docs: https://bitwarden.com/help/api/
- Search: "bitwarden cli tutorial", "bitwarden cli automation", "bitwarden ci cd"

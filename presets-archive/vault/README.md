# Vault - Secrets Management Platform

Secure, store, and tightly control access to tokens, passwords, certificates, and encryption keys.

## Quick Start
```yaml
- preset: vault
```

## Features
- **Secrets Management**: Store and manage sensitive data securely
- **Dynamic Secrets**: Generate credentials on-demand for databases, cloud providers
- **Data Encryption**: Encrypt/decrypt data without storing it
- **Leasing and Renewal**: Automatic secret rotation and revocation
- **Cross-platform**: Linux and macOS support
- **Multiple Auth Methods**: Tokens, userpass, LDAP, GitHub, AWS, and more

## Basic Usage
```bash
# Check version and status
vault version
vault status

# Store a secret
vault kv put secret/myapp password=secret123 api_key=abc

# Read a secret
vault kv get secret/myapp
vault kv get -field=password secret/myapp

# List secrets
vault kv list secret/

# Delete a secret
vault kv delete secret/myapp

# Enable authentication method
vault auth enable userpass
vault write auth/userpass/users/admin password=securepass

# Login with userpass
vault login -method=userpass username=admin
```

## Advanced Configuration
```yaml
# Production deployment with server mode
- preset: vault
  with:
    mode: server                  # Production mode
    port: "8200"
    address: "0.0.0.0"           # Listen on all interfaces
    start_service: true
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Vault |
| start_service | bool | true | Start Vault service after installation |
| mode | string | dev | Run mode (dev for testing, server for production) |
| port | string | 8200 | Vault server port |
| address | string | 127.0.0.1 | Vault server bind address |

## Platform Support
- ✅ Linux (apt, dnf, yum, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration
- **Config directory**: `/etc/vault.d/` (Linux), `/usr/local/etc/vault/` (macOS)
- **Data directory**: `/var/lib/vault/data/`
- **Service file**: `/etc/systemd/system/vault.service` (Linux)
- **Default port**: 8200
- **Environment**: `VAULT_ADDR`, `VAULT_TOKEN`

## Dev Mode vs Server Mode

### Dev Mode (Default)
- ✅ Fast setup for testing and development
- ✅ Pre-initialized and unsealed automatically
- ⚠️ Data is NOT persistent (in-memory only)
- ⚠️ Root token auto-generated and displayed
- ⚠️ TLS disabled by default

```bash
# After dev mode installation
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN='<root-token-from-output>'
vault status
```

### Server Mode (Production)
Create `/etc/vault.d/vault.hcl`:

```hcl
storage "file" {
  path = "/var/lib/vault/data"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = 0
  tls_cert_file = "/etc/vault.d/tls/cert.pem"
  tls_key_file  = "/etc/vault.d/tls/key.pem"
}

api_addr = "https://vault.example.com:8200"
cluster_addr = "https://vault.example.com:8201"
ui = true
```

Initialize and unseal:

```bash
# Start service
sudo systemctl start vault

# Initialize (SAVE OUTPUT!)
vault operator init

# Unseal (requires 3 of 5 keys by default)
vault operator unseal <key1>
vault operator unseal <key2>
vault operator unseal <key3>

# Login with root token
vault login <root-token>
```

## Real-World Examples

### Database Credentials Management
```bash
# Enable database secrets engine
vault secrets enable database

# Configure PostgreSQL connection
vault write database/config/mydb \
  plugin_name=postgresql-database-plugin \
  connection_url="postgresql://{{username}}:{{password}}@localhost:5432/mydb" \
  allowed_roles="readonly,readwrite" \
  username="vault" \
  password="vaultpass"

# Create role with dynamic credentials
vault write database/roles/readonly \
  db_name=mydb \
  creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}'; \
    GRANT SELECT ON ALL TABLES IN SCHEMA public TO \"{{name}}\";" \
  default_ttl="1h" \
  max_ttl="24h"

# Generate credentials (automatically rotates)
vault read database/creds/readonly
```

### Application Secrets in CI/CD
```yaml
# In your CI/CD pipeline
- name: Deploy application with Vault secrets
  preset: vault
  with:
    mode: server
  become: true

- name: Fetch secrets
  shell: |
    export VAULT_ADDR='https://vault.example.com:8200'
    export VAULT_TOKEN='{{ ci_vault_token }}'
    vault kv get -format=json secret/myapp/prod > secrets.json
  register: secrets

- name: Deploy with secrets
  shell: |
    export DB_PASSWORD=$(jq -r '.data.data.db_password' secrets.json)
    export API_KEY=$(jq -r '.data.data.api_key' secrets.json)
    ./deploy.sh
```

### Encryption as a Service
```bash
# Enable transit engine
vault secrets enable transit

# Create encryption key
vault write -f transit/keys/myapp

# Encrypt data
vault write transit/encrypt/myapp plaintext=$(echo "secret data" | base64)
# Returns: ciphertext:v1:abcd1234...

# Decrypt data
vault write transit/decrypt/myapp ciphertext=v1:abcd1234...
# Returns: plaintext:c2VjcmV0IGRhdGE=
echo "c2VjcmV0IGRhdGE=" | base64 -d
# Output: secret data
```

## Agent Use
- Secure secret storage for applications and services
- Dynamic credential generation for databases and cloud providers
- Encryption/decryption services for sensitive data
- Certificate authority for PKI infrastructure
- CI/CD pipeline secret injection
- Kubernetes secrets management via Vault Agent

## Troubleshooting

### Vault is sealed
```bash
# Check seal status
vault status

# Unseal with keys
vault operator unseal <key1>
vault operator unseal <key2>
vault operator unseal <key3>
```

### Permission denied errors
```bash
# Check token capabilities
vault token capabilities <token> secret/path

# Login with appropriate credentials
vault login -method=userpass username=admin

# Check policy
vault policy read default
```

### Service won't start
```bash
# Check logs (Linux)
journalctl -u vault -f

# Check configuration
vault server -config=/etc/vault.d/vault.hcl -test

# Check file permissions
ls -la /var/lib/vault/data
```

## Uninstall
```yaml
- preset: vault
  with:
    state: absent
```

**⚠️ Warning**: Backup all secrets before uninstalling! Data will be lost.

```bash
# Backup secrets before uninstall
vault kv list -format=json secret/ > vault-backup.json
```

## Resources
- Official docs: https://www.vaultproject.io/docs
- GitHub: https://github.com/hashicorp/vault
- Search: "vault secrets management", "hashicorp vault tutorial"

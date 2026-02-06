# Vault Preset

**Status:** ✓ Installed successfully

## Quick Start (Dev Mode)

```bash
# Set environment variables (from installation output)
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN='<root-token-from-output>'

# Check status
vault status

# Store a secret
vault kv put secret/myapp password=secret123

# Read a secret
vault kv get secret/myapp
```

## Configuration

- **Config directory:** `/etc/vault.d/`
- **Dev mode:** Runs in-memory, data is NOT persistent
- **Server mode:** Requires configuration file
- **Default port:** 8200

## Dev Mode (Default)

- ✅ Fast setup for testing
- ✅ Pre-initialized and unsealed
- ⚠️ Data is NOT persistent
- ⚠️ Root token auto-generated

## Server Mode

Create `/etc/vault.d/vault.hcl`:

```hcl
storage "file" {
  path = "/var/lib/vault/data"
}

listener "tcp" {
  address     = "127.0.0.1:8200"
  tls_disable = 1
}

api_addr = "http://127.0.0.1:8200"
ui = true
```

Start server:

```bash
sudo systemctl start vault
vault operator init  # Save unseal keys and root token!
vault operator unseal  # Use unseal key
```

## Common Operations

```bash
# Store secret
vault kv put secret/app/config \
  db_user=admin \
  db_pass=secret

# Read secret
vault kv get secret/app/config
vault kv get -field=db_pass secret/app/config

# List secrets
vault kv list secret/

# Delete secret
vault kv delete secret/app/config

# Enable auth method
vault auth enable userpass
vault write auth/userpass/users/myuser password=pass123
```

## Unsealing Vault (Production)

```bash
# Vault starts sealed - requires 3 of 5 unseal keys
vault operator unseal <key1>
vault operator unseal <key2>
vault operator unseal <key3>
```

## Uninstall

```yaml
- preset: vault
  with:
    state: absent
```

**⚠️ Warning:** Backup secrets before uninstalling!

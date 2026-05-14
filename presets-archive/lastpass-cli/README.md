# lastpass-cli - LastPass Command-Line Interface

Command-line interface for the LastPass password manager, enabling secure password retrieval and management from scripts and automation workflows.

## Quick Start
```yaml
- preset: lastpass-cli
```

## Features
- **Vault access**: Read/write passwords from LastPass vault
- **Secure notes**: Store and retrieve secure notes
- **Secret generation**: Generate strong passwords
- **Scriptable**: Integrate with automation and CI/CD
- **MFA support**: Two-factor authentication compatible
- **Session management**: Login persistence with agent

## Basic Usage
```bash
# Login to LastPass
lpass login email@example.com

# List all accounts
lpass ls

# Show password (copies to clipboard)
lpass show --password github.com

# Show full account details
lpass show github.com

# Search for accounts
lpass ls | grep aws

# Generate password
lpass generate mysite.com 20

# Add new account
lpass add --name mysite.com --username user --password pass

# Edit account
lpass edit mysite.com

# Logout
lpass logout
```

## Advanced Configuration
```yaml
- preset: lastpass-cli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove lastpass-cli |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (use native LastPass app or WSL)

## Configuration
- **Agent socket**: `~/.lpass/agent.sock` (session persistence)
- **Cache**: `~/.lpass/` (encrypted credential cache)
- **Session timeout**: Configurable via `LPASS_AGENT_TIMEOUT`

## Real-World Examples

### CI/CD Secret Retrieval
```bash
# Export secrets as environment variables
export DB_PASSWORD=$(lpass show --password production/database)
export API_KEY=$(lpass show --password production/api-key)

# Use in deployment
./deploy.sh --db-pass "$DB_PASSWORD" --api-key "$API_KEY"
```

### Automated Deployment Script
```yaml
# Retrieve deployment credentials
- name: Login to LastPass
  shell: echo "$LPASS_PASSWORD" | lpass login --trust email@example.com
  environment:
    LPASS_PASSWORD: "{{ vault_master_password }}"

- name: Get database password
  shell: lpass show --password production/postgres
  register: db_pass

- name: Deploy application
  shell: ./deploy.sh --db-password "{{ db_pass.stdout }}"
```

### Rotate Passwords
```bash
# Generate new password
NEW_PASS=$(lpass generate --no-symbols database/prod 32)

# Update database
mysql -u admin -p"$OLD_PASS" -e "SET PASSWORD FOR 'app'@'%' = PASSWORD('$NEW_PASS');"

# Update LastPass
echo "$NEW_PASS" | lpass edit --password --non-interactive database/prod
```

### Batch Secret Export
```bash
# Export all AWS credentials
lpass ls aws/ | while read -r line; do
  name=$(echo "$line" | awk '{print $2}')
  password=$(lpass show --password "$name")
  echo "$name: $password"
done > aws-secrets.txt
```

## Agent Use
- Retrieve secrets in CI/CD pipelines
- Automated credential rotation
- Secret injection into deployment workflows
- Integration testing with real credentials
- Temporary access provisioning

## Troubleshooting

### Login fails with MFA
Use `--trust` flag to remember device:
```bash
lpass login --trust email@example.com
```

### Agent timeout
Session expired, login again:
```bash
lpass login email@example.com
```

### Permission denied on agent socket
Fix socket permissions:
```bash
chmod 600 ~/.lpass/agent.sock
```

### Clipboard not working
Use `--password` to output to stdout:
```bash
lpass show --password site.com
```

## Security Notes
- **Never commit passwords**: Use environment variables or secure vaults
- **Session management**: Use `LPASS_AGENT_TIMEOUT` for auto-logout
- **Trust sparingly**: Only use `--trust` on secure systems
- **Audit access**: LastPass logs all CLI access attempts

## Uninstall
```yaml
- preset: lastpass-cli
  with:
    state: absent
```

**Note**: Does not delete LastPass account or vault data.

## Resources
- Official docs: https://lastpass.com/support.php?cmd=showcategory&id=97
- GitHub: https://github.com/lastpass/lastpass-cli
- Search: "lastpass cli automation", "lpass scripting"

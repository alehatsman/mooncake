# Gopass - Team Password Manager

Gopass is a rewrite of pass (the standard Unix password manager) in Go with additional features for teams, including secrets management, Git synchronization, and multiple storage backends.

## Quick Start

```yaml
- preset: gopass
```

```bash
# Initialize gopass
gopass init

# Store a password
gopass insert work/email

# Retrieve a password
gopass show work/email

# Generate a password
gopass generate work/github 20
```

## Features

- **Git-based**: Built-in Git support for synchronization
- **Team-ready**: Multi-user support with GPG encryption
- **Multiple stores**: Organize secrets into separate stores
- **Templates**: Generate structured secrets (API keys, database configs)
- **Integration**: Browser plugins, native apps, CLI
- **Migration**: Import from 1Password, LastPass, KeePass

## Basic Usage

```bash
# Initialize gopass with your GPG key
gopass init your-gpg-email@example.com

# Add a secret
gopass insert personal/gmail
gopass insert work/aws-key

# Show a secret
gopass show personal/gmail

# Copy to clipboard (auto-clears after 45s)
gopass show -c personal/gmail

# Generate random password
gopass generate work/postgres 32

# List all secrets
gopass ls

# Search for secrets
gopass search aws

# Edit a secret
gopass edit work/database

# Delete a secret
gopass rm work/old-api-key
```

## Advanced Configuration

```yaml
- preset: gopass
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove gopass |

## Platform Support

- ✅ Linux (binary install)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Password store**: `~/.local/share/gopass/stores/root/` (Linux), `~/Library/Application Support/gopass/stores/root/` (macOS)
- **Config file**: `~/.config/gopass/config.yml`
- **GPG keys**: System GPG keyring

## Real-World Examples

### Team Password Store Setup

```bash
# Initialize with team members
gopass init --store team \
  alice@example.com \
  bob@example.com \
  charlie@example.com

# Set up Git remote
cd ~/.local/share/gopass/stores/team
git remote add origin git@github.com:company/passwords.git
gopass sync

# Add team secret
gopass insert --store=team production/database
```

### CI/CD Secret Management

```bash
# Store API key for CI
gopass insert ci/docker-registry-token

# Retrieve in scripts
export DOCKER_TOKEN=$(gopass show -o ci/docker-registry-token)
docker login -u user --password-stdin <<< "$DOCKER_TOKEN"
```

### Structured Secrets (Templates)

```bash
# Create database config template
gopass templates edit database

# Template content:
# Host: {{ .Host }}
# Port: {{ .Port }}
# Username: {{ .Username }}
# Password: {{ .Password }}

# Use template
gopass generate --template database prod/mysql
```

### Import from 1Password

```bash
# Export from 1Password as CSV
# Then import
gopass import 1password 1password-export.csv
```

### Multiple Stores

```bash
# Create separate stores for different contexts
gopass init --store personal you@example.com
gopass init --store work work@company.com
gopass init --store shared team@company.com

# Use specific store
gopass show --store work aws/api-key
```

## Agent Use

- Manage infrastructure secrets in deployment automation
- Rotate credentials programmatically
- Share team secrets securely via Git
- Inject secrets into CI/CD pipelines
- Generate secure random passwords
- Track secret access and changes via Git history

## Troubleshooting

### GPG key not found

List available keys:
```bash
gpg --list-secret-keys --keyid-format=long
```

Create new key if needed:
```bash
gpg --full-generate-key
```

### Git sync conflicts

```bash
# Pull changes first
gopass sync

# Force push (caution!)
cd ~/.local/share/gopass/stores/root
git push --force
```

### Permission denied errors

Check GPG agent:
```bash
gpg-connect-agent reloadagent /bye
```

## Uninstall

```yaml
- preset: gopass
  with:
    state: absent
```

**Note**: Password stores in `~/.local/share/gopass/` are preserved after uninstall.

## Resources

- Official docs: https://gopass.pw/
- GitHub: https://github.com/gopasspw/gopass
- Search: "gopass tutorial", "gopass team secrets", "gopass git sync"

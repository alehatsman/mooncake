# pass - Unix Password Manager

The standard Unix password manager using GPG encryption. Simple, secure, and git-friendly password storage.

## Quick Start
```yaml
- preset: pass
```

## Features
- **GPG encryption**: Passwords encrypted with your GPG key
- **Git integration**: Version control for password changes
- **Simple CLI**: Easy to use command-line interface
- **Cross-platform**: Linux, macOS, BSD
- **Team friendly**: Share passwords with GPG key groups
- **Extensions**: Plugin system for additional features

## Basic Usage
```bash
# Initialize password store
pass init your-gpg-key-id

# Add a password
pass insert email/work
pass insert github.com

# Generate random password
pass generate email/personal 20

# Retrieve password
pass email/work
pass -c email/work  # Copy to clipboard

# List all passwords
pass

# Search passwords
pass grep github

# Edit password
pass edit email/work

# Remove password
pass rm email/work

# Git operations
pass git log
pass git push
```

## Advanced Configuration

### Initialize with git
```yaml
- preset: pass
  become: true

- name: Initialize password store
  shell: pass init {{ gpg_key_id }}

- name: Enable git
  shell: pass git init
  cwd: ~/.password-store
```

### Multi-user setup
```yaml
- name: Initialize for team
  shell: |
    pass init user1@example.com user2@example.com user3@example.com
    pass git init
    pass git remote add origin git@github.com:team/passwords.git
```

### Generate and store password
```yaml
- name: Generate database password
  shell: pass generate production/database 32
  register: db_pass

- name: Use password in config
  template:
    src: app.conf.j2
    dest: /etc/app/app.conf
  vars:
    db_password: "{{ db_pass.stdout }}"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove pass |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ BSD (pkg)
- ❌ Windows (WSL only)

## Configuration
- **Password store**: `~/.password-store/` (default)
- **GPG keys**: `~/.gnupg/`
- **Config file**: `~/.password-store/.gpg-id` (GPG key IDs)
- **Git support**: Automatic when initialized

## Real-World Examples

### Development secrets
```bash
# Store API keys
pass insert api/openai
pass insert api/stripe
pass insert api/sendgrid

# Use in scripts
export OPENAI_KEY=$(pass api/openai)
```

### CI/CD integration
```yaml
- name: Install pass
  preset: pass
  become: true

- name: Import GPG key
  shell: |
    echo "$GPG_PRIVATE_KEY" | gpg --import
    echo "$GPG_PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --edit-key "$GPG_KEY_ID" trust quit

- name: Retrieve secrets
  shell: pass show production/database-url
  register: db_url
  environment:
    GPG_TTY: /dev/tty
```

### Password rotation
```bash
# Generate new password
pass generate -i email/work 24

# Show git history
pass git log email/work

# Revert to previous
pass git revert HEAD
```

## Extensions

### pass-otp (TOTP)
```bash
# Install extension
pass-otp --help

# Add TOTP secret
pass otp insert github.com

# Generate code
pass otp github.com

# Copy to clipboard
pass otp -c github.com
```

### pass-tomb (encrypted storage)
```bash
# Store passwords in encrypted container
pass tomb init
```

### pass-update
```bash
# Bulk password updates
pass-update --help
```

## Pass Store Structure
```
~/.password-store/
├── .gpg-id              # GPG key IDs
├── .git/                # Git repository
├── email/
│   ├── work.gpg
│   └── personal.gpg
├── ssh/
│   └── github.gpg
└── production/
    ├── database.gpg
    └── api-keys.gpg
```

## Environment Variables
```bash
# Custom password store location
export PASSWORD_STORE_DIR=~/my-passwords

# Clipboard timeout (seconds)
export PASSWORD_STORE_CLIP_TIME=30

# Default password length
export PASSWORD_STORE_GENERATED_LENGTH=25

# Git autocommit
export PASSWORD_STORE_GIT_AUTO_COMMIT=true
```

## Agent Use
- Store and retrieve secrets in automated workflows
- Generate secure passwords for provisioning
- Version control password changes
- Team password management with GPG groups
- Rotate credentials on schedule
- Export secrets to CI/CD pipelines
- Audit password access via git history

## Troubleshooting

### GPG key not found
```bash
# List GPG keys
gpg --list-keys

# Generate new key
gpg --full-generate-key

# Initialize pass with key ID
pass init your-key-id
```

### Clipboard not working
```bash
# Install clipboard tools
# Linux (X11)
sudo apt-get install xclip

# Linux (Wayland)
sudo apt-get install wl-clipboard

# macOS - built-in pbcopy
```

### Git errors
```bash
# Reinitialize git
cd ~/.password-store
git init
git add .
git commit -m "Initial commit"
```

### Cannot decrypt
```bash
# Check GPG key
gpg --list-secret-keys

# Re-encrypt with new key
pass init new-key-id
```

## Best Practices
- **Use strong GPG keys**: 4096-bit RSA or EdDSA
- **Enable git**: Track all password changes
- **Backup GPG keys**: Store securely offline
- **Use pass generate**: Random passwords are stronger
- **Organize with directories**: Group by service/environment
- **Set clipboard timeout**: Auto-clear sensitive data
- **Regular rotation**: Update critical passwords quarterly
- **Audit access**: Review git logs regularly

## Uninstall
```yaml
- preset: pass
  with:
    state: absent
```

**Note**: This removes the `pass` binary but keeps your password store at `~/.password-store/`. Backup before deleting.

## Resources
- Official site: https://www.passwordstore.org/
- GitHub: https://github.com/zx2c4/password-store
- Extensions: https://www.passwordstore.org/#extensions
- Search: "pass unix password manager", "pass gpg tutorial", "pass git integration"

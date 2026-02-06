# age - File Encryption

Modern, simple file encryption tool. A replacement for GPG with a focus on simplicity and good defaults.

## Quick Start
```yaml
- preset: age
```

## Basic Usage
```bash
# Generate key pair
age-keygen -o key.txt
# Public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p

# Encrypt file
age -r age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p -o secrets.enc secrets.txt

# Decrypt file
age -d -i key.txt -o secrets.txt secrets.enc

# Encrypt to multiple recipients
age -r age1recipient1... -r age1recipient2... -o file.enc file.txt

# Encrypt with passphrase
age -p -o secrets.enc secrets.txt
age -d -o secrets.txt secrets.enc
```

## Key Management
```bash
# Generate keypair
age-keygen -o ~/.age/key.txt

# Print public key
age-keygen -y ~/.age/key.txt

# Multiple identities
age-keygen -o ~/.age/work.txt
age-keygen -o ~/.age/personal.txt

# Use specific identity to decrypt
age -d -i ~/.age/work.txt file.enc
```

## Encryption Examples
```bash
# Encrypt to recipient's public key
age -r age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p secrets.txt > secrets.enc

# Encrypt to multiple recipients
age -r age1alice... -r age1bob... -r age1charlie... file.txt > file.enc

# Encrypt with passphrase (interactive)
age -p secrets.txt > secrets.enc

# Encrypt with passphrase from environment
echo "$PASSWORD" | age -p -a -o secrets.enc secrets.txt

# Encrypt directory (tar + age)
tar czf - ~/docs | age -r age1... > docs.tar.gz.age
```

## Decryption Examples
```bash
# Decrypt with identity file
age -d -i ~/.age/key.txt secrets.enc > secrets.txt

# Decrypt to stdout
age -d -i key.txt file.enc

# Try multiple identities
age -d -i ~/.age/work.txt -i ~/.age/personal.txt file.enc > file.txt

# Decrypt with passphrase
age -d secrets.enc > secrets.txt

# Decrypt tar archive
age -d -i key.txt docs.tar.gz.age | tar xzf -
```

## SSH Keys Integration
```bash
# Encrypt to SSH public key
age -R ~/.ssh/id_ed25519.pub secrets.txt > secrets.enc
age -R ~/.ssh/id_rsa.pub secrets.txt > secrets.enc

# Decrypt with SSH private key
age -d -i ~/.ssh/id_ed25519 secrets.enc > secrets.txt

# GitHub user's SSH keys
curl https://github.com/username.keys | age -R - secrets.txt > secrets.enc
```

## Practical Workflows

### Configuration Files
```bash
# Encrypt config
age -r age1recipient... config.yaml > config.yaml.age

# Decrypt for use
age -d -i ~/.age/key.txt config.yaml.age > config.yaml

# Use directly in scripts
export DB_PASSWORD=$(age -d -i ~/.age/key.txt passwords.age | grep DB_PASSWORD | cut -d= -f2)
```

### Backup Scripts
```bash
#!/bin/bash
BACKUP_KEY="age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"

# Encrypted backup
tar czf - ~/important | age -r $BACKUP_KEY > backup-$(date +%Y%m%d).tar.gz.age

# Upload to cloud
aws s3 cp backup-$(date +%Y%m%d).tar.gz.age s3://backups/
```

### Team Secrets
```bash
# Create recipients file
cat > team.txt <<EOF
# Alice
age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
# Bob
age1lggyhqrw2nlhcxprm67z43rta597azn8gknawjehu9d9dl0jq3yqqvfafg
# Charlie
age1ztmn9lykvegj0xgp85lp0gjsq9zs4dp73uqx8ewue92q63m7p6qs5v8nl3
EOF

# Encrypt for team
age -R team.txt secrets.txt > secrets.enc

# Each member can decrypt with their own key
age -d -i ~/.age/key.txt secrets.enc
```

### Git Repository Secrets
```bash
# .gitignore
secrets.txt

# Encrypt before commit
age -r age1... secrets.txt > secrets.txt.age
git add secrets.txt.age
git commit -m "Add encrypted secrets"

# Decrypt after pull
age -d -i ~/.age/key.txt secrets.txt.age > secrets.txt
```

## CI/CD Integration
```bash
# GitHub Actions - store key as secret
- name: Decrypt secrets
  run: |
    echo "${{ secrets.AGE_KEY }}" > key.txt
    age -d -i key.txt secrets.enc > secrets.txt

# GitLab CI
decrypt:
  script:
    - echo "$AGE_KEY" | age -d > secrets.txt secrets.enc
```

## Age vs GPG
| Feature | age | GPG |
|---------|-----|-----|
| Complexity | Simple | Complex |
| Key format | Short, readable | Long, cryptic |
| Algorithms | Modern (X25519, ChaCha20) | Many options |
| Defaults | Secure by default | Requires configuration |
| Learning curve | Minutes | Hours/days |
| File format | Simple, compact | Complex |

## Best Practices
- **Store keys securely**: Use `~/.age/` with `chmod 700`
- **Backup keys**: Encrypted keys in multiple locations
- **One key per context**: Work, personal, projects
- **Use SSH keys**: Leverage existing infrastructure
- **Recipients file**: For team/multi-recipient scenarios
- **Armor output**: Use `-a` for text-safe output

## File Formats
```bash
# Binary format (default)
age -r age1... file.txt > file.age

# ASCII-armored (for email, copy-paste)
age -a -r age1... file.txt > file.age

# Can decrypt either format
age -d -i key.txt file.age
```

## Tips
- **Public keys start with** `age1`
- **Private keys contain** `AGE-SECRET-KEY-1`
- **Recipient files**: One public key per line, `#` for comments
- **Streaming**: Works with pipes for large files
- **No metadata**: Encrypted files don't reveal recipients
- **Deterministic**: Same input + key = same output

## Common Errors
```bash
# Error: no identities match
# Solution: Wrong key file or file encrypted to different recipient
age -d -i correct-key.txt file.enc

# Error: bad header
# Solution: File not encrypted with age or corrupted
file file.enc  # Check if actually age-encrypted

# Error: incomplete private key
# Solution: Key file truncated, restore from backup
```

## Agent Use
- Encrypt CI/CD secrets
- Secure configuration management
- Automated backup encryption
- Key rotation workflows
- Team secret distribution

## Uninstall
```yaml
- preset: age
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/FiloSottile/age
- Spec: https://age-encryption.org/
- Search: "age encryption examples", "age vs gpg"

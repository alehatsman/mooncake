# SOPS - Secrets OPerationS

Encrypt/decrypt files with multiple key management services (AWS KMS, GCP KMS, Azure Key Vault, age, PGP). Edit secrets in plain text, store encrypted.

## Quick Start
```yaml
- preset: sops
```

## Features
- **Multiple KMS**: AWS KMS, GCP KMS, Azure Key Vault, HashiCorp Vault
- **age and PGP**: Use modern age or PGP keys
- **Partial encryption**: Encrypt only values, leave keys readable
- **Editor integration**: Edit encrypted files in your editor
- **CI/CD friendly**: Environment-based key selection
- **Format support**: YAML, JSON, ENV, INI, binary
- **Git integration**: Automatic encryption with git-crypt alternative

## Basic Usage
```bash
# Check version
sops --version

# Encrypt file
sops --encrypt secrets.yaml > secrets.enc.yaml

# Decrypt file
sops --decrypt secrets.enc.yaml > secrets.yaml

# Edit encrypted file (decrypts, opens editor, re-encrypts)
sops secrets.enc.yaml

# Encrypt in-place
sops --encrypt --in-place secrets.yaml
```

## Configuration

### .sops.yaml
```yaml
# Root .sops.yaml
creation_rules:
  # Production secrets (AWS KMS)
  - path_regex: environments/prod/.*\.yaml$
    kms: 'arn:aws:kms:us-east-1:123456789:key/abc-def'
    aws_profile: production

  # Staging secrets (age)
  - path_regex: environments/staging/.*\.yaml$
    age: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p

  # Development (PGP)
  - path_regex: environments/dev/.*\.yaml$
    pgp: '85D77543B3D624B63CEA9E6DBC17301B491B3F21'

  # Default (age)
  - age: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

## AWS KMS

### Setup
```bash
# Set AWS credentials
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=us-east-1

# Encrypt with KMS
sops --kms 'arn:aws:kms:us-east-1:123:key/abc' secrets.yaml

# Decrypt (auto-detects KMS key)
sops --decrypt secrets.yaml
```

### Multiple Keys
```bash
# Multiple KMS keys for redundancy
sops --kms 'arn:aws:kms:us-east-1:123:key/abc,arn:aws:kms:us-west-2:123:key/def' \
  secrets.yaml
```

## GCP KMS

### Setup
```bash
# Set GCP credentials
export GOOGLE_APPLICATION_CREDENTIALS=~/gcp-key.json

# Encrypt with GCP KMS
sops --gcp-kms \
  'projects/myproject/locations/global/keyRings/sops/cryptoKeys/sops-key' \
  secrets.yaml
```

## Azure Key Vault

### Setup
```bash
# Set Azure credentials
export AZURE_TENANT_ID=...
export AZURE_CLIENT_ID=...
export AZURE_CLIENT_SECRET=...

# Encrypt with Azure
sops --azure-kv \
  'https://myvault.vault.azure.net/keys/sops-key/abc123' \
  secrets.yaml
```

## age (Recommended for Simplicity)

### Generate Keys
```bash
# Generate age key
age-keygen -o keys.txt

# Output:
# Public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
# Private key in keys.txt
```

### Encrypt with age
```bash
# Set private key
export SOPS_AGE_KEY_FILE=~/keys.txt

# Or inline
export SOPS_AGE_KEY=$(cat keys.txt | grep 'AGE-SECRET-KEY' | cut -d' ' -f3)

# Encrypt
sops --age age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p \
  secrets.yaml
```

## PGP

### Generate Keys
```bash
# Generate PGP key
gpg --gen-key

# List keys
gpg --list-keys

# Export public key
gpg --armor --export user@example.com > pubkey.asc
```

### Encrypt with PGP
```bash
# Using fingerprint
sops --pgp 85D77543B3D624B63CEA9E6DBC17301B491B3F21 secrets.yaml

# Using email
sops --pgp user@example.com secrets.yaml

# Multiple recipients
sops --pgp 'key1,key2,key3' secrets.yaml
```

## Editing Secrets

### Edit Encrypted File
```bash
# Opens in $EDITOR, auto-encrypts on save
sops secrets.enc.yaml

# Specify editor
EDITOR=vim sops secrets.enc.yaml
```

### Partial Encryption
```yaml
# Original file (secrets.yaml)
database:
  host: db.example.com
  password: secret123  # Will be encrypted
api:
  endpoint: https://api.example.com
  token: abc123def456  # Will be encrypted
```

```bash
# Encrypt only values (keys stay plaintext)
sops --encrypt secrets.yaml

# Result: keys visible, values encrypted
```

## Decrypt for Use

### Inline Decryption
```bash
# Decrypt to stdout
sops --decrypt secrets.yaml

# Use in scripts
DB_PASSWORD=$(sops --decrypt --extract '["database"]["password"]' secrets.yaml)

# Apply to Kubernetes
sops --decrypt secrets.enc.yaml | kubectl apply -f -
```

### Extract Specific Values
```bash
# Extract single value
sops --decrypt --extract '["api"]["token"]' secrets.yaml

# JSON path
sops --decrypt --extract '["users"][0]["password"]' secrets.json
```

## CI/CD Integration

### GitHub Actions with age
```yaml
- name: Install SOPS
  run: |
    wget https://github.com/mozilla/sops/releases/latest/download/sops-v3.8.1.linux.amd64
    sudo mv sops-v3.8.1.linux.amd64 /usr/local/bin/sops
    sudo chmod +x /usr/local/bin/sops

- name: Decrypt secrets
  env:
    SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
  run: |
    sops --decrypt secrets.enc.yaml > secrets.yaml
    export $(cat secrets.yaml | grep -v '^#' | xargs)
```

### GitLab CI with AWS KMS
```yaml
decrypt:
  before_script:
    - apt-get update && apt-get install -y sops
  script:
    - sops --decrypt secrets.enc.yaml > secrets.yaml
    - export $(cat secrets.yaml | xargs)
  only:
    - main
```

### Jenkins
```groovy
pipeline {
  agent any
  environment {
    AWS_ACCESS_KEY_ID = credentials('aws-key-id')
    AWS_SECRET_ACCESS_KEY = credentials('aws-secret-key')
  }
  stages {
    stage('Decrypt') {
      steps {
        sh 'sops --decrypt config/secrets.enc.yaml > secrets.yaml'
        sh 'source secrets.yaml && ./deploy.sh'
      }
    }
  }
}
```

## Git Integration

### git-diff Support
```bash
# .gitattributes
*.enc.yaml diff=sopsdiffer

# .gitconfig
[diff "sopsdiffer"]
  textconv = sops --decrypt
```

### Pre-commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit

# Encrypt secrets before commit
for file in $(git diff --cached --name-only | grep 'secrets.*\.yaml$'); do
  if ! grep -q "sops:" "$file"; then
    echo "Encrypting $file"
    sops --encrypt --in-place "$file"
    git add "$file"
  fi
done
```

## Real-World Examples

### Kubernetes Secrets
```yaml
- name: Decrypt Kubernetes secrets
  shell: sops --decrypt k8s/secrets.enc.yaml > /tmp/secrets.yaml
  environment:
    SOPS_AGE_KEY: "{{ age_private_key }}"

- name: Apply secrets
  shell: kubectl apply -f /tmp/secrets.yaml

- name: Cleanup decrypted file
  shell: rm /tmp/secrets.yaml
```

### Terraform with SOPS
```yaml
- name: Decrypt terraform.tfvars
  shell: |
    sops --decrypt terraform.tfvars.enc > terraform.tfvars
  environment:
    AWS_PROFILE: production

- name: Apply Terraform
  shell: terraform apply -auto-approve
  cwd: /infrastructure

- name: Remove plaintext secrets
  shell: rm terraform.tfvars
```

### Docker Build Secrets
```yaml
- name: Decrypt build secrets
  shell: sops --decrypt secrets.enc.env > .env

- name: Build Docker image
  shell: docker build --secret id=env,src=.env -t myapp .

- name: Cleanup
  shell: rm .env
```

## Format Support

### YAML
```bash
sops --encrypt secrets.yaml
```

### JSON
```bash
sops --encrypt secrets.json
```

### ENV files
```bash
sops --encrypt .env
```

### INI files
```bash
sops --encrypt config.ini
```

### Binary files
```bash
sops --encrypt private-key.pem
```

## Key Rotation

### Rotate Keys
```bash
# Add new key
sops --rotate --add-kms 'arn:aws:kms:us-east-1:123:key/new' secrets.yaml

# Remove old key
sops --rotate --rm-kms 'arn:aws:kms:us-east-1:123:key/old' secrets.yaml

# Both at once
sops --rotate \
  --add-kms 'arn:aws:kms:us-east-1:123:key/new' \
  --rm-kms 'arn:aws:kms:us-east-1:123:key/old' \
  secrets.yaml
```

## Troubleshooting

### Cannot Decrypt
```bash
# Check keys
sops --decrypt --verbose secrets.yaml

# Verify KMS access
aws kms describe-key --key-id arn:aws:kms:...

# Check age key
echo $SOPS_AGE_KEY
```

### File Not Encrypted
```bash
# Verify encryption
grep 'sops:' secrets.yaml

# Check metadata
sops --decrypt --extract '["sops"]' secrets.yaml
```

## Best Practices
- Use `.sops.yaml` for consistent encryption rules
- Store private keys in secrets management (not in repo)
- Rotate keys periodically
- Use multiple keys for redundancy
- Encrypt files with `.enc.yaml` suffix for clarity
- Never commit decrypted secrets
- Use age for simplicity, KMS for production
- Add encrypted files to `.gitattributes` for diff support

## Platform Support
- ✅ Linux (amd64, arm64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows
- ✅ Docker containers

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated secrets decryption in CI/CD
- Secure configuration management
- Multi-environment secret handling
- Key rotation automation
- Secrets synchronization
- Compliance and audit logging

## Advanced Configuration
```yaml
- preset: sops
  with:
    state: present
```

## Uninstall
```yaml
- preset: sops
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/mozilla/sops
- Documentation: https://github.com/mozilla/sops#usage
- age: https://github.com/FiloSottile/age
- Tutorial: https://github.com/mozilla/sops#important-information-on-types
- Search: "sops encryption", "sops age", "sops kubernetes"

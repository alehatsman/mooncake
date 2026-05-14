# nkeys - NATS Cryptographic Key Utility

Command-line tool for creating and managing NKEYs, the cryptographic keys used in NATS security.

## Quick Start

```yaml
- preset: nkeys
```

## Features

- **Key generation**: Create user, account, operator, and server keys
- **Key signing**: Sign messages and authentication tokens
- **Public key extraction**: Get public keys from private keys
- **Seed management**: Securely handle key seeds
- **JWT support**: Work with NATS JWT tokens
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage

```bash
# Generate user key
nk -gen user -pubout

# Generate account key
nk -gen account -pubout

# Generate operator key
nk -gen operator -pubout

# Generate server key
nk -gen server -pubout

# Extract public key from seed
nk -inkey user.nk -pubout

# Sign data
echo "data" | nk -sign -inkey user.nk

# Verify signature
nk -verify -inkey public.key -sigfile signature.sig -infile data.txt
```

## Advanced Configuration

```yaml
# Install nkeys
- preset: nkeys

# Generate operator key
- name: Create operator key
  shell: nk -gen operator -pubout > operator.nk
  creates: operator.nk

# Generate account key
- name: Create account key
  shell: nk -gen account -pubout > account.nk
  creates: account.nk

# Generate user key
- name: Create user key
  shell: nk -gen user -pubout > user.nk
  creates: user.nk

# Set secure permissions
- name: Secure key files
  file:
    path: "{{ item }}"
    mode: "0600"
  loop:
    - operator.nk
    - account.nk
    - user.nk
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove nkeys |

## Platform Support

- ✅ Linux (binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Windows (binary install)

## Configuration

- **Key files**: Store in secure location with 0600 permissions
- **Seed format**: Seeds start with 'S' prefix (e.g., SUAAVK2...)
- **Public key format**: Public keys start with type prefix (U/A/O/N)

## Real-World Examples

### Complete NATS Security Setup
```yaml
# Setup NATS security infrastructure
- name: Install nkeys
  preset: nkeys

- name: Install nsc (NATS account management)
  preset: nsc

- name: Create secure key directory
  file:
    path: /etc/nats/keys
    state: directory
    mode: "0700"
  become: true

- name: Generate operator key
  shell: nk -gen operator -pubout
  register: operator_key
  become: true

- name: Save operator seed
  copy:
    content: "{{ operator_key.stdout_lines[0] }}"
    dest: /etc/nats/keys/operator.nk
    mode: "0600"
  become: true

- name: Generate account key
  shell: nk -gen account -pubout
  register: account_key
  become: true

- name: Save account seed
  copy:
    content: "{{ account_key.stdout_lines[0] }}"
    dest: /etc/nats/keys/account.nk
    mode: "0600"
  become: true
```

### User Credential Generation
```bash
# Generate user key
USER_KEY=$(nk -gen user -pubout)

# Extract seed and public key
SEED=$(echo "$USER_KEY" | head -1)
PUBLIC=$(echo "$USER_KEY" | tail -1)

# Save securely
echo "$SEED" > ~/.nkeys/user.nk
chmod 600 ~/.nkeys/user.nk

echo "Public Key: $PUBLIC"
```

### Sign and Verify Data
```bash
# Sign data with private key
echo "Important message" | nk -sign -inkey user.nk > message.sig

# Verify signature with public key
nk -verify \
  -inkey public.key \
  -sigfile message.sig \
  -infile message.txt
```

### Automated Key Rotation
```yaml
# Rotate user keys regularly
- name: Generate new user key
  shell: nk -gen user -pubout
  register: new_user_key

- name: Backup old key
  copy:
    src: /etc/nats/keys/user.nk
    dest: /etc/nats/keys/user.nk.{{ ansible_date_time.epoch }}
    remote_src: true
  become: true

- name: Install new key
  copy:
    content: "{{ new_user_key.stdout_lines[0] }}"
    dest: /etc/nats/keys/user.nk
    mode: "0600"
  become: true

- name: Update NATS configuration
  shell: nsc edit user --name myuser --public-key {{ new_user_key.stdout_lines[1] }}
  become: true
```

## Key Types

### Operator Keys
```bash
# Operator - top level authority
nk -gen operator -pubout
# Seed: SOAAVK2QN...
# Public: OABC7YZ...
```

### Account Keys
```bash
# Account - represents a tenant or organization
nk -gen account -pubout
# Seed: SAAAK3VN...
# Public: AABC7YZ...
```

### User Keys
```bash
# User - represents an individual user/application
nk -gen user -pubout
# Seed: SUAAVK2Q...
# Public: UABC7YZ...
```

### Server/Cluster Keys
```bash
# Server - represents a NATS server
nk -gen server -pubout
# Seed: SNAAVK2Q...
# Public: NABC7YZ...

# Cluster - for server-to-server authentication
nk -gen cluster -pubout
```

## Integration with NSC

```bash
# Use nkeys with nsc (NATS Security CLI)
# Generate operator
OPERATOR=$(nk -gen operator -pubout | head -1)

# Initialize nsc with operator
nsc init --operator MyOperator --seed "$OPERATOR"

# Add account
nsc add account --name MyAccount

# Add user
nsc add user --name MyUser
```

## Security Best Practices

```bash
# Store seeds securely
mkdir -p ~/.nkeys
chmod 700 ~/.nkeys

# Never commit seeds to version control
echo "*.nk" >> .gitignore

# Use environment variables for automation
export NKEYS_PATH=~/.nkeys

# Rotate keys periodically
# - Operator keys: Rarely (major security event only)
# - Account keys: Annually or on security event
# - User keys: Monthly or on compromise
# - Server keys: On server replacement
```

## Agent Use

- Automate NATS security infrastructure setup
- Generate and rotate cryptographic keys programmatically
- Implement zero-trust authentication for microservices
- Create secure multi-tenant NATS deployments
- Manage user credentials in CI/CD pipelines
- Sign and verify messages in distributed systems

## Troubleshooting

### Invalid seed format
```bash
# Seeds must start with 'S'
# Check seed validity
nk -inkey myseed.nk -pubout

# Regenerate if corrupted
nk -gen user -pubout > newseed.nk
```

### Permission denied errors
```bash
# Fix key file permissions
chmod 600 *.nk
chmod 700 ~/.nkeys

# Verify ownership
ls -l ~/.nkeys/
```

### Key type mismatch
```bash
# Verify key type from public key prefix
# O = Operator
# A = Account
# U = User
# N = Server

# Extract public key
nk -inkey key.nk -pubout
```

## Uninstall

```yaml
- preset: nkeys
  with:
    state: absent
```

## Resources

- NATS docs: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro
- NKeys library: https://github.com/nats-io/nkeys
- NSC tool: https://github.com/nats-io/nsc
- Search: "nats nkeys tutorial", "nats security setup", "nkeys authentication"

# NSC - NATS Account Management Tool

CLI tool for managing NATS accounts, users, and permissions in a secure, decentralized way.

## Quick Start

```yaml
- preset: nsc
```

## Features

- **Account management**: Create and configure NATS accounts
- **User management**: Add users with fine-grained permissions
- **JWT generation**: Create signed JSON Web Tokens
- **Decentralized security**: No central auth server required
- **Export/Import**: Share account configurations securely
- **Multi-operator**: Support multiple NATS operators
- **Permission control**: Define publish/subscribe rules per user

## Basic Usage

```bash
# Initialize NSC with operator
nsc init --operator MyOperator

# Add account
nsc add account --name MyAccount

# Add user to account
nsc add user --name myuser --account MyAccount

# List accounts
nsc list accounts

# List users in account
nsc list users --account MyAccount

# Generate user credentials
nsc generate creds --account MyAccount --name myuser

# Describe account
nsc describe account MyAccount

# Edit user permissions
nsc edit user --name myuser --allow-pub "events.>" --allow-sub "data.>"
```

## Advanced Configuration

```yaml
# Install NSC
- preset: nsc

# Install nkeys for key generation
- preset: nkeys

# Initialize operator
- name: Initialize NATS operator
  shell: |
    nsc init --operator {{ operator_name }} \
      --operator-jwt-server-url {{ jwt_server_url }}
  creates: ~/.nsc/nats/{{ operator_name }}

# Create account
- name: Create production account
  shell: |
    nsc add account --name production
  register: account_created

# Add admin user
- name: Create admin user
  shell: |
    nsc add user --name admin \
      --account production \
      --allow-pub ">" \
      --allow-sub ">"

# Add service user with restricted permissions
- name: Create service user
  shell: |
    nsc add user --name api-service \
      --account production \
      --allow-pub "api.requests.>" \
      --allow-sub "api.responses.>"

# Generate credentials for deployment
- name: Generate user credentials
  shell: nsc generate creds --account production --name api-service
  register: user_creds

- name: Save credentials
  copy:
    content: "{{ user_creds.stdout }}"
    dest: /etc/nats/api-service.creds
    mode: "0600"
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove NSC |

## Platform Support

- ✅ Linux (binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Windows (binary install, chocolatey)

## Configuration

- **NSC directory**: `~/.nsc/` (stores operators, accounts, users)
- **Keystore**: `~/.nkeys/` (stores private keys)
- **Environment**: `NSC_HOME` (override default directory)

## Real-World Examples

### Complete NATS Security Setup
```yaml
# Setup secure NATS infrastructure
- name: Install NATS tooling
  preset: nsc

- name: Initialize operator
  shell: nsc init --operator MyOrg
  creates: ~/.nsc/nats/MyOrg

- name: Create accounts for environments
  shell: nsc add account --name {{ item }}
  loop:
    - production
    - staging
    - development

- name: Create service users
  shell: |
    nsc add user --name {{ item.user }} \
      --account {{ item.account }} \
      --allow-pub "{{ item.pub }}" \
      --allow-sub "{{ item.sub }}"
  loop:
    - { user: "api-gateway", account: "production", pub: "api.>", sub: "services.>" }
    - { user: "data-processor", account: "production", pub: "events.>", sub: "data.>" }
    - { user: "monitoring", account: "production", pub: "", sub: ">" }
```

### Multi-Tenant Configuration
```bash
# Create tenant accounts
nsc add account --name tenant-a
nsc add account --name tenant-b

# Add users per tenant
nsc add user --name tenant-a-user --account tenant-a \
  --allow-pub "tenant-a.>" \
  --allow-sub "tenant-a.>"

nsc add user --name tenant-b-user --account tenant-b \
  --allow-pub "tenant-b.>" \
  --allow-sub "tenant-b.>"

# Tenants are isolated by default
```

### Permission Patterns
```bash
# Read-only user
nsc add user --name reader \
  --allow-sub "data.>" \
  --deny-pub ">"

# Write-only user
nsc add user --name writer \
  --allow-pub "events.>" \
  --deny-sub ">"

# Service-specific permissions
nsc add user --name order-service \
  --allow-pub "orders.created,orders.updated" \
  --allow-sub "orders.requests.>"

# Admin with request-reply pattern
nsc add user --name admin \
  --allow-pub ">" \
  --allow-sub ">" \
  --allow-pub-response
```

### CI/CD Integration
```yaml
# Generate and deploy credentials
- name: Create deployment user
  shell: |
    nsc add user --name {{ app_name }}-{{ environment }} \
      --account {{ environment }} \
      --allow-pub "{{ app_name }}.>" \
      --allow-sub "_INBOX.>,{{ app_name }}.>"

- name: Generate credentials
  shell: |
    nsc generate creds \
      --account {{ environment }} \
      --name {{ app_name }}-{{ environment }}
  register: creds

- name: Deploy credentials to Kubernetes
  shell: |
    kubectl create secret generic nats-creds \
      --from-literal=creds="{{ creds.stdout }}" \
      --namespace {{ namespace }}
  when: creds.changed
```

### Account Export/Import
```bash
# Export account JWT
nsc describe account MyAccount --json > account.jwt

# Import account to another operator
nsc add account --name ImportedAccount --account-jwt-file account.jwt

# Push account to JWT server
nsc push --account MyAccount
```

## Permission Syntax

### Subject Patterns
```bash
# Wildcards
">"           # Match everything
"foo.>"       # Match foo.bar, foo.baz.qux, etc.
"foo.*"       # Match foo.bar, foo.baz (single level)
"foo.*.qux"   # Match foo.bar.qux, foo.baz.qux

# Multiple subjects (comma-separated)
"foo.bar,baz.qux,events.>"
```

### Permission Types
```bash
# Publishing
--allow-pub "subject.>"       # Allow publishing
--deny-pub "subject.>"        # Deny publishing

# Subscribing
--allow-sub "subject.>"       # Allow subscribing
--deny-sub "subject.>"        # Deny subscribing

# Request-Reply
--allow-pub-response          # Allow response in request-reply
--response-ttl 5s             # Response time limit
```

## Limits and Quotas

```bash
# Connection limits
nsc edit user --name myuser \
  --max-connections 10 \
  --max-payload 1048576 \      # 1MB
  --max-subscriptions 100

# Data limits
nsc edit account --name MyAccount \
  --max-connections 1000 \
  --max-data 10GB \
  --max-exports 10 \
  --max-imports 10
```

## Agent Use

- Automate NATS security infrastructure provisioning
- Implement multi-tenant messaging architectures
- Enforce least-privilege access for microservices
- Rotate user credentials programmatically
- Deploy decentralized auth without central server
- Manage permissions as code in version control

## Troubleshooting

### Operator not found
```bash
# Check NSC home directory
echo $NSC_HOME
ls ~/.nsc/nats/

# Reinitialize if needed
nsc init --operator MyOperator
```

### Permission denied
```bash
# Check user permissions
nsc describe user myuser --account MyAccount

# Update permissions
nsc edit user --name myuser \
  --allow-pub ">" \
  --allow-sub ">"
```

### Credentials not working
```bash
# Regenerate credentials
nsc generate creds --account MyAccount --name myuser

# Verify JWT validity
nsc describe jwt --file user.creds
```

## Uninstall

```yaml
- preset: nsc
  with:
    state: absent
```

## Resources

- Official docs: https://docs.nats.io/using-nats/nats-tools/nsc
- GitHub: https://github.com/nats-io/nsc
- NATS security: https://docs.nats.io/running-a-nats-service/configuration/securing_nats
- Search: "nats nsc tutorial", "nats account management", "nats security"

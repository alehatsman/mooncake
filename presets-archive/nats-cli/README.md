# nats-cli - NATS Command-Line Client

NATS CLI is the official command-line interface for NATS, a cloud-native messaging system. It provides powerful tools for managing NATS servers, accounts, and messaging operations directly from the terminal.

## Quick Start

```yaml
- preset: nats-cli
```

## Features

- **Unified CLI**: Complete NATS management from the command line
- **Account Management**: Create, manage, and configure NATS accounts and users
- **Monitoring**: Real-time server status and performance metrics
- **Debugging**: Trace message flows and diagnose connectivity issues
- **Context-based**: Switch between NATS servers with saved contexts
- **Cross-platform**: Linux, macOS, and compatible with various package managers

## Basic Usage

```bash
# Check version and verify installation
nats --version

# Show general help
nats --help

# Show subcommand help
nats account --help

# Connect to a NATS server
nats -s nats://localhost:4222 server info

# List accounts
nats account list

# Create a new account
nats account create myaccount

# Publish a message
nats pub my.subject "Hello NATS"

# Subscribe to a topic
nats sub my.subject

# View server info
nats server info

# Check connection status
nats server ping
```

## Advanced Configuration

```yaml
# Basic installation
- preset: nats-cli

# With specific version (requires manual specification via environment)
- preset: nats-cli
  with:
    state: present

# Prepare for uninstallation
- preset: nats-cli
  with:
    state: absent
```

## Parameters

| Parameter | Type   | Default | Description                  |
|-----------|--------|---------|------------------------------|
| state     | string | present | Install or remove tool       |

## Platform Support

- ✅ macOS (Homebrew - natscli package)
- ✅ Linux (Go installation via go install)
- ✅ Windows (via Go - manual setup required)

## Configuration

- **Config directory**: `~/.config/nats/` (Linux), `~/Library/Application Support/nats/` (macOS)
- **Context storage**: Stores NATS server connections and credentials
- **Environment variable**: `NATS_URL` to specify server address
- **Default port**: 4222

## Real-World Examples

### CI/CD Pipeline - Server Health Check

```bash
# Verify NATS server is running before deploying
nats -s nats://production.example.com:4222 server ping
if [ $? -eq 0 ]; then
  echo "NATS server is healthy"
  # Continue with deployment
else
  echo "ERROR: NATS server is not responding"
  exit 1
fi
```

### Message Debugging Workflow

```bash
# Subscribe to all messages in a subject hierarchy
nats -s nats://localhost:4222 sub "app.events.*" --wait 10

# Publish test messages
nats -s nats://localhost:4222 pub "app.events.order" '{"order_id": "12345", "amount": 99.99}'

# Check server statistics
nats -s nats://localhost:4222 server stats
```

### Account and User Management

```bash
# Create new account with specific settings
nats account create customer-service

# List all users in an account
nats account list-users customer-service

# View account details
nats account info customer-service
```

## Agent Use

- Verify NATS connectivity in deployment pipelines
- Automate account and user provisioning workflows
- Monitor message queue depth and publish/subscribe metrics
- Validate configuration before service startup
- Publish test messages for integration testing
- Diagnose connectivity issues programmatically

## Troubleshooting

### Connection Refused

**Problem**: `nats: failed to connect: dial: no such host`

**Solution**: Ensure NATS server is running and accessible:
```bash
# Try connecting with explicit server address
nats -s nats://localhost:4222 server info

# Check if server is listening on expected port
netstat -tuln | grep 4222  # Linux
lsof -i :4222  # macOS
```

### Command Not Found

**Problem**: `nats: command not found`

**Solution**: Verify installation:
```bash
command -v nats
which nats
# If not found, reinstall:
brew install natscli  # macOS
go install github.com/nats-io/natscli/nats@latest  # Linux
```

### Permission Denied

**Problem**: Permission errors during installation

**Solution**: Some installations require elevated privileges:
```bash
# If needed, use sudo (uncommon)
sudo go install github.com/nats-io/natscli/nats@latest
```

## Uninstall

```yaml
- preset: nats-cli
  with:
    state: absent
```

## Resources

- Official docs: https://docs.nats.io/nats-tools/nats_cli
- GitHub: https://github.com/nats-io/natscli
- NATS documentation: https://docs.nats.io/
- Search: "nats cli tutorial", "nats cli examples", "nats account management"

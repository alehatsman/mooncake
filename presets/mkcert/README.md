# mkcert - Local HTTPS Certificate Generation Tool

Generate locally-trusted development HTTPS certificates with automatic CA installation and browser support.

## Quick Start

```yaml
- preset: mkcert
```

## Features

- **Zero configuration**: Instantly create valid HTTPS certificates for localhost and custom domains
- **Automatic CA installation**: Installs root certificate automatically in system trust stores
- **Cross-platform CA support**: Works with macOS keychain, Linux system store, and Windows certificate store
- **Multi-domain certificates**: Create certificates for multiple domains and IP addresses in one operation
- **OpenSSL compatible**: Generated certificates work with all servers and browsers
- **Development-focused**: Perfect for local development, testing, and staging environments
- **Simple CLI**: Straightforward command-line interface with sensible defaults

## Basic Usage

```bash
# Check version
mkcert --version

# List installed CA
mkcert -CAROOT

# Create certificate for localhost
mkcert localhost 127.0.0.1

# Create certificate for domain
mkcert example.com www.example.com

# Create certificate for subdomain
mkcert "*.example.local"

# Uninstall CA from system trust store
mkcert -uninstall
```

## Advanced Configuration

```yaml
# Install with defaults (recommended)
- preset: mkcert

# Install with cleanup option
- preset: mkcert
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (`present`) or remove (`absent`) |

## Platform Support

- ✅ Linux (NSS - Mozilla/Chrome/Chromium)
- ✅ macOS (Keychain)
- ❌ Windows (not yet supported, use Go install method)

## Configuration

- **CA location**: `~/.local/share/mkcert/` (Linux), `~/Library/Application Support/mkcert/` (macOS)
- **CA root certificate**: Auto-installed in system trust stores
- **Certificate format**: PKCS #8 (modern format with AES-256 encryption)
- **File permissions**: 0600 for private keys (user read/write only)

## Real-World Examples

### Local Development Web Server

```bash
# Create certificate for development domain
mkcert dev.local

# Start web server with HTTPS
python -m http.server 8443 \
  --certfile dev.local.pem \
  --keyfile dev.local-key.pem

# Access at https://dev.local:8443
```

### Docker Development Environment

```bash
# Generate certificate once on host
mkcert localhost 127.0.0.1 docker.local

# Copy certificate to Docker image (optional)
# Then start service with:
# openssl s_server -cert cert.pem -key key.pem -port 443
```

### Testing with curl

```bash
# Create certificate
mkcert test.local

# Verify certificate with curl
curl --cacert ~/.local/share/mkcert/rootCA.pem https://test.local:8443
```

### Nginx Configuration

```bash
# Create certificates
mkcert api.example.local

# Configure Nginx
server {
    listen 443 ssl http2;
    server_name api.example.local;

    ssl_certificate /path/to/api.example.local.pem;
    ssl_certificate_key /path/to/api.example.local-key.pem;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

## Agent Use

- Automated HTTPS certificate generation for local development environments
- CI/CD pipeline setup for testing HTTPS functionality
- Containerized development environment certificate provisioning
- Automated testing of SSL/TLS implementations
- Development domain configuration with automatic browser trust

## Troubleshooting

### Certificate not trusted in browser

Verify CA is installed:

```bash
# Check CA location
mkcert -CAROOT

# Reinstall CA to trust store
mkcert -install
```

### "too many open files" error

This is a limitation of Firefox's NSS database. Use a different browser or increase system limits:

```bash
# Increase file descriptor limit
ulimit -n 4096
```

### Private key permissions issues

Ensure key file has correct permissions:

```bash
chmod 600 *.pem
```

### Certificate generated but browser shows warning

1. Verify domain matches certificate name
2. Ensure CA is installed: `mkcert -install`
3. Clear browser cache and restart browser
4. Check certificate validity: `openssl x509 -text -noout -in cert.pem`

## Uninstall

```yaml
- preset: mkcert
  with:
    state: absent
```

This removes mkcert binary. CA remains installed in system trust store.

To fully uninstall and remove CA from trust:

```bash
mkcert -uninstall
```

## Resources

- Official docs: https://github.com/FiloSottile/mkcert
- GitHub: https://github.com/FiloSottile/mkcert
- Issue tracker: https://github.com/FiloSottile/mkcert/issues
- Search: "mkcert localhost", "mkcert certificate generation", "local HTTPS development"

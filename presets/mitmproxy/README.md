# mitmproxy - Interactive HTTPS Proxy for Debugging

Free and open source interactive HTTPS proxy for testing, debugging, and pentesting. Intercept, inspect, and modify HTTP/HTTPS traffic in real-time with a user-friendly interface.

## Quick Start

```yaml
- preset: mitmproxy
```

## Features

- **Interactive Console**: Real-time traffic inspection and modification
- **HTTPS Decryption**: Decrypt HTTPS traffic for debugging (with certificate injection)
- **Request/Response Editing**: Modify headers, bodies, and parameters on-the-fly
- **Multiple Interfaces**: `mitmproxy` (TUI), `mitmweb` (web UI), `mitmdump` (headless)
- **Scripting**: Python addons for custom traffic manipulation
- **Replay**: Record and replay HTTP requests for testing
- **Cross-platform**: Linux, macOS with full support

## Basic Usage

```bash
# Check version
mitmproxy --version

# Start interactive proxy (TUI mode)
mitmproxy

# Start web-based interface (browser UI)
mitmweb

# Start headless proxy (no UI, for scripts)
mitmdump

# Proxy on custom port
mitmproxy -p 8888

# Save traffic to file
mitmdump -w traffic.dump

# Load and replay traffic
mitmdump -r traffic.dump
```

## Advanced Configuration

```yaml
- preset: mitmproxy
  with:
    state: present    # Install or remove
```

Configure your browser/client to use proxy:

```bash
# Linux/macOS applications
export http_proxy=http://127.0.0.1:8080
export https_proxy=http://127.0.0.1:8080

# Browser proxy settings
Proxy host: 127.0.0.1
Proxy port: 8080
```

Install CA certificate for HTTPS decryption:

```bash
# macOS
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain \
  ~/.mitmproxy/mitmproxy-ca-cert.pem

# Linux (Firefox)
# Settings > Privacy > Certificates > View Certificates > Import
# Select ~/.mitmproxy/mitmproxy-ca-cert.pem
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Configuration

- **Config directory**: `~/.mitmproxy/` (Linux/macOS)
- **CA certificate**: `~/.mitmproxy/mitmproxy-ca-cert.pem`
- **CA key**: `~/.mitmproxy/mitmproxy-ca.key`
- **Default listen port**: 8080
- **Client mode**: Connect to upstream proxy (advanced)

## Platform Support

- ✅ Linux (pip, apt via install script)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Real-World Examples

### API Debugging and Inspection

Inspect and debug mobile app API calls:

```bash
# Start mitmproxy
mitmproxy

# In another terminal, configure client
export http_proxy=http://127.0.0.1:8080
export https_proxy=http://127.0.0.1:8080

# Make requests through proxy
curl https://api.example.com/users

# In mitmproxy TUI:
# - View requests/responses
# - Edit request body before sending
# - Inspect response headers
```

### Request/Response Modification

Modify live traffic for testing:

```bash
# Start mitmweb (web UI)
mitmweb

# Navigate browser to https://localhost:8081
# Browse to http://example.com

# In mitmweb:
# 1. Intercept requests
# 2. Modify headers (add Authorization, etc.)
# 3. Change request body
# 4. Replay modified request
```

### Python Addon for Traffic Manipulation

Create custom traffic rules with Python:

```python
# addon.py
from mitmproxy import http

class AddCustomHeader:
    def request(self, flow: http.HTTPFlow) -> None:
        # Add auth header to specific domains
        if "api.example.com" in flow.request.host:
            flow.request.headers["Authorization"] = "Bearer token123"

    def response(self, flow: http.HTTPFlow) -> None:
        # Log response times
        print(f"{flow.request.url}: {flow.response.status_code}")

addons = [AddCustomHeader()]
```

Run with addon:

```bash
mitmproxy -s addon.py
```

### Recording and Replaying Traffic

Test and debug with recorded traffic:

```bash
# Record traffic to file
mitmdump -w test-traffic.dump

# Later, replay same traffic for testing
mitmdump -r test-traffic.dump --server-replay-nopop

# Clients see replayed responses without hitting real server
```

### Integration Testing with mitmproxy

Verify application behavior with modified responses:

```bash
# Test error handling
mitmdump -s test-addon.py  # Returns 500 errors

# Test rate limiting
mitmdump -s test-addon.py  # Returns 429 Retry-After

# Application should handle gracefully
pytest integration_tests.py
```

## Agent Use

- **API contract testing**: Intercept and verify API requests match expected format
- **Security testing**: Inject headers, modify requests to test input validation
- **Performance analysis**: Capture traffic for analysis and optimization
- **Integration testing**: Record real API responses, replay for deterministic tests
- **Protocol debugging**: Inspect WebSocket, HTTP/2 traffic from applications

## Troubleshooting

### HTTPS traffic not decrypted

Install mitmproxy CA certificate in system trust store:

```bash
# macOS - Add to System Keychain
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain \
  ~/.mitmproxy/mitmproxy-ca-cert.pem

# Linux - Different methods per distro
# For Firefox: Settings > Privacy > Certificates > Import
# For system-wide (Ubuntu):
sudo cp ~/.mitmproxy/mitmproxy-ca-cert.pem /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

### Certificate warnings in browser

Clear browser cache and restart after certificate installation:

```bash
# Chrome: Settings > Privacy > Clear browsing data > Cookies/Cache
# Firefox: Edit > Preferences > Privacy > Clear Data
# Safari: Develop > Clear Caches
```

### Port already in use

Use different port:

```bash
# Listen on port 8888
mitmproxy -p 8888

# Update client proxy settings to 127.0.0.1:8888
```

### Connection refused from client

Verify proxy is running and accessible:

```bash
# Check if mitmproxy is listening
lsof -i :8080

# Test connectivity
curl -x http://127.0.0.1:8080 http://example.com
```

### Performance/Memory issues

Filter traffic to reduce load:

```bash
# Exclude certain hosts
mitmproxy --ignore-hosts "^.*\.example\.com$"

# Limit log size
mitmdump -w traffic.dump --limit-size 100m
```

## Uninstall

```yaml
- preset: mitmproxy
  with:
    state: absent
```

**Note:** Configuration directory and CA certificates are preserved. Remove `~/.mitmproxy/` manually if desired.

## Resources

- Official docs: https://docs.mitmproxy.org/
- GitHub: https://github.com/mitmproxy/mitmproxy
- Web UI: https://docs.mitmproxy.org/stable/tools-mitmweb/
- Python API: https://docs.mitmproxy.org/stable/addons.html
- Search: "mitmproxy API debugging", "mitmproxy certificate setup", "mitmproxy Python addon"

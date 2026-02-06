# ngrok - Secure Tunneling to Localhost

Secure introspectable tunnels to localhost for webhook testing, demos, and remote access.

## Quick Start

```yaml
- preset: ngrok
```

## Features

- **Instant public URLs**: Expose localhost to the internet instantly
- **HTTPS tunnels**: Automatic SSL/TLS encryption
- **Replay requests**: Inspect and replay webhook payloads
- **Custom domains**: Use your own domain names (paid plans)
- **TCP tunnels**: Expose any TCP service
- **Request inspector**: Web UI to debug HTTP traffic
- **Authentication**: Password protect your tunnels

## Basic Usage

```bash
# Expose HTTP service on port 3000
ngrok http 3000

# Expose with custom subdomain (requires paid plan)
ngrok http --subdomain=myapp 8080

# Expose HTTPS service
ngrok http https://localhost:8443

# Expose TCP service
ngrok tcp 22

# Start with authentication
ngrok http --auth="username:password" 8080

# Use configuration file
ngrok start myapp
```

## Advanced Configuration

```yaml
# Install ngrok
- preset: ngrok

# Configure authentication token
- name: Setup ngrok auth token
  shell: ngrok config add-authtoken {{ ngrok_token }}

# Create ngrok configuration
- name: Deploy ngrok config
  template:
    src_template: ngrok.yml.j2
    dest: ~/.config/ngrok/ngrok.yml
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove ngrok |

## Platform Support

- ✅ Linux (binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Windows (binary install)

## Configuration

- **Config file**: `~/.config/ngrok/ngrok.yml`
- **Auth token**: Required for advanced features (get from https://dashboard.ngrok.com/)
- **Web UI**: http://localhost:4040 (when tunnel is running)

## Real-World Examples

### Webhook Development
```bash
# Start local dev server
npm run dev &

# Expose to internet for webhook testing
ngrok http 3000

# Use the generated URL in webhook config
# https://abc123.ngrok-free.app
```

### Mobile App Testing
```bash
# Expose local API server
ngrok http 8080

# Use ngrok URL in mobile app:
# https://xyz789.ngrok-free.app/api
```

### Configuration File
```yaml
# ~/.config/ngrok/ngrok.yml
version: "2"
authtoken: YOUR_AUTH_TOKEN

tunnels:
  website:
    proto: http
    addr: 8080
    subdomain: myapp

  api:
    proto: http
    addr: 3000
    auth: "user:password"
    inspect: true

  ssh:
    proto: tcp
    addr: 22
```

### CI/CD Integration
```yaml
# Test webhooks in CI/CD
- name: Install ngrok
  preset: ngrok

- name: Configure ngrok
  shell: ngrok config add-authtoken {{ ngrok_token }}

- name: Start application
  shell: npm start
  async: true
  cwd: /app

- name: Start ngrok tunnel
  shell: ngrok http 3000 --log=stdout > ngrok.log &
  async: true

- name: Wait for tunnel
  shell: sleep 5

- name: Get tunnel URL
  shell: curl -s http://localhost:4040/api/tunnels | jq -r '.tunnels[0].public_url'
  register: tunnel_url

- name: Configure webhook
  shell: |
    curl -X POST https://api.service.com/webhooks \
      -d '{"url": "{{ tunnel_url.stdout }}/webhook"}'

- name: Run integration tests
  shell: npm test
  cwd: /app
```

### Demo Environment
```bash
# Start multiple services
ngrok start website api --config=~/.config/ngrok/ngrok.yml

# Access:
# https://myapp.ngrok-free.app -> website
# https://myapp-api.ngrok-free.app -> API
```

### Secure Remote Access
```bash
# Expose SSH
ngrok tcp 22

# Connect from anywhere:
# ssh user@0.tcp.ngrok.io -p 12345

# Expose database temporarily
ngrok tcp 5432  # PostgreSQL

# Connect: psql -h 0.tcp.ngrok.io -p 54321 -U user db
```

## Request Inspector

When ngrok is running, access the web UI at http://localhost:4040:

- **View all requests**: See headers, body, timing
- **Replay requests**: Resend webhooks for testing
- **Filter requests**: Search by path, status, method
- **Export requests**: Save for debugging

```bash
# Open inspector in browser
open http://localhost:4040

# Or access API
curl http://localhost:4040/api/tunnels
```

## Common Use Cases

### Webhook Testing
```bash
# Stripe webhooks
ngrok http 3000
# Use https://xyz.ngrok-free.app/webhook in Stripe dashboard

# GitHub webhooks
ngrok http --subdomain=myrepo 8080
# Configure in GitHub: https://myrepo.ngrok-free.app/github
```

### Client Demos
```bash
# Show work-in-progress to clients
ngrok http 3000 --auth="demo:pass123"
# Share: https://abc.ngrok-free.app (protected)
```

### Mobile Development
```bash
# Test mobile app against local backend
ngrok http 8080
# Use in mobile app: https://xyz.ngrok-free.app
```

## Agent Use

- Test webhooks from third-party services during development
- Create temporary public endpoints for integration testing
- Enable remote debugging of local applications
- Provide demo environments without deploying to staging
- Test mobile applications against local backend services
- Share development progress with distributed teams

## Troubleshooting

### Authentication required
```bash
# Get auth token from https://dashboard.ngrok.com/
ngrok config add-authtoken YOUR_TOKEN

# Verify configuration
cat ~/.config/ngrok/ngrok.yml
```

### Port already in use
```bash
# ngrok web UI port conflict
ngrok http 3000 --web-port=4041

# Multiple tunnels
ngrok start tunnel1 tunnel2 --config=ngrok.yml
```

### Tunnel not accessible
```bash
# Check tunnel status
curl http://localhost:4040/api/tunnels | jq

# Verify local service is running
curl http://localhost:3000

# Check firewall rules
sudo ufw status  # Linux
```

## Uninstall

```yaml
- preset: ngrok
  with:
    state: absent
```

## Resources

- Official docs: https://ngrok.com/docs
- Dashboard: https://dashboard.ngrok.com/
- Pricing: https://ngrok.com/pricing
- GitHub: https://github.com/ngrok/ngrok
- Search: "ngrok webhook testing", "ngrok tutorial", "ngrok alternatives"

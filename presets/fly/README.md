# Fly CLI - Global App Deployment

Deploy applications globally on Fly.io edge platform. Run apps close to users with automatic SSL, scaling, and zero-config deployments.

## Quick Start
```yaml
- preset: fly
```

## Features
- **Global deployment**: Deploy to 30+ regions worldwide in seconds
- **Edge computing**: Run apps close to users for low latency
- **Automatic SSL**: Free SSL certificates via Let's Encrypt
- **Zero-config Docker**: Deploy from Dockerfile with no configuration
- **Live scaling**: Scale to zero or thousands of instances automatically

## Basic Usage
```bash
# Authenticate
flyctl auth login

# Launch new app
flyctl launch

# Deploy application
flyctl deploy

# Open app in browser
flyctl open

# View logs
flyctl logs

# Check app status
flyctl status

# Scale app
flyctl scale count 3

# List apps
flyctl apps list
```

## Advanced Configuration
```yaml
- preset: fly
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Fly CLI |

## Platform Support
- ✅ Linux (shell script installer)
- ✅ macOS (Homebrew)
- ✅ Windows (PowerShell installer)

## Configuration
- **Config file**: `~/.fly/config.yml`
- **API token**: Stored in config after `flyctl auth login`
- **Project config**: `fly.toml` in project directory

## Real-World Examples

### Deploy Node.js App
```bash
# Create fly.toml
flyctl launch --name my-app --region sea

# Deploy
flyctl deploy

# Scale based on traffic
flyctl autoscale standard min=1 max=10
```

### Deploy with Environment Variables
```bash
# Set secrets
flyctl secrets set DATABASE_URL="postgres://..."
flyctl secrets set API_KEY="secret-key"

# Deploy
flyctl deploy
```

### Multi-Region Deployment
```yaml
# CI/CD pipeline
- name: Install Fly CLI
  preset: fly

- name: Deploy to multiple regions
  shell: |
    flyctl auth token ${{ secrets.FLY_API_TOKEN }}
    flyctl scale count 2 --region sea
    flyctl scale count 2 --region fra
    flyctl deploy
```

### Database with Fly Postgres
```bash
# Create Postgres cluster
flyctl postgres create --name my-db

# Attach to app
flyctl postgres attach my-db --app my-app

# Connect
flyctl postgres connect my-db
```

## Agent Use
- Deploy applications to edge locations from CI/CD pipelines
- Automate multi-region deployments for global applications
- Manage application scaling based on traffic patterns
- Configure and deploy microservices architectures
- Set up staging and production environments
- Monitor application health and logs programmatically

## Troubleshooting

### Authentication failed
```bash
# Re-authenticate
flyctl auth login

# Use API token
flyctl auth token $FLY_API_TOKEN

# Check authentication
flyctl auth whoami
```

### Deployment fails
```bash
# Check build logs
flyctl logs

# Validate fly.toml
flyctl config validate

# Force rebuild
flyctl deploy --build-only
```

### App not accessible
```bash
# Check health
flyctl status

# View recent logs
flyctl logs --limit 100

# Restart app
flyctl apps restart my-app
```

### Certificate issues
```bash
# Check certificates
flyctl certs list

# Add certificate
flyctl certs add example.com

# Check certificate validation
flyctl certs show example.com
```

## Uninstall
```yaml
- preset: fly
  with:
    state: absent
```

## Resources
- Official docs: https://fly.io/docs/
- GitHub: https://github.com/superfly/flyctl
- Community: https://community.fly.io/
- Pricing: https://fly.io/docs/about/pricing/
- Search: "fly.io deployment", "flyctl tutorial", "fly.io postgres"

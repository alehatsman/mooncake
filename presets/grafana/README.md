# Grafana Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Access web UI
open http://localhost:3000  # macOS
xdg-open http://localhost:3000  # Linux

# Default credentials
Username: admin
Password: admin  # (change on first login)
```

## Configuration

- **Config file:** `/etc/grafana/grafana.ini` (Linux), `/usr/local/etc/grafana/grafana.ini` (macOS)
- **Data directory:** `/var/lib/grafana`
- **Web UI port:** 3000 (default)

## Common Operations

```bash
# Restart Grafana
sudo systemctl restart grafana-server  # Linux
brew services restart grafana  # macOS

# Check health
curl http://localhost:3000/api/health

# Reset admin password
grafana-cli admin reset-admin-password newpassword
```

## Adding Data Sources

1. Navigate to Configuration → Data Sources
2. Click "Add data source"
3. Common sources:
   - Prometheus: `http://localhost:9090`
   - MySQL: `localhost:3306`
   - PostgreSQL: `localhost:5432`

## Creating Dashboards

1. Click "+" → Dashboard
2. Add Panel → Choose visualization
3. Select data source and write query
4. Save dashboard

## API Usage

```bash
# List dashboards
curl -u admin:admin http://localhost:3000/api/search

# Create API key
curl -u admin:admin -X POST \
  http://localhost:3000/api/auth/keys \
  -H "Content-Type: application/json" \
  -d '{"name":"mykey","role":"Admin"}'
```

## Import Dashboards

```bash
# Import from grafana.com (e.g., Node Exporter dashboard)
Dashboard ID: 1860
```

## Uninstall

```yaml
- preset: grafana
  with:
    state: absent
```

**Note:** Configuration and data in `/var/lib/grafana` preserved after uninstall.

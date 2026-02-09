# Grafana - Data Visualization and Monitoring

Open-source analytics and interactive visualization platform. Query, visualize, alert on, and understand your metrics from Prometheus, Elasticsearch, Loki, and 100+ data sources.

## Quick Start
```yaml
- preset: grafana
```

Access web UI: `http://localhost:3000`
**Default credentials**: admin / admin (change on first login)

## Features
- **100+ data sources**: Prometheus, Elasticsearch, MySQL, Postgres, Loki, InfluxDB, and more
- **Beautiful dashboards**: Interactive visualizations with dozens of panel types
- **Alerting**: Define alerts and send notifications to Slack, PagerDuty, email, etc.
- **Provisioning**: Auto-configure datasources and dashboards via YAML
- **Plugins**: Extend with community panels, datasources, and apps
- **Authentication**: Support for OAuth, LDAP, SAML, and more
- **Organizations**: Multi-tenancy with teams and permissions
- **Variables**: Dynamic dashboards with template variables
- **Annotations**: Mark events on graphs (deployments, incidents)

## Basic Usage
```bash
# Access web UI
open http://localhost:3000  # macOS
xdg-open http://localhost:3000  # Linux

# Check status
curl http://localhost:3000/api/health

# Service management
sudo systemctl status grafana-server  # Linux
brew services info grafana  # macOS

# Restart service
sudo systemctl restart grafana-server  # Linux
brew services restart grafana  # macOS
```

## Advanced Configuration

### Install Grafana
```yaml
- preset: grafana
  with:
    state: present
```

### With Prometheus datasource
```yaml
- name: Install monitoring stack
  preset: prometheus

- name: Install Grafana
  preset: grafana

- name: Wait for Grafana to start
  shell: |
    for i in {1..30}; do
      curl -s http://localhost:3000/api/health && break
      sleep 1
    done

- name: Configure Prometheus datasource
  shell: |
    curl -X POST http://admin:admin@localhost:3000/api/datasources \
      -H "Content-Type: application/json" \
      -d '{
        "name": "Prometheus",
        "type": "prometheus",
        "url": "http://localhost:9090",
        "access": "proxy",
        "isDefault": true
      }'
```

### Production deployment
```yaml
- name: Install Grafana
  preset: grafana
  become: true

- name: Configure Grafana
  template:
    src: grafana.ini.j2
    dest: /etc/grafana/grafana.ini
    mode: '0640'
  become: true

- name: Restart Grafana
  service:
    name: grafana-server
    state: restarted
  become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Grafana |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman) - systemd service
- ✅ macOS (Homebrew) - launchd service
- ❌ Windows (use official installer)

## Configuration

### File locations
- **Config**: `/etc/grafana/grafana.ini` (Linux), `/usr/local/etc/grafana/grafana.ini` (macOS)
- **Data**: `/var/lib/grafana`
- **Logs**: `/var/log/grafana`
- **Plugins**: `/var/lib/grafana/plugins`
- **Provisioning**: `/etc/grafana/provisioning/`

### grafana.ini key settings
```ini
[server]
protocol = http
http_addr = 0.0.0.0
http_port = 3000
domain = grafana.example.com
root_url = https://grafana.example.com/

[database]
type = postgres
host = localhost:5432
name = grafana
user = grafana
password = secret

[auth]
disable_login_form = false

[auth.anonymous]
enabled = false

[smtp]
enabled = true
host = smtp.gmail.com:587
user = alerts@example.com
password = secret
from_address = alerts@example.com
```

## Datasource Provisioning

### /etc/grafana/provisioning/datasources/prometheus.yml
```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://localhost:9090
    isDefault: true
    jsonData:
      timeInterval: 30s
      httpMethod: POST
    editable: false

  - name: Loki
    type: loki
    access: proxy
    url: http://localhost:3100
    jsonData:
      maxLines: 1000

  - name: Postgres
    type: postgres
    url: localhost:5432
    database: myapp
    user: grafana
    secureJsonData:
      password: 'secret'
    jsonData:
      sslmode: 'disable'
      maxOpenConns: 100
      maxIdleConns: 100
      connMaxLifetime: 14400
```

### Create via mooncake
```yaml
- name: Provision datasources
  template:
    src: datasources.yml.j2
    dest: /etc/grafana/provisioning/datasources/default.yml
    mode: '0644'
  become: true

- name: Restart Grafana
  service:
    name: grafana-server
    state: restarted
  become: true
```

## Dashboard Provisioning

### /etc/grafana/provisioning/dashboards/default.yml
```yaml
apiVersion: 1

providers:
  - name: 'Default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: true
```

### Import dashboard JSON
```yaml
- name: Create dashboard directory
  file:
    path: /var/lib/grafana/dashboards
    state: directory
    owner: grafana
    group: grafana
    mode: '0755'
  become: true

- name: Copy dashboard
  copy:
    src: node-exporter-dashboard.json
    dest: /var/lib/grafana/dashboards/node-exporter.json
    owner: grafana
    group: grafana
    mode: '0644'
  become: true
```

## API Usage

### Authentication
```bash
# Using credentials
curl -u admin:admin http://localhost:3000/api/health

# Create API key
API_KEY=$(curl -u admin:admin -X POST \
  http://localhost:3000/api/auth/keys \
  -H "Content-Type: application/json" \
  -d '{"name":"automation","role":"Admin"}' | jq -r '.key')

# Use API key
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/api/dashboards/home
```

### Common operations
```bash
# List dashboards
curl -u admin:admin http://localhost:3000/api/search

# Get dashboard by UID
curl -u admin:admin http://localhost:3000/api/dashboards/uid/abc123

# Create organization
curl -u admin:admin -X POST \
  http://localhost:3000/api/orgs \
  -H "Content-Type: application/json" \
  -d '{"name":"My Org"}'

# Create user
curl -u admin:admin -X POST \
  http://localhost:3000/api/admin/users \
  -H "Content-Type: application/json" \
  -d '{"name":"User","login":"user","email":"user@example.com","password":"secret"}'

# Create team
curl -u admin:admin -X POST \
  http://localhost:3000/api/teams \
  -H "Content-Type: application/json" \
  -d '{"name":"Engineering","email":"eng@example.com"}'
```

## Plugin Management

### Install plugins
```bash
# Install via CLI
grafana-cli plugins install grafana-clock-panel
grafana-cli plugins install grafana-piechart-panel
grafana-cli plugins install grafana-worldmap-panel

# Restart after plugin install
sudo systemctl restart grafana-server

# List installed plugins
grafana-cli plugins ls

# Update plugin
grafana-cli plugins update grafana-clock-panel

# Remove plugin
grafana-cli plugins remove grafana-clock-panel
```

### Popular plugins
```bash
# Visualization
grafana-cli plugins install grafana-piechart-panel
grafana-cli plugins install grafana-polystat-panel
grafana-cli plugins install grafana-worldmap-panel

# Datasources
grafana-cli plugins install grafana-googlesheets-datasource
grafana-cli plugins install grafana-simple-json-datasource

# Apps
grafana-cli plugins install grafana-kubernetes-app
```

## Alerting

### Alert via UI
1. Edit dashboard panel
2. Alert tab → Create Alert
3. Define conditions (e.g., "WHEN avg() OF query(A, 5m) IS ABOVE 80")
4. Configure notifications
5. Save dashboard

### Alert via provisioning
```yaml
# /etc/grafana/provisioning/alerting/alerts.yml
apiVersion: 1

groups:
  - name: system_alerts
    interval: 1m
    rules:
      - uid: cpu_alert
        title: High CPU Usage
        condition: A
        data:
          - refId: A
            queryType: ''
            relativeTimeRange:
              from: 600
              to: 0
            datasourceUid: prometheus_uid
            model:
              expr: 'avg(cpu_usage) > 80'
        noDataState: NoData
        execErrState: Error
        for: 5m
        annotations:
          description: 'CPU usage is above 80%'
        labels:
          severity: warning
```

### Notification channels
```bash
# Create Slack channel
curl -u admin:admin -X POST \
  http://localhost:3000/api/alert-notifications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Slack",
    "type": "slack",
    "isDefault": true,
    "settings": {
      "url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
      "uploadImage": true
    }
  }'

# Create email channel
curl -u admin:admin -X POST \
  http://localhost:3000/api/alert-notifications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Email",
    "type": "email",
    "settings": {
      "addresses": "ops@example.com"
    }
  }'
```

## Use Cases

### Complete Observability Stack
```yaml
- name: Install Prometheus
  preset: prometheus

- name: Install Loki
  preset: loki-server

- name: Install Grafana
  preset: grafana

- name: Provision datasources
  template:
    src: datasources.yml.j2
    dest: /etc/grafana/provisioning/datasources/observability.yml
  become: true

- name: Import dashboards
  copy:
    src: "{{ item }}"
    dest: /var/lib/grafana/dashboards/
  loop:
    - node-exporter.json
    - loki-logs.json
    - application-metrics.json
  become: true

- name: Restart Grafana
  service:
    name: grafana-server
    state: restarted
  become: true
```

### Multi-Tenant Setup
```yaml
- name: Install Grafana
  preset: grafana
  become: true

- name: Create organizations
  shell: |
    for org in TeamA TeamB TeamC; do
      curl -u admin:admin -X POST \
        http://localhost:3000/api/orgs \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"$org\"}"
    done

- name: Create users and assign to orgs
  shell: |
    # Script to create users and add to teams
    # (details omitted for brevity)
```

### Automated Dashboard Backups
```yaml
- name: Backup Grafana dashboards
  shell: |
    mkdir -p /backup/grafana/$(date +%Y-%m-%d)
    for dash in $(curl -s -u admin:admin \
      http://localhost:3000/api/search | jq -r '.[].uid'); do
      curl -s -u admin:admin \
        "http://localhost:3000/api/dashboards/uid/$dash" \
        > "/backup/grafana/$(date +%Y-%m-%d)/$dash.json"
    done
  when: backup_enabled
```

## Dashboard Best Practices
- **Use folders**: Organize dashboards by team/service
- **Template variables**: Make dashboards reusable ($instance, $namespace)
- **Consistent naming**: Follow naming conventions for clarity
- **Row repeaters**: Dynamically create rows based on variables
- **Mixed datasources**: Combine metrics, logs, and traces in one view
- **Links**: Add links between related dashboards
- **Annotations**: Mark deployments and incidents
- **Time ranges**: Set appropriate default time ranges

## Template Variables

### Define variables
```
Name: instance
Type: Query
Label: Instance
Datasource: Prometheus
Query: label_values(up, instance)
```

### Use in queries
```promql
rate(http_requests_total{instance="$instance"}[5m])
```

### Use in panel titles
```
HTTP Requests - $instance
```

## Common Dashboards

### Import from grafana.com
```
Node Exporter Full: 1860
Kubernetes Cluster: 7249
Loki Dashboard: 12019
Nginx: 12708
PostgreSQL: 9628
```

### Import via UI
1. Dashboard → Import
2. Enter dashboard ID (e.g., 1860)
3. Select Prometheus datasource
4. Click Import

### Import via API
```bash
curl -u admin:admin -X POST \
  http://localhost:3000/api/dashboards/import \
  -H "Content-Type: application/json" \
  -d '{
    "dashboard": {
      "id": null,
      "uid": null,
      "title": "Node Exporter",
      "tags": ["prometheus"],
      "timezone": "browser"
    },
    "folderId": 0,
    "overwrite": false
  }'
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install Grafana
  preset: grafana
```

### With provisioning
```yaml
- name: Install Grafana
  preset: grafana
  become: true

- name: Provision datasources
  template:
    src: datasources.yml.j2
    dest: /etc/grafana/provisioning/datasources/default.yml
    mode: '0644'
  become: true

- name: Provision dashboards
  copy:
    src: dashboards/
    dest: /var/lib/grafana/dashboards/
    owner: grafana
    group: grafana
  become: true

- name: Restart Grafana
  service:
    name: grafana-server
    state: restarted
  become: true
```

### Complete monitoring stack
```yaml
- name: Setup monitoring
  hosts: monitoring
  tasks:
    - preset: prometheus
    - preset: grafana

    - name: Configure Prometheus datasource
      shell: |
        curl -X POST http://admin:admin@localhost:3000/api/datasources \
          -H "Content-Type: application/json" \
          -d '{
            "name": "Prometheus",
            "type": "prometheus",
            "url": "http://localhost:9090",
            "access": "proxy",
            "isDefault": true
          }'
```

## Agent Use
- **Observability setup**: Automated monitoring stack deployment
- **Dashboard provisioning**: Version-controlled dashboards as code
- **Multi-tenant configuration**: Automated org/team/user setup
- **Datasource management**: Standardized datasource configurations
- **Alert configuration**: Consistent alerting across environments
- **Backup automation**: Scheduled dashboard and configuration backups
- **CI/CD integration**: Deploy and configure Grafana in pipelines

## Troubleshooting

### Cannot connect to datasource
```bash
# Test datasource connectivity
curl -v http://localhost:9090  # Prometheus
curl -v http://localhost:3100  # Loki

# Check Grafana logs
sudo journalctl -u grafana-server -f  # Linux
tail -f /usr/local/var/log/grafana/grafana.log  # macOS

# Test from Grafana server
docker exec grafana curl http://prometheus:9090
```

### Permission denied errors
```bash
# Check ownership
ls -la /var/lib/grafana
ls -la /etc/grafana

# Fix permissions
sudo chown -R grafana:grafana /var/lib/grafana
sudo chown -R grafana:grafana /etc/grafana
```

### Plugin not loading
```bash
# Restart after plugin install
sudo systemctl restart grafana-server

# Check plugin directory
ls -la /var/lib/grafana/plugins/

# Enable plugin in grafana.ini
[plugins]
allow_loading_unsigned_plugins = your-plugin-id
```

### Reset admin password
```bash
# Reset via CLI
grafana-cli admin reset-admin-password newpassword

# Or via SQLite (if using default DB)
sudo sqlite3 /var/lib/grafana/grafana.db \
  "UPDATE user SET password = 'hash', salt = 'salt' WHERE login = 'admin'"
```

### Dashboard not saving
```bash
# Check disk space
df -h /var/lib/grafana

# Check database
sudo sqlite3 /var/lib/grafana/grafana.db ".tables"

# Check permissions
ls -la /var/lib/grafana/grafana.db
```

### High memory usage
```bash
# Limit concurrent queries in grafana.ini
[data_proxy]
max_idle_connections = 100
max_idle_connections_per_host = 10

# Reduce retention in datasources
# Configure query timeout
[dataproxy]
timeout = 30
```

## Security

### Change default credentials
```bash
# First login: you'll be prompted to change password
# Or reset via CLI:
grafana-cli admin reset-admin-password <new-password>
```

### Enable HTTPS
```ini
# grafana.ini
[server]
protocol = https
cert_file = /etc/grafana/ssl/cert.pem
cert_key = /etc/grafana/ssl/key.pem
```

### Configure authentication
```ini
# OAuth (GitHub)
[auth.github]
enabled = true
allow_sign_up = true
client_id = YOUR_CLIENT_ID
client_secret = YOUR_CLIENT_SECRET
scopes = user:email,read:org
auth_url = https://github.com/login/oauth/authorize
token_url = https://github.com/login/oauth/access_token
api_url = https://api.github.com/user
allowed_organizations = myorg
```

### Disable anonymous access
```ini
[auth.anonymous]
enabled = false
```

## Uninstall
```yaml
- preset: grafana
  with:
    state: absent
```

**Note**: Configuration and data in `/var/lib/grafana` are preserved. Remove manually if needed.

## Resources
- Official: https://grafana.com/
- Documentation: https://grafana.com/docs/
- Tutorials: https://grafana.com/tutorials/
- Dashboards: https://grafana.com/grafana/dashboards/
- Plugins: https://grafana.com/grafana/plugins/
- Community: https://community.grafana.com/
- GitHub: https://github.com/grafana/grafana
- Search: "grafana tutorial", "grafana dashboard", "grafana provisioning"

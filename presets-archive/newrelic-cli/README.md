# New Relic CLI - Observability Platform Command Line

Command-line interface for New Relic observability platform to manage applications, infrastructure, and alerts.

## Quick Start

```yaml
- preset: newrelic-cli
```

## Features

- **Profile management**: Configure multiple New Relic accounts
- **Entity operations**: Query and manage monitored entities
- **NRQL queries**: Run queries against New Relic database
- **APM management**: Control application monitoring settings
- **Alert management**: Create and manage alert policies
- **Dashboard operations**: Deploy and update dashboards
- **JSON output**: Machine-readable output for automation

## Basic Usage

```bash
# Check version
newrelic version

# Configure profile
newrelic profile add --profile production --apiKey YOUR_API_KEY --region us

# List profiles
newrelic profile list

# Search entities
newrelic entity search --name myapp

# Run NRQL query
newrelic nrql query --accountId 12345 --query "SELECT * FROM Transaction WHERE appName = 'myapp'"

# Get entity details
newrelic entity get --guid ENTITY_GUID
```

## Advanced Configuration

```yaml
# Install New Relic CLI
- preset: newrelic-cli

# Configure profile from variables
- name: Setup New Relic profile
  shell: |
    newrelic profile add \
      --profile {{ environment }} \
      --apiKey {{ newrelic_api_key }} \
      --region {{ newrelic_region }}
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove New Relic CLI |

## Platform Support

- ✅ Linux (apt, yum, dnf, binary install)
- ✅ macOS (Homebrew, binary install)
- ✅ Windows (MSI installer, binary install)

## Configuration

- **Config file**: `~/.newrelic/newrelic-cli.json`
- **Profiles**: Multiple account configurations supported
- **Environment variables**: `NEW_RELIC_API_KEY`, `NEW_RELIC_REGION`

## Real-World Examples

### CI/CD Deployment Marker
```bash
# Record deployment in New Relic
newrelic apm deployment create \
  --applicationId 12345 \
  --revision ${GIT_COMMIT} \
  --user "${CI_USER}" \
  --description "Deploy to production"
```

### Query Application Performance
```bash
# Get error rate for last hour
newrelic nrql query --accountId 12345 --query "
  SELECT percentage(count(*), WHERE error IS true)
  FROM Transaction
  WHERE appName = 'myapp'
  SINCE 1 hour ago
"
```

### Automated Alert Management
```yaml
# Deploy alert policy via Mooncake
- name: Query existing alert policy
  shell: |
    newrelic alerts policy list --name "Production Alerts" --output json
  register: policy_check

- name: Create alert policy if missing
  shell: |
    newrelic alerts policy create \
      --name "Production Alerts" \
      --incidentPreference "PER_POLICY"
  when: policy_check.stdout | from_json | length == 0
```

### Infrastructure Monitoring
```bash
# Search for hosts by tag
newrelic entity search --type HOST --tag "environment:production"

# Get host metrics
newrelic nrql query --accountId 12345 --query "
  SELECT average(cpuPercent), average(memoryUsedPercent)
  FROM SystemSample
  WHERE hostname = 'web-01'
  SINCE 30 minutes ago
  TIMESERIES
"
```

### Dashboard Deployment
```bash
# Export dashboard as JSON
newrelic dashboard get --guid DASHBOARD_GUID --output json > dashboard.json

# Create dashboard from JSON
newrelic dashboard create --dashboard dashboard.json
```

## Common Commands

### Entity Management
```bash
# Search entities
newrelic entity search --query "type = 'APPLICATION'"

# Tag entity
newrelic entity tags create --guid ENTITY_GUID --tag "env:production"

# Delete tag
newrelic entity tags delete --guid ENTITY_GUID --tag "env:staging"
```

### NRQL Queries
```bash
# Basic query
newrelic nrql query --accountId 12345 --query "SELECT count(*) FROM Transaction"

# Query with time range
newrelic nrql query --accountId 12345 --query "
  SELECT average(duration)
  FROM Transaction
  SINCE 1 day ago
  UNTIL 1 hour ago
"

# Export to JSON
newrelic nrql query --accountId 12345 --query "..." --output json > results.json
```

### APM Operations
```bash
# List applications
newrelic apm application list

# Get application details
newrelic apm application get --applicationId 12345

# Record deployment
newrelic apm deployment create --applicationId 12345 --revision v1.2.3
```

## Agent Use

- Record deployments in CI/CD pipelines automatically
- Query application metrics for automated health checks
- Create and update alert policies as code
- Export and version control dashboards
- Automate entity tagging based on infrastructure state
- Generate performance reports in automated workflows
- Integrate with incident management systems

## Troubleshooting

### Authentication errors
```bash
# Verify API key
newrelic profile list

# Test connection
newrelic entity search --query "type = 'APPLICATION'" --limit 1

# Set API key via environment
export NEW_RELIC_API_KEY=your_key_here
newrelic entity search --query "type = 'APPLICATION'"
```

### Region configuration
```bash
# US region (default)
newrelic profile add --profile us --apiKey KEY --region us

# EU region
newrelic profile add --profile eu --apiKey KEY --region eu

# Use specific profile
newrelic entity search --profile eu --query "..."
```

## Uninstall

```yaml
- preset: newrelic-cli
  with:
    state: absent
```

## Resources

- Official docs: https://docs.newrelic.com/docs/new-relic-cli/
- GitHub: https://github.com/newrelic/newrelic-cli
- NRQL reference: https://docs.newrelic.com/docs/nrql/
- Search: "newrelic cli tutorial", "newrelic nrql examples", "newrelic cli automation"

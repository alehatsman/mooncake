# Meltano - Open-Source ELT Data Integration Platform

Build, deploy, and manage data pipelines for Extract, Load, Transform workflows without vendor lock-in.

## Quick Start

```yaml
- preset: meltano
```

## Features

- **Singer Taps & Targets**: Access 400+ pre-built data connectors for sources and destinations
- **ELT Pipelines**: Define complex data integration workflows with dbt transformations
- **Orchestration**: Schedule and execute pipelines using Airflow or native scheduler
- **Version Control**: Commit your entire data stack (taps, targets, loaders) to git
- **Cross-platform**: Linux, macOS with Python virtual environment isolation
- **Plugin Ecosystem**: Extend with custom extractors, loaders, and transformers
- **Local Development**: Test pipelines locally before deploying to production

## Basic Usage

```bash
# Check version
meltano --version

# Get help on any command
meltano --help

# Initialize a new meltano project
meltano init my_project

# List available Singer taps (data sources)
meltano discovery select

# Add a tap to your project
meltano add extractor tap-postgres

# Add a target (destination)
meltano add loader target-postgres

# Run an extract and load
meltano run tap-postgres target-postgres

# View configuration
meltano config list

# Test database connection
meltano invoke tap-postgres test
```

## Advanced Configuration

```yaml
# Install with custom home directory for meltano projects
- preset: meltano
  with:
    state: present

# Uninstall meltano
- preset: meltano
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove meltano |

## Configuration

- **Meltano Home**: `~/.local/share/meltano/` (Linux), `~/Library/Application Support/Meltano/` (macOS)
- **Projects Directory**: Convention is `~/meltano-projects/`
- **Python**: Requires Python 3.7+
- **Virtual Environment**: Meltano creates isolated Python environments for each project

## Platform Support

- ✅ Linux (pip3, requires Python 3.7+)
- ✅ macOS (Homebrew, requires Python 3.7+)
- ❌ Windows (not yet supported)

## Real-World Examples

### PostgreSQL to Snowflake Data Pipeline

Extract from PostgreSQL and load into Snowflake for analytics:

```bash
# Initialize project
meltano init etl_postgres_snowflake

# Add Singer tap for PostgreSQL
meltano add extractor tap-postgres

# Configure PostgreSQL connection
meltano config tap-postgres set host mydb.example.com
meltano config tap-postgres set user postgres
meltano config tap-postgres set password mypassword
meltano config tap-postgres set database myapp_db

# Add Snowflake target
meltano add loader target-snowflake

# Configure Snowflake connection
meltano config target-snowflake set account xy12345
meltano config target-snowflake set username etl_user
meltano config target-snowflake set password mypassword
meltano config target-snowflake set database analytics

# Run the pipeline
meltano run tap-postgres target-snowflake
```

### API to Data Warehouse with Transformation

Extract from REST API, load to Postgres, and transform with dbt:

```bash
meltano init etl_api_warehouse

# Add tap for a REST API (e.g., GitHub)
meltano add extractor tap-github

# Configure API
meltano config tap-github set organizations myorg
meltano config tap-github set repositories myrepo

# Add Postgres target
meltano add loader target-postgres

# Configure target
meltano config target-postgres set host localhost
meltano config target-postgres set database warehouse

# Add dbt for transformations
meltano add transformer dbt-postgres

# Define transformation models in dbt
echo "select * from tap_github_repositories" > transform/models/repositories.sql

# Run full pipeline: extract, load, transform
meltano run tap-github target-postgres dbt-postgres:run
```

### CI/CD Integration

Orchestrate pipelines in your deployment workflow:

```yaml
# deployment.yml - Deploy meltano and schedule pipelines
- name: Deploy meltano project
  preset: meltano
  become: true

- name: Install project dependencies
  shell: |
    cd ~/meltano-projects/analytics
    meltano install
    meltano invoke dbt:deps

- name: Verify tap connection
  shell: meltano invoke tap-postgres test

- name: Run full pipeline
  shell: meltano run tap-postgres target-postgres dbt-postgres:run
```

## Agent Use

- **Data Pipeline Orchestration**: Deploy ELT workflows as part of infrastructure provisioning
- **Data Integration Verification**: Test data source connectivity before deploying analytics infrastructure
- **Analytics Platform Setup**: Automate the setup of complete data stacks (extract, load, transform)
- **Multi-Environment Pipelines**: Deploy pipelines to dev, staging, and production environments
- **Data Quality Validation**: Integrate assertions and tests into automated data pipelines
- **Migration Automation**: Migrate data between sources programmatically during infrastructure changes

## Troubleshooting

### Python version mismatch

Meltano requires Python 3.7 or higher:

```bash
# Check Python version
python3 --version

# Install Python if needed (macOS)
brew install python@3.11

# For Linux, use package manager
apt-get install python3.11  # Ubuntu/Debian
dnf install python3.11      # Fedora
```

### Tap or target not found

When adding taps/targets, ensure the name is correct:

```bash
# Search for available taps
meltano discovery select

# View documentation
meltano invoke tap-name about
```

### Permission denied during install

If installing to system directories, use `become: true` in the preset invocation.

### Module not found errors

Install project dependencies:

```bash
cd ~/.local/share/meltano/my-project
meltano install
```

### Pipeline execution fails

Check logs for detailed error information:

```bash
# Run with verbose output
meltano run --verbose tap-postgres target-postgres

# Check specific tap
meltano invoke tap-postgres test
```

## Uninstall

```yaml
- preset: meltano
  with:
    state: absent
```

This will remove the meltano binary but preserve existing meltano projects in `~/.local/share/meltano/`.

## Resources

- **Official Documentation**: https://meltano.com/docs/
- **Singer Specification**: https://singer.io/ (the protocol Meltano uses)
- **Available Connectors**: https://hub.meltano.com/
- **GitHub Repository**: https://github.com/meltanolabs/meltano
- **Community Slack**: https://meltano.com/slack
- **Search**: "meltano ELT tutorial", "meltano Singer taps", "meltano dbt integration", "meltano Airflow orchestration"

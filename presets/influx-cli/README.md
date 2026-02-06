# influx-cli - InfluxDB Command-Line Client

Command-line interface for interacting with InfluxDB servers. Query, write, and manage InfluxDB from the terminal.

## Quick Start
```yaml
- preset: influx-cli
```

## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`

## Basic Usage
```bash
# Check version
influx version

# Connect to InfluxDB
influx -host localhost -port 8086

# Execute a query
influx -execute 'SHOW DATABASES'

# Write data
influx -execute 'CREATE DATABASE mydb'

# Query data
influx -database mydb -execute 'SELECT * FROM measurements'

# Import data
influx -import -path=data.txt -precision=s

# Export data
influx -database mydb -execute 'SELECT * FROM measurements' -format csv > export.csv
```

## Advanced Configuration
```yaml
- preset: influx-cli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove influx-cli |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration
- **Config file**: `~/.influxdbv2/configs` (InfluxDB 2.x)
- **Default host**: `localhost:8086`
- **Authentication**: Token-based or username/password

## Real-World Examples

### CI/CD Health Checks
```bash
# Check if InfluxDB is responsive
influx ping

# Verify database exists before deployment
influx -execute 'SHOW DATABASES' | grep -q myapp_db
```

### Data Migration
```bash
# Export from source
influx -host source.example.com -database mydb \
  -execute 'SELECT * FROM measurements' \
  -format csv > data.csv

# Import to destination
influx -host dest.example.com -database mydb \
  -import -path=data.csv
```

## Agent Use
- Automated database health checks
- CI/CD pipeline data validation
- Backup and restore automation
- Database provisioning
- Query execution in deployment scripts

## Uninstall
```yaml
- preset: influx-cli
  with:
    state: absent
```

## Resources
- Official docs: https://docs.influxdata.com/influxdb/
- CLI reference: https://docs.influxdata.com/influxdb/v2/reference/cli/influx/
- Search: "influx cli tutorial", "influxdb command line"

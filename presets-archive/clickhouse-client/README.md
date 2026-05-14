# ClickHouse Client - Database CLI

Command-line client for ClickHouse, a fast open-source column-oriented OLAP database management system.

## Quick Start
```yaml
- preset: clickhouse-client
```

## Features
- **Fast queries**: Optimized for analytical queries on large datasets
- **Column-oriented**: Efficient compression and query performance
- **SQL interface**: Standard SQL with extensions
- **Interactive mode**: Tab completion and history
- **Batch processing**: Execute SQL from files or stdin
- **Multiple formats**: JSON, CSV, TabSeparated, and more

## Basic Usage
```bash
# Connect to local server
clickhouse-client

# Connect to remote server
clickhouse-client --host=clickhouse.example.com --port=9000

# Connect with credentials
clickhouse-client --user=admin --password=secret

# Execute query
clickhouse-client --query="SELECT count() FROM system.tables"

# Execute from file
clickhouse-client < queries.sql

# Output as JSON
clickhouse-client --query="SELECT * FROM users" --format=JSON

# Multi-line query mode
clickhouse-client --multiline
```

## Query Examples
```sql
-- Create database
CREATE DATABASE analytics;

-- Create table
CREATE TABLE analytics.events (
    timestamp DateTime,
    user_id UInt64,
    event_type String,
    properties String
) ENGINE = MergeTree()
ORDER BY (timestamp, user_id);

-- Insert data
INSERT INTO analytics.events VALUES
    ('2024-01-01 10:00:00', 1, 'page_view', '{"page": "/home"}'),
    ('2024-01-01 10:05:00', 1, 'click', '{"button": "signup"}');

-- Query data
SELECT
    toDate(timestamp) AS date,
    event_type,
    count() AS events
FROM analytics.events
GROUP BY date, event_type
ORDER BY date DESC;

-- Aggregations
SELECT
    user_id,
    uniqExact(event_type) AS unique_events,
    count() AS total_events
FROM analytics.events
GROUP BY user_id;
```

## Advanced Usage
```bash
# Benchmark query
clickhouse-client --query="SELECT count() FROM large_table" --time

# Parallel queries
clickhouse-client --query="SELECT * FROM table" --max_threads=8

# Stream data from file
cat data.csv | clickhouse-client --query="INSERT INTO table FORMAT CSV"

# Export table to CSV
clickhouse-client --query="SELECT * FROM table FORMAT CSV" > export.csv

# Show query execution plan
clickhouse-client --query="EXPLAIN SELECT * FROM table WHERE id = 1"

# Vertical output (like \G in MySQL)
clickhouse-client --query="SELECT * FROM users LIMIT 1" --vertical
```

## Real-World Examples

### Data Loading
```yaml
- name: Install ClickHouse client
  preset: clickhouse-client

- name: Create database schema
  shell: |
    clickhouse-client --host={{ ch_host }} --query="
    CREATE DATABASE IF NOT EXISTS analytics;
    CREATE TABLE IF NOT EXISTS analytics.events (
      timestamp DateTime,
      event String,
      user_id UInt64
    ) ENGINE = MergeTree() ORDER BY timestamp;
    "

- name: Load data from CSV
  shell: |
    cat /data/events.csv | clickhouse-client \
      --host={{ ch_host }} \
      --query="INSERT INTO analytics.events FORMAT CSV"
```

### Query and Export
```yaml
- name: Run analytics query
  shell: |
    clickhouse-client --host={{ ch_host }} --query="
    SELECT
      toDate(timestamp) AS date,
      count() AS events,
      uniq(user_id) AS unique_users
    FROM analytics.events
    WHERE date >= today() - 7
    GROUP BY date
    ORDER BY date
    " --format=JSONEachRow > report.json
```

### Monitoring
```bash
# Check server status
clickhouse-client --query="SELECT version()"

# Show running queries
clickhouse-client --query="SELECT * FROM system.processes"

# Database sizes
clickhouse-client --query="
SELECT
    database,
    formatReadableSize(sum(bytes)) AS size
FROM system.parts
GROUP BY database
ORDER BY sum(bytes) DESC
"
```

## Configuration

### Config File
```ini
# ~/.clickhouse-client/config.xml
<config>
    <host>clickhouse.example.com</host>
    <port>9000</port>
    <user>default</user>
    <password></password>
    <database>default</database>
</config>
```

## Platform Support
- ✅ Linux (apt, yum, binary)
- ✅ macOS (Homebrew, binary)
- ❌ Windows (WSL)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Query ClickHouse from automation scripts
- Load data into ClickHouse in CI/CD
- Export query results for reporting
- Monitor ClickHouse cluster health
- Perform batch data operations


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install clickhouse-client
  preset: clickhouse-client

- name: Use clickhouse-client in automation
  shell: |
    # Custom configuration here
    echo "clickhouse-client configured"
```
## Uninstall
```yaml
- preset: clickhouse-client
  with:
    state: absent
```

## Resources
- Official site: https://clickhouse.com
- Documentation: https://clickhouse.com/docs/
- GitHub: https://github.com/ClickHouse/ClickHouse
- Search: "clickhouse tutorial", "clickhouse client", "clickhouse sql examples"

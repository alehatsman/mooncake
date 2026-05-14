# cqlsh - Cassandra Query Language Shell

cqlsh is an interactive command-line interface for executing CQL (Cassandra Query Language) commands against Apache Cassandra clusters.

## Quick Start
```yaml
- preset: cqlsh
```

## Features
- **Interactive shell**: REPL for CQL queries
- **Batch execution**: Run CQL scripts from files
- **Import/export**: Load and export CSV data
- **Auto-completion**: Tab completion for CQL keywords and keyspaces
- **Result formatting**: Table and JSON output formats
- **Connection security**: SSL/TLS support

## Basic Usage
```bash
# Connect to local Cassandra
cqlsh

# Connect to remote host
cqlsh 192.168.1.100

# Connect with authentication
cqlsh -u username -p password localhost

# Connect with SSL
cqlsh --ssl localhost

# Execute CQL file
cqlsh -f script.cql

# Execute single command
cqlsh -e "SELECT * FROM system.local;"

# Output format options
cqlsh --format json
cqlsh --format csv
```

## Advanced Configuration
```yaml
- preset: cqlsh
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove cqlsh |

## Platform Support
- ✅ Linux (pip, apt, yum)
- ✅ macOS (pip, Homebrew)
- ❌ Windows (not yet supported)

## Configuration
- **Config file**: `~/.cassandra/cqlshrc` (optional)
- **History file**: `~/.cassandra/cqlsh_history`
- **Default host**: localhost
- **Default port**: 9042

## Real-World Examples

### Create Keyspace and Table
```cql
-- Connect
cqlsh localhost

-- Create keyspace
CREATE KEYSPACE myapp
WITH replication = {
  'class': 'SimpleStrategy',
  'replication_factor': 3
};

-- Use keyspace
USE myapp;

-- Create table
CREATE TABLE users (
  user_id uuid PRIMARY KEY,
  email text,
  name text,
  created_at timestamp
);

-- Insert data
INSERT INTO users (user_id, email, name, created_at)
VALUES (uuid(), 'john@example.com', 'John Doe', toTimestamp(now()));

-- Query data
SELECT * FROM users;
```

### Import CSV Data
```bash
# Create table first
cqlsh -e "CREATE TABLE myapp.products (
  product_id int PRIMARY KEY,
  name text,
  price decimal
);"

# Import CSV (products.csv)
# Format: product_id,name,price
# 1,Widget,19.99
# 2,Gadget,29.99

cqlsh -e "COPY myapp.products FROM 'products.csv' WITH HEADER=true;"
```

### Export Data to CSV
```bash
# Export table to CSV
cqlsh -e "COPY myapp.users TO 'users_backup.csv' WITH HEADER=true;"

# Export query results
cqlsh -e "COPY myapp.users (user_id, email) TO 'emails.csv' WITH HEADER=true;"
```

### Batch Operations
```cql
-- batch.cql file
BEGIN BATCH
  INSERT INTO users (user_id, email, name)
  VALUES (uuid(), 'alice@example.com', 'Alice');

  INSERT INTO users (user_id, email, name)
  VALUES (uuid(), 'bob@example.com', 'Bob');

  UPDATE users SET name = 'Bob Smith'
  WHERE user_id = 123e4567-e89b-12d3-a456-426614174000;
APPLY BATCH;
```

```bash
# Execute batch file
cqlsh -f batch.cql
```

### Secondary Indexes
```cql
-- Create secondary index
CREATE INDEX ON users (email);

-- Query using index
SELECT * FROM users WHERE email = 'john@example.com';

-- Create custom index
CREATE CUSTOM INDEX name_idx ON users (name)
USING 'org.apache.cassandra.index.sasi.SASIIndex';
```

### Time Series Data
```cql
-- Time series table
CREATE TABLE events (
  sensor_id int,
  event_time timestamp,
  temperature decimal,
  humidity decimal,
  PRIMARY KEY (sensor_id, event_time)
) WITH CLUSTERING ORDER BY (event_time DESC);

-- Insert event
INSERT INTO events (sensor_id, event_time, temperature, humidity)
VALUES (1, toTimestamp(now()), 72.5, 45.2);

-- Query recent events
SELECT * FROM events
WHERE sensor_id = 1
AND event_time > '2024-01-01'
LIMIT 100;
```

### JSON Operations
```cql
-- Insert JSON
INSERT INTO users JSON '{
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "email": "jane@example.com",
  "name": "Jane Doe"
}';

-- Select as JSON
SELECT JSON * FROM users;
```

### cqlshrc Configuration
```ini
# ~/.cassandra/cqlshrc
[authentication]
username = myuser
password = mypass

[connection]
hostname = cassandra.example.com
port = 9042
timeout = 60

[cql]
version = 3.4.5

[ui]
color = on
datetimeformat = %Y-%m-%d %H:%M:%S%z
completekey = tab
```

### Python Script Integration
```python
from cassandra.cluster import Cluster
from cassandra.auth import PlainTextAuthProvider

# Connect
auth_provider = PlainTextAuthProvider(
    username='myuser',
    password='mypass'
)
cluster = Cluster(['localhost'], auth_provider=auth_provider)
session = cluster.connect('myapp')

# Execute query
rows = session.execute("SELECT * FROM users")
for row in rows:
    print(row.email, row.name)

# Prepared statements
prepared = session.prepare("INSERT INTO users (user_id, email, name) VALUES (?, ?, ?)")
session.execute(prepared, (uuid.uuid4(), 'test@example.com', 'Test User'))

cluster.shutdown()
```

## cqlsh Commands
```bash
# Inside cqlsh
HELP                  # Show available commands
DESCRIBE KEYSPACES    # List all keyspaces
DESCRIBE TABLES       # List tables in current keyspace
DESCRIBE TABLE users  # Show table schema
SOURCE 'script.cql'   # Execute CQL file
TRACING ON            # Enable query tracing
TRACING OFF           # Disable tracing
PAGING ON             # Enable result paging
PAGING OFF            # Disable paging
EXPAND ON             # Vertical output format
EXPAND OFF            # Table output format
EXIT                  # Quit cqlsh
```

## Agent Use
- Automated database schema migrations
- Data import/export in ETL pipelines
- Database health checks and monitoring
- Backup and restore operations
- CI/CD database initialization
- Automated testing with test data

## Troubleshooting

### Connection refused
Check Cassandra status:
```bash
# Check if Cassandra is running
systemctl status cassandra

# Check port
netstat -tuln | grep 9042

# Test connectivity
telnet localhost 9042
```

### Authentication failed
Verify credentials:
```bash
# Connect with explicit credentials
cqlsh -u cassandra -p cassandra localhost

# Check user permissions
SELECT * FROM system_auth.roles;
```

### Import/export errors
Check file permissions and format:
```bash
# Verify CSV format (no quotes around strings by default)
# Ensure NULL values are represented correctly

# Import with options
cqlsh -e "COPY myapp.users FROM 'data.csv'
WITH HEADER=true
AND DELIMITER=','
AND NULL='NULL';"
```

## Uninstall
```yaml
- preset: cqlsh
  with:
    state: absent
```

## Resources
- Official docs: https://cassandra.apache.org/doc/latest/cassandra/tools/cqlsh.html
- CQL reference: https://cassandra.apache.org/doc/latest/cassandra/cql/
- Python driver: https://docs.datastax.com/en/developer/python-driver/
- Search: "cqlsh tutorial", "cassandra cql examples"

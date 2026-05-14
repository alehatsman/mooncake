# QuestDB - Time-Series Database

High-performance time-series database optimized for fast ingestion and SQL queries with time-series extensions.

## Quick Start

```yaml
- preset: questdb
```

## Features

- **SQL interface**: Standard SQL with time-series extensions
- **Fast ingestion**: 4M+ rows/second with InfluxDB line protocol
- **PostgreSQL wire**: Compatible with PostgreSQL clients and tools
- **Built-in web console**: Query and visualize data in browser
- **Zero configuration**: Works out of the box with sensible defaults
- **Column-oriented**: Optimized for time-series workloads
- **Cross-platform**: Linux, macOS, Docker support

## Basic Usage

```bash
# Check version
questdb --version

# Start server (default ports: 9000 HTTP, 8812 PostgreSQL, 9009 InfluxDB)
questdb start

# Web console
open http://localhost:9000

# Stop server
questdb stop

# Query via curl
curl -G http://localhost:9000/exec \
  --data-urlencode "query=SELECT * FROM sensors LATEST BY id"
```

## Advanced Configuration

```yaml
# Install QuestDB
- preset: questdb
  register: questdb_result
  become: true

# Verify installation
- name: Wait for QuestDB to be ready
  assert:
    http:
      url: http://localhost:9000/health
      status: 200
  retries: 10
  delay: 2

# Query data
- name: Execute SQL query
  shell: |
    curl -G http://localhost:9000/exec \
      --data-urlencode "query=SELECT count() FROM trades"
  register: query_result
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (`present` or `absent`) |

## Platform Support

- ✅ Linux (binary install, Docker)
- ✅ macOS (Homebrew, binary install)
- ✅ Docker (official images available)
- ❌ Windows (use Docker or WSL)

## Configuration

- **Config file**: `conf/server.conf` (in data directory)
- **Data directory**: `~/.questdb/db/` or `/var/lib/questdb/`
- **HTTP port**: 9000 (REST API, web console)
- **PostgreSQL port**: 8812 (PostgreSQL wire protocol)
- **InfluxDB port**: 9009 (InfluxDB line protocol)
- **Web console**: http://localhost:9000

## Real-World Examples

### IoT Sensor Data Collection

```yaml
# Deploy QuestDB for IoT platform
- preset: questdb
  become: true

# Configure systemd service
- name: Create QuestDB service
  service:
    name: questdb
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description=QuestDB Time-Series Database
        After=network.target

        [Service]
        Type=simple
        User=questdb
        WorkingDirectory=/var/lib/questdb
        ExecStart=/usr/local/bin/questdb start -d /var/lib/questdb
        Restart=always
        RestartSec=10
        LimitNOFILE=65536

        [Install]
        WantedBy=multi-user.target
  when: os == "linux"
  become: true

# Create tables
- name: Initialize sensor tables
  shell: |
    curl -G http://localhost:9000/exec \
      --data-urlencode "query=CREATE TABLE sensors (
        device_id SYMBOL,
        temperature DOUBLE,
        humidity DOUBLE,
        timestamp TIMESTAMP
      ) timestamp(timestamp) PARTITION BY DAY;"
```

Ingest data via InfluxDB line protocol:

```bash
# Send sensor readings
echo "sensors,device_id=sensor1 temperature=22.5,humidity=65.0" | \
  nc localhost 9009

# Bulk ingestion
cat sensors.txt | nc localhost 9009
```

### Financial Market Data

```yaml
# Install QuestDB for trading platform
- preset: questdb
  become: true

# Create market data tables
- name: Setup trading tables
  shell: |
    curl -G http://localhost:9000/exec \
      --data-urlencode "query=CREATE TABLE trades (
        symbol SYMBOL,
        side SYMBOL,
        price DOUBLE,
        quantity LONG,
        timestamp TIMESTAMP
      ) timestamp(timestamp) PARTITION BY DAY;"

- name: Create OHLC aggregation
  shell: |
    curl -G http://localhost:9000/exec \
      --data-urlencode "query=CREATE TABLE ohlc_1m AS (
        SELECT
          symbol,
          first(price) as open,
          max(price) as high,
          min(price) as low,
          last(price) as close,
          sum(quantity) as volume,
          timestamp
        FROM trades
        SAMPLE BY 1m ALIGN TO CALENDAR
      );"
```

Query examples:

```sql
-- Latest price per symbol
SELECT * FROM trades LATEST BY symbol;

-- OHLC bars
SELECT
  symbol,
  first(price) as open,
  max(price) as high,
  min(price) as low,
  last(price) as close,
  sum(quantity) as volume
FROM trades
WHERE symbol = 'BTCUSD'
SAMPLE BY 1h ALIGN TO CALENDAR;

-- Moving average
SELECT
  timestamp,
  price,
  avg(price) OVER (ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20
FROM trades
WHERE symbol = 'BTCUSD';
```

### Application Performance Monitoring

```yaml
# Deploy QuestDB for APM data
- preset: questdb
  become: true

# Create metrics table
- name: Setup APM tables
  shell: |
    curl -G http://localhost:9000/exec \
      --data-urlencode "query=CREATE TABLE metrics (
        service SYMBOL,
        endpoint SYMBOL,
        method SYMBOL,
        status_code INT,
        duration_ms DOUBLE,
        timestamp TIMESTAMP
      ) timestamp(timestamp) PARTITION BY HOUR;"
```

Python client example:

```python
import psycopg2
from datetime import datetime

# Connect via PostgreSQL protocol
conn = psycopg2.connect(
    host="localhost",
    port=8812,
    user="admin",
    password="quest",
    database="qdb"
)

# Insert metrics
cursor = conn.cursor()
cursor.execute("""
    INSERT INTO metrics VALUES (
        'api-service',
        '/users',
        'GET',
        200,
        42.5,
        '2024-01-01T12:00:00.000000Z'
    )
""")
conn.commit()

# Query p95 latency
cursor.execute("""
    SELECT
        service,
        endpoint,
        approx_percentile(duration_ms, 0.95) as p95
    FROM metrics
    WHERE timestamp > now() - 1h
    SAMPLE BY 5m
""")
for row in cursor.fetchall():
    print(row)
```

### Log Analytics

```bash
# Ingest application logs
curl -X POST http://localhost:9000/imp \
  -F data=@logs.csv \
  -F name=app_logs \
  -F timestamp=timestamp \
  -F partitionBy=DAY

# Query logs
curl -G http://localhost:9000/exec \
  --data-urlencode "query=
    SELECT * FROM app_logs
    WHERE level = 'ERROR'
      AND timestamp > dateadd('h', -1, now())
    ORDER BY timestamp DESC
    LIMIT 100
  "
```

## PostgreSQL Integration

```bash
# Connect with psql
psql -h localhost -p 8812 -U admin -d qdb

# Connect with Python
import psycopg2
conn = psycopg2.connect(
    host="localhost", port=8812,
    user="admin", password="quest", database="qdb"
)

# Connect with Go
import "github.com/lib/pq"
db, _ := sql.Open("postgres",
    "host=localhost port=8812 user=admin password=quest dbname=qdb")

# Connect with Node.js
const { Client } = require('pg');
const client = new Client({
  host: 'localhost', port: 8812,
  user: 'admin', password: 'quest', database: 'qdb'
});
```

## InfluxDB Line Protocol

```bash
# Single measurement
echo "sensors,location=roof temperature=23.5 1465839830100400000" | nc localhost 9009

# Multiple fields
echo "weather,city=london temp=15.2,humidity=82,pressure=1013.25" | nc localhost 9009

# Batch insert
cat <<EOF | nc localhost 9009
cpu,host=server1 usage=45.2
cpu,host=server2 usage=38.9
memory,host=server1 used=8192
EOF
```

## Web Console Features

```yaml
# Access web console
- name: Open QuestDB console
  shell: open http://localhost:9000

# Features:
# - SQL editor with syntax highlighting
# - Table browser and schema viewer
# - Query result visualization (charts)
# - Import CSV files via drag-and-drop
# - Export query results
```

## Performance Tuning

```bash
# Configure in server.conf
cat > /var/lib/questdb/conf/server.conf <<EOF
# Increase writer queue
cairo.max.uncommitted.rows=500000

# Optimize for write throughput
http.receive.buffer.size=4m

# Enable parallel indexing
cairo.parallel.index.threshold=1000000

# Set commit lag for better batching
line.tcp.commit.interval.default=2000
EOF

# Restart to apply
questdb stop && questdb start
```

## SQL Extensions

```sql
-- LATEST BY (get most recent row per group)
SELECT * FROM trades LATEST BY symbol;

-- SAMPLE BY (time-series aggregation)
SELECT avg(price) FROM trades SAMPLE BY 1h;

-- ASOF JOIN (time-based join)
SELECT * FROM trades
ASOF JOIN quotes ON (symbol);

-- FILL (handle gaps in time series)
SELECT timestamp, avg(value)
FROM metrics
SAMPLE BY 1m FILL(LINEAR);
```

## Agent Use

- Store and query IoT sensor data at scale
- Financial market data analysis and backtesting
- Application performance monitoring and metrics
- Log aggregation and analytics
- Real-time dashboards and monitoring
- Time-series forecasting and anomaly detection
- Infrastructure monitoring and capacity planning
- Business intelligence on time-stamped events

## Troubleshooting

### Server won't start

Check logs and ports:

```bash
# Check if ports are in use
lsof -i :9000
lsof -i :8812

# View logs
tail -f /var/lib/questdb/log/stdout.txt

# Check disk space
df -h /var/lib/questdb
```

### High memory usage

Adjust memory settings:

```bash
# Set max heap in server.conf
export JAVA_OPTS="-Xms4g -Xmx8g"
questdb start

# Monitor memory
curl http://localhost:9000/metrics | grep memory
```

### Slow queries

Optimize table structure:

```sql
-- Add indexes on symbol columns
CREATE TABLE trades (
    symbol SYMBOL INDEX,
    price DOUBLE,
    timestamp TIMESTAMP
) timestamp(timestamp);

-- Check table structure
SHOW TABLES;
SHOW COLUMNS FROM trades;
```

### Data corruption

```bash
# Backup database
cp -r /var/lib/questdb/db /backup/

# Repair table
curl -G http://localhost:9000/exec \
  --data-urlencode "query=REPAIR TABLE trades"

# Vacuum old partitions
curl -G http://localhost:9000/exec \
  --data-urlencode "query=VACUUM TABLE trades"
```

## Uninstall

```yaml
- preset: questdb
  with:
    state: absent
```

**Note**: This removes QuestDB binary but preserves data in `/var/lib/questdb/`.

## Resources

- Official docs: https://questdb.io/docs/
- GitHub: https://github.com/questdb/questdb
- SQL reference: https://questdb.io/docs/reference/sql/
- Examples: https://github.com/questdb/questdb-kubernetes
- Search: "questdb tutorial", "questdb time-series", "questdb influxdb"

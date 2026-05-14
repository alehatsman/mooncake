# TimescaleDB - Time-Series PostgreSQL

PostgreSQL extension optimized for time-series data. Get 10-100x faster queries on time-series workloads while keeping full SQL and PostgreSQL ecosystem compatibility. Metrics, IoT, financial data, and more.

## Quick Start
```yaml
- preset: timescaledb
```

## Features
- **Full SQL support**: PostgreSQL + time-series optimizations
- **Automatic partitioning**: Transparent hypertables (time-based chunks)
- **High compression**: 90-95% compression ratios on time-series data
- **Continuous aggregates**: Materialized views that auto-update
- **Data retention**: Automatic data deletion policies
- **Fast queries**: 10-100x faster than vanilla PostgreSQL for time-series
- **Scalability**: Handles billions of rows per table
- **Ecosystem**: All PostgreSQL tools, libraries, and extensions work
- **No query rewriting**: Drop-in replacement for PostgreSQL time-series tables

## Basic Usage

### Enable extension
```sql
-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;
```

### Create hypertable
```sql
-- Create regular table
CREATE TABLE metrics (
    time        TIMESTAMPTZ NOT NULL,
    device_id   INT NOT NULL,
    temperature DOUBLE PRECISION,
    humidity    DOUBLE PRECISION
);

-- Convert to hypertable (automatic partitioning)
SELECT create_hypertable('metrics', 'time');

-- Insert data (same as regular PostgreSQL)
INSERT INTO metrics VALUES
    ('2024-01-01 00:00:00', 1, 20.5, 65.2),
    ('2024-01-01 00:05:00', 1, 20.7, 64.8);

-- Query (same as regular PostgreSQL)
SELECT * FROM metrics
WHERE time > NOW() - INTERVAL '1 day'
  AND device_id = 1
ORDER BY time DESC;
```

### Time-series functions
```sql
-- Time bucketing
SELECT time_bucket('1 hour', time) AS hour,
       AVG(temperature) AS avg_temp
FROM metrics
WHERE time > NOW() - INTERVAL '7 days'
GROUP BY hour
ORDER BY hour DESC;

-- First/Last aggregates
SELECT device_id,
       first(temperature, time) AS first_temp,
       last(temperature, time) AS last_temp
FROM metrics
WHERE time > NOW() - INTERVAL '1 day'
GROUP BY device_id;

-- Gaps filling
SELECT time_bucket_gapfill('5 minutes', time) AS bucket,
       AVG(temperature) AS avg_temp
FROM metrics
WHERE time > NOW() - INTERVAL '1 hour'
GROUP BY bucket
ORDER BY bucket;
```

## Advanced Configuration

### Install TimescaleDB on existing PostgreSQL
```yaml
- name: Install PostgreSQL
  preset: postgres

- name: Install TimescaleDB
  preset: timescaledb

- name: Enable TimescaleDB extension
  shell: |
    psql -U postgres -d mydb -c "CREATE EXTENSION IF NOT EXISTS timescaledb;"
```

### Production deployment
```yaml
- name: Install PostgreSQL + TimescaleDB
  hosts: timeseries-db
  tasks:
    - preset: postgres
      with:
        version: "15"
        databases:
          - timeseries
          - metrics
        users:
          - name: tsdb
            password: "{{ db_password }}"
      become: true

    - preset: timescaledb
      become: true

    - name: Configure PostgreSQL for time-series
      template:
        src: postgresql.conf.j2
        dest: /etc/postgresql/15/main/postgresql.conf
      become: true

    - name: Restart PostgreSQL
      service:
        name: postgresql
        state: restarted
      become: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove TimescaleDB |

## Platform Support
- ✅ Linux (apt, yum, dnf) - PostgreSQL 12-16
- ✅ macOS (Homebrew)
- ✅ Docker (official timescale/timescaledb images)
- ❌ Windows (use Docker or WSL2)

## Hypertables

### Create hypertable
```sql
-- Standard table
CREATE TABLE sensor_data (
    time        TIMESTAMPTZ NOT NULL,
    sensor_id   INTEGER NOT NULL,
    value       DOUBLE PRECISION
);

-- Convert to hypertable (partitioned by time)
SELECT create_hypertable('sensor_data', 'time');

-- With space partitioning (multi-dimensional)
SELECT create_hypertable('sensor_data', 'time',
    partitioning_column => 'sensor_id',
    number_partitions => 4
);

-- With chunk time interval (default 7 days)
SELECT create_hypertable('sensor_data', 'time',
    chunk_time_interval => INTERVAL '1 day'
);
```

### Chunk management
```sql
-- Show chunks
SELECT show_chunks('sensor_data');

-- Drop old chunks
SELECT drop_chunks('sensor_data', INTERVAL '90 days');

-- Show chunk statistics
SELECT * FROM timescaledb_information.chunks
WHERE hypertable_name = 'sensor_data';
```

## Continuous Aggregates

### Create continuous aggregate
```sql
-- Materialized view that auto-updates
CREATE MATERIALIZED VIEW sensor_data_hourly
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 hour', time) AS hour,
       sensor_id,
       AVG(value) AS avg_value,
       MAX(value) AS max_value,
       MIN(value) AS min_value,
       COUNT(*) AS count
FROM sensor_data
GROUP BY hour, sensor_id;

-- Add refresh policy (auto-update)
SELECT add_continuous_aggregate_policy('sensor_data_hourly',
    start_offset => INTERVAL '2 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes'
);

-- Query continuous aggregate (fast!)
SELECT * FROM sensor_data_hourly
WHERE hour > NOW() - INTERVAL '7 days'
  AND sensor_id = 42
ORDER BY hour DESC;
```

### Refresh policies
```sql
-- Manual refresh
CALL refresh_continuous_aggregate('sensor_data_hourly',
    NOW() - INTERVAL '7 days', NOW()
);

-- Remove policy
SELECT remove_continuous_aggregate_policy('sensor_data_hourly');
```

## Compression

### Enable compression
```sql
-- Enable compression on hypertable
ALTER TABLE sensor_data SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'sensor_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Add compression policy (auto-compress after 7 days)
SELECT add_compression_policy('sensor_data', INTERVAL '7 days');

-- Compress chunks manually
SELECT compress_chunk(i) FROM show_chunks('sensor_data', older_than => INTERVAL '7 days') AS i;

-- Decompress chunk (for updates)
SELECT decompress_chunk('_timescaledb_internal._hyper_1_10_chunk');
```

### Compression stats
```sql
-- View compression stats
SELECT * FROM timescaledb_information.compression_settings
WHERE hypertable_name = 'sensor_data';

-- Chunk compression status
SELECT chunk_name,
       before_compression_total_bytes,
       after_compression_total_bytes,
       ROUND((1 - after_compression_total_bytes::numeric / before_compression_total_bytes) * 100, 2) AS compression_ratio
FROM chunk_compression_stats('sensor_data')
ORDER BY chunk_name;
```

## Data Retention

### Retention policies
```sql
-- Drop chunks older than 90 days
SELECT add_retention_policy('sensor_data', INTERVAL '90 days');

-- Different retention per table
SELECT add_retention_policy('metrics', INTERVAL '30 days');
SELECT add_retention_policy('logs', INTERVAL '7 days');
SELECT add_retention_policy('audit', INTERVAL '365 days');

-- Remove retention policy
SELECT remove_retention_policy('sensor_data');

-- View retention policies
SELECT * FROM timescaledb_information.jobs
WHERE proc_name = 'policy_retention';
```

## Time-Series Functions

### Time bucketing
```sql
-- 5-minute buckets
SELECT time_bucket('5 minutes', time) AS bucket,
       AVG(value) AS avg_value
FROM sensor_data
GROUP BY bucket
ORDER BY bucket DESC;

-- Daily buckets (origin at midnight)
SELECT time_bucket('1 day', time, 'UTC') AS day,
       SUM(value) AS total_value
FROM sensor_data
GROUP BY day;
```

### First/Last aggregates
```sql
-- First and last values in time range
SELECT sensor_id,
       first(value, time) AS first_reading,
       last(value, time) AS last_reading,
       first(time, time) AS first_time,
       last(time, time) AS last_time
FROM sensor_data
WHERE time > NOW() - INTERVAL '1 hour'
GROUP BY sensor_id;
```

### Gap filling
```sql
-- Fill gaps with interpolation
SELECT time_bucket_gapfill('5 minutes', time) AS bucket,
       sensor_id,
       AVG(value) AS avg_value,
       interpolate(AVG(value)) AS interpolated_value
FROM sensor_data
WHERE time > NOW() - INTERVAL '1 hour'
  AND sensor_id = 1
GROUP BY bucket, sensor_id
ORDER BY bucket;
```

### Locf (Last Observation Carried Forward)
```sql
SELECT time_bucket_gapfill('5 minutes', time) AS bucket,
       AVG(value) AS avg_value,
       locf(AVG(value)) AS filled_value
FROM sensor_data
WHERE time > NOW() - INTERVAL '1 hour'
GROUP BY bucket;
```

## Use Cases

### IoT Sensor Data
```yaml
- name: Setup TimescaleDB for IoT
  hosts: iot-db
  tasks:
    - preset: postgres
    - preset: timescaledb

    - name: Create IoT database
      shell: |
        psql -U postgres -c "CREATE DATABASE iot;"
        psql -U postgres -d iot -c "CREATE EXTENSION timescaledb;"
        psql -U postgres -d iot -c "
          CREATE TABLE sensor_readings (
            time TIMESTAMPTZ NOT NULL,
            device_id INTEGER NOT NULL,
            temperature DOUBLE PRECISION,
            humidity DOUBLE PRECISION,
            battery DOUBLE PRECISION
          );
          SELECT create_hypertable('sensor_readings', 'time');
          ALTER TABLE sensor_readings SET (timescaledb.compress);
          SELECT add_compression_policy('sensor_readings', INTERVAL '1 day');
          SELECT add_retention_policy('sensor_readings', INTERVAL '90 days');
        "
```

### Application Metrics
```yaml
- name: Setup metrics database
  shell: |
    psql -U postgres -d metrics -c "
      CREATE TABLE http_requests (
        time TIMESTAMPTZ NOT NULL,
        endpoint TEXT NOT NULL,
        method TEXT NOT NULL,
        status_code INTEGER,
        duration_ms DOUBLE PRECISION,
        user_id INTEGER
      );
      SELECT create_hypertable('http_requests', 'time');

      -- Continuous aggregate for hourly metrics
      CREATE MATERIALIZED VIEW http_requests_hourly
      WITH (timescaledb.continuous) AS
      SELECT time_bucket('1 hour', time) AS hour,
             endpoint,
             method,
             COUNT(*) AS request_count,
             AVG(duration_ms) AS avg_duration,
             PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_duration
      FROM http_requests
      GROUP BY hour, endpoint, method;

      SELECT add_continuous_aggregate_policy('http_requests_hourly',
        start_offset => INTERVAL '2 hours',
        end_offset => INTERVAL '1 hour',
        schedule_interval => INTERVAL '30 minutes'
      );
    "
```

### Financial Time-Series
```yaml
- name: Setup financial data
  shell: |
    psql -U postgres -d trading -c "
      CREATE TABLE stock_prices (
        time TIMESTAMPTZ NOT NULL,
        symbol TEXT NOT NULL,
        price NUMERIC(10,2),
        volume BIGINT,
        exchange TEXT
      );
      SELECT create_hypertable('stock_prices', 'time',
        partitioning_column => 'symbol',
        number_partitions => 10
      );

      -- Create index for fast symbol lookups
      CREATE INDEX ON stock_prices (symbol, time DESC);

      -- Continuous aggregate for daily OHLCV
      CREATE MATERIALIZED VIEW stock_prices_daily
      WITH (timescaledb.continuous) AS
      SELECT time_bucket('1 day', time) AS day,
             symbol,
             first(price, time) AS open,
             max(price) AS high,
             min(price) AS low,
             last(price, time) AS close,
             sum(volume) AS volume
      FROM stock_prices
      GROUP BY day, symbol;
    "
```

## PostgreSQL Configuration

### Recommended settings
```ini
# postgresql.conf

# Memory
shared_buffers = 2GB
effective_cache_size = 6GB
maintenance_work_mem = 512MB
work_mem = 16MB

# TimescaleDB specific
timescaledb.max_background_workers = 8
max_worker_processes = 16

# Checkpoints
checkpoint_timeout = 10min
max_wal_size = 2GB
min_wal_size = 512MB

# Query performance
random_page_cost = 1.1  # For SSD
effective_io_concurrency = 200

# Autovacuum
autovacuum = on
autovacuum_max_workers = 4
```

## Grafana Integration

### Configure TimescaleDB datasource
```yaml
- name: Add TimescaleDB to Grafana
  shell: |
    curl -X POST http://admin:admin@localhost:3000/api/datasources \
      -H "Content-Type: application/json" \
      -d '{
        "name": "TimescaleDB",
        "type": "postgres",
        "url": "localhost:5432",
        "database": "metrics",
        "user": "grafana",
        "secureJsonData": {
          "password": "secret"
        },
        "jsonData": {
          "sslmode": "disable",
          "postgresVersion": 1500,
          "timescaledb": true
        }
      }'
```

### Query example in Grafana
```sql
SELECT
  time_bucket('$__interval', time) AS time,
  sensor_id,
  AVG(value) AS avg_value
FROM sensor_data
WHERE
  time >= $__timeFrom() AND time < $__timeTo()
  AND sensor_id = ANY($sensor_ids)
GROUP BY time, sensor_id
ORDER BY time
```

## Mooncake Usage

### Basic installation
```yaml
- name: Install TimescaleDB
  preset: timescaledb
```

### Complete monitoring stack
```yaml
- name: Deploy TimescaleDB
  hosts: db-server
  tasks:
    - preset: postgres
      with:
        version: "15"
        databases:
          - metrics
        users:
          - name: metrics_user
            password: "{{ db_password }}"

    - preset: timescaledb

    - name: Initialize metrics schema
      shell: |
        psql -U postgres -d metrics << 'EOF'
        CREATE EXTENSION IF NOT EXISTS timescaledb;

        CREATE TABLE application_metrics (
          time TIMESTAMPTZ NOT NULL,
          application TEXT NOT NULL,
          metric_name TEXT NOT NULL,
          value DOUBLE PRECISION,
          tags JSONB
        );

        SELECT create_hypertable('application_metrics', 'time');

        ALTER TABLE application_metrics SET (
          timescaledb.compress,
          timescaledb.compress_segmentby = 'application, metric_name'
        );

        SELECT add_compression_policy('application_metrics', INTERVAL '7 days');
        SELECT add_retention_policy('application_metrics', INTERVAL '90 days');

        CREATE MATERIALIZED VIEW metrics_hourly
        WITH (timescaledb.continuous) AS
        SELECT time_bucket('1 hour', time) AS hour,
               application,
               metric_name,
               AVG(value) AS avg_value,
               MAX(value) AS max_value,
               MIN(value) AS min_value
        FROM application_metrics
        GROUP BY hour, application, metric_name;

        SELECT add_continuous_aggregate_policy('metrics_hourly',
          start_offset => INTERVAL '2 hours',
          end_offset => INTERVAL '1 hour',
          schedule_interval => INTERVAL '30 minutes'
        );
        EOF
```

## Agent Use
- **Metrics storage**: High-performance time-series data for monitoring
- **IoT data**: Sensor readings, device telemetry at scale
- **Financial data**: Stock prices, trading data with fast queries
- **Log aggregation**: Structured logs with time-series indexing
- **Analytics**: Real-time dashboards with continuous aggregates
- **Data retention**: Automatic aging out of old data
- **Downsampling**: Continuous aggregates for historical data

## Troubleshooting

### Extension not available
```bash
# Check if TimescaleDB is installed
dpkg -l | grep timescaledb  # Debian/Ubuntu
rpm -qa | grep timescaledb  # RHEL/CentOS

# Verify PostgreSQL version compatibility
psql --version
# TimescaleDB 2.x requires PostgreSQL 12-16
```

### Hypertable creation fails
```sql
-- Check if table has data
SELECT COUNT(*) FROM my_table;

-- Must have time column with NOT NULL constraint
\d my_table

-- Create index on time column first
CREATE INDEX ON my_table (time DESC);
```

### Slow queries
```sql
-- Check if hypertable chunks are being used
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM sensor_data
WHERE time > NOW() - INTERVAL '1 day';

-- Add indexes on commonly filtered columns
CREATE INDEX ON sensor_data (sensor_id, time DESC);

-- Enable compression for old data
SELECT add_compression_policy('sensor_data', INTERVAL '7 days');
```

### High disk usage
```sql
-- Check compression status
SELECT * FROM chunk_compression_stats('sensor_data');

-- Manually compress old chunks
SELECT compress_chunk(i)
FROM show_chunks('sensor_data', older_than => INTERVAL '7 days') AS i;

-- Add retention policy
SELECT add_retention_policy('sensor_data', INTERVAL '90 days');
```

### Continuous aggregate not updating
```sql
-- Check refresh policy
SELECT * FROM timescaledb_information.jobs
WHERE proc_name = 'policy_refresh_continuous_aggregate';

-- Manual refresh
CALL refresh_continuous_aggregate('sensor_data_hourly',
  NOW() - INTERVAL '7 days', NOW()
);

-- Check for errors
SELECT * FROM timescaledb_information.job_stats
WHERE job_id = <job_id>;
```

## Best Practices
- **Time column**: Always use TIMESTAMPTZ (timestamp with timezone)
- **Indexes**: Create indexes on frequently filtered columns (not time - already indexed)
- **Chunk interval**: Set based on data retention and query patterns (default 7 days)
- **Compression**: Enable after data is no longer frequently updated
- **Continuous aggregates**: Use for dashboards and reporting (10-100x faster)
- **Retention policies**: Automatically drop old data to save space
- **Segmentby**: Use high-cardinality columns for compression segmentby
- **Orderby**: Match your typical query order (usually time DESC)

## Performance Tips
- **Batch inserts**: Insert 1000-10000 rows at a time for best performance
- **Disable autovacuum**: During bulk loads, re-enable after
- **Use COPY**: Faster than INSERT for bulk loading
- **Parallelize**: TimescaleDB queries use parallel workers automatically
- **Partition pruning**: Filter on time column for chunk exclusion

## Uninstall
```yaml
- preset: timescaledb
  with:
    state: absent
```

**Note**: PostgreSQL and data are preserved. Drop databases manually if needed.

## Resources
- Official: https://www.timescale.com/
- Documentation: https://docs.timescale.com/
- Tutorials: https://docs.timescale.com/tutorials/latest/
- GitHub: https://github.com/timescale/timescaledb
- Forum: https://www.timescale.com/forum/
- Slack: https://timescaledb.slack.com/
- Search: "timescaledb tutorial", "timescaledb hypertable", "timescaledb compression"

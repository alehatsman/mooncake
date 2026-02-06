# PostgreSQL - Relational Database

The world's most advanced open source relational database with powerful features for data integrity, transactions, and ACID compliance.

## Quick Start

```yaml
- preset: postgres
```

## Features

- **ACID Compliant**: Full transaction support with strong consistency
- **Advanced SQL**: JSON/JSONB, full-text search, window functions, CTEs
- **Extensible**: PostGIS, TimescaleDB, pg_vector, and 100+ extensions
- **Scalable**: Streaming replication, logical replication, partitioning
- **Cross-Platform**: Linux, macOS, Windows, BSD, containers
- **Performance**: Index types (B-tree, Hash, GiST, GIN, BRIN), query planner

## Basic Usage

```bash
# Connect to default database
psql -U postgres

# Connect to specific database
psql -U postgres -d mydb

# Check version
psql --version

# Check server status
sudo systemctl status postgresql      # Linux
brew services list | grep postgresql  # macOS

# Run SQL query
psql -U postgres -c "SELECT version()"

# List databases
psql -U postgres -l
```

## Advanced Configuration

```yaml
# Basic installation with default version
- preset: postgres
  with:
    state: present
    start_service: true

# Install specific version
- preset: postgres
  with:
    version: "16"
    start_service: true
    port: "5432"

# Create database and user
- preset: postgres
  with:
    version: "16"
    start_service: true
    create_user: appuser
    create_database: appdb

# Uninstall (preserves data)
- preset: postgres
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |
| version | string | 16 | PostgreSQL version (14, 15, 16) |
| start_service | bool | true | Start service after installation |
| create_user | string | - | PostgreSQL user to create |
| create_database | string | - | Database to create |
| port | string | 5432 | PostgreSQL port |

## Platform Support

- ✅ Linux (apt, dnf, yum, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (use official installer)

## Configuration

- **Config file**:
  - Linux: `/etc/postgresql/{version}/main/postgresql.conf`
  - macOS: `/usr/local/var/postgresql@{version}/postgresql.conf`
- **Data directory**:
  - Linux: `/var/lib/postgresql/{version}/main`
  - macOS: `/usr/local/var/postgresql@{version}`
- **HBA config**:
  - Linux: `/etc/postgresql/{version}/main/pg_hba.conf`
  - macOS: `/usr/local/var/postgresql@{version}/pg_hba.conf`
- **Socket**:
  - Linux: `/var/run/postgresql/`
  - macOS: `/tmp`
- **Default port**: 5432
- **Default user**: postgres

## Real-World Examples

### Web Application Database

```yaml
- name: Setup PostgreSQL for web app
  preset: postgres
  with:
    version: "16"
    start_service: true
    create_database: webapp_prod
    create_user: webapp

- name: Configure database permissions
  shell: |
    psql -U postgres -c "ALTER USER webapp WITH PASSWORD '{{ db_password }}'"
    psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE webapp_prod TO webapp"
  become: true
```

### Multi-Database Development Environment

```yaml
- name: Install PostgreSQL
  preset: postgres
  with:
    version: "16"

- name: Create multiple databases
  shell: |
    createdb -U postgres dev_db
    createdb -U postgres test_db
    createdb -U postgres staging_db
```

### Data Analytics Setup

```yaml
- name: Install PostgreSQL for analytics
  preset: postgres
  with:
    version: "16"
    start_service: true

- name: Install TimescaleDB extension
  shell: |
    sudo apt-get install -y postgresql-16-timescaledb
    psql -U postgres -d analytics -c "CREATE EXTENSION IF NOT EXISTS timescaledb"
  when: apt_available
  become: true
```

## Common Operations

### Database Management

```bash
# Create database
createdb mydb

# Create database with owner
createdb -O myuser mydb

# Drop database
dropdb mydb

# Rename database
psql -U postgres -c "ALTER DATABASE oldname RENAME TO newname"

# Copy database
createdb -T template newdb
```

### User Management

```bash
# Create user
createuser myuser

# Create user with password
psql -U postgres -c "CREATE USER myuser WITH PASSWORD 'secret'"

# Grant privileges
psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE mydb TO myuser"

# Make user superuser
psql -U postgres -c "ALTER USER myuser WITH SUPERUSER"

# List users
psql -U postgres -c "\du"
```

### Backup and Restore

```bash
# Backup single database
pg_dump mydb > backup.sql

# Backup with compression
pg_dump mydb | gzip > backup.sql.gz

# Backup all databases
pg_dumpall > all_databases.sql

# Restore database
psql mydb < backup.sql

# Restore from compressed backup
gunzip -c backup.sql.gz | psql mydb

# Backup in custom format (faster restore)
pg_dump -Fc mydb > backup.dump
pg_restore -d mydb backup.dump
```

### psql Commands

```sql
-- List databases
\l

-- Connect to database
\c dbname

-- List tables
\dt

-- Describe table
\d tablename

-- List schemas
\dn

-- List functions
\df

-- List users/roles
\du

-- Show table sizes
\dt+

-- Execute external file
\i script.sql

-- Timing queries
\timing on

-- Quit
\q
```

## Connection Strings

```bash
# Standard format
postgresql://user:password@localhost:5432/database

# With SSL
postgresql://user:password@localhost:5432/database?sslmode=require

# Python (psycopg2)
conn = psycopg2.connect(
    host="localhost",
    database="mydb",
    user="myuser",
    password="secret"
)

# Node.js (pg)
const pool = new Pool({
  host: 'localhost',
  database: 'mydb',
  user: 'myuser',
  password: 'secret',
  port: 5432,
})

# Go (pgx)
conn, err := pgx.Connect(context.Background(),
    "postgresql://myuser:secret@localhost:5432/mydb")
```

## Performance Tuning

Key configuration parameters in `postgresql.conf`:

```conf
# Memory settings
shared_buffers = 256MB              # 25% of RAM
effective_cache_size = 1GB          # 50-75% of RAM
work_mem = 4MB                      # RAM / max_connections / 4
maintenance_work_mem = 64MB         # RAM / 16

# Connections
max_connections = 100               # Typical: 100-200

# Checkpoint settings
checkpoint_completion_target = 0.9
wal_buffers = 16MB

# Query planner
random_page_cost = 1.1              # SSD: 1.1, HDD: 4.0
effective_io_concurrency = 200      # SSD: 200, HDD: 2
```

## Agent Use

- Provision databases for application deployments
- Create isolated test/staging environments
- Automate backup and restore operations
- Manage database schemas and migrations
- Setup read replicas for scaling
- Configure connection pooling (PgBouncer)
- Monitor query performance and slow queries
- Automate user and permission management

## Troubleshooting

### Service won't start

Check logs:
```bash
# Linux
sudo journalctl -u postgresql -n 50

# macOS
tail -f /usr/local/var/log/postgresql@16.log
```

Common issues:
- Port already in use: Check `lsof -i :5432`
- Data directory permissions: Should be owned by postgres user
- Disk full: Check `df -h`

### Connection refused

```bash
# Check if PostgreSQL is running
sudo systemctl status postgresql  # Linux
brew services list | grep postgresql  # macOS

# Check listening ports
sudo lsof -i :5432

# Verify pg_hba.conf allows your connection
sudo cat /etc/postgresql/*/main/pg_hba.conf | grep host
```

### Authentication failed

Edit `pg_hba.conf` to allow connections:
```conf
# Trust local connections (development only!)
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust

# Password authentication (production)
local   all             all                                     md5
host    all             all             127.0.0.1/32            md5
```

Reload configuration:
```bash
sudo systemctl reload postgresql  # Linux
brew services restart postgresql  # macOS
```

### Out of disk space

Check space:
```bash
df -h

# Find large tables
psql -U postgres -c "SELECT pg_size_pretty(pg_total_relation_size('tablename'))"

# Vacuum to reclaim space
psql -U postgres -d mydb -c "VACUUM FULL"
```

## Uninstall

```yaml
- preset: postgres
  with:
    state: absent
```

Note: Data directory is preserved by default. To remove all data:
```bash
# Linux
sudo rm -rf /var/lib/postgresql

# macOS
rm -rf /usr/local/var/postgresql@*
```

## Resources

- Official docs: https://www.postgresql.org/docs/
- Tutorial: https://www.postgresqltutorial.com/
- Performance: https://www.postgresql.org/docs/current/performance-tips.html
- Extensions: https://www.postgresql.org/docs/current/contrib.html
- Search: "postgresql tutorial", "postgres performance tuning", "postgresql best practices"

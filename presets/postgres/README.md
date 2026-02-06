# PostgreSQL Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Connect to PostgreSQL
psql -U postgres

# Check status
sudo systemctl status postgresql  # Linux
brew services list | grep postgresql  # macOS

# Connect to database
psql -U postgres -d database_name
```

## Configuration

- **Config file:** `/etc/postgresql/*/main/postgresql.conf` (Linux), `/usr/local/var/postgres/postgresql.conf` (macOS)
- **Data directory:** `/var/lib/postgresql/data` (Linux), `/usr/local/var/postgres` (macOS)
- **Default port:** 5432
- **Socket:** `/var/run/postgresql/` (Linux), `/tmp` (macOS)

## Common Operations

```bash
# Restart PostgreSQL
sudo systemctl restart postgresql  # Linux
brew services restart postgresql  # macOS

# Create database
createdb mydb

# Create user
createuser myuser
psql -c "ALTER USER myuser WITH PASSWORD 'password'"

# Grant privileges
psql -c "GRANT ALL PRIVILEGES ON DATABASE mydb TO myuser"

# Backup database
pg_dump mydb > backup.sql

# Restore database
psql mydb < backup.sql

# List databases
psql -l

# Connect and run SQL
psql -U postgres -d mydb -c "SELECT version()"
```

## SQL Commands

```sql
-- List databases
\l

-- Connect to database
\c database_name

-- List tables
\dt

-- Describe table
\d table_name

-- List users
\du

-- Exit
\q
```

## Connection String

```
postgresql://user:password@localhost:5432/database
```

## Python Usage

```python
import psycopg2

conn = psycopg2.connect(
    host="localhost",
    database="mydb",
    user="myuser",
    password="password"
)

cur = conn.cursor()
cur.execute("SELECT version()")
print(cur.fetchone())
```

## Uninstall

```yaml
- preset: postgres
  with:
    state: absent
```

**Note:** Data directory is preserved after uninstall.

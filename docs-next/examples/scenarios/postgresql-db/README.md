# PostgreSQL Database Setup

Install and configure PostgreSQL on Ubuntu with a sample database schema.

## What This Does

This scenario demonstrates:

- Installing PostgreSQL database server
- Starting and enabling the PostgreSQL service
- Creating a database and user
- Granting proper permissions
- Loading initial schema with tables and data
- Verifying database connectivity
- Running queries to test the setup

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed

## Files

- `setup.yml` - Main playbook
- `files/init.sql` - Initial database schema and sample data

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom database settings
mooncake run setup.yml --var db_name=mydb --var db_user=myuser --var db_password=mypass123
```

## Variables

You can customize these variables:

- `db_name` (default: `myapp_db`) - Database name
- `db_user` (default: `myapp_user`) - Database user
- `db_password` (default: `myapp_password_123`) - User password
- `postgres_version` (default: `14`) - PostgreSQL version

## What Gets Created

### Database Objects

**Database:** `myapp_db`

**Tables:**

- `users` - User accounts with username, email, full_name
- `posts` - User posts with title and content

**Views:**

- `active_users` - View of active users only

**Functions:**

- `get_user_post_count()` - Count posts for a user

**Sample Data:**

- 4 users (Alice, Bob, Charlie, Diana)
- 4 posts

### Indexes

- Username index
- Email index
- User ID foreign key index

## Using Your Database

### Connect with psql

```bash
# As the created user
PGPASSWORD=myapp_password_123 psql -h localhost -U myapp_user -d myapp_db

# As postgres superuser
sudo -u postgres psql myapp_db
```

### Sample Queries

```sql
-- List all users
SELECT * FROM users;

-- List active users
SELECT * FROM active_users;

-- List posts with usernames
SELECT u.username, p.title, p.created_at
FROM posts p
JOIN users u ON p.user_id = u.id
ORDER BY p.created_at DESC;

-- Get post count for a user
SELECT get_user_post_count(1);

-- Insert a new user
INSERT INTO users (username, email, full_name)
VALUES ('eve', 'eve@example.com', 'Eve Wilson');

-- Insert a new post
INSERT INTO posts (user_id, title, content)
VALUES (1, 'My New Post', 'This is my newest post!');
```

### Python Connection Example

```python
import psycopg2

conn = psycopg2.connect(
    host="localhost",
    database="myapp_db",
    user="myapp_user",
    password="myapp_password_123"
)

cur = conn.cursor()
cur.execute("SELECT * FROM users;")
rows = cur.fetchall()

for row in rows:
    print(row)

cur.close()
conn.close()
```

### Node.js Connection Example

```javascript
const { Client } = require('pg');

const client = new Client({
  host: 'localhost',
  database: 'myapp_db',
  user: 'myapp_user',
  password: 'myapp_password_123',
});

client.connect();

client.query('SELECT * FROM users', (err, res) => {
  console.log(res.rows);
  client.end();
});
```

## Database Management

### Check Status

```bash
sudo systemctl status postgresql
```

### View Logs

```bash
sudo tail -f /var/log/postgresql/postgresql-*-main.log
```

### Backup Database

```bash
# As postgres user
sudo -u postgres pg_dump myapp_db > myapp_db_backup.sql

# As created user
PGPASSWORD=myapp_password_123 pg_dump -h localhost -U myapp_user myapp_db > backup.sql
```

### Restore Database

```bash
# As postgres user
sudo -u postgres psql myapp_db < myapp_db_backup.sql

# As created user
PGPASSWORD=myapp_password_123 psql -h localhost -U myapp_user myapp_db < backup.sql
```

### Access PostgreSQL Shell

```bash
# As postgres superuser
sudo -u postgres psql

# List databases
\l

# Connect to database
\c myapp_db

# List tables
\dt

# Describe table
\d users

# List users/roles
\du

# Quit
\q
```

## Cleanup

To remove the database setup:

```bash
# Drop database and user
sudo -u postgres psql -c "DROP DATABASE IF EXISTS myapp_db;"
sudo -u postgres psql -c "DROP USER IF EXISTS myapp_user;"

# Optionally remove PostgreSQL
sudo systemctl stop postgresql
sudo apt-get remove --purge postgresql postgresql-contrib
sudo rm -rf /var/lib/postgresql/
sudo rm -rf /etc/postgresql/
```

## Learning Points

This example teaches:

- Installing PostgreSQL from Ubuntu repositories
- Starting and managing PostgreSQL service
- Creating databases and users programmatically
- Setting up proper database permissions
- Running SQL scripts from files
- Testing database connectivity
- Basic SQL operations (CREATE, INSERT, SELECT)
- Using views and functions
- Database security basics

## Security Notes

**Important:** This example uses a simple password for demonstration. In production:

- Use strong, randomly generated passwords
- Store passwords in environment variables or secret management
- Configure `pg_hba.conf` for proper authentication
- Enable SSL/TLS connections
- Use connection pooling
- Regular backups and monitoring
- Keep PostgreSQL updated

## Next Steps

After setup, try:

- Adding more tables and relationships
- Creating triggers and stored procedures
- Setting up replication
- Configuring pgAdmin for GUI management
- Implementing full-text search
- Adding PostGIS for spatial data
- Performance tuning and optimization

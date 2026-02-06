# manticore - High-Performance Search Engine

Modern, full-text search engine forked from Sphinx, designed for fast search across large datasets with SQL-like query syntax and real-time indexing.

## Quick Start
```yaml
- preset: manticore
```

## Features
- **Real-time indexing**: Index documents instantly without rebuilding
- **SQL-compatible**: Query with familiar SQL syntax
- **Full-text search**: Advanced text search with stemming, morphology
- **JSON support**: Native JSON field indexing and querying
- **Fast**: Written in C++, optimized for speed
- **Distributed**: Sharding and replication for scalability

## Basic Usage
```bash
# Start Manticore server
searchd --config /etc/manticoresearch/manticore.conf

# Connect with MySQL client
mysql -h localhost -P 9306

# Create real-time index
CREATE TABLE products (title text, price float, tags multi);

# Insert document
INSERT INTO products (id, title, price, tags)
VALUES (1, 'Wireless Mouse', 29.99, (1,2,3));

# Full-text search
SELECT * FROM products WHERE MATCH('wireless');

# Filter and sort
SELECT * FROM products WHERE price < 50 ORDER BY price DESC;

# Faceted search
SELECT *, GROUPBY() as tags_count FROM products
GROUP BY tags ORDER BY tags_count DESC;
```

## Advanced Configuration
```yaml
- preset: manticore
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove manticore |

## Platform Support
- ✅ Linux (apt, yum, Docker)
- ✅ macOS (Homebrew, Docker)
- ✅ Windows (Docker)

## Configuration
- **Config file**: `/etc/manticoresearch/manticore.conf`
- **Data directory**: `/var/lib/manticore`
- **MySQL protocol**: Port 9306 (default)
- **HTTP API**: Port 9308 (default)
- **Binary protocol**: Port 9312 (default)

## Real-World Examples

### Basic Configuration
```ini
# /etc/manticoresearch/manticore.conf
searchd {
    listen = 9306:mysql41
    listen = 9308:http
    listen = 9312
    log = /var/log/manticore/searchd.log
    query_log = /var/log/manticore/query.log
    pid_file = /var/run/manticore/searchd.pid
    binlog_path = /var/lib/manticore/data
    data_dir = /var/lib/manticore
}
```

### E-Commerce Search Index
```sql
-- Create products index
CREATE TABLE products (
    title text indexed,
    description text indexed,
    category string attribute indexed,
    price float,
    stock int,
    rating float,
    tags multi,
    attributes json,
    created_at timestamp
)
min_infix_len='3'
morphology='stem_en';

-- Insert sample products
INSERT INTO products (id, title, description, category, price, stock, rating, tags, attributes)
VALUES
(1, 'Laptop Computer', 'High performance laptop', 'electronics', 999.99, 50, 4.5, (1,2), '{"brand":"Dell","color":"silver"}'),
(2, 'Wireless Mouse', 'Ergonomic wireless mouse', 'accessories', 29.99, 200, 4.8, (1,3), '{"brand":"Logitech","color":"black"}');

-- Full-text search with filters
SELECT *, WEIGHT() as relevance
FROM products
WHERE MATCH('@title,description laptop OR computer')
  AND price BETWEEN 500 AND 1500
  AND stock > 0
ORDER BY relevance DESC, rating DESC
LIMIT 10;
```

### Faceted Search
```sql
-- Get facets for filtering
SELECT category, COUNT(*) as count
FROM products
WHERE MATCH('laptop')
GROUP BY category
ORDER BY count DESC;

-- Price range facets
SELECT
    INTERVAL(price, 0, 100, 500, 1000) as price_range,
    COUNT(*) as count
FROM products
WHERE MATCH('laptop')
GROUP BY price_range;
```

### Autocomplete Search
```sql
-- Create autocomplete index
CREATE TABLE autocomplete (
    title text indexed stored,
    category string attribute
)
min_prefix_len='2'
dict='keywords'
morphology='none';

-- Autocomplete query
SELECT title FROM autocomplete
WHERE MATCH('^lap')
ORDER BY WEIGHT() DESC
LIMIT 10;
```

### JSON Field Queries
```sql
-- Query JSON attributes
SELECT * FROM products
WHERE attributes.brand = 'Dell'
  AND attributes.color = 'silver';

-- Index JSON array
SELECT * FROM products
WHERE ALL(attributes.features) = 'waterproof';
```

## Agent Use
- Product search in e-commerce applications
- Document search and knowledge bases
- Log analysis and filtering
- Real-time data indexing and querying
- Site search implementation

## Troubleshooting

### Manticore won't start
Check configuration syntax:
```bash
searchd --config /etc/manticoresearch/manticore.conf --console
```

Check logs:
```bash
tail -f /var/log/manticore/searchd.log
```

### Permission denied errors
Fix data directory permissions:
```bash
sudo chown -R manticore:manticore /var/lib/manticore
sudo chown -R manticore:manticore /var/log/manticore
```

### Out of memory errors
Tune memory settings in config:
```ini
searchd {
    max_matches = 1000
    rt_mem_limit = 128M
}
```

### Slow queries
Enable query optimization:
```sql
-- Analyze query execution
SHOW META;

-- Show query plan
EXPLAIN SELECT * FROM products WHERE MATCH('laptop');
```

Add proper indexes:
```sql
-- Create secondary indexes on frequently filtered columns
ALTER TABLE products ADD COLUMN category_idx string attribute indexed;
```

## Uninstall
```yaml
- preset: manticore
  with:
    state: absent
```

**Note**: Does not remove data directory. Remove manually if needed:
```bash
sudo rm -rf /var/lib/manticore
```

## Resources
- Official site: https://manticoresearch.com/
- Documentation: https://manual.manticoresearch.com/
- GitHub: https://github.com/manticoresoftware/manticoresearch
- Docker: https://hub.docker.com/r/manticoresearch/manticore
- Search: "manticore search engine", "manticore sphinx fork"

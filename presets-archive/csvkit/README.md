# csvkit - CSV Swiss Army Knife

Suite of command-line tools for converting to and working with CSV. SQL queries on CSV, statistics, format conversion.

## Quick Start
```yaml
- preset: csvkit
```

## Features
- **Format Conversion**: Convert Excel, JSON, SQL to CSV and vice versa
- **SQL Queries**: Run SQL queries directly on CSV files without database
- **Statistics**: Generate descriptive statistics on CSV columns
- **Column Operations**: Cut, join, sort, and stack CSV files
- **Data Cleaning**: Filter, grep, and transform CSV data
- **Python-Based**: Cross-platform with easy installation
- **Pipe-Friendly**: Unix-style command chaining

## Basic Usage
```bash
# Convert Excel to CSV
in2csv data.xlsx > data.csv

# SQL query on CSV
csvsql --query "SELECT * FROM data WHERE age > 25" data.csv

# Get statistics
csvstat data.csv

# Select columns
csvcut -c name,email,age data.csv

# Filter rows
csvgrep -c status -m "active" data.csv

# Join files
csvjoin -c id users.csv orders.csv

# Sort by column
csvsort -c age data.csv
```

## Core Tools
```bash
# in2csv - Convert various formats to CSV
in2csv data.xlsx > data.csv
in2csv data.json > data.csv

# csvcut - Select columns
csvcut -c 1,3,5 data.csv
csvcut -c name,email data.csv

# csvgrep - Filter rows
csvgrep -c status -m "active" data.csv

# csvstat - Statistics
csvstat data.csv

# csvsql - SQL queries on CSV
csvsql --query "SELECT * FROM data WHERE age > 25" data.csv

# csvjoin - Join CSV files
csvjoin -c id data1.csv data2.csv

# csvsort - Sort rows
csvsort -c age -r data.csv

# csvstack - Stack CSV files
csvstack file1.csv file2.csv file3.csv
```

## Format Conversion
```bash
# Excel to CSV
in2csv data.xlsx > data.csv
in2csv --sheet "Sheet2" data.xlsx > sheet2.csv

# JSON to CSV
in2csv data.json > data.csv

# CSV to JSON
csvjson data.csv > data.json

# Multiple sheets
in2csv data.xlsx --names  # List sheet names
in2csv --sheet "Sales" data.xlsx > sales.csv
```

## Column Selection
```bash
# Select by index
csvcut -c 1,2,3 data.csv

# Select by name
csvcut -c name,email,age data.csv

# Exclude columns
csvcut -C 4,5 data.csv  # Exclude columns 4 and 5

# Reorder columns
csvcut -c email,name,age data.csv

# Show column names
csvcut -n data.csv
```

## Filtering Rows
```bash
# Exact match
csvgrep -c status -m "active" data.csv

# Regex pattern
csvgrep -c email -r ".*@gmail\.com" data.csv

# Numeric comparison
csvgrep -c age -f age_filter.txt data.csv

# Inverse match
csvgrep -c status -m "inactive" -i data.csv

# Multiple conditions (AND)
csvgrep -c status -m "active" data.csv | \
  csvgrep -c age -r "^[3-9]"

# Case insensitive
csvgrep -c name -m "john" --case-insensitive data.csv
```

## SQL Queries
```bash
# Simple query
csvsql --query "SELECT * FROM data WHERE age > 30" data.csv

# Aggregation
csvsql --query "SELECT status, COUNT(*) as count FROM data GROUP BY status" data.csv

# Join multiple files
csvsql --query "SELECT a.name, b.department FROM users a JOIN departments b ON a.dept_id = b.id" users.csv departments.csv

# Insert into database
csvsql --db sqlite:///data.db --insert data.csv

# Query database
sql2csv --db sqlite:///data.db --query "SELECT * FROM data"
```

## Statistics
```bash
# All statistics
csvstat data.csv

# Specific columns
csvstat -c age,salary data.csv

# Summary only
csvstat --count data.csv

# Min/max/mean/median
csvstat --mean --median data.csv

# Null values
csvstat --nulls data.csv
```

## Joining Files
```bash
# Join on column
csvjoin -c id users.csv orders.csv

# Different column names
csvjoin -c "user_id,id" users.csv orders.csv

# Left join
csvjoin --left -c id users.csv orders.csv

# Outer join
csvjoin --outer -c id users.csv orders.csv
```

## Sorting
```bash
# Sort by column
csvsort -c age data.csv

# Reverse sort
csvsort -c salary -r data.csv

# Multiple columns
csvsort -c department,salary data.csv

# Numeric sort
csvsort -c amount --numeric data.csv
```

## Stacking Files
```bash
# Combine files vertically
csvstack file1.csv file2.csv file3.csv > combined.csv

# Add source column
csvstack -g "jan,feb,mar" jan.csv feb.csv mar.csv > yearly.csv

# With headers
csvstack -n source file1.csv file2.csv > output.csv
```

## Data Cleaning
```bash
# Remove duplicate rows
csvsql --query "SELECT DISTINCT * FROM data" data.csv

# Fill missing values
csvsql --query "SELECT COALESCE(age, 0) as age FROM data" data.csv

# Trim whitespace
csvcut -c name data.csv | sed 's/^ *//; s/ *$//'

# Fix encoding
iconv -f ISO-8859-1 -t UTF-8 data.csv > data_utf8.csv
```

## CI/CD Integration
```bash
# Validate CSV structure
csvstat data.csv > /dev/null || {
  echo "Invalid CSV"
  exit 1
}

# Check row count
ROWS=$(csvstat --count data.csv | tail -1)
if [ $ROWS -eq 0 ]; then
  echo "Empty CSV file"
  exit 1
fi

# Convert and validate
in2csv data.xlsx > data.csv
csvclean data.csv  # Creates data_out.csv and data_err.csv
if [ -s data_err.csv ]; then
  echo "CSV has errors"
  cat data_err.csv
  exit 1
fi
```

## Data Analysis Workflows
```bash
# Explore structure
csvcut -n data.csv  # Show columns
csvstat data.csv    # Show statistics

# Filter and summarize
csvgrep -c country -m "USA" data.csv | \
  csvsql --query "SELECT state, AVG(sales) as avg_sales FROM stdin GROUP BY state"

# Multi-step transformation
in2csv data.xlsx | \
  csvcut -c name,age,salary | \
  csvgrep -c age -r "^[3-5]" | \
  csvsort -c salary -r | \
  csvjson > output.json

# Generate report
csvstat --mean --median sales.csv > report.txt
```

## Database Integration
```bash
# Load CSV to SQLite
csvsql --db sqlite:///mydb.db --insert data.csv

# Load to PostgreSQL
csvsql --db postgresql://user:pass@localhost/mydb --insert data.csv

# Query and export
sql2csv --db sqlite:///mydb.db \
  --query "SELECT * FROM data WHERE created_at > date('now', '-7 days')" \
  > recent.csv

# Bulk import
for file in *.csv; do
  csvsql --db sqlite:///combined.db --insert "$file"
done
```

## Advanced Examples
```bash
# Top 10 by value
csvsql --query "SELECT * FROM data ORDER BY amount DESC LIMIT 10" data.csv

# Pivot table
csvsql --query "
  SELECT
    department,
    SUM(CASE WHEN quarter='Q1' THEN sales ELSE 0 END) as Q1,
    SUM(CASE WHEN quarter='Q2' THEN sales ELSE 0 END) as Q2
  FROM data
  GROUP BY department
" data.csv

# Data validation
csvstat data.csv | grep "Nulls" | \
  awk '{if ($2 > 0) print "Column", $1, "has null values"}'

# Deduplication
csvsql --query "
  SELECT *, COUNT(*) as dupe_count
  FROM data
  GROUP BY email
  HAVING COUNT(*) > 1
" data.csv
```

## Comparison
| Feature | csvkit | xsv | miller | awk |
|---------|--------|-----|--------|-----|
| SQL queries | Yes | No | Limited | No |
| Statistics | Built-in | Yes | Yes | No |
| Format conversion | Excel,JSON | Limited | JSON,more | No |
| Speed | Moderate | Fast | Fast | Fast |
| Language | Python | Rust | Go | AWK |

## Best Practices
- **Use csvclean** to find and fix malformed rows
- **Chain commands** with pipes for complex workflows
- **Use csvsql** for complex queries vs multiple filters
- **Convert to CSV first** with in2csv for consistency
- **Check column names** with `csvcut -n` before processing
- **Validate with csvstat** before loading to database
- **Use --no-header-row** for headerless files

## Tips
- Python-based (easy to install)
- Handles encoding issues well
- SQL support is powerful
- Great for Excel → CSV → Database workflows
- Works with stdin/stdout (pipeable)
- Good error messages
- Integrates with pandas workflows

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated CSV validation
- Data format conversion
- ETL pipeline preprocessing
- Report generation from CSV
- Database loading automation
- Data quality checks


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install csvkit
  preset: csvkit

- name: Use csvkit in automation
  shell: |
    # Custom configuration here
    echo "csvkit configured"
```
## Uninstall
```yaml
- preset: csvkit
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/wireservice/csvkit
- Docs: https://csvkit.readthedocs.io/
- Search: "csvkit tutorial", "csvkit examples"

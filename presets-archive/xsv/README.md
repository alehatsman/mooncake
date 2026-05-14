# xsv - Fast CSV Toolkit

Lightning-fast CSV command line toolkit in Rust. Index, slice, select, search, and analyze massive CSV files.

## Features
- **Blazing fast**: 10-100x faster than Python tools
- **Indexing**: Create indexes for instant operations
- **Statistics**: Built-in stats (mean, median, stddev)
- **Joining**: SQL-like joins on CSV files
- **Splitting**: Split by rows, size, or column value
- **Low memory**: Stream processing for huge files
- **Rich operations**: Select, search, sort, frequency analysis
- **Format conversion**: CSV, TSV, custom delimiters

## Quick Start
```yaml
- preset: xsv
```

## Basic Usage
```bash
# Show headers
xsv headers data.csv

# Count rows
xsv count data.csv

# Preview data
xsv slice -l 10 data.csv

# Pretty table
xsv table data.csv | less -S
```

## Indexing (Speed Boost)
```bash
# Create index (one-time, enables fast operations)
xsv index data.csv

# Operations are faster with index
xsv count data.csv    # Instant with index
xsv slice data.csv    # Much faster

# Check if indexed
ls data.csv.idx
```

## Selecting Columns
```bash
# Select by index
xsv select 1,2,3 data.csv

# Select by name
xsv select name,email,age data.csv

# Select range
xsv select 1-5 data.csv

# Exclude columns
xsv select '!password,!secret' data.csv

# Reorder
xsv select email,name,age data.csv
```

## Slicing Rows
```bash
# First 10 rows
xsv slice -l 10 data.csv

# Skip first 100, take 50
xsv slice -s 100 -l 50 data.csv

# Last 10 rows
xsv slice -l -10 data.csv

# Every 10th row
xsv slice -i 10 data.csv

# Random sample
xsv sample 100 data.csv
```

## Searching
```bash
# Search in all columns
xsv search "pattern" data.csv

# Search specific column
xsv search -s email "gmail" data.csv

# Case-insensitive
xsv search -i "john" data.csv

# Regex
xsv search -s phone "^\d{3}-\d{3}-\d{4}$" data.csv

# Invert match
xsv search -v "inactive" data.csv

# Select columns from search results
xsv search "pattern" data.csv | xsv select name,email
```

## Sorting
```bash
# Sort by column
xsv sort -s age data.csv

# Reverse sort
xsv sort -s salary -R data.csv

# Multiple columns
xsv sort -s department,salary data.csv

# Numeric sort
xsv sort -s amount -N data.csv

# Random shuffle
xsv shuffle data.csv
```

## Statistics
```bash
# All columns
xsv stats data.csv

# Specific columns
xsv select age,salary data.csv | xsv stats

# With nulls
xsv stats --nulls data.csv

# Output as table
xsv stats data.csv | xsv table
```

**Statistics shown:**
- Type inference (String, Integer, Float)
- Min/Max values
- Sum
- Mean/Median
- Standard deviation
- Unique values count
- Null count

## Frequency Analysis
```bash
# Count unique values
xsv frequency -s status data.csv

# Top 10 values
xsv frequency -s category data.csv | xsv slice -l 10

# Multiple columns
xsv frequency -s department,role data.csv

# With percentages
xsv frequency -s country data.csv | xsv table
```

## Joining
```bash
# Inner join
xsv join id users.csv id orders.csv

# Left join
xsv join --left id users.csv id orders.csv

# Different column names
xsv join user_id users.csv id orders.csv

# Multiple key columns
xsv join user_id,product_id file1.csv user_id,product_id file2.csv
```

## Splitting
```bash
# Split by rows (1000 rows per file)
xsv split -s 1000 output/ data.csv

# Split by size (10MB per file)
xsv split --size 10000000 output/ data.csv

# By column value
xsv partition department output/ data.csv
```

## Format Conversion
```bash
# CSV to TSV
xsv fmt -t '\t' data.csv > data.tsv

# Change delimiter
xsv fmt -d '|' data.csv > data.psv

# Fix quoting
xsv fmt data.csv > clean.csv

# Remove quotes
xsv fmt --quote-never data.csv
```

## Flattening
```bash
# Transpose
xsv transpose data.csv

# Flatten (rows to single row)
xsv flatten data.csv

# Explode (single row to multiple rows)
xsv explode -s tags data.csv
```

## Deduplication
```bash
# Remove duplicate rows
xsv dedup data.csv

# Deduplicate by column
xsv dedup -s email data.csv

# Keep duplicates
xsv dedup --dupes-output dupes.csv data.csv > unique.csv
```

## Data Validation
```bash
# Check for valid CSV
xsv count data.csv > /dev/null && echo "Valid CSV"

# Find rows with wrong column count
xsv check data.csv

# Schema inference
xsv stats data.csv | xsv select field,type

# Check for nulls
xsv stats --nulls data.csv | xsv search -s nulls -v "^0$"
```

## Filtering
```bash
# Select subset of data
xsv search -s age "^[3-5]" data.csv | \
  xsv select name,email | \
  xsv sort -s name

# Complex filtering (with external tools)
xsv select age,salary data.csv | \
  awk -F, '$1 > 30 && $2 > 50000' | \
  xsv table

# Filter by multiple columns
xsv search -s status "active" data.csv | \
  xsv search -s role "admin"
```

## CI/CD Integration
```bash
# Validate CSV
if ! xsv count data.csv > /dev/null 2>&1; then
  echo "Invalid CSV file"
  exit 1
fi

# Check minimum rows
ROWS=$(xsv count data.csv)
if [ $ROWS -lt 100 ]; then
  echo "Not enough data: $ROWS rows"
  exit 1
fi

# Verify columns exist
if ! xsv headers data.csv | grep -q "required_column"; then
  echo "Missing required column"
  exit 1
fi

# Generate summary
xsv stats data.csv | xsv select field,type,min,max > schema.csv
```

## Performance Optimization
```bash
# Always index large files first
xsv index large.csv

# Use specific columns
xsv select col1,col2 large.csv | xsv count  # Fast

# Chain operations efficiently
xsv select important_cols data.csv | \
  xsv search "pattern" | \
  xsv stats

# Parallel processing
split -l 100000 huge.csv chunk_
for chunk in chunk_*; do
  xsv stats "$chunk" > "${chunk}.stats" &
done
wait
```

## Large File Handling
```bash
# Count rows (instant with index)
xsv index huge.csv
xsv count huge.csv

# Sample for exploration
xsv sample 1000 huge.csv | xsv table

# Process in chunks
xsv split -s 100000 chunks/ huge.csv
for file in chunks/*.csv; do
  xsv stats "$file"
done

# Stream through pipeline
xsv select useful_cols huge.csv | \
  xsv search "filter" | \
  xsv sample 10000 > subset.csv
```

## Real-World Examples
```bash
# Data profiling
xsv stats data.csv | xsv table | less -S

# Find duplicates
xsv dedup -s email --dupes-output duplicates.csv users.csv

# Top 10 customers by sales
xsv sort -s total_sales -R sales.csv | \
  xsv slice -l 10 | \
  xsv select customer_name,total_sales | \
  xsv table

# Merge monthly files
xsv cat rows jan.csv feb.csv mar.csv > q1.csv

# Extract active users
xsv search -s status "active" users.csv | \
  xsv select email,created_at | \
  xsv sort -s created_at -R > active_users.csv

# Data quality report
echo "=== Data Quality Report ===" > report.txt
echo "Total rows: $(xsv count data.csv)" >> report.txt
echo "" >> report.txt
echo "Column Statistics:" >> report.txt
xsv stats --nulls data.csv | xsv table >> report.txt
```

## Comparison
| Feature | xsv | csvkit | miller | awk |
|---------|-----|--------|--------|-----|
| Speed | Fastest | Slow | Fast | Fast |
| Indexing | Yes | No | No | No |
| Large files | Excellent | Poor | Good | Good |
| Stats | Built-in | Built-in | Built-in | Manual |
| Language | Rust | Python | Go | AWK |

## Advanced Techniques
```bash
# Self-join (find duplicates)
xsv join email data.csv email data.csv | \
  xsv select email | \
  xsv dedup

# Pivot-like operation
xsv frequency -s category,status data.csv | \
  xsv table

# Running statistics
xsv stats --everything data.csv | \
  xsv transpose | \
  xsv table

# Data diff
comm -3 \
  <(xsv select id old.csv | sort) \
  <(xsv select id new.csv | sort)
```

## Best Practices
- **Always index** large files first (`xsv index`)
- **Select columns early** to reduce data size
- **Use stats** for quick data profiling
- **Sample** before processing huge files
- **Chain operations** with pipes efficiently
- **Check with xsv check** for malformed CSVs
- **Use --no-headers** for headerless files

## Tips
- 10-100x faster than Python-based tools
- Handles multi-GB files easily
- Index makes repeated operations instant
- Low memory usage (streaming)
- Single binary (easy deployment)
- No runtime dependencies
- Great for CI/CD pipelines

## Advanced Configuration

### Automated Data Pipeline
```bash
#!/bin/bash
# process-csv.sh
INPUT=$1
OUTPUT=${INPUT%.csv}_clean.csv

# Index for speed
xsv index "$INPUT"

# Validation
if ! xsv count "$INPUT" > /dev/null 2>&1; then
  echo "Invalid CSV: $INPUT"
  exit 1
fi

# Process
xsv select required_cols "$INPUT" | \
  xsv search -s status "active" | \
  xsv dedup -s email | \
  xsv sort -s created_at > "$OUTPUT"

echo "Processed: $(xsv count $OUTPUT) rows"
```

### CI/CD Validation
```yaml
# .github/workflows/validate-data.yml
- name: Validate CSV files
  run: |
    for file in data/*.csv; do
      echo "Validating $file..."
      xsv count "$file" || exit 1
      xsv check "$file" || exit 1
      ROWS=$(xsv count "$file")
      if [ $ROWS -lt 10 ]; then
        echo "Too few rows: $ROWS"
        exit 1
      fi
    done
```

### Performance Monitoring
```bash
# Monitor CSV processing performance
time xsv index large.csv
time xsv count large.csv
time xsv stats large.csv
time xsv frequency -s category large.csv | head -n 20
```

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (binary, Cargo)
- ✅ BSD systems

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove xsv |

## Agent Use
- Fast CSV validation
- Data profiling automation
- Large dataset preprocessing
- Quality check pipelines
- Schema inference
- Duplicate detection

## Uninstall
```yaml
- preset: xsv
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/BurntSushi/xsv
- Search: "xsv csv toolkit", "xsv examples"

# fx - Interactive JSON Viewer

Terminal JSON viewer with JavaScript syntax. Interactive exploration, pretty printing, and streaming JSON support.

## Quick Start
```yaml
- preset: fx
```

## Basic Usage
```bash
# View JSON file
fx data.json

# From stdin
cat data.json | fx
curl https://api.github.com/users/octocat | fx

# With query
fx data.json '.name'
echo '{"name":"alice","age":30}' | fx '.name'
```

## Interactive Mode
```bash
# Open in interactive mode
fx data.json

# Navigation keys:
  ↑/↓     - Move up/down
  ←/→     - Collapse/expand
  g/G     - Jump to top/bottom
  /       - Search
  n/N     - Next/previous search result
  .       - Enter filter mode
  q       - Quit
```

## JavaScript Queries
```bash
# Access properties
fx data.json '.users'
fx data.json '.users[0]'
fx data.json '.users[0].name'

# Array operations
fx data.json '.users.length'
fx data.json '.users.map(u => u.name)'
fx data.json '.users.filter(u => u.age > 18)'

# Chaining
fx data.json '.users.filter(u => u.active).map(u => u.email)'

# Object transformation
fx data.json 'Object.keys(this)'
fx data.json 'Object.values(this)'
fx data.json 'Object.entries(this)'

# Reduce
fx data.json '.items.reduce((sum, item) => sum + item.price, 0)'
```

## Filtering
```bash
# Simple filter
echo '[1,2,3,4,5]' | fx 'this.filter(x => x > 2)'

# Object filtering
fx users.json '.filter(u => u.role === "admin")'

# Multiple conditions
fx data.json '.filter(item => item.price > 10 && item.stock > 0)'

# Find single item
fx users.json '.find(u => u.id === 123)'

# Some/every
fx data.json '.every(item => item.validated)'
fx data.json '.some(item => item.error)'
```

## Mapping
```bash
# Extract fields
fx users.json '.map(u => u.name)'

# Transform objects
fx users.json '.map(u => ({id: u.id, email: u.email}))'

# Computed properties
fx orders.json '.map(o => ({...o, total: o.price * o.quantity}))'

# Nested mapping
fx data.json '.categories.map(c => c.items.map(i => i.name))'
```

## Sorting
```bash
# Sort numbers
fx data.json '.sort((a, b) => a - b)'

# Sort strings
fx users.json '.sort((a, b) => a.name.localeCompare(b.name))'

# Sort by property
fx items.json '.sort((a, b) => a.price - b.price)'

# Reverse sort
fx data.json '.sort((a, b) => b.value - a.value)'
```

## Aggregation
```bash
# Count
fx data.json '.length'

# Sum
fx orders.json '.reduce((sum, o) => sum + o.total, 0)'

# Average
fx scores.json '.reduce((sum, s) => sum + s, 0) / this.length'

# Min/max
fx data.json '.reduce((min, x) => Math.min(min, x), Infinity)'
fx data.json '.reduce((max, x) => Math.max(max, x), -Infinity)'

# Group by
fx users.json '.reduce((acc, u) => {
  (acc[u.role] = acc[u.role] || []).push(u);
  return acc;
}, {})'
```

## Output Formats
```bash
# Pretty print (default)
fx data.json

# Compact (one line)
fx data.json --compact

# Raw output (no quotes for strings)
fx data.json '.name' --raw-output

# Monochrome (no colors)
fx data.json --monochrome
```

## Streaming JSON
```bash
# Process large JSON files
fx large-file.json '.items.slice(0, 100)'

# Stream processing
cat stream.jsonl | fx --slurp '.map(obj => obj.id)'

# Combine multiple files
fx file1.json file2.json file3.json
```

## CI/CD Integration
```bash
# Extract value for scripts
VERSION=$(fx package.json '.version' --raw-output)

# Validation
if fx config.json '.enabled' --raw-output | grep -q true; then
  echo "Feature enabled"
fi

# Transform for deployment
fx config.json '.environments.production' > deploy-config.json

# Check array length
COUNT=$(fx data.json '.items.length' --raw-output)
if [ $COUNT -eq 0 ]; then
  echo "No items found"
  exit 1
fi
```

## API Response Processing
```bash
# GitHub API
curl -s https://api.github.com/repos/owner/repo | \
  fx '.stargazers_count'

# Extract nested data
curl -s https://api.example.com/users | \
  fx '.data.map(u => ({id: u.id, name: u.name}))'

# Filter and transform
curl -s https://api.example.com/posts | \
  fx '.filter(p => p.published).map(p => p.title)'

# Aggregate
curl -s https://api.example.com/stats | \
  fx '.reduce((sum, s) => sum + s.views, 0)'
```

## Configuration Files
```bash
# Read npm package.json
fx package.json '.scripts'
fx package.json '.dependencies'

# Update value (with jq fallback)
fx config.json | jq '.port = 3000' > config.json

# Extract environment config
fx config.json '.environments.staging'

# List keys
fx settings.json 'Object.keys(this)'
```

## Advanced Examples
```bash
# Complex transformation
fx data.json '
  this
    .filter(item => item.status === "active")
    .map(item => ({
      id: item.id,
      name: item.name.toUpperCase(),
      total: item.price * item.quantity,
      tags: item.tags.join(",")
    }))
    .sort((a, b) => b.total - a.total)
'

# Nested object navigation
fx api-response.json '
  this.data.results
    .filter(r => r.score > 0.8)
    .map(r => r.metadata.title)
'

# Date filtering
fx events.json '
  this.filter(e => new Date(e.timestamp) > new Date("2024-01-01"))
'

# String manipulation
fx users.json '
  this.map(u => ({
    ...u,
    username: u.email.split("@")[0]
  }))
'
```

## Comparison
| Feature | fx | jq | gron | jless |
|---------|-----|-----|------|-------|
| Syntax | JavaScript | jq | Flatten | Viewer |
| Interactive | Yes | No | No | Yes |
| Streaming | Yes | Yes | No | No |
| Learning curve | Low | High | Low | Low |

## Best Practices
- Use **interactive mode** for exploration
- **Filter before mapping** for performance
- Use `--raw-output` for script integration
- Leverage JavaScript's array methods
- Stream large files instead of loading all
- Use `.slice()` to preview large datasets
- Combine with curl for API workflows

## Tips
- Press `.` in interactive mode to enter filter
- JavaScript knowledge transfers directly
- Faster startup than Python-based tools
- Supports JSON, JSONL, and streaming
- Great for quick API response inspection
- Use arrow functions for concise queries
- Plays well with shell pipelines

## Agent Use
- API response parsing
- Configuration file queries
- Data transformation pipelines
- Log file analysis (JSON logs)
- CI/CD data extraction
- Interactive debugging

## Uninstall
```yaml
- preset: fx
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/antonmedv/fx
- Search: "fx json viewer", "fx javascript queries"

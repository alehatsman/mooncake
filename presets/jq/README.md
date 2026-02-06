# jq - JSON Processor

Lightweight and flexible command-line JSON processor. Query, filter, and transform JSON data with ease.

## Quick Start
```yaml
- preset: jq
```

## Basic Usage
```bash
# Pretty-print JSON
echo '{"name":"John","age":30}' | jq '.'
cat data.json | jq '.'

# Get specific field
jq '.name' data.json
jq '.user.email' data.json

# Get array element
jq '.[0]' array.json
jq '.items[2]' data.json

# Get multiple fields
jq '.name, .age' data.json
jq '{name: .name, email: .email}' data.json
```

## Filtering and Selection
```bash
# Array operations
jq '.[]' array.json                    # Iterate array elements
jq '.[].name' data.json                # Get name from each element
jq '.items[] | select(.active)' data.json  # Filter active items

# Select by condition
jq '.[] | select(.age > 25)' users.json
jq '.[] | select(.status == "active")' data.json
jq '.[] | select(.price < 100)' products.json

# Get array length
jq '. | length' array.json
jq '.items | length' data.json
```

## Transformations
```bash
# Map over array
jq '[.[] | .name]' users.json
jq 'map(.name)' users.json
jq 'map({id: .id, name: .name})' data.json

# Add/modify fields
jq '. + {new_field: "value"}' data.json
jq '.age = 35' user.json
jq '.items[].price *= 1.1' products.json  # Increase prices by 10%

# Rename fields
jq '{username: .name, email_addr: .email}' user.json

# Sort array
jq 'sort_by(.name)' users.json
jq 'sort_by(.price) | reverse' products.json

# Group by field
jq 'group_by(.category)' items.json
```

## Combining Data
```bash
# Merge objects
jq '. * {role: "admin"}' user.json
jq 'reduce .[] as $item ({}; . + $item)' objects.json

# Flatten nested arrays
jq 'flatten' nested.json
jq '[.[] | .items[]]' data.json

# Unique values
jq 'unique' array.json
jq '[.[] | .category] | unique' items.json
```

## String Operations
```bash
# String manipulation
jq '.name | ascii_downcase' data.json
jq '.name | ascii_upcase' data.json
jq '.text | split(",")' data.json
jq '.items | join(", ")' data.json

# String interpolation
jq '"Hello, \(.name)!"' user.json
jq '"\(.firstname) \(.lastname)"' user.json

# Test string
jq '.email | test("@gmail.com")' user.json
jq 'select(.name | startswith("A"))' users.json
```

## Conditional Logic
```bash
# If-then-else
jq 'if .age > 18 then "adult" else "minor" end' user.json
jq '.items[] | if .stock > 0 then "available" else "sold out" end' data.json

# Case statement
jq '
  if .status == "active" then "✓"
  elif .status == "pending" then "⏳"
  else "✗"
  end
' records.json
```

## Working with Multiple Files
```bash
# Combine multiple JSON files
jq -s '.' file1.json file2.json file3.json

# Merge arrays from multiple files
jq -s 'add' file1.json file2.json

# Compare files
jq --slurpfile data2 file2.json '. + $data2[0]' file1.json
```

## Output Formats
```bash
# Compact output (no pretty-print)
jq -c '.' data.json

# Raw output (no JSON encoding)
jq -r '.name' user.json
jq -r '.items[].name' data.json

# Tab-separated values
jq -r '.[] | [.name, .age, .city] | @tsv' users.json

# CSV output
jq -r '.[] | [.id, .name, .price] | @csv' products.json

# URL encoding
jq -r '@uri' string.json
```

## Advanced Queries
```bash
# Recursive descent
jq '.. | .name? | select(. != null)' nested.json

# Path operations
jq 'path(.items[0])' data.json
jq 'getpath(["user", "email"])' data.json

# Key operations
jq 'keys' object.json
jq 'keys_unsorted' object.json
jq 'to_entries' object.json
jq 'from_entries' array.json

# Type checking
jq 'type' data.json
jq '.[] | select(type == "string")' array.json

# Math operations
jq '[.[] | .price] | add' products.json
jq '[.[] | .score] | add / length' scores.json  # Average
jq '[.[] | .value] | min' data.json
jq '[.[] | .value] | max' data.json
```

## Real-World Examples
```bash
# Extract email list from users
jq -r '.users[].email' response.json

# Get all TODO items that are incomplete
jq '.todos[] | select(.completed == false) | .title' todos.json

# Transform API response to simple format
curl api.example.com/users | jq '[.[] | {id, name, email}]'

# Find users in specific city with age > 25
jq '.users[] | select(.city == "NYC" and .age > 25)' data.json

# Calculate total price of items in cart
jq '[.cart[].price] | add' cart.json

# Group items by category and count
jq 'group_by(.category) | map({category: .[0].category, count: length})' items.json

# Convert array of objects to object keyed by id
jq 'map({(.id): .}) | add' array.json
```

## Using with APIs
```bash
# Pretty-print API response
curl https://api.example.com/data | jq '.'

# Extract specific field from API
curl https://api.github.com/users/username | jq '.name'

# Save formatted response
curl https://api.example.com/data | jq '.' > formatted.json

# Chain API calls
curl https://api.example.com/users | jq '.[0].id' | xargs -I {} curl https://api.example.com/users/{}
```

## Configuration
- **No config file needed** - jq is stateless
- **Arguments**: Pass filters directly as command-line arguments
- **Modules**: Can define reusable functions in `.jq` files

## Agent Use
- Parse and extract data from JSON APIs
- Transform configuration files
- Filter and aggregate log data
- Data pipeline processing
- CI/CD script data manipulation
- Kubernetes resource queries

## Uninstall
```yaml
- preset: jq
  with:
    state: absent
```

## Resources
- Official docs: https://jqlang.github.io/jq/
- Interactive playground: https://jqplay.org/
- Search: "jq cookbook", "jq cheat sheet"

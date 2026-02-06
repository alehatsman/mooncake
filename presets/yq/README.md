# yq - YAML/JSON/XML Processor

Portable command-line YAML, JSON, and XML processor. Like jq, but for YAML (and more).

## Quick Start
```yaml
- preset: yq
```

## Features
- **Multi-format support**: YAML, JSON, XML, properties, CSV
- **Comment preservation**: Maintains YAML comments during transformations
- **In-place editing**: Direct file modification with automatic backups
- **Format conversion**: Seamless conversion between YAML, JSON, XML
- **Deep merge**: Intelligent merging of complex nested structures
- **Kubernetes-native**: Built-in support for multi-document YAML manifests
- **Cross-platform**: Linux, macOS, Windows

## Advanced Configuration
```yaml
# Install yq (default)
- preset: yq

# Uninstall yq
- preset: yq
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Windows (scoop, choco)

## Basic Usage
```bash
# Pretty-print YAML
yq '.' config.yaml
cat values.yaml | yq '.'

# Get specific field
yq '.database.host' config.yaml
yq '.spec.replicas' deployment.yaml

# Get array element
yq '.[0]' array.yaml
yq '.items[2]' data.yaml

# Get multiple fields
yq '.name, .version' chart.yaml
yq '{name: .name, version: .version}' data.yaml
```

## Working with YAML
```bash
# Read YAML file
yq eval '.' config.yaml
yq e '.' config.yaml  # Short form

# Modify YAML in place
yq -i '.version = "2.0"' config.yaml
yq -i '.replicas = 3' deployment.yaml

# Delete field
yq -i 'del(.deprecated_field)' config.yaml

# Add new field
yq -i '.new_field = "value"' config.yaml
```

## Array Operations
```bash
# Iterate array
yq '.items[]' data.yaml
yq '.[]' array.yaml

# Filter array
yq '.items[] | select(.active == true)' data.yaml
yq '.users[] | select(.age > 25)' users.yaml

# Get array length
yq '.items | length' data.yaml

# Append to array
yq -i '.items += ["new_item"]' data.yaml
yq -i '.tags += ["production"]' config.yaml

# Update array element
yq -i '.items[0].name = "updated"' data.yaml
```

## Kubernetes Resource Manipulation
```bash
# Get image from deployment
yq '.spec.template.spec.containers[0].image' deployment.yaml

# Update image
yq -i '.spec.template.spec.containers[0].image = "nginx:1.21"' deployment.yaml

# Update replicas
yq -i '.spec.replicas = 5' deployment.yaml

# Add label
yq -i '.metadata.labels.env = "production"' deployment.yaml

# Add annotation
yq -i '.metadata.annotations."app.kubernetes.io/version" = "1.0"' deployment.yaml

# Get all container names
yq '.spec.template.spec.containers[].name' deployment.yaml

# Update resource limits
yq -i '.spec.template.spec.containers[0].resources.limits.memory = "512Mi"' deployment.yaml
```

## Format Conversion
```bash
# YAML to JSON
yq -o json '.' config.yaml
yq eval -o json '.' config.yaml > config.json

# JSON to YAML
yq -p json '.' data.json
yq -p json eval '.' data.json > data.yaml

# XML to JSON
yq -p xml -o json '.' data.xml

# Properties to YAML
yq -p props '.' application.properties > application.yaml

# Multiple document YAML
yq eval-all '.' multi-doc.yaml
yq ea 'select(documentIndex == 0)' multi-doc.yaml  # Get first document
```

## Merging Files
```bash
# Merge two YAML files
yq eval-all '. as $item ireduce ({}; . * $item)' file1.yaml file2.yaml

# Merge with precedence
yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' base.yaml override.yaml

# Merge arrays
yq eval-all '. as $item ireduce ([]; . + $item)' array1.yaml array2.yaml

# Deep merge Helm values
yq eval-all '. as $item ireduce ({}; . *+ $item)' values.yaml values-prod.yaml
```

## Helm Values Files
```bash
# Get specific value
yq '.image.repository' values.yaml
yq '.service.port' values.yaml

# Update image tag
yq -i '.image.tag = "v1.2.3"' values.yaml

# Enable feature
yq -i '.features.monitoring = true' values.yaml

# Set nested value
yq -i '.database.postgres.host = "db.example.com"' values.yaml

# Merge environment-specific values
yq eval-all '. as $item ireduce ({}; . *+ $item)' values.yaml values-production.yaml > final-values.yaml
```

## String Operations
```bash
# String concatenation
yq '.prefix + "-" + .suffix' data.yaml

# String transformation
yq '.name | upcase' data.yaml
yq '.name | downcase' data.yaml

# String contains
yq 'select(.email | contains("@example.com"))' users.yaml

# Split string
yq '.path | split("/")' data.yaml

# Join array to string
yq '.items | join(", ")' data.yaml
```

## Conditional Logic
```bash
# If-then-else
yq 'if .env == "prod" then "production" else "development" end' config.yaml

# Select with condition
yq '.[] | select(.status == "active")' items.yaml

# Multiple conditions
yq '.[] | select(.age > 25 and .active == true)' users.yaml

# Alternative operator
yq '.optional_field // "default"' config.yaml
```

## Comments Preservation
```bash
# Read and maintain comments
yq '.' config.yaml  # Comments preserved by default

# Update value while keeping comments
yq -i '.version = "2.0"' config.yaml  # Comments preserved

# Add comment
yq -i '. | . head_comment="# Updated on 2024-01-01"' config.yaml
```

## Working with Multiple Documents
```bash
# Split multi-document YAML
yq eval-all 'select(documentIndex == 0)' multi.yaml > doc1.yaml
yq eval-all 'select(documentIndex == 1)' multi.yaml > doc2.yaml

# Process all documents
yq eval-all '.metadata.namespace = "production"' resources.yaml

# Filter documents
yq eval-all 'select(.kind == "Deployment")' resources.yaml
```

## Environment Variables
```bash
# Use environment variable
yq '.config.api_key = strenv(API_KEY)' config.yaml

# Substitute environment variables
export VERSION=1.2.3
yq -i '.version = env(VERSION)' config.yaml

# With default value
yq '.timeout = (env(TIMEOUT) // 30)' config.yaml
```

## Advanced Queries
```bash
# Recursive descent
yq '.. | select(has("password"))' config.yaml

# Get all keys
yq 'keys' object.yaml

# Get paths
yq 'path(.items[])' data.yaml

# Type checking
yq '.[] | select(type == "!!str")' data.yaml

# Calculate
yq '[.items[].price] | add' cart.yaml
yq '[.metrics[].value] | add / length' metrics.yaml  # Average
```

## CI/CD Examples
```bash
# Update deployment image in CI
yq -i '.spec.template.spec.containers[0].image = "app:$CI_COMMIT_SHA"' k8s/deployment.yaml

# Set replicas based on environment
yq -i ".spec.replicas = ${REPLICAS:-3}" deployment.yaml

# Inject secrets into config
yq -i ".database.password = \"$DB_PASSWORD\"" config.yaml

# Validate required fields exist
yq '.database.host' config.yaml || { echo "Missing database.host"; exit 1; }

# Generate ConfigMap from YAML
kubectl create configmap app-config --from-file=config.yaml --dry-run=client -o yaml | \
  yq '.data."config.yaml" = load_str("config.yaml")' | \
  kubectl apply -f -
```

## Real-World Examples
```bash
# Extract all container images from K8s manifests
yq '.. | select(has("image")).image' k8s/*.yaml | sort -u

# Update all deployments to use specific image pull policy
yq -i '.spec.template.spec.containers[].imagePullPolicy = "Always"' k8s/deploy-*.yaml

# Get all service ports
yq '.spec.ports[].port' service.yaml

# Convert Helm values to environment variables
yq -o props '.' values.yaml > .env

# Merge multiple configuration files
yq eval-all '. as $item ireduce ({}; . *+ $item)' base.yaml dev.yaml local.yaml > merged.yaml

# Validate YAML syntax
yq '.' config.yaml >/dev/null && echo "Valid YAML" || echo "Invalid YAML"

# Extract secrets that need attention
yq '.. | select(type == "!!str") | select(. == "*changeme*" or . == "*TODO*")' config.yaml
```

## Output Formats
```bash
# JSON output
yq -o json '.' config.yaml

# Compact JSON
yq -o json -I 0 '.' config.yaml

# Properties format
yq -o props '.' config.yaml

# CSV (from array)
yq -o csv '.' data.yaml

# XML output
yq -o xml '.' data.yaml

# Colored output
yq -C '.' config.yaml

# No colors
yq --no-colors '.' config.yaml
```

## Tips and Tricks
```bash
# Pretty-print and sort keys
yq 'sort_keys(.)' config.yaml

# Remove null values
yq 'del(.. | select(. == null))' config.yaml

# Compact empty arrays/objects
yq 'del(.. | select(length == 0))' config.yaml

# Show only changed fields (diff)
diff <(yq 'sort_keys(.)' old.yaml) <(yq 'sort_keys(.)' new.yaml)

# Validate all YAML files in directory
find . -name "*.yaml" -exec yq '.' {} \; >/dev/null

# Backup before in-place edit
yq -i '.version = "2.0"' config.yaml
# (yq automatically creates config.yaml.bak)
```

## Configuration
- **No config file needed** - yq is stateless
- **Comments**: Preserved by default (unlike jq)
- **Formatting**: Maintains original YAML formatting where possible

## Agent Use
- Kubernetes manifest manipulation
- Helm values file management
- CI/CD pipeline configuration
- Docker Compose file editing
- Configuration file transformations
- YAML validation and linting
- Multi-environment config management

## Uninstall
```yaml
- preset: yq
  with:
    state: absent
```

## Resources
- Official docs: https://mikefarah.gitbook.io/yq/
- GitHub: https://github.com/mikefarah/yq
- Search: "yq examples", "yq kubernetes", "yq helm values"

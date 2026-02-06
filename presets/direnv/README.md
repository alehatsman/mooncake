# direnv - Environment Switcher

Automatic environment variable loading per directory. Load/unload environment variables based on current directory.

## Quick Start
```yaml
- preset: direnv
```

## Basic Usage
```bash
# Initialize in project
cd myproject
echo 'export DATABASE_URL=postgres://localhost/mydb' > .envrc
direnv allow

# Auto-loads when entering directory
cd myproject  # DATABASE_URL is set

# Auto-unloads when leaving
cd ..  # DATABASE_URL is unset
```

## Shell Integration
```bash
# Bash (~/.bashrc)
eval "$(direnv hook bash)"

# Zsh (~/.zshrc)
eval "$(direnv hook zsh)"

# Fish (~/.config/fish/config.fish)
direnv hook fish | source

# After adding hook, restart shell or:
source ~/.bashrc
```

## .envrc Files
```bash
# Simple variables
export DATABASE_URL=postgres://localhost/dev
export API_KEY=test_key_123
export DEBUG=true

# Path modifications
PATH_add bin
PATH_add node_modules/.bin

# Load from file
dotenv .env

# Conditional
if [ "$USER" = "alice" ]; then
  export ENVIRONMENT=development
fi

# Functions
use_nix() {
  # Custom logic
}
```

## Common Patterns
```bash
# Load .env file
dotenv

# Load specific env file
dotenv .env.local

# Optional .env
dotenv_if_exists .env.local

# Python virtualenv
layout python python3.11

# Node.js project
PATH_add node_modules/.bin
export NODE_ENV=development

# Ruby
layout ruby

# Go
layout go

# Rust
watch_file Cargo.toml
eval "$(lorri direnv)"
```

## Path Management
```bash
# Add to PATH
PATH_add bin
PATH_add scripts
PATH_add node_modules/.bin

# Prepend to PATH
path_add PATH ~/custom/bin

# Python PATH
PATH_add venv/bin
```

## Python Projects
```bash
# .envrc
layout python python3.11

# Or specific version
layout python python3.9

# With virtualenv name
layout python-venv myenv

# Manual activation
source venv/bin/activate
```

## Node.js Projects
```bash
# .envrc
# Add node_modules binaries
PATH_add node_modules/.bin

# Set NODE_ENV
export NODE_ENV=development

# Load from .env
dotenv

# Use specific Node version (with asdf)
use asdf nodejs 20.10.0
```

## Go Projects
```bash
# .envrc
layout go

# Or manual
export GOPATH=$(pwd)/.go
PATH_add $GOPATH/bin

# Custom build tags
export GOTAGS=integration
```

## Ruby Projects
```bash
# .envrc
layout ruby

# Or with specific Ruby
use ruby 3.2.0

# Load bundler
eval "$(bundle env)"
```

## Secrets Management
```bash
# Load from 1Password
export API_KEY=$(op item get "API Key" --fields credential)

# Load from Vault
export DB_PASSWORD=$(vault kv get -field=password secret/db)

# From encrypted file
export $(sops -d secrets.enc.env | xargs)

# Conditional secrets
if [ -f .env.local ]; then
  dotenv .env.local
fi
```

## Multi-Environment
```bash
# .envrc
case "$USER" in
  alice)
    export ENVIRONMENT=development
    dotenv .env.dev
    ;;
  bob)
    export ENVIRONMENT=staging
    dotenv .env.staging
    ;;
esac

# Or by hostname
case "$(hostname)" in
  dev-*)
    dotenv .env.dev
    ;;
  prod-*)
    dotenv .env.prod
    ;;
esac
```

## Custom Functions
```bash
# .envrc
use_postgres() {
  export DATABASE_URL=postgres://localhost/mydb
  export PGHOST=localhost
  export PGUSER=dev
  export PGDATABASE=mydb
}

use_postgres

# AWS profile
use_aws() {
  export AWS_PROFILE=$1
  export AWS_REGION=us-east-1
}

use_aws development
```

## Allow/Deny
```bash
# Allow .envrc (required after changes)
direnv allow

# Allow with path
direnv allow /path/to/project

# Deny (block)
direnv deny

# Check status
direnv status
```

## Watching Files
```bash
# .envrc
# Reload when file changes
watch_file config.yaml

# Watch multiple files
watch_file package.json
watch_file Gemfile

# Reload on file change
if [[ -f config.yaml ]]; then
  export CONFIG=$(cat config.yaml)
fi
```

## Layouts
```bash
# Python
layout python python3.11

# Python with venv
layout python-venv

# Node
layout node

# Go
layout go

# Ruby
layout ruby

# PHP
layout php

# Custom layout
layout() {
  # Your custom logic
}
```

## Direnv Library
```bash
# ~/.config/direnv/direnvrc
# Custom layouts and functions

layout_python-custom() {
  local python=${1:-python3}
  [[ $# -gt 0 ]] && shift
  unset PYTHONHOME

  if [[ -d venv ]]; then
    source venv/bin/activate
  else
    $python -m venv venv
    source venv/bin/activate
  fi
}

# Use in .envrc
layout python-custom python3.11
```

## Debugging
```bash
# Show what will be loaded
direnv export bash

# Verbose mode
direnv allow -v

# Check status
direnv status

# Show environment diff
direnv exec . env | diff <(env) -

# Reload
direnv reload
```

## CI/CD Integration
```bash
# Load .envrc in CI (not automatic)
# GitHub Actions
- name: Load environment
  run: |
    eval "$(direnv export bash)"
    echo "DATABASE_URL=$DATABASE_URL" >> $GITHUB_ENV

# Or skip direnv in CI
if ! command -v direnv &> /dev/null; then
  source .env
fi
```

## Security Best Practices
```bash
# Never commit .envrc with secrets
echo '.envrc' >> .gitignore

# Use .envrc.sample for team
# .envrc.sample (committed)
export DATABASE_URL=postgres://localhost/mydb
export API_KEY=your_key_here

# .envrc (not committed)
export DATABASE_URL=postgres://prod/db
export API_KEY=real_secret_key

# Git commit hook
# Prevent committing .envrc
echo '
if git diff --cached --name-only | grep -q "^.envrc$"; then
  echo "Error: .envrc should not be committed"
  exit 1
fi
' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## Project Templates
```bash
# Web app
export DATABASE_URL=postgres://localhost/myapp_dev
export REDIS_URL=redis://localhost:6379
export SECRET_KEY_BASE=$(openssl rand -hex 64)
PATH_add bin
dotenv_if_exists .env.local

# Microservice
export SERVICE_NAME=myservice
export PORT=3000
export LOG_LEVEL=debug
export JAEGER_ENDPOINT=http://localhost:14268
PATH_add scripts

# Data science
layout python python3.11
export JUPYTER_CONFIG_DIR=./.jupyter
export DATA_DIR=./data
PATH_add notebooks
```

## Comparison
| Feature | direnv | dotenv | envrc | autoenv |
|---------|--------|--------|-------|---------|
| Auto-load | Yes | No | Manual | Yes |
| Auto-unload | Yes | No | No | Yes |
| Security | Allow list | N/A | N/A | Risky |
| Layouts | Yes | No | No | No |
| Language support | Many | N/A | N/A | Limited |

## Best Practices
- **Always .gitignore .envrc** if it contains secrets
- **Use .envrc.sample** for team templates
- **Explicitly allow** after every .envrc change
- **Use layouts** for language-specific setup
- **Watch config files** for auto-reload
- **Never commit secrets** to version control
- **Use secret managers** (1Password, Vault) for sensitive data

## Tips
- Automatic environment switching
- No manual source commands
- Works with any shell
- Per-directory configuration
- Unloads on exit
- Fast (< 5ms overhead)
- Secure (allow-list model)

## Agent Use
- Automated development environments
- Multi-project setup
- Secret injection
- CI/CD environment management
- Team environment consistency
- Configuration automation

## Uninstall
```yaml
- preset: direnv
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/direnv/direnv
- Docs: https://direnv.net/
- Wiki: https://github.com/direnv/direnv/wiki
- Search: "direnv examples", "direnv python"

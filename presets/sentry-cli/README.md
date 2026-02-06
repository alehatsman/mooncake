# sentry-cli - Sentry Command Line Interface

Command-line tool for Sentry error tracking. Upload source maps, manage releases, send events, and interact with Sentry projects from CI/CD pipelines.

## Quick Start
```yaml
- preset: sentry-cli
```

## Features
- **Release management**: Create and manage releases
- **Source maps**: Upload JavaScript/TypeScript source maps
- **Debug symbols**: Upload native debug symbols (iOS, Android, C++)
- **Events**: Send custom events and errors
- **Deploy tracking**: Mark deployments and track health
- **Multi-platform**: Works with web, mobile, and native apps
- **CI/CD ready**: Designed for automation pipelines

## Basic Usage
```bash
# Check version
sentry-cli --version

# Login
sentry-cli login

# Create release
sentry-cli releases new 1.0.0

# Upload source maps
sentry-cli releases files 1.0.0 upload-sourcemaps ./dist

# Finalize release
sentry-cli releases finalize 1.0.0

# List releases
sentry-cli releases list

# Send test event
sentry-cli send-event -m "Test event"
```

## Authentication

### Interactive Login
```bash
# Browser-based auth
sentry-cli login

# Stores token in ~/.sentryclirc
```

### Auth Token
```bash
# Environment variable
export SENTRY_AUTH_TOKEN=your-token-here

# Config file (~/.sentryclirc)
[auth]
token=your-token-here

# Command line
sentry-cli --auth-token YOUR_TOKEN releases list
```

### Organization and Project
```bash
# Set defaults
export SENTRY_ORG=my-org
export SENTRY_PROJECT=my-project

# Or in .sentryclirc
[defaults]
org=my-org
project=my-project
url=https://sentry.io/
```

## Release Management

### Create Release
```bash
# Simple release
sentry-cli releases new 1.0.0

# With version control
sentry-cli releases new -p my-project 1.0.0

# Auto-detect from Git
sentry-cli releases new $(git describe --tags)
```

### Upload Source Maps
```bash
# Upload directory
sentry-cli releases files 1.0.0 upload-sourcemaps ./dist

# With URL prefix
sentry-cli releases files 1.0.0 upload-sourcemaps ./dist \
  --url-prefix '~/static/js/'

# Specific files
sentry-cli releases files 1.0.0 upload-sourcemaps \
  app.min.js app.min.js.map

# Ignore certain files
sentry-cli releases files 1.0.0 upload-sourcemaps ./dist \
  --ignore node_modules
```

### Associate Commits
```bash
# Auto-discover commits
sentry-cli releases set-commits 1.0.0 --auto

# Specify repository
sentry-cli releases set-commits 1.0.0 --commit "my-repo@abc123"

# Range of commits
sentry-cli releases set-commits 1.0.0 --commit "my-repo@v0.9.0..v1.0.0"
```

### Deploy Tracking
```bash
# Mark deployment
sentry-cli releases deploys 1.0.0 new -e production

# With environment
sentry-cli releases deploys 1.0.0 new \
  -e production \
  -n "Production Deploy" \
  --started $(date -u +%s) \
  --finished $(date -u +%s)
```

### Finalize Release
```bash
# Mark release as finished
sentry-cli releases finalize 1.0.0

# With timestamp
sentry-cli releases finalize 1.0.0 --started $(date -u +%s)
```

## CI/CD Integration

### GitHub Actions - Node.js
```yaml
- name: Install sentry-cli
  run: npm install -g @sentry/cli

- name: Build
  run: npm run build

- name: Create Sentry release
  env:
    SENTRY_AUTH_TOKEN: ${{ secrets.SENTRY_AUTH_TOKEN }}
    SENTRY_ORG: my-org
    SENTRY_PROJECT: my-project
  run: |
    VERSION=$(git describe --tags)
    sentry-cli releases new $VERSION
    sentry-cli releases set-commits $VERSION --auto
    sentry-cli releases files $VERSION upload-sourcemaps ./dist \
      --url-prefix '~/static/'
    sentry-cli releases finalize $VERSION
    sentry-cli releases deploys $VERSION new -e production
```

### GitLab CI
```yaml
variables:
  SENTRY_ORG: my-org
  SENTRY_PROJECT: my-project

deploy:
  script:
    - npm install -g @sentry/cli
    - npm run build
    - VERSION=$(git describe --tags)
    - sentry-cli releases new $VERSION
    - sentry-cli releases files $VERSION upload-sourcemaps ./dist
    - sentry-cli releases finalize $VERSION
  only:
    - main
```

### Docker Build
```dockerfile
# Install sentry-cli
RUN curl -sL https://sentry.io/get-cli/ | bash

# Build and upload
RUN npm run build && \
    sentry-cli releases new $VERSION && \
    sentry-cli releases files $VERSION upload-sourcemaps ./dist && \
    sentry-cli releases finalize $VERSION
```

## Mobile Apps

### iOS - Upload dSYM
```bash
# Upload debug symbols
sentry-cli upload-dif --org my-org --project ios-app \
  path/to/App.app.dSYM

# From Xcode archive
sentry-cli upload-dif --org my-org --project ios-app \
  ~/Library/Developer/Xcode/Archives/*/*.xcarchive

# With BCSymbolMaps
sentry-cli upload-dif \
  --include-sources \
  path/to/symbols/
```

### Android - ProGuard Mapping
```bash
# Upload ProGuard mapping
sentry-cli upload-proguard \
  --android-manifest app/build/outputs/AndroidManifest.xml \
  --write-properties app/build/outputs/sentry-debug-meta.properties \
  app/build/outputs/mapping/release/

# With specific version
sentry-cli releases files 1.0.0 upload-proguard \
  app/build/outputs/mapping/release/mapping.txt
```

### React Native
```bash
# Generate and upload source maps
react-native bundle \
  --platform android \
  --dev false \
  --entry-file index.js \
  --bundle-output android.bundle \
  --sourcemap-output android.bundle.map

sentry-cli releases files 1.0.0 upload-sourcemaps \
  android.bundle android.bundle.map \
  --rewrite
```

## Advanced Features

### Custom Events
```bash
# Send event
sentry-cli send-event -m "Deployment completed"

# With level and tags
sentry-cli send-event \
  -m "Critical error" \
  --level error \
  --tag environment:production \
  --tag region:us-east-1

# From JSON
echo '{"message": "Test"}' | sentry-cli send-event
```

### Project Management
```bash
# List projects
sentry-cli projects list

# Create project
sentry-cli projects create --org my-org --team my-team new-project

# List issues
sentry-cli issues list --status unresolved
```

### Info Commands
```bash
# Get info
sentry-cli info

# Debug output
sentry-cli --log-level=debug releases list
```

## Configuration

### Config File Locations
1. `.sentryclirc` in project root
2. `~/.sentryclirc` (home directory)
3. Environment variables

### .sentryclirc Format
```ini
[auth]
token=your-auth-token

[defaults]
url=https://sentry.io/
org=my-organization
project=my-project

[http]
keepalive=true
timeout=30
```

### Environment Variables
```bash
# Authentication
export SENTRY_AUTH_TOKEN=token
export SENTRY_API_KEY=key  # Legacy

# Project
export SENTRY_ORG=my-org
export SENTRY_PROJECT=my-project
export SENTRY_URL=https://sentry.io/

# Behavior
export SENTRY_LOG_LEVEL=info
export SENTRY_NO_PROGRESS_BAR=1
```

## Real-World Examples

### Complete Deployment Workflow
```yaml
- name: Install dependencies
  shell: npm ci

- name: Build application
  shell: npm run build
  environment:
    NODE_ENV: production

- name: Create Sentry release
  vars:
    version: "{{ lookup('pipe', 'git describe --tags') }}"
  shell: |
    sentry-cli releases new {{ version }}
    sentry-cli releases set-commits {{ version }} --auto
  environment:
    SENTRY_AUTH_TOKEN: "{{ sentry_token }}"
    SENTRY_ORG: "{{ org_name }}"
    SENTRY_PROJECT: "{{ project_name }}"

- name: Upload source maps
  shell: |
    sentry-cli releases files {{ version }} upload-sourcemaps ./dist \
      --url-prefix '~/assets/' \
      --ignore node_modules \
      --validate
  environment:
    SENTRY_AUTH_TOKEN: "{{ sentry_token }}"

- name: Deploy application
  shell: kubectl apply -f deployment.yaml
  register: deploy

- name: Mark deployment in Sentry
  when: deploy.rc == 0
  shell: |
    sentry-cli releases deploys {{ version }} new \
      -e {{ environment }} \
      -n "Deploy to {{ environment }}"
    sentry-cli releases finalize {{ version }}
  environment:
    SENTRY_AUTH_TOKEN: "{{ sentry_token }}"
```

### Monorepo with Multiple Projects
```bash
# Upload for frontend
sentry-cli releases new frontend@1.0.0
sentry-cli releases files frontend@1.0.0 \
  upload-sourcemaps ./apps/frontend/dist \
  --url-prefix '~/frontend/'

# Upload for backend
sentry-cli releases new backend@1.0.0
sentry-cli releases files backend@1.0.0 \
  upload-sourcemaps ./apps/backend/dist \
  --url-prefix '~/api/'

# Finalize both
sentry-cli releases finalize frontend@1.0.0
sentry-cli releases finalize backend@1.0.0
```

## Troubleshooting

### Upload Failures
```bash
# Increase timeout
export SENTRY_HTTP_TIMEOUT=60

# Enable debug logging
sentry-cli --log-level=debug releases files 1.0.0 upload-sourcemaps ./dist

# Verify files
ls -R dist/
```

### Authentication Issues
```bash
# Check token
sentry-cli info

# Test auth
sentry-cli projects list

# Clear cache
rm ~/.sentryclirc
```

### Source Maps Not Working
```bash
# Validate source maps
sentry-cli releases files 1.0.0 upload-sourcemaps ./dist --validate

# Check URL prefix
# URL in Sentry error: https://example.com/static/js/main.123.js
# Use: --url-prefix '~/static/js/'

# List uploaded files
sentry-cli releases files 1.0.0 list
```

## Best Practices
- Store auth token in secrets management (not in code)
- Use semantic versioning for releases
- Always upload source maps for JavaScript/TypeScript
- Associate commits with releases for better context
- Mark deployments to track error rates per deploy
- Finalize releases when deployment complete
- Use `--validate` flag to catch issues early
- Clean up old releases periodically

## Platform Support
- ✅ Linux (glibc)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows
- ✅ Docker containers

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated release creation in CI/CD
- Source map upload automation
- Deployment tracking
- Error monitoring integration
- Release health monitoring
- Multi-environment deployments

## Advanced Configuration
```yaml
- preset: sentry-cli
  with:
    state: present
```

## Uninstall
```yaml
- preset: sentry-cli
  with:
    state: absent
```

## Resources
- Documentation: https://docs.sentry.io/cli/
- GitHub: https://github.com/getsentry/sentry-cli
- Installation: https://docs.sentry.io/cli/installation/
- Releases: https://github.com/getsentry/sentry-cli/releases
- Search: "sentry-cli source maps", "sentry-cli ci cd", "sentry-cli releases"

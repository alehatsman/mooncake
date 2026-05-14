# hadolint - Dockerfile Linter

Smarter Dockerfile linter with best practices validation. Catches errors, enforces conventions, suggests improvements.

## Quick Start
```yaml
- preset: hadolint
```


## Features
- **Cross-platform**: Linux and macOS support
- **Simple installation**: One command to install
- **Package manager integration**: Uses system package managers
- **Easy uninstall**: Clean removal with `state: absent`
## Basic Usage
```bash
# Lint single Dockerfile
hadolint Dockerfile

# Lint with custom name
hadolint Dockerfile.prod

# Multiple files
hadolint Dockerfile*

# From stdin
docker run --rm -i hadolint/hadolint < Dockerfile
```


## Advanced Configuration
```yaml
- preset: hadolint
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove hadolint |
## Output Formats
```bash
# Default (colorized)
hadolint Dockerfile

# JSON
hadolint --format json Dockerfile

# Checkstyle (for CI)
hadolint --format checkstyle Dockerfile

# CodeClimate
hadolint --format codeclimate Dockerfile

# GitLab Code Quality
hadolint --format gitlab_codequality Dockerfile

# SARIF (for GitHub)
hadolint --format sarif Dockerfile

# TTY (color output)
hadolint --format tty Dockerfile
```

## Common Rules
```dockerfile
# DL3006: Always tag images
FROM ubuntu:latest  # Bad
FROM ubuntu:22.04   # Good

# DL3008: Pin package versions (apt)
RUN apt-get install nginx  # Bad
RUN apt-get install nginx=1.18.0-0ubuntu1  # Good

# DL3009: Delete apt cache
RUN apt-get update && apt-get install nginx
# Bad

RUN apt-get update && apt-get install nginx \
  && rm -rf /var/lib/apt/lists/*
# Good

# DL3015: Avoid ADD for remote URLs
ADD https://example.com/file.tar.gz /tmp/  # Bad
RUN curl -o /tmp/file.tar.gz https://example.com/file.tar.gz  # Good

# DL3020: Use COPY instead of ADD
ADD file.txt /app/  # Bad
COPY file.txt /app/  # Good

# DL3025: Use JSON format for CMD/ENTRYPOINT
CMD /bin/sh -c "nginx"  # Bad
CMD ["nginx", "-g", "daemon off;"]  # Good

# DL3059: Multiple consecutive RUN
RUN apt-get update
RUN apt-get install nginx
# Bad - creates extra layers

RUN apt-get update && apt-get install nginx
# Good - single layer
```

## Ignoring Rules
```bash
# Ignore specific rules
hadolint --ignore DL3006 Dockerfile

# Multiple ignores
hadolint --ignore DL3006 --ignore DL3008 Dockerfile

# Inline ignore
# hadolint ignore=DL3006
FROM ubuntu:latest

# Inline multiple
# hadolint ignore=DL3006,DL3008
```

## Configuration File
```yaml
# .hadolint.yaml
ignored:
  - DL3006  # Allow untagged FROM
  - DL3008  # Allow unpinned apt packages

trustedRegistries:
  - docker.io
  - gcr.io
  - ghcr.io

label-schema:
  author: text
  version: semver
  maintainer: email
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Lint Dockerfile
  run: hadolint Dockerfile

# With specific format
- name: Lint Dockerfile
  run: |
    hadolint --format sarif Dockerfile > hadolint.sarif

# GitLab CI
lint:dockerfile:
  image: hadolint/hadolint:latest-debian
  script:
    - hadolint Dockerfile
  allow_failure: false

# CircleCI
- run:
    name: Lint Dockerfile
    command: |
      docker run --rm -i hadolint/hadolint < Dockerfile
```

## GitHub Actions Example
```yaml
name: Lint

on: [push, pull_request]

jobs:
  hadolint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run hadolint
        uses: hadolint/hadolint-action@v3.1.0
        with:
          dockerfile: Dockerfile
          format: sarif
          output-file: hadolint.sarif

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: hadolint.sarif
```

## Pre-commit Hook
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/hadolint/hadolint
    rev: v2.12.0
    hooks:
      - id: hadolint
        args: [--ignore, DL3008]
```

## Docker Integration
```bash
# Run in Docker
docker run --rm -i hadolint/hadolint < Dockerfile

# With config file
docker run --rm -i \
  -v $(pwd)/.hadolint.yaml:/.config/hadolint.yaml \
  hadolint/hadolint < Dockerfile

# Multiple files
find . -name Dockerfile* | xargs -I {} \
  docker run --rm -i hadolint/hadolint < {}
```

## Rule Categories
```bash
# DL3xxx - Dockerfile best practices
# DL4xxx - Maintainer deprecated
# DL5xxx - pip best practices
# DL6xxx - pipenv best practices
# DL7xxx - npm best practices
# DL8xxx - apt best practices
# DL9xxx - apk best practices
# SC xxxx - ShellCheck rules
```

## Advanced Usage
```bash
# Strict mode (all warnings are errors)
hadolint --strict-labels Dockerfile

# Require labels
hadolint --require-label maintainer:email \
  --require-label version:semver \
  Dockerfile

# Failure threshold
hadolint --failure-threshold warning Dockerfile

# No color output
hadolint --no-color Dockerfile

# Verbose
hadolint -v Dockerfile
```

## Best Practices Enforced
```dockerfile
# Use specific base image versions
FROM node:18-alpine  # Good

# Avoid latest
FROM node:latest  # Warning

# Minimize layers
RUN apt-get update \
  && apt-get install -y nginx \
  && rm -rf /var/lib/apt/lists/*

# Use COPY over ADD
COPY app.js /app/

# JSON array for CMD
CMD ["node", "server.js"]

# Don't run as root
USER node

# Use WORKDIR
WORKDIR /app

# Label schema
LABEL maintainer="team@example.com" \
      version="1.0.0" \
      description="My app"
```

## Fixing Common Issues
```dockerfile
# Issue: DL3018 - Pin apk package versions
# Before
RUN apk add nginx

# After
RUN apk add --no-cache nginx=1.22.1-r0

# Issue: DL3003 - Use WORKDIR instead of cd
# Before
RUN cd /app && npm install

# After
WORKDIR /app
RUN npm install

# Issue: DL3045 - COPY with more than 2 arguments requires --chown
# Before
COPY --chown=node:node package*.json ./

# After (if only 2 args)
COPY package.json package-lock.json ./
RUN chown -R node:node /app

# Issue: SC2046 - Quote to prevent word splitting
# Before
RUN echo $(cat file.txt)

# After
RUN echo "$(cat file.txt)"
```

## Integration with Other Tools
```bash
# With Trivy (scan after lint)
hadolint Dockerfile && \
  docker build -t myapp . && \
  trivy image myapp

# With Docker build
hadolint Dockerfile && docker build -t myapp .

# With dive (analyze layers)
hadolint Dockerfile && \
  docker build -t myapp . && \
  dive myapp
```

## Comparison
| Feature | hadolint | dockerfile_lint | FROM:latest |
|---------|----------|-----------------|-------------|
| Rules | 100+ | 50+ | 10+ |
| ShellCheck | Yes | No | No |
| Config file | Yes | Yes | No |
| CI/CD | Easy | Moderate | Limited |
| Speed | Fast | Fast | Fast |

## Tips
- Catches 90% of common Dockerfile issues
- Includes ShellCheck for RUN instructions
- JSON output great for CI/CD
- Rules based on Docker best practices
- Fast (< 1 second)
- Works offline
- No dependencies

## Agent Use
- Automated Dockerfile validation
- CI/CD quality gates
- Pre-commit hooks
- Security baseline enforcement
- Best practices compliance
- Multi-stage build verification

## Uninstall
```yaml
- preset: hadolint
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/hadolint/hadolint
- Rules: https://github.com/hadolint/hadolint#rules
- Search: "hadolint rules", "dockerfile linter"

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

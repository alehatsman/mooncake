# task - Task Runner

Task runner and build tool. Define tasks in YAML with dependencies, variables, and cross-platform support.

## Quick Start
```yaml
- preset: task
```

## Features
- **YAML configuration**: Easy-to-read Taskfile.yml instead of cryptic Makefile syntax
- **Smart caching**: Incremental builds with sources/generates tracking
- **Cross-platform**: Works seamlessly on Linux, macOS, Windows
- **Parallel execution**: Run dependent tasks concurrently
- **Variable support**: Global and task-local variables with shell expansion
- **Task includes**: Organize large projects with multiple Taskfiles
- **Watch mode**: Auto-rebuild on file changes

## Advanced Configuration
```yaml
# Install task (default)
- preset: task

# Uninstall task
- preset: task
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
# List available tasks
task --list
task -l

# Run a task
task build
task test
task deploy

# Run with variables
task deploy ENV=production

# Run silently
task --silent build

# Run with output
task --verbose test
```

## Taskfile Syntax
Create `Taskfile.yml` in project root:

```yaml
version: '3'

tasks:
  # Simple task
  build:
    cmds:
      - go build -o bin/app

  # Task with description
  test:
    desc: Run all tests
    cmds:
      - go test ./...

  # Task with dependencies
  deploy:
    deps: [build, test]
    cmds:
      - kubectl apply -f k8s/

  # Task with variables
  serve:
    cmds:
      - python -m http.server {{.PORT}}
    vars:
      PORT: 8080

  # Task with sources and generates (smart caching)
  compile:
    sources:
      - src/**/*.go
    generates:
      - bin/app
    cmds:
      - go build -o bin/app
```

## Common Patterns
```yaml
version: '3'

# Global variables
vars:
  APP_NAME: myapp
  VERSION:
    sh: git describe --tags --always

# Environment variables
env:
  CGO_ENABLED: 0
  GOOS: linux

tasks:
  # Default task
  default:
    cmds:
      - task --list

  # Multi-command task
  ci:
    desc: CI pipeline
    cmds:
      - task: lint
      - task: test
      - task: build

  # Conditional execution
  install:
    cmds:
      - cmd: npm install
        platforms: [darwin, linux]
      - cmd: choco install nodejs
        platforms: [windows]

  # Task with preconditions
  deploy:
    preconditions:
      - test -f bin/app
      - kubectl cluster-info
    cmds:
      - kubectl apply -f k8s/

  # Silent task
  clean:
    silent: true
    cmds:
      - rm -rf build/
      - rm -rf dist/

  # Task that ignores errors
  cleanup:
    ignore_error: true
    cmds:
      - docker rm -f mycontainer
      - rm -f *.tmp
```

## Real-World Examples

### Go Project
```yaml
version: '3'

vars:
  BINARY: app
  VERSION:
    sh: git describe --tags --always

tasks:
  default:
    cmds:
      - task --list

  build:
    desc: Build binary
    sources:
      - '**/*.go'
    generates:
      - bin/{{.BINARY}}
    cmds:
      - go build -o bin/{{.BINARY}}

  test:
    desc: Run tests
    cmds:
      - go test -v -race -coverprofile=coverage.out ./...

  lint:
    desc: Run linters
    cmds:
      - golangci-lint run

  run:
    desc: Run application
    deps: [build]
    cmds:
      - ./bin/{{.BINARY}}

  ci:
    desc: Full CI pipeline
    cmds:
      - task: lint
      - task: test
      - task: build
```

### Docker Workflow
```yaml
version: '3'

vars:
  IMAGE: myapp
  TAG:
    sh: git rev-parse --short HEAD

tasks:
  build:
    desc: Build Docker image
    cmds:
      - docker build -t {{.IMAGE}}:{{.TAG}} .
      - docker tag {{.IMAGE}}:{{.TAG}} {{.IMAGE}}:latest

  push:
    desc: Push to registry
    deps: [build]
    cmds:
      - docker push {{.IMAGE}}:{{.TAG}}
      - docker push {{.IMAGE}}:latest

  run:
    desc: Run container locally
    cmds:
      - docker run -p 8080:8080 {{.IMAGE}}:{{.TAG}}

  stop:
    desc: Stop container
    cmds:
      - docker stop {{.IMAGE}} || true

  logs:
    desc: View container logs
    cmds:
      - docker logs -f {{.IMAGE}}
```

### Node.js Project
```yaml
version: '3'

tasks:
  install:
    desc: Install dependencies
    sources:
      - package.json
      - package-lock.json
    generates:
      - node_modules/.installed
    cmds:
      - npm ci
      - touch node_modules/.installed

  dev:
    desc: Start dev server
    deps: [install]
    cmds:
      - npm run dev

  build:
    desc: Production build
    deps: [install]
    sources:
      - src/**/*
    generates:
      - dist/**/*
    cmds:
      - npm run build

  test:
    desc: Run tests
    deps: [install]
    cmds:
      - npm test

  lint:
    desc: Run ESLint
    deps: [install]
    cmds:
      - npm run lint

  format:
    desc: Format code
    deps: [install]
    cmds:
      - npm run format

  clean:
    desc: Clean build artifacts
    cmds:
      - rm -rf dist/ node_modules/ .next/
```

## Advanced Features

### Task Includes
```yaml
# Taskfile.yml
version: '3'

includes:
  docker: ./docker/Taskfile.yml
  k8s: ./k8s/Taskfile.yml

tasks:
  all:
    cmds:
      - task: docker:build
      - task: k8s:deploy
```

### Dynamic Variables
```yaml
tasks:
  deploy:
    vars:
      TIMESTAMP:
        sh: date +%Y%m%d_%H%M%S
      GIT_SHA:
        sh: git rev-parse HEAD
    cmds:
      - echo "Deploying {{.GIT_SHA}} at {{.TIMESTAMP}}"
```

### Task Aliases
```yaml
tasks:
  build:
    aliases: [b, compile]
    cmds:
      - go build

  test:
    aliases: [t]
    cmds:
      - go test ./...
```

## Watch Mode
```yaml
tasks:
  watch:
    desc: Watch for changes
    watch: true
    sources:
      - '**/*.go'
    cmds:
      - task: build
      - task: test
```

Run with: `task --watch watch`

## Tips
- **Smart caching**: Use `sources` and `generates` for incremental builds
- **Dependencies**: `deps` run in parallel, sequential deps use `cmds: [task: name]`
- **Cross-platform**: Use `platforms` for OS-specific commands
- **Variables**: Global in `vars:`, task-local in task's `vars:`
- **Dotenv**: Auto-loads `.env` files
- **Includes**: Organize large projects with multiple Taskfiles

## vs Make
| Feature | task | Make |
|---------|------|------|
| Config format | YAML | Makefile |
| Learning curve | Easy | Steep |
| Cross-platform | Excellent | Challenging |
| Incremental builds | Yes (sources/generates) | Yes (timestamps) |
| Parallel execution | Yes (deps) | Yes (-j) |

## Configuration
```yaml
# Taskfile.yml
version: '3'

# Global settings
set: ['pipefail']
shopt: ['globstar']
dotenv: ['.env', '.env.local']
silent: false
method: checksum  # or timestamp or none

# Task settings
tasks:
  example:
    silent: true
    method: timestamp
    run: once  # or always or when_changed
    cmds:
      - echo "Hello"
```

## Agent Use
- Consistent command interface across projects
- Cross-platform CI/CD pipelines
- Smart caching for build optimization
- Complex dependency management

## Uninstall
```yaml
- preset: task
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/go-task/task
- Docs: https://taskfile.dev/
- Search: "taskfile examples", "task vs make"

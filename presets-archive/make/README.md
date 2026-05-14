# make - GNU Make Build Automation

Classic build automation tool that uses Makefiles to define tasks and their dependencies for compiling code, running tests, and automating workflows.

## Quick Start
```yaml
- preset: make
```

## Features
- **Dependency tracking**: Automatically rebuilds only what changed
- **Parallel execution**: Run independent tasks concurrently with -j
- **Pattern rules**: Define templates for similar build steps
- **Cross-platform**: Works on all Unix-like systems
- **Universal standard**: Used across countless projects
- **Simple syntax**: Easy to learn, powerful capabilities

## Basic Usage
```bash
# Run default target
make

# Run specific target
make build

# Run multiple targets
make clean build test

# Parallel execution (4 jobs)
make -j4

# Dry run (show what would be executed)
make -n

# Show variables
make -p

# Force rebuild
make -B

# Keep going after errors
make -k

# Silent mode
make -s
```

## Advanced Configuration
```yaml
- preset: make
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove make |

## Platform Support
- ✅ Linux (pre-installed or apt, dnf, yum)
- ✅ macOS (Xcode Command Line Tools)
- ✅ BSD (pre-installed)
- ✅ Windows (via MinGW, WSL, or Cygwin)

## Configuration
- **Makefile**: Project build instructions (Makefile or makefile)
- **GNUmakefile**: GNU Make-specific features
- **Environment variables**: Can override Makefile variables

## Real-World Examples

### Simple Makefile
```makefile
# Variables
CC = gcc
CFLAGS = -Wall -O2
TARGET = myapp

# Default target
all: $(TARGET)

# Build target
$(TARGET): main.o utils.o
	$(CC) $(CFLAGS) -o $@ $^

# Object files
%.o: %.c
	$(CC) $(CFLAGS) -c $<

# Clean build artifacts
clean:
	rm -f *.o $(TARGET)

# Run tests
test: $(TARGET)
	./$(TARGET) --test

.PHONY: all clean test
```

### Modern Project Makefile
```makefile
.PHONY: all build test clean install help

# Default goal
.DEFAULT_GOAL := help

## build: Compile the application
build:
	go build -o bin/app ./cmd/app

## test: Run unit tests
test:
	go test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## lint: Run linters
lint:
	golangci-lint run

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out

## install: Install binary to /usr/local/bin
install: build
	sudo cp bin/app /usr/local/bin/

## help: Show this help message
help:
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
```

### CI/CD Integration
```yaml
- name: Install dependencies
  shell: make deps

- name: Build application
  shell: make build

- name: Run tests
  shell: make test

- name: Deploy
  shell: make deploy ENV=production
  when: branch == "main"
```

### Docker Multi-Stage Build
```makefile
.PHONY: docker-build docker-push

IMAGE_NAME = myapp
VERSION = $(shell git describe --tags --always)

docker-build:
	docker build -t $(IMAGE_NAME):$(VERSION) .
	docker tag $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME):latest

docker-push: docker-build
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

docker-run:
	docker run -p 8080:8080 $(IMAGE_NAME):latest
```

## Agent Use
- Build automation in CI/CD pipelines
- Project compilation and packaging
- Development workflow standardization
- Multi-language project orchestration
- Deployment automation

## Troubleshooting

### Make: command not found
Install make:
```bash
# Linux
sudo apt install make        # Debian/Ubuntu
sudo dnf install make        # Fedora
sudo yum install make        # CentOS/RHEL

# macOS
xcode-select --install
```

### Tab vs spaces error
Makefiles require TABS, not spaces:
```makefile
target:
<TAB>command here    # Must be TAB, not spaces
```

### Variable not expanding
Use parentheses or braces:
```makefile
# Wrong
echo $VAR

# Correct
echo $(VAR)
echo ${VAR}
```

### Parallel build failures
Some targets can't run in parallel:
```makefile
# Force sequential execution
.NOTPARALLEL: target1 target2
```

## Common Patterns

### Phony Targets
```makefile
.PHONY: all build test clean

# Prevents conflict with files named "test" or "clean"
```

### Automatic Variables
```makefile
%.o: %.c
	$(CC) -c $< -o $@
# $< = first prerequisite (%.c)
# $@ = target name (%.o)
# $^ = all prerequisites
```

### Conditional Execution
```makefile
ifeq ($(ENV),production)
    FLAGS = -O3
else
    FLAGS = -g -O0
endif
```

## Uninstall
```yaml
- preset: make
  with:
    state: absent
```

**Note**: On many systems, make is a core utility and removing it may affect system functionality.

## Resources
- GNU Make manual: https://www.gnu.org/software/make/manual/
- Makefile tutorial: https://makefiletutorial.com/
- Search: "makefile tutorial", "gnu make examples", "makefile best practices"

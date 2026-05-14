# stack - Haskell Build Tool

Cross-platform build tool for Haskell projects. Reproducible builds with automatic dependency management and GHC version selection.

## Quick Start
```yaml
- preset: stack
```

## Features
- **Reproducible builds**: Locked dependencies via stack.yaml and Stackage snapshots
- **Auto GHC management**: Downloads and manages GHC versions automatically
- **Multi-package projects**: Build and test multiple packages together
- **Stackage integration**: Curated package sets for compatibility
- **Cross-platform**: Works seamlessly on Linux, macOS, Windows
- **Fast compilation**: Incremental builds and parallel compilation
- **Docker integration**: Build in containers for reproducibility

## Basic Usage
```bash
# Create new project
stack new myproject
cd myproject

# Build project
stack build

# Run executable
stack exec myproject-exe

# Run tests
stack test

# Start REPL
stack ghci

# Clean build artifacts
stack clean
```

## Advanced Configuration
```yaml
# Install stack (default)
- preset: stack

# Uninstall stack
- preset: stack
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ✅ Windows (scoop, choco)

## Project Structure
```
myproject/
├── stack.yaml              # Stack configuration
├── package.yaml            # Hpack configuration (or .cabal file)
├── myproject.cabal         # Cabal package description
├── app/
│   └── Main.hs             # Main executable
├── src/
│   └── Lib.hs              # Library code
└── test/
    └── Spec.hs             # Test suite
```

## Configuration
- **Global config**: `~/.stack/config.yaml`
- **Project config**: `stack.yaml` in project root
- **GHC versions**: `~/.stack/programs/` - Downloaded GHC compilers
- **Dependencies**: `~/.stack/snapshots/` - Built dependencies
- **Build artifacts**: `.stack-work/` in project directory

## Stack Configuration
```yaml
# stack.yaml
resolver: lts-21.22         # Stackage LTS version

packages:
  - .                       # Current directory

extra-deps:                 # Packages not in Stackage
  - acme-missiles-0.3

flags:
  package-name:
    flag-name: true

ghc-options:
  "$everything": -O2        # Optimize all packages
  "$locals": -Wall          # Warn for local packages
```

## Common Commands
```bash
# Initialize project
stack new myproject simple    # Simple template
stack new myproject yesod     # Yesod web framework

# Setup GHC
stack setup                   # Install GHC for current project

# Build
stack build                   # Build all targets
stack build --fast            # Fast build (no optimization)
stack build --pedantic        # Strict warnings

# Run
stack exec myproject          # Run executable
stack run                     # Build and run

# Test
stack test                    # Run all tests
stack test --coverage         # Generate coverage report

# REPL
stack ghci                    # Start GHCi with project loaded
stack repl                    # Alias for ghci

# Install
stack install                 # Copy executables to ~/.local/bin

# Clean
stack clean                   # Remove build artifacts
stack purge                   # Remove .stack-work entirely
```

## Dependency Management
```yaml
# Use Stackage snapshot (recommended)
resolver: lts-21.22

# Override specific package version
extra-deps:
  - text-2.0.1
  - bytestring-0.11.4.0

# Use package from Git
extra-deps:
  - git: https://github.com/user/repo
    commit: abc123

# Use local package
packages:
  - .
  - ../shared-library
```

## Real-World Examples

### Web Application (Servant)
```yaml
# stack.yaml
resolver: lts-21.22

packages:
  - .

extra-deps:
  - servant-0.19
  - servant-server-0.19
```

```haskell
-- Main.hs
{-# LANGUAGE DataKinds #-}
{-# LANGUAGE TypeOperators #-}

import Servant
import Network.Wai.Handler.Warp

type API = "hello" :> Get '[PlainText] String

server :: Server API
server = return "Hello, world!"

main :: IO ()
main = run 8080 (serve (Proxy :: Proxy API) server)
```

### Library with Tests
```yaml
# package.yaml
name: my-library
version: 0.1.0.0

dependencies:
  - base >= 4.7 && < 5
  - text

library:
  source-dirs: src

tests:
  my-library-test:
    main: Spec.hs
    source-dirs: test
    dependencies:
      - my-library
      - hspec
```

### Multi-Package Project
```yaml
# stack.yaml
resolver: lts-21.22

packages:
  - backend
  - frontend
  - shared

# backend/package.yaml
name: backend
dependencies:
  - shared
  - servant

# frontend/package.yaml
name: frontend
dependencies:
  - shared
  - reflex
```

## Build Options
```bash
# Optimization
stack build --fast              # No optimization (faster builds)
stack build --optimize          # Enable optimizations

# Parallel compilation
stack build --jobs=4            # Use 4 cores
stack build -j                  # Use all cores

# Specific targets
stack build myproject:lib       # Build library only
stack build myproject:exe       # Build executable only
stack build myproject:test      # Build tests

# File watching
stack build --file-watch        # Rebuild on file changes
stack test --file-watch         # Rerun tests on changes
```

## Docker Integration
```yaml
# stack.yaml
resolver: lts-21.22
docker:
  enable: true
  image: fpco/stack-build:lts-21.22
```

```bash
# Build in Docker
stack --docker build

# Run in Docker
stack --docker exec myproject
```

## CI/CD Integration
```yaml
# GitHub Actions
- name: Setup Stack
  uses: haskell/actions/setup@v2
  with:
    ghc-version: '9.4.5'
    enable-stack: true

- name: Build
  run: |
    stack build --test --no-run-tests

- name: Test
  run: stack test
```

## Troubleshooting

### GHC version conflicts
```bash
# Check current GHC
stack ghc -- --version

# Use specific resolver
stack build --resolver lts-21.22

# Install GHC for resolver
stack setup
```

### Dependency issues
```bash
# Update package index
stack update

# Check dependency tree
stack dot --external | dot -Tpng -o deps.png

# Force rebuild
stack clean && stack build
```

### Out of memory
```bash
# Reduce parallelism
stack build -j1

# Increase RTS memory
stack build --ghc-options="+RTS -M2G -RTS"
```

## Tips
- Use `--fast` during development for faster builds
- `--file-watch` for continuous testing
- Pin resolver version for reproducibility
- Use Stackage LTS for stable package sets
- `stack exec` to run with correct environment
- Keep `.stack-work/` in `.gitignore`

## Best Practices
- **Lock dependencies**: Commit stack.yaml.lock to version control
- **Use Stackage**: Leverage curated package sets
- **Test automation**: Run tests in CI/CD
- **Documentation**: Use Haddock comments
- **Code formatting**: Use ormolu or stylish-haskell
- **Linting**: Enable -Wall and other GHC warnings

## Agent Use
- Automated Haskell project builds
- Dependency management
- Multi-package project orchestration
- Reproducible build environments
- CI/CD pipeline integration
- Cross-platform Haskell development

## Uninstall
```yaml
- preset: stack
  with:
    state: absent
```

**Note:** Global Stack configuration and downloaded GHC versions (`~/.stack/`) are preserved after uninstall.

## Resources
- Official docs: https://docs.haskellstack.org/
- Stackage: https://www.stackage.org/
- GitHub: https://github.com/commercialhaskell/stack
- Search: "stack haskell tutorial", "stackage lts"

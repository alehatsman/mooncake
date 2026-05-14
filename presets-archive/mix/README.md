# Mix - Elixir Build Tool and Task Runner

Build and manage Elixir projects with dependency management, task execution, and interactive shell access.

## Quick Start

```yaml
- preset: mix
```

## Features

- **Build tool**: Compile Elixir projects with automatic dependency resolution
- **Package management**: Add, update, and manage project dependencies
- **Task runner**: Execute predefined and custom tasks within projects
- **Interactive shell**: Access Elixir REPL (iex) with project context loaded
- **Testing**: Run unit and integration tests with proper environment setup
- **Cross-platform**: Works on Linux and macOS with Erlang/OTP support

## Basic Usage

```bash
# Check version and verify installation
mix --version

# Initialize new Elixir project
mix new my_app
cd my_app

# Install dependencies
mix deps.get

# Compile project
mix compile

# Run tests
mix test

# Execute custom task
mix ecto.migrate

# Interactive shell with project loaded
iex -S mix
```

## Advanced Configuration

```yaml
# Install with all options
- preset: mix
  with:
    state: present  # Install the tool
  become: true      # Use sudo if needed
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (`present`) or remove (`absent`) |

## Platform Support

- ✅ Linux (apt, dnf, pacman, yum)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

- **Elixir home**: `~/.asdf/shims/` (if using asdf) or system PATH
- **Mix cache**: `~/.mix/` (stores dependencies and builds)
- **OTP installation**: Required for Elixir runtime
- **Build directory**: `_build/` (in project directory)

## Real-World Examples

### Elixir Web Application Setup

```bash
# Create new Phoenix web app
mix phx.new my_website --live
cd my_website

# Install dependencies
mix deps.get

# Create database
mix ecto.create

# Run development server
mix phx.server
```

### Testing in CI/CD Pipeline

```yaml
# Test project before deployment
- preset: mix
  become: true

- name: Run tests
  shell: cd /app && mix test
  register: test_result

- name: Verify tests passed
  assert:
    command:
      cmd: "echo {{ test_result.rc }}"
      exit_code: 0
```

### Project Migration and Release

```bash
# Run database migrations
mix ecto.migrate

# Create optimized release
mix release

# Start the release
_build/prod/rel/my_app/bin/my_app start
```

## Agent Use

- Automated Elixir project initialization and configuration
- Dependency management and version updates in CI/CD pipelines
- Running test suites with coverage collection
- Building and deploying Elixir/Phoenix applications
- Extracting project metadata and version information

## Troubleshooting

### Mix command not found

Mix is installed as part of Elixir. Verify Elixir is installed:

```bash
# Check if Elixir is available
elixir --version

# If not, reinstall the preset
mooncake run -c preset.yml
```

### Dependency resolution errors

```bash
# Force dependency fetch and compilation
mix deps.get
mix clean
mix compile
```

### OTP version mismatch

Ensure Erlang/OTP is installed - Elixir requires it:

```bash
# Check OTP version
erl -eval 'erlang:halt()'

# On macOS, reinstall with Homebrew
brew install erlang
```

## Uninstall

```yaml
- preset: mix
  with:
    state: absent
```

This removes Elixir and Mix. Project files and dependencies are preserved.

## Resources

- Official docs: https://hexdocs.pm/mix/
- GitHub: https://github.com/elixir-lang/elixir
- Elixir guide: https://elixir-lang.org/getting-started/introduction.html
- Search: "elixir mix tutorial", "mix new project", "elixir dependencies"

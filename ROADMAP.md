# Mooncake Roadmap

This document outlines the vision and planned features for Mooncake.

## Project Vision

Mooncake aims to be a simple, powerful provisioning tool that:
- Makes system configuration accessible and maintainable
- Provides excellent error messages and debugging
- Works seamlessly across Linux, macOS, and Windows
- Maintains a small, focused feature set

## Current Status (v0.x)

✅ **Core Features**
- Shell command execution
- File and directory operations
- Template rendering (pongo2)
- Conditionals (when)
- Variables and system facts
- Loops (with_items, with_filetree)
- Include files and variables
- Tag filtering
- Register (capture output)
- Sudo/privilege escalation
- Dry-run mode
- Animated TUI
- System information (explain command)

## Release Planning

### v1.0 - Stability & Polish (Target: Q2 2026)

**Focus:** Production-ready release with excellent UX

- [ ] **Validation & Error Messages**
  - [ ] Comprehensive validation for all step types
  - [ ] Clear error messages with suggestions
  - [ ] Better line number reporting
  - [ ] Validation of template syntax before rendering

- [ ] **Documentation**
  - [x] Complete README with all features
  - [x] Comprehensive examples (01-10)
  - [ ] Video tutorials
  - [ ] API documentation (godoc)

- [ ] **Testing**
  - [ ] Increase test coverage to 90%+
  - [ ] Integration tests for real-world scenarios
  - [ ] Cross-platform testing (Linux, macOS, Windows)

- [ ] **Performance**
  - [ ] Parallel execution where possible
  - [ ] Optimize file operations
  - [ ] Benchmark suite

### v1.1 - Enhanced Features (Target: Q3 2026)

**Focus:** Quality-of-life improvements

- [ ] **Better Loops**
  - [ ] with_dict for iterating over key-value pairs
  - [ ] with_sequence for numeric ranges
  - [ ] Loop variables (index, first, last)

- [ ] **Improved Conditionals**
  - [ ] unless (inverse of when)
  - [ ] changed_when and failed_when
  - [ ] Multiple when conditions (any/all)

- [ ] **File Operations**
  - [ ] File copying (copy action)
  - [ ] File synchronization (sync action)
  - [ ] Archive operations (unzip, untar)
  - [ ] File permissions detection and preservation

- [ ] **Template Enhancements**
  - [ ] Custom filters
  - [ ] Template includes/inheritance
  - [ ] Multiple template engines (Go templates option)

### v1.2 - Developer Experience (Target: Q4 2026)

**Focus:** Making Mooncake easier to use and debug

- [ ] **Debugging**
  - [ ] Interactive debugger (step through configs)
  - [ ] Breakpoints in configurations
  - [ ] Variable inspector
  - [ ] Step timing and profiling

- [ ] **IDE Support**
  - [ ] YAML schema for autocompletion
  - [ ] VS Code extension
  - [ ] Syntax highlighting
  - [ ] Linting integration

- [ ] **CLI Improvements**
  - [ ] Interactive mode for step selection
  - [ ] Config validation command
  - [ ] Diff mode (show what changed)
  - [ ] Graph visualization of includes

### v2.0 - Advanced Features (Target: 2027)

**Focus:** Power user features

- [ ] **Remote Execution**
  - [ ] SSH support for remote hosts
  - [ ] Host inventory management
  - [ ] Parallel execution across hosts

- [ ] **Advanced Control Flow**
  - [ ] Handlers (trigger on change)
  - [ ] Rescue blocks (error handling)
  - [ ] Retry logic with backoff
  - [ ] Async tasks

- [ ] **Secrets Management**
  - [ ] Encrypted variables (vault)
  - [ ] Environment variable loading
  - [ ] Secret providers (AWS, HashiCorp Vault)

- [ ] **Extensibility**
  - [ ] Plugin system
  - [ ] Custom actions
  - [ ] External modules

## Feature Considerations

Features we're considering but not committed to:

### Under Consideration
- **Ansible compatibility layer** - Import existing Ansible playbooks
- **Container support** - Docker/Podman operations
- **Package management abstraction** - Universal package installer
- **Service management** - systemd/launchd abstraction
- **Network operations** - Download files, API calls
- **Database operations** - SQL execution, migrations
- **Cloud provider support** - AWS, GCP, Azure resources

### Explicitly Not Planned
- **Configuration management daemon** - Mooncake is run-once, not agent-based
- **Built-in orchestration** - Use external tools for complex orchestration
- **GUI** - CLI and TUI only
- **DSL other than YAML** - YAML is our configuration language

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute to these features.

### How to Propose Features

1. Check if feature is already in roadmap or issues
2. Open a GitHub issue with `[Feature Request]` prefix
3. Describe use case and proposed solution
4. For complex features, create a proposal in `docs/proposals/`

### Priority Guidelines

Features are prioritized based on:
1. **User value** - How many users benefit?
2. **Complexity** - Implementation effort required
3. **Maintenance** - Ongoing maintenance burden
4. **Alignment** - Fits project vision?

## Version History

### v0.1 - Initial Release
- Basic shell, file, template actions
- Variables and conditionals
- Include support

### v0.2 - Loops and Tags
- with_items and with_filetree
- Tag filtering
- Register for capturing output

### v0.3 - UX Improvements
- Animated TUI
- Dry-run mode
- System facts (explain command)
- Improved error messages

---

**Last Updated:** February 2026

**Questions?** Open an issue or start a discussion!

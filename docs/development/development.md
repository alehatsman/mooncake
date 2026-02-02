# Development Guide

Quick guide for contributing to Mooncake.

## Quick Start

1. **Check what's needed:** Browse [issues](https://github.com/alehatsman/mooncake/issues) or [roadmap](roadmap.md)
2. **Create issue:** Describe the bug/feature
3. **Create branch:** Use naming pattern below
4. **Make changes:** Write code + tests
5. **Submit PR:** Follow checklist

**For major features:** Write a [proposal](proposals.md) first for design feedback.

## Project Organization

- **[Roadmap](roadmap.md)** - Release planning and strategic vision
- **[GitHub Issues](https://github.com/alehatsman/mooncake/issues)** - Bug tracking and task management
- **[Proposals](proposals.md)** - Design documents for major features

Use labels: `good first issue`, `help wanted`, `priority: high`, `breaking-change`

## Branch Strategy

**Branch naming:**
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation
- `test/description` - Tests
- `refactor/description` - Refactoring

**Commit messages:**
```
Brief summary (50 chars or less)

Optional detailed explanation. Use present tense.
Explain what and why, not how.

Closes: #123
```

## Testing Requirements

All PRs must include tests (80%+ coverage for new features).

**Run tests:**
```bash
go test ./...                    # All tests
go test ./... -race              # With race detection
go test ./... -coverprofile=c.out  # Coverage report
```

**Bug fixes:** Add test reproducing the bug first.

## Documentation Requirements

Update docs for user-facing changes:
- [ ] README.md or docs/
- [ ] examples/ (if new feature)
- [ ] Code comments (exported functions)
- [ ] ROADMAP.md (check off completed items)

## Pull Request Process

**Before submitting:**
```bash
go fmt ./...   # Format
go vet ./...   # Lint
go test ./...  # Test
```

**PR description should include:**
- What this PR does
- Why it's needed
- How it was tested

**Review:** CI must pass → maintainer reviews → address feedback → merge

## Code Organization

**Key packages:**
- `internal/config/` - Configuration parsing and validation
- `internal/executor/` - Execution engine (action handlers)
- `internal/facts/` - System information collection
- `internal/logger/` - Console and TUI output
- `internal/template/` - Template rendering
- `internal/expression/` - Condition evaluation

**Adding a new action:** See [contributing.md](contributing.md) for detailed patterns.

**Quick pattern:**
1. Add struct to `internal/config/config.go`
2. Create handler in `internal/executor/[action]_step.go`
3. Update dispatcher in `internal/executor/executor.go`
4. Write tests in `internal/executor/[action]_step_test.go`
5. Update docs and examples

## Release Process

**Semantic versioning:** `MAJOR.MINOR.PATCH`

Full release process: [releasing.md](releasing.md)

## Debugging

**Enable debug logging:**
```bash
mooncake run --log-level debug
```

**Run specific test:**
```bash
go test -v -run TestName ./internal/executor
```

**Check coverage:**
```bash
go test -coverprofile=c.out ./...
go tool cover -html=c.out
```

## Resources

- **Main guide:** [Guide](../index.md)
- **Contributing:** [Contributing](contributing.md)
- **Roadmap:** [Roadmap](roadmap.md)
- **Examples:** [Examples](../examples/index.md)
- **Proposals:** [Proposals](proposals.md)

## Questions?

- Open an issue
- Start a discussion
- Check [Contributing](contributing.md)

# Development Guide

Guide for developing and contributing to Mooncake.

## Project Management System

Mooncake uses a three-tier system for managing development:

### 1. ROADMAP.md - Strategic Vision

**Purpose:** High-level roadmap and release planning

**Contains:**
- Project vision and goals
- Release milestones (v1.0, v1.1, v2.0)
- Planned features grouped by version
- Features under consideration
- Features explicitly not planned

**When to update:**
- Planning new releases
- Quarterly reviews
- Major feature decisions
- After completing milestones

**Link:** [ROADMAP.md](roadmap.md)

---

### 2. GitHub Issues - Task Tracking

**Purpose:** Detailed tracking of bugs and features

**Issue types:**
- 🐛 **Bug** - Something is broken
- ✨ **Feature** - New functionality
- 📝 **Documentation** - Docs improvements
- 🧪 **Testing** - Test coverage improvements
- 🎨 **UX** - User experience improvements

**Labels:**
- `good first issue` - Easy for newcomers
- `help wanted` - Looking for contributors
- `priority: high` - Important issues
- `proposal` - Feature proposals
- `breaking-change` - Breaking changes

**Workflow:**
1. Create issue
2. Discuss approach
3. Assign to milestone
4. Link to PR when working
5. Close when merged

---

### 3. docs/proposals/ - Detailed Designs

**Purpose:** Detailed technical design for complex features

**When to create:**
- Major new features
- Breaking changes
- Architectural decisions
- Complex implementations

**Template:** See [proposals](proposals.md)

**Example:** See [proposals](proposals.md) for examples

**Process:**
1. Copy template
2. Fill in details
3. Open PR with proposal
4. Community reviews
5. Accept/reject decision
6. Implement if accepted

---

## Development Workflow

### Starting New Work

**1. Check existing work:**
```bash
# Check issues
# Check ROADMAP.md
# Check proposals/
```

**2. For bugs:**
- Create issue if not exists
- Include reproduction steps
- Create branch: `fix/issue-description`
- Fix and test
- Submit PR

**3. For small features:**
- Create issue
- Discuss approach
- Create branch: `feature/feature-name`
- Implement with tests
- Update docs
- Submit PR

**4. For major features:**
- Check ROADMAP.md
- Create issue
- Write proposal in docs/proposals/
- Get feedback
- After approval: implement
- Create branch: `feature/feature-name`
- Implement in phases
- Submit PR(s)

### Branch Strategy

```
main (stable)
  ├── feature/with-dict-iteration
  ├── fix/register-nil-check
  ├── docs/improve-readme
  └── test/increase-coverage
```

**Branch naming:**
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation only
- `test/description` - Tests only
- `refactor/description` - Code refactoring

### Commit Messages

**Format:**
```
Brief summary (50 chars or less)

More detailed explanation if needed. Wrap at 72 characters.
Explain what and why, not how.

- Bullet points are fine
- Use present tense: "Add feature" not "Added feature"

Related: #123
Closes: #456
```

**Examples:**
```
Add with_dict loop iteration support

Implements dictionary iteration allowing users to loop over key-value
pairs. Each iteration provides item.key and item.value variables.

Closes: #123
```

```
Fix nil pointer in register when step fails

Check for nil result before accessing fields to prevent panic when
steps fail during register operations.

Fixes: #456
```

### Testing Requirements

All PRs must include tests:

**Unit tests:**
```go
func TestFeature(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

**Coverage requirements:**
- New features: 80%+ coverage
- Bug fixes: Add test reproducing bug
- Refactoring: Maintain or improve coverage

**Run tests:**
```bash
# All tests
go test ./...

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# With race detection
go test ./... -race

# Specific package
go test ./internal/executor -v
```

### Documentation Requirements

**Must update if changed:**
- [ ] README.md - If user-facing change
- [ ] examples/ - Add example if new feature
- [ ] ROADMAP.md - Check off if from roadmap
- [ ] Code comments - Document exported functions

**Documentation checklist:**
- [ ] Feature explained clearly
- [ ] Code examples provided
- [ ] Error cases documented
- [ ] Links to related docs

### Pull Request Process

**1. Before submitting:**
```bash
# Format code
go fmt ./...

# Run linter
go vet ./...

# Run tests
go test ./...

# Check coverage
go test ./... -coverprofile=coverage.out
```

**2. PR description:**
- What: What does this PR do?
- Why: Why is this needed?
- How: How was it implemented?
- Testing: How was it tested?
- Checklist: Complete the checklist

**3. Review process:**
- CI checks must pass
- Code review by maintainer
- Address feedback
- Squash commits if needed
- Merge when approved

---

## Code Organization

### Package Structure

```
internal/
├── config/          # Configuration parsing
│   ├── config.go           # Step definitions
│   ├── reader.go           # YAML parsing
│   ├── validator.go        # Validation logic
│   └── diagnostic.go       # Error reporting
│
├── executor/        # Execution engine
│   ├── executor.go         # Main execution logic
│   ├── context.go          # Execution context
│   ├── shell_step.go       # Shell handler
│   ├── file_step.go        # File handler
│   ├── template_step.go    # Template handler
│   ├── result.go           # Result handling
│   └── dryrun.go           # Dry-run logging
│
├── expression/      # Condition evaluation
│   └── evaluator.go        # When condition eval
│
├── facts/           # System information
│   ├── facts.go            # Fact collection
│   ├── linux.go            # Linux facts
│   ├── darwin.go           # macOS facts
│   └── windows.go          # Windows facts
│
├── filetree/        # File tree walking
│   └── walker.go           # Directory iteration
│
├── logger/          # Logging and UI
│   ├── logger.go           # Logger interface
│   ├── console_logger.go   # Console output
│   └── tui_logger.go       # Animated TUI
│
├── pathutil/        # Path handling
│   └── path.go             # Path resolution
│
└── template/        # Template rendering
    └── renderer.go         # Pongo2 wrapper
```

### Adding New Features

**New action type (e.g., `copy`):**

1. Add to config:
```go
// internal/config/config.go
type Copy struct {
    Src  string `yaml:"src"`
    Dest string `yaml:"dest"`
}

type Step struct {
    // ...
    Copy *Copy `yaml:"copy"`
}
```

2. Add handler:
```go
// internal/executor/copy_step.go
func HandleCopy(step config.Step, ec *ExecutionContext) error {
    // Implementation
}
```

3. Update dispatcher:
```go
// internal/executor/executor.go
func dispatchStepAction(step config.Step, ec *ExecutionContext) error {
    switch {
    case step.Copy != nil:
        return HandleCopy(step, ec)
    // ... other cases
    }
}
```

4. Add tests:
```go
// internal/executor/copy_step_test.go
func TestHandleCopy(t *testing.T) {
    // Tests
}
```

5. Document:
- Update README.md
- Add example
- Update ROADMAP.md

---

## Release Process

### Version Numbering

Semantic versioning: `MAJOR.MINOR.PATCH`

- **MAJOR:** Breaking changes
- **MINOR:** New features (backward compatible)
- **PATCH:** Bug fixes

### Release Checklist

**Before release:**
- [ ] All milestone issues closed
- [ ] Tests passing
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped

**Release steps:**
1. Create release branch
2. Update version
3. Tag release
4. Build binaries
5. Create GitHub release
6. Announce release

---

## Debugging Tips

### Debug Mode

```bash
mooncake run --config config.yml --log-level debug
```

### Common Issues

**Test failing:**
```bash
# Run specific test
go test -v -run TestName ./internal/executor

# With debug output
go test -v ./internal/executor
```

**Coverage too low:**
```bash
# See uncovered lines
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Race condition:**
```bash
go test -race ./...
```

---

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

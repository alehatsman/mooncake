# Contributing to Mooncake

Thanks for your interest in contributing to Mooncake! 🎉

## Ways to Contribute

- 🐛 Report bugs
- 💡 Suggest features
- 📝 Improve documentation
- 🧪 Add tests
- ✨ Implement features
- 📚 Create examples
- 🎨 Improve UX

## Getting Started

### Development Setup

1. **Clone the repository**
```bash
git clone https://github.com/alehatsman/mooncake.git
cd mooncake
```

2. **Install dependencies**
```bash
go mod download
```

3. **Run tests**
```bash
go test ./...
```

4. **Run with coverage**
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

5. **Build locally**
```bash
go build -o mooncake cmd/mooncake.go
./mooncake --help
```

### Project Structure

```
mooncake/
├── cmd/
│   └── mooncake.go          # CLI entry point
├── internal/
│   ├── config/              # Configuration parsing and validation
│   ├── executor/            # Step execution logic
│   ├── expression/          # Condition evaluation
│   ├── facts/               # System information collection
│   ├── filetree/            # File tree iteration
│   ├── logger/              # Logging and TUI
│   ├── pathutil/            # Path resolution
│   └── template/            # Template rendering
├── examples/                # Example configurations
├── README.md                # Main documentation
├── ROADMAP.md              # Feature roadmap
└── CONTRIBUTING.md         # This file
```

## Contribution Workflow

### 1. Find or Create an Issue

- Check existing [issues](https://github.com/alehatsman/mooncake/issues)
- For bugs: describe steps to reproduce
- For features: explain use case and proposed solution
- Wait for discussion/approval before starting work

### 2. Fork and Branch

```bash
# Fork the repo on GitHub, then:
git clone https://github.com/YOUR_USERNAME/mooncake.git
cd mooncake
git checkout -b feature/your-feature-name
```

Branch naming:
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation
- `test/description` - Tests only

### 3. Make Your Changes

**Write good commit messages:**
```
Add support for with_dict loop iteration

- Implement DictIterator in filetree package
- Add with_dict handling in executor
- Add tests for dict iteration
- Update documentation with examples
```

**Follow Go conventions:**
- Run `go fmt ./...`
- Run `go vet ./...`
- Add tests for new code
- Update documentation

**Keep commits focused:**
- One logical change per commit
- Separate refactoring from features
- Separate tests from implementation

### 4. Add Tests

All new features must include tests:

```go
// internal/executor/executor_test.go
func TestWithDict(t *testing.T) {
    // Arrange
    config := []config.Step{
        {
            Name: "Test dict iteration",
            Shell: pointer("echo {{item.key}}: {{item.value}}"),
            WithDict: pointer("{{my_dict}}"),
        },
    }

    // Act
    result := Execute(config, context)

    // Assert
    assert.NoError(t, result.Error)
    assert.Equal(t, 3, result.StepsExecuted)
}
```

**Test coverage:**
- Aim for 80%+ coverage on new code
- Test happy path and error cases
- Test edge cases

### 5. Update Documentation

If your change affects users:

- [ ] Update README.md
- [ ] Add example in examples/
- [ ] Add entry to ROADMAP.md (if feature)
- [ ] Update relevant example READMEs

### 6. Submit Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a PR on GitHub with:

**Title:** Clear, concise description
```
Add with_dict loop iteration support
```

**Description template:**
```markdown
## What does this PR do?

Adds support for iterating over dictionaries using with_dict.

## Why is this needed?

Users often need to iterate over key-value pairs, currently only
list iteration is supported.

## How was it implemented?

- Added DictIterator in filetree package
- Extended executor to handle with_dict
- Added comprehensive tests

## Examples

\`\`\`yaml
- vars:
    ports:
      web: 80
      api: 8080
      admin: 9000

- name: Configure port
  shell: echo "{{item.key}} runs on port {{item.value}}"
  with_dict: "{{ports}}"
\`\`\`

## Testing

- [x] Added unit tests
- [x] Tested manually with examples
- [x] Updated documentation

## Checklist

- [x] Tests pass
- [x] Code formatted (`go fmt`)
- [x] Documentation updated
- [x] Example added
```

## Code Style

### Go Style Guide

Follow [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Key points:**
- Use `gofmt` for formatting
- Keep functions small and focused
- Write clear, descriptive names
- Add comments for exported functions
- Use early returns to reduce nesting

**Example:**
```go
// ExecuteStep executes a single configuration step within the given execution context.
// It validates the step, checks skip conditions, and dispatches to the appropriate handler.
func ExecuteStep(step config.Step, ec *ExecutionContext) error {
    // Validate step configuration
    if err := step.Validate(); err != nil {
        return err
    }

    // Check if step should be skipped
    shouldSkip, skipReason, err := checkSkipConditions(step, ec)
    if err != nil {
        return err
    }
    if shouldSkip {
        logSkipped(step, skipReason, ec)
        return nil
    }

    // Execute the step
    return dispatchStepAction(step, ec)
}
```

### Configuration Style

When adding examples:
- Use clear, descriptive names
- Add comments explaining non-obvious choices
- Keep examples focused on one feature
- Test examples before committing

## Testing Guidelines

### Unit Tests

```bash
# Run all tests
go test ./...

# Run specific package
go test ./internal/executor

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run with race detector
go test ./... -race
```

### Integration Tests

Add integration tests in `internal/executor/executor_test.go` for:
- End-to-end workflows
- Interaction between features
- Real file system operations

### Example Testing

Before submitting:
```bash
# Test examples work
mooncake run --config examples/01-hello-world/config.yml --dry-run
mooncake run --config examples/05-templates/config.yml --dry-run

# Test all examples
for example in examples/*/config.yml; do
    echo "Testing $example"
    mooncake run --config $example --dry-run || exit 1
done
```

## Documentation Guidelines

### README Updates

When updating README.md:
- Maintain existing structure
- Use clear, concise language
- Include code examples
- Link to detailed examples
- Test all code examples

### Example Documentation

Each example should have:
- README.md with clear explanation
- "What You'll Learn" section
- "Quick Start" commands
- "Key Concepts" section
- Working configuration that can be run

### Code Comments

```go
// Good: Explains why
// Use nested execution context to isolate loop variables
curEc := ec.Clone()

// Bad: Explains what (obvious from code)
// Copy the execution context
curEc := ec.Clone()
```

## Feature Proposals

For significant features, create a proposal in `docs/proposals/`:

```markdown
# Proposal: With Dict Iteration

## Problem

Users need to iterate over dictionaries (key-value pairs) but currently
only list iteration is supported with with_items.

## Proposed Solution

Add `with_dict` that iterates over dictionaries, providing `item.key`
and `item.value` in each iteration.

## Design

### Configuration Syntax

\`\`\`yaml
- vars:
    ports:
      web: 80
      api: 8080

- name: Configure port
  shell: echo "{{item.key}}: {{item.value}}"
  with_dict: "{{ports}}"
\`\`\`

### Implementation

1. Add WithDict field to Step struct
2. Implement dict iteration in executor
3. Add tests

### Alternatives Considered

1. Extend with_items to handle dicts - Rejected, too implicit
2. Use template filters - Rejected, not ergonomic

## Open Questions

- Should we support nested dicts?
- What about empty dicts?
```

## Pull Request Review Process

1. **Automated checks** - CI must pass
2. **Code review** - Maintainer reviews code
3. **Documentation review** - Check docs updated
4. **Testing verification** - Verify tests adequate
5. **Final approval** - Merge when approved

**Review criteria:**
- Code quality and style
- Test coverage
- Documentation completeness
- Backward compatibility
- Performance impact

## Community Guidelines

- Be respectful and constructive
- Welcome newcomers
- Help others learn
- Focus on the problem, not the person
- Assume good intent

## Getting Help

- **Questions:** Open a [discussion](https://github.com/alehatsman/mooncake/discussions)
- **Bugs:** Open an [issue](https://github.com/alehatsman/mooncake/issues)
- **GitHub Discussions:** [Ask questions and share ideas](https://github.com/alehatsman/mooncake/discussions)

## Recognition

Contributors are recognized in:
- Git commit history
- Release notes
- CONTRIBUTORS.md (if we create it)

Thank you for contributing to Mooncake! 🚀

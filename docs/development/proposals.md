# Feature Proposals

This directory contains detailed design proposals for new Mooncake features.

## Purpose

Proposals help:
- Think through design before implementation
- Get feedback from community
- Document decision-making process
- Provide reference during implementation

## When to Write a Proposal

Write a proposal for:
- **New major features** - Significant additions to Mooncake
- **Breaking changes** - Changes that affect existing configurations
- **Complex features** - Features requiring architectural decisions
- **Controversial features** - Features that may have multiple approaches

**Don't need a proposal for:**
- Bug fixes
- Documentation updates
- Small improvements
- Tests

## Proposal Template

Create a new file: `NNNN-feature-name.md`

```markdown
# Proposal: Feature Name

- **Author:** Your Name (@github-username)
- **Status:** Draft | Under Review | Accepted | Rejected | Implemented
- **Created:** YYYY-MM-DD
- **Updated:** YYYY-MM-DD

## Summary

One paragraph explanation of the feature.

## Motivation

Why is this feature needed? What problem does it solve?

### Use Cases

Concrete examples of when users would use this feature:

1. **Use case 1:** User wants to...
2. **Use case 2:** User needs to...

## Proposed Solution

### Configuration Syntax

\`\`\`yaml
# Example of how users would use this feature
- name: Example step
  new_action:
    parameter: value
\`\`\`

### Implementation Overview

High-level approach:
1. Changes to config package
2. Changes to executor
3. New packages/files needed

### Detailed Design

#### Data Structures

\`\`\`go
type NewFeature struct {
    // Fields...
}
\`\`\`

#### Execution Flow

1. Step 1...
2. Step 2...

#### Error Handling

How errors are detected and reported.

### Examples

Complete working examples:

\`\`\`yaml
# Example 1: Basic usage
- name: Basic example
  new_action:
    param: value
\`\`\`

## Alternatives Considered

### Alternative 1: Different Approach

**Pros:**
- Advantage 1
- Advantage 2

**Cons:**
- Disadvantage 1
- Disadvantage 2

**Why rejected:** Explanation

### Alternative 2: Another Approach

[Same format]

## Compatibility

### Backward Compatibility

Does this break existing configurations?
- [ ] Yes (requires migration guide)
- [x] No

### Migration Path

If breaking: How do users migrate?

## Implementation Plan

### Phase 1: Core Implementation
- [ ] Task 1
- [ ] Task 2

### Phase 2: Documentation
- [ ] README updates
- [ ] Example creation
- [ ] Proposal in docs

### Phase 3: Testing
- [ ] Unit tests
- [ ] Integration tests
- [ ] Manual testing

### Estimated Effort

- Implementation: X hours/days
- Testing: X hours/days
- Documentation: X hours/days
- **Total:** X hours/days

## Open Questions

1. **Question 1:** What about edge case X?
2. **Question 2:** How should we handle Y?

## References

- Related issues: #123, #456
- Related PRs: #789
- External docs: [link]

## Decision

**Date:** YYYY-MM-DD
**Decision:** Accepted | Rejected
**Reason:** Why was this decision made?
```

## Proposal Process

### 1. Draft

- Copy template
- Fill in details
- Focus on motivation and use cases

### 2. Community Review

- Open PR with proposal
- Label: `proposal`
- Gather feedback
- Update based on comments

### 3. Decision

- Maintainer reviews
- Community discusses
- Accept, reject, or request changes

### 4. Implementation

- Accepted proposals can be implemented
- Link PR to proposal
- Update proposal status

## Existing Proposals

<!-- Add links to proposals as they're created -->

None yet! Be the first to propose a feature.

## Example Proposals

### Good Examples

**with_dict Iteration**
```
Problem: Clear use case
Solution: Well-defined syntax
Examples: Multiple working examples
Implementation: Clear approach
Alternatives: Considered and rejected with reasons
```

### What to Avoid

**Bad Proposal**
```
Problem: Vague "make it better"
Solution: No concrete syntax
Examples: None or incomplete
Implementation: "Just add the feature"
Alternatives: None considered
```

## Tips for Good Proposals

1. **Start with use cases** - Real problems users have
2. **Show examples first** - Syntax before implementation
3. **Consider alternatives** - Show you've thought it through
4. **Keep it focused** - One feature per proposal
5. **Be specific** - Concrete syntax and behavior
6. **Think about errors** - How will failures be handled?
7. **Consider compatibility** - Impact on existing configs

## Questions?

- Open an issue with `[Proposal]` prefix
- Discuss in community channels
- Tag maintainers for review

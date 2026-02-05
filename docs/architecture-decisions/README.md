# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) documenting significant architectural decisions made in the mooncake project.

## What is an ADR?

An Architecture Decision Record (ADR) is a document that captures an important architectural decision made along with its context and consequences. ADRs help teams:

- Understand why certain decisions were made
- Avoid revisiting settled decisions
- Onboard new team members
- Maintain consistency across the codebase

## ADR Format

Each ADR follows this structure:

1. **Title**: Short, descriptive name
2. **Status**: Proposed, Accepted, Deprecated, Superseded
3. **Context**: What problem are we trying to solve?
4. **Decision**: What approach did we choose?
5. **Alternatives Considered**: What other options were evaluated?
6. **Consequences**: What are the positive and negative impacts?

## Index

| # | Title | Status | Date |
|---|-------|--------|------|
| [001](./001-handler-based-action-architecture.md) | Handler-Based Action Architecture | Accepted | 2026-02-05 |

## Creating a New ADR

1. Copy the template (if available) or use an existing ADR as reference
2. Number sequentially (e.g., 002-your-decision-title.md)
3. Fill in all sections thoroughly
4. Add entry to the index table above
5. Submit for review

## ADR Lifecycle

- **Proposed**: Under discussion, not yet implemented
- **Accepted**: Decision made and being/has been implemented
- **Deprecated**: No longer recommended but still in use
- **Superseded**: Replaced by a newer decision (link to new ADR)

## Guidelines

- **Be Concise**: ADRs should be readable in 10-15 minutes
- **Be Specific**: Include code examples and implementation details
- **Be Honest**: Document both benefits and drawbacks
- **Be Historical**: Capture the context at the time of the decision
- **Be Immutable**: Don't edit old ADRs; supersede them with new ones if needed

## References

- [ADR Documentation](https://adr.github.io/)
- [Sustainable Architectural Design Decisions](https://www.infoq.com/articles/sustainable-architectural-design-decisions/)

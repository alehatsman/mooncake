# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) documenting key architectural decisions made during the development of Mooncake.

## ADRs

- [ADR 000: Planner Execution Model](000-planner-execution-model.md) - Three-phase execution model (parse → plan → execute)
- [ADR 001: Handler-Based Action Architecture](001-handler-based-action-architecture.md) - Modular action system with handler interface
- [ADR 002: Preset Expansion System](002-preset-expansion-system.md) - Flat preset architecture with parameter injection

## What is an ADR?

Architecture Decision Records capture important architectural decisions along with their context and consequences. They help developers understand:

- Why the system is structured the way it is
- What alternatives were considered
- What trade-offs were made
- What constraints influenced the decision

## Format

Each ADR follows this structure:

- **Status**: Proposed, Accepted, Deprecated, Superseded
- **Context**: The issue motivating this decision
- **Decision**: The change being proposed or adopted
- **Consequences**: The resulting context after applying the decision

## Contributing

When making significant architectural changes, document them as ADRs. Number them sequentially (003, 004, etc.).

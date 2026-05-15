// Package control implements the kernel-level state machines for
// Mooncake's compound steps — transactions (spec-30, LIFO rollback)
// and try/catch/finally (spec-23 §2, branch routing).
//
// In kernel terms (see docs-working-v2/vision/kernel.md), compound
// steps are graph-shape concepts: transactions are grouping
// subgraphs, try/catch/finally are branch-edge routings. This
// package exposes the typed state + pure-logic operations so
// frontends that want to reason about compound-step shape (drift
// remediation, MCP apply_approved, explain walking from a failed
// transaction back to its body) can do so without importing
// internal/executor.
//
// Scope:
//   - State types: TxnState, TryState. Owned by the executor's
//     OpenTxns / OpenTries maps; consulted from the dispatch path.
//   - Pure-logic operations on those types: skip-reason determination,
//     try-failure / catch-failure recording.
//
// Out of scope (stays in internal/executor):
//   - Handler dispatch for Reverse() — needs the action registry.
//   - The *executor.Result snapshot of completed transaction body
//     children — needs the executor.Result type, which can't live
//     here without a circular import. Stays in
//     executor.ExecutionContext.CompletedByTxn.
//   - Wrapper methods on ExecutionContext that call into this package
//     — they're a thin convenience shim over the free functions here.
//
// This package is the result of R0.1 in
// docs-working-v2/arch-report/2026-05-15-refactoring-plan.md.
package control

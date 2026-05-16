// Package apply implements the kernel's local-apply entry point.
//
// This is the Kernel.Apply() function from docs-working/vision/kernel.md:
// the typed mutation kernel's local-machine execution path. Frontends
// (CLI, MCP server, agent loop, future SDK) construct a *Config and
// call NewRunner(cfg).Run(ctx) to invoke it; orchestration that used
// to live inside cmd.run (gocyclo 33, the largest function in the
// CLI) now lives here.
//
// Scope:
//   - Config struct: typed inputs the kernel needs to apply a plan
//     against the local machine. CLI flags lower into a Config;
//     MCP / SDK / agent-loop callers construct one directly.
//   - Runner.Run: validate Config, set up the event substrate
//     (publisher + subscribers), install signal handling, dispatch
//     to executor.Start, return the apply result.
//
// Out of scope (intentionally — wave-3 R1.1b):
//   - Typed *KernelResult return shape with Reverse() method. R1.1a
//     leaves the return flat (error); R1.1b crystallizes the kernel
//     surface contract.
//   - Fleet (multi-host) topology. internal/fleet.Orchestrator owns
//     that; it composes apply.Runner per peer (R2.1a / R2.1b).
//
// This package is the result of R1.1a in
// docs-working/arch-report/2026-05-15-refactoring-plan.md.
package apply

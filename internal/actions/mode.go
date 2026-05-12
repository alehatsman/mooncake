package actions

// Mode is the high-level dispatch mode for an action handler.
//
// Spec 16 (docs-working/spec-16-unify-dryrun-execute.md) collapses the
// previous parallel non-mutating paths (Handler.DryRun and Handler.Check)
// into a single ModePlan. ModeExecute is the normal mutating path.
//
// During the migration the legacy ExecutionContext.DryRun and CheckMode
// bools remain the source of truth; ExecutionContext.Mode() derives from
// them so new code can be written against Mode and migrated incrementally.
type Mode int

const (
	// ModeExecute performs real work: side effects, mutations, commands.
	ModeExecute Mode = iota
	// ModePlan inspects target state and predicts what would change.
	// No side effects. Replaces the legacy DryRun + Check methods.
	ModePlan
)

// String returns a stable human-readable name for the mode.
func (m Mode) String() string {
	switch m {
	case ModeExecute:
		return "execute"
	case ModePlan:
		return "plan"
	default:
		return "unknown"
	}
}

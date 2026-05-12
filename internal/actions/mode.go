package actions

// Mode is the high-level dispatch mode for an action handler.
//
// Spec 16 collapsed the previous parallel non-mutating paths (Handler.DryRun
// and Handler.Check) into a single ModePlan. ModeApply is the normal
// mutating path.
//
// Spec 21 renamed the apply-mode constant (formerly ModeExecute) to ModeApply
// for alignment with the CLI's `mooncake apply` subcommand and modern IaC
// vocabulary (Terraform et al.).
type Mode int

const (
	// ModeApply performs real work: side effects, mutations, commands.
	ModeApply Mode = iota
	// ModePlan inspects target state and predicts what would change.
	// No side effects. Replaces the legacy DryRun + Check methods.
	ModePlan
)

// String returns a stable human-readable name for the mode.
func (m Mode) String() string {
	switch m {
	case ModeApply:
		return "apply"
	case ModePlan:
		return "plan"
	default:
		return "unknown"
	}
}

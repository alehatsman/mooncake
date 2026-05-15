package control

// TxnState is the kernel's per-transaction state machine. Owned by
// the executor's OpenTxns map; populated as transaction body
// children run; consulted by TxnSkipReason to gate subsequent
// siblings.
//
// The in-order list of completed body children (with their *Result
// snapshots for Reverse()) lives in
// internal/executor.ExecutionContext.CompletedByTxn — it can't sit
// here because *Result is in the executor package.
type TxnState struct {
	// Failed is true once any body child of this transaction has
	// errored. Subsequent body children skip; rollback triggers on
	// the failing child's path.
	Failed bool

	// RolledBack is true once rollback has been attempted. Set
	// whether rollback fully succeeded or only partially reverted —
	// it's the signal to fire on_rollback children regardless.
	RolledBack bool

	// PartialRollback is true if any Reverse() in the LIFO walk
	// returned an error. Surfaces ROLLBACK INCOMPLETE in run output.
	PartialRollback bool
}

// TxnSkipReason returns a non-empty string when a transaction child
// should skip this run. The reason is suitable for the user-facing
// step-skipped message.
//
// state may be nil — for the first child of a transaction, no
// TxnState has been allocated yet. In that case nothing skips.
//
// role is the child's TxnRole ("body" or "rollback"). Body children
// skip when the transaction has already failed. Rollback children
// skip when the transaction committed successfully (on_rollback only
// fires on rollback).
func TxnSkipReason(state *TxnState, role string) string {
	if state == nil {
		return ""
	}
	switch role {
	case "body":
		if state.Failed {
			return "transaction rolled back"
		}
	case "rollback":
		if !state.RolledBack {
			return "transaction committed (on_rollback skipped)"
		}
	}
	return ""
}

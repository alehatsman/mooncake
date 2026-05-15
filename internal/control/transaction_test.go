package control

import "testing"

// TestTxnSkipReason_NilState — first child of a transaction; no state
// has been allocated yet. Nothing skips on this basis.
func TestTxnSkipReason_NilState(t *testing.T) {
	for _, role := range []string{"body", "rollback", "finally", ""} {
		if got := TxnSkipReason(nil, role); got != "" {
			t.Errorf("TxnSkipReason(nil, %q) = %q, want empty", role, got)
		}
	}
}

// TestTxnSkipReason_BodySkipsAfterFailure — once a body child fails,
// subsequent body children skip with the spec-30 message.
func TestTxnSkipReason_BodySkipsAfterFailure(t *testing.T) {
	state := &TxnState{Failed: true}
	got := TxnSkipReason(state, "body")
	if got != "transaction rolled back" {
		t.Errorf("got %q, want %q", got, "transaction rolled back")
	}
}

// TestTxnSkipReason_RollbackSkipsOnCommit — rollback children only
// run on rollback; if the txn committed, they skip.
func TestTxnSkipReason_RollbackSkipsOnCommit(t *testing.T) {
	committed := &TxnState{Failed: false, RolledBack: false}
	got := TxnSkipReason(committed, "rollback")
	if got != "transaction committed (on_rollback skipped)" {
		t.Errorf("got %q, want commit-skipped message", got)
	}

	rolledBack := &TxnState{Failed: true, RolledBack: true}
	if got := TxnSkipReason(rolledBack, "rollback"); got != "" {
		t.Errorf("rolled-back rollback should not skip; got %q", got)
	}
}

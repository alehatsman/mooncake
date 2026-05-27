package diff

import (
	"fmt"
	"io"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func init() {
	Register(matchTransaction)
	Register(matchTry)
}

// SynthesizeCompound builds a typed *actions.Diff for a transaction-
// parent or try-parent step. Compounds have no Handler (they're
// structural wrappers in the plan), so their Diff cannot come from
// the Differ ABI — the plan-render call site calls this helper to
// inject a synthesized payload that internal/diff can dispatch on.
//
// Returns nil when the step is not a parent (TxnParent/TryParent
// set), or when neither Transaction nor Try is populated.
//
// `siblings` is the full plan step list; this helper walks it to
// collect children by TxnParent/TryParent + TxnRole/TryRole. Order
// is preserved from the plan.
func SynthesizeCompound(parent config.Step, siblings []config.Step) *actions.Diff {
	// A parent has Transaction or Try set AND empty TxnRole/TryRole.
	// (Children carry the role tags; they're not parents.)
	isTxnParent := len(parent.Transaction) > 0 && parent.TxnRole == ""
	isTryParent := len(parent.Try) > 0 && parent.TryRole == ""
	if !isTxnParent && !isTryParent {
		return nil
	}

	switch {
	case isTxnParent:
		var body, rollback []string
		for _, s := range siblings {
			if s.TxnParent != parent.ID {
				continue
			}
			name := s.Name
			if name == "" {
				name = s.ID
			}
			switch s.TxnRole {
			case "body":
				body = append(body, name)
			case "rollback":
				rollback = append(rollback, name)
			}
		}
		return &actions.Diff{
			Resource: actions.ResourceRef{
				Kind:       actions.ResourceOther,
				Identifier: parent.Name,
				Attributes: map[string]string{"kind": "transaction"},
			},
			Operation: actions.OpUpdate,
			After: &actions.TransactionDiff{
				Name:              parent.Name,
				AllowIrreversible: parent.AllowIrreversible,
				BodyChildren:      body,
				RollbackChildren:  rollback,
			},
		}
	case isTryParent:
		var try, catch, finally []string
		for _, s := range siblings {
			if s.TryParent != parent.ID {
				continue
			}
			name := s.Name
			if name == "" {
				name = s.ID
			}
			switch s.TryRole {
			case "try":
				try = append(try, name)
			case "catch":
				catch = append(catch, name)
			case "finally":
				finally = append(finally, name)
			}
		}
		return &actions.Diff{
			Resource: actions.ResourceRef{
				Kind:       actions.ResourceOther,
				Identifier: parent.Name,
				Attributes: map[string]string{"kind": "try"},
			},
			Operation: actions.OpUpdate,
			After: &actions.TryDiff{
				Name:            parent.Name,
				TryChildren:     try,
				CatchChildren:   catch,
				FinallyChildren: finally,
			},
		}
	}
	return nil
}

// matchTransaction recognises a synthesized actions.Diff describing
// a transaction-parent step. Dispatch is on Attributes["kind"]==
// "transaction". Accepts the typed *actions.TransactionDiff pointer
// or the JSON-decoded map shape.
func matchTransaction(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "transaction" {
		return nil
	}
	td, ok := toTransactionDiff(d.After)
	if !ok {
		return nil
	}
	return &transactionRenderer{op: d.Operation, payload: td}
}

func toTransactionDiff(v any) (actions.TransactionDiff, bool) {
	switch x := v.(type) {
	case *actions.TransactionDiff:
		if x == nil {
			return actions.TransactionDiff{}, false
		}
		return *x, true
	case actions.TransactionDiff:
		return x, true
	case map[string]interface{}:
		out := actions.TransactionDiff{}
		if s, ok := x["name"].(string); ok {
			out.Name = s
		}
		if b, ok := x["allow_irreversible"].(bool); ok {
			out.AllowIrreversible = b
		}
		out.BodyChildren = stringSlice(x["body_children"])
		out.RollbackChildren = stringSlice(x["rollback_children"])
		any := out.Name != "" || len(out.BodyChildren) > 0 || len(out.RollbackChildren) > 0
		return out, any
	}
	return actions.TransactionDiff{}, false
}

type transactionRenderer struct {
	op      actions.Operation
	payload actions.TransactionDiff
}

func (r *transactionRenderer) Kind() string { return "transaction" }

// Render writes a compact tree:
//
//	transaction deploy-app (2 body, 1 rollback)
//	  body:
//	    - build-image
//	    - push-image
//	  on_rollback:
//	    - cleanup-image
//
// Child diffs render under their own sibling step lines in the
// plan output — this renderer surfaces the compound shape only,
// not the per-child typed diffs (which would duplicate output).
func (r *transactionRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		head := "  transaction"
		if r.payload.Name != "" {
			head += " " + r.payload.Name
		}
		counts := fmt.Sprintf("(%d body, %d rollback)",
			len(r.payload.BodyChildren), len(r.payload.RollbackChildren))
		head += " " + counts
		if r.payload.AllowIrreversible {
			head += " [allow_irreversible]"
		}
		if _, err := fmt.Fprintln(w, head); err != nil {
			return err
		}
		if len(r.payload.BodyChildren) > 0 {
			if _, err := fmt.Fprintln(w, "    body:"); err != nil {
				return err
			}
			for _, c := range r.payload.BodyChildren {
				if _, err := fmt.Fprintln(w, "      - "+c); err != nil {
					return err
				}
			}
		}
		if len(r.payload.RollbackChildren) > 0 {
			if _, err := fmt.Fprintln(w, "    on_rollback:"); err != nil {
				return err
			}
			for _, c := range r.payload.RollbackChildren {
				if _, err := fmt.Fprintln(w, "      - "+c); err != nil {
					return err
				}
			}
		}
		return nil
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "transaction %s: body=%d rollback=%d\n",
			r.payload.Name, len(r.payload.BodyChildren), len(r.payload.RollbackChildren))
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// matchTry recognises a synthesized actions.Diff describing a try-
// parent step. Dispatch is on Attributes["kind"]=="try". Accepts
// the typed *actions.TryDiff pointer or the JSON-decoded map shape.
func matchTry(detail any) Renderer {
	d, ok := detail.(*actions.Diff)
	if !ok || d == nil {
		return nil
	}
	if d.Resource.Attributes["kind"] != "try" {
		return nil
	}
	td, ok := toTryDiff(d.After)
	if !ok {
		return nil
	}
	return &tryRenderer{op: d.Operation, payload: td}
}

func toTryDiff(v any) (actions.TryDiff, bool) {
	switch x := v.(type) {
	case *actions.TryDiff:
		if x == nil {
			return actions.TryDiff{}, false
		}
		return *x, true
	case actions.TryDiff:
		return x, true
	case map[string]interface{}:
		out := actions.TryDiff{}
		if s, ok := x["name"].(string); ok {
			out.Name = s
		}
		out.TryChildren = stringSlice(x["try_children"])
		out.CatchChildren = stringSlice(x["catch_children"])
		out.FinallyChildren = stringSlice(x["finally_children"])
		any := out.Name != "" || len(out.TryChildren) > 0 ||
			len(out.CatchChildren) > 0 || len(out.FinallyChildren) > 0
		return out, any
	}
	return actions.TryDiff{}, false
}

type tryRenderer struct {
	op      actions.Operation
	payload actions.TryDiff
}

func (r *tryRenderer) Kind() string { return "try" }

// Render writes a compact tree mirroring the transaction renderer:
//
//	try fetch-and-validate (2 try, 1 catch, 1 finally)
//	  try:
//	    - fetch-config
//	    - validate-config
//	  catch:
//	    - alert-on-failure
//	  finally:
//	    - cleanup
func (r *tryRenderer) Render(w io.Writer, format Format) error {
	switch format {
	case FormatText, "":
		head := "  try"
		if r.payload.Name != "" {
			head += " " + r.payload.Name
		}
		counts := fmt.Sprintf("(%d try, %d catch, %d finally)",
			len(r.payload.TryChildren), len(r.payload.CatchChildren), len(r.payload.FinallyChildren))
		head += " " + counts
		if _, err := fmt.Fprintln(w, head); err != nil {
			return err
		}
		for _, group := range []struct {
			label    string
			children []string
		}{
			{"try", r.payload.TryChildren},
			{"catch", r.payload.CatchChildren},
			{"finally", r.payload.FinallyChildren},
		} {
			if len(group.children) == 0 {
				continue
			}
			if _, err := fmt.Fprintln(w, "    "+group.label+":"); err != nil {
				return err
			}
			for _, c := range group.children {
				if _, err := fmt.Fprintln(w, "      - "+c); err != nil {
					return err
				}
			}
		}
		return nil
	case FormatJSON, FormatYAML:
		_, err := fmt.Fprintf(w, "try %s: try=%d catch=%d finally=%d\n",
			r.payload.Name, len(r.payload.TryChildren),
			len(r.payload.CatchChildren), len(r.payload.FinallyChildren))
		return err
	}
	return fmt.Errorf("unsupported format: %s", format)
}

// stringSlice coerces a JSON-decoded []interface{} into []string,
// silently skipping non-string entries. Shared by both
// toTransactionDiff and toTryDiff.
func stringSlice(v any) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

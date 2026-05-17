package http_request

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	mode := actions.ModeApply
	if plan {
		mode = actions.ModePlan
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:  r,
			PathUtil:  pathutil.NewPathExpander(r),
			Logger:    logger.NewLogger(logger.ErrorLevel),
			Evaluator: expression.NewExprEvaluator(),
			Mode:      mode,
			Stats:     executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func TestHandler_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestMetadata(t *testing.T) {
	m := (&Handler{}).Metadata()
	if m.Name != "http.request" {
		t.Errorf("Name = %q, want http.request", m.Name)
	}
	if !m.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if m.SupportsBecome {
		t.Error("SupportsBecome must stay false; HTTP-with-sudo is nonsense")
	}
	if m.RequiresSudo {
		t.Error("RequiresSudo should be false")
	}
	wantEvent := "http.requested"
	found := false
	for _, e := range m.EmitsEvents {
		if e == wantEvent {
			found = true
		}
	}
	if !found {
		t.Errorf("EmitsEvents should include %q; got %v", wantEvent, m.EmitsEvents)
	}
}

func TestPermissions(t *testing.T) {
	cases := []struct {
		name string
		step *config.Step
	}{
		{"nil step", nil},
		{"nil HTTPRequest", &config.Step{}},
		{"basic GET", &config.Step{HTTPRequest: &config.HTTPRequest{URL: "http://x"}}},
		{"with auth", &config.Step{HTTPRequest: &config.HTTPRequest{
			URL:  "http://x",
			Auth: &config.HTTPAuth{Bearer: "tok"},
		}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ps := (&Handler{}).Permissions(c.step)
			if !ps.Network {
				t.Error("Network must always be true")
			}
			if ps.Sudo {
				t.Error("Sudo must always be false")
			}
			if len(ps.FilesystemWrite) != 0 {
				t.Errorf("Wave 1 has no save_to: yet; FilesystemWrite should be empty, got %v", ps.FilesystemWrite)
			}
		})
	}
}

// TestRun_Plan_ReadMethodNoChange — GET in plan mode reports
// WouldChange=false because a read can't change anything.
func TestRun_Plan_ReadMethodNoChange(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL: "http://localhost/health",
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Error("GET in plan mode must NOT report WouldChange=true (read-only)")
	}
	if !r.Checkable {
		t.Error("plan-mode result should be Checkable")
	}
	if !strings.Contains(r.Reason, "GET") || !strings.Contains(r.Reason, "http://localhost/health") {
		t.Errorf("plan reason missing method+URL: %q", r.Reason)
	}
	if !strings.Contains(r.Reason, "read-only") {
		t.Errorf("plan reason should call out read-only: %q", r.Reason)
	}
}

// TestRun_Plan_WriteMethodWouldChange — POST/PUT/DELETE/PATCH in plan
// mode report WouldChange=true; they don't execute the network call.
func TestRun_Plan_WriteMethodWouldChange(t *testing.T) {
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		t.Run(m, func(t *testing.T) {
			step := &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://localhost/x",
				Method:         m,
				IdempotencyKey: "k", // make POST/PATCH pass Validate
				Risk:           "",
			}}
			res, err := (&Handler{}).Run(newCtx(t, true), step)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			r := res.(*executor.Result)
			if !r.WouldChange {
				t.Errorf("%s in plan must report WouldChange=true", m)
			}
		})
	}
}

// TestRun_Plan_BodyHintInReason — when a body is set, plan mode reports
// "body=N bytes" so the operator can see what would be sent.
func TestRun_Plan_BodyHintInReason(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://localhost/x",
		Method:         "POST",
		Body:           `{"x":1}`,
		IdempotencyKey: "k",
	}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "body=7 bytes") {
		t.Errorf("plan reason missing body-size hint: %q", r.Reason)
	}
}

// TestRun_Plan_RedactBodyHidesSize — redact_body=true keeps the size
// out of the plan reason too (matches what we do in logs/diffs).
func TestRun_Plan_RedactBodyHidesSize(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://localhost/x",
		Method:         "POST",
		Body:           `{"secret":"s"}`,
		IdempotencyKey: "k",
		RedactBody:     true,
	}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if strings.Contains(r.Reason, "bytes") {
		t.Errorf("redact_body=true should hide byte count in plan reason: %q", r.Reason)
	}
	if !strings.Contains(r.Reason, "redacted") {
		t.Errorf("plan reason should mention redaction: %q", r.Reason)
	}
}

// TestPermissions_SaveToAddsFilesystemWrite — Wave 3: declaring
// save_to adds the path to FilesystemWrite so spec-44 doctor can
// flag obvious misuse before the network call runs.
func TestPermissions_SaveToAddsFilesystemWrite(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:    "http://x",
		SaveTo: "/var/run/hook.json",
	}}
	ps := (&Handler{}).Permissions(step)
	if !ps.Network {
		t.Error("Network must remain true")
	}
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != "/var/run/hook.json" {
		t.Errorf("FilesystemWrite = %v, want [/var/run/hook.json]", ps.FilesystemWrite)
	}
}

// TestPermissions_EmptySaveToHasNoFilesystemWrite — whitespace-only
// SaveTo is treated as unset (matches the runApply check) so spec-44
// doctor doesn't report a phantom write.
func TestPermissions_EmptySaveToHasNoFilesystemWrite(t *testing.T) {
	for _, val := range []string{"", "   ", "\t\n"} {
		step := &config.Step{HTTPRequest: &config.HTTPRequest{URL: "http://x", SaveTo: val}}
		ps := (&Handler{}).Permissions(step)
		if len(ps.FilesystemWrite) != 0 {
			t.Errorf("SaveTo=%q: FilesystemWrite = %v, want empty", val, ps.FilesystemWrite)
		}
	}
}

// TestValidate_RejectsSaveToOnProbe — probes are read-only
// inspection; persisting the probe response confuses the audit
// story. save_to belongs on the top-level request.
func TestValidate_RejectsSaveToOnProbe(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL: "http://x",
		Probe: &config.HTTPRequest{
			URL:    "http://x/health",
			Method: "GET",
			SaveTo: "/tmp/probe.json",
		},
	}}
	err := (&Handler{}).Validate(step)
	if err == nil {
		t.Fatal("expected Validate to reject save_to on probe")
	}
	if !strings.Contains(err.Error(), "save_to") || !strings.Contains(err.Error(), "probe") {
		t.Errorf("error should mention save_to + probe; got: %v", err)
	}
}

// TestValidate_AllowsSaveToOnReverse — reverse is a real follow-up
// request that can legitimately want to persist its response (e.g.
// rollback receipts). Only probes are restricted.
func TestValidate_AllowsSaveToOnReverse(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://x/hooks",
		Method:         "POST",
		IdempotencyKey: "k",
		Reverse: &config.HTTPRequest{
			URL:    "http://x/hooks/{{ .response.json.id }}",
			Method: "DELETE",
			SaveTo: "/var/log/rollback-receipts/{{ .response.json.id }}.json",
		},
	}}
	if err := (&Handler{}).Validate(step); err != nil {
		t.Errorf("Validate must accept save_to on reverse; got: %v", err)
	}
}

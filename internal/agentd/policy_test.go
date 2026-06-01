package agentd

import (
	"encoding/json"
	"testing"

	"github.com/alehatsman/mooncake/internal/executor"
)

// TestSubmitRequest_PolicyDeserializes proves the optional `policy`
// object on the POST /v1/runs body deserializes straight into
// executor.Policy — the wire shape a controller (moongit) sends to gate
// an unattended daemon-driven run (#11).
func TestSubmitRequest_PolicyDeserializes(t *testing.T) {
	body := `{"plan_path":"/p.yml","policy":{"denied_actions":["shell","cmd"],"deny_network":true,"max_risk":6}}`
	var req submitRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Policy == nil {
		t.Fatal("policy not deserialized from request body")
	}
	if len(req.Policy.DeniedActions) != 2 || req.Policy.DeniedActions[0] != "shell" {
		t.Errorf("denied_actions wrong: %+v", req.Policy.DeniedActions)
	}
	if !req.Policy.DenyNetwork || req.Policy.MaxRisk != 6 {
		t.Errorf("deny_network/max_risk wrong: %+v", req.Policy)
	}
}

// TestSubmitRequest_NoPolicy proves omitting policy yields a nil gate
// (ungated run — backward-compatible with existing clients).
func TestSubmitRequest_NoPolicy(t *testing.T) {
	var req submitRequest
	if err := json.Unmarshal([]byte(`{"plan_path":"/p.yml"}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Policy != nil {
		t.Errorf("expected nil policy when omitted, got %+v", req.Policy)
	}
}

// TestStorePolicyRoundTrip proves the persisted Run carries the policy
// across the disk round-trip the worker reads from before applying.
func TestStorePolicyRoundTrip(t *testing.T) {
	s := newStore(t)
	run, err := s.Create(SubmitReq{
		PlanPath: "/tmp/x.yml",
		BaseDir:  "/tmp",
		Policy:   &executor.Policy{DeniedActions: []string{"shell"}, MaxRisk: 5},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(run.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Policy == nil {
		t.Fatal("policy did not survive the store round-trip")
	}
	if len(got.Policy.DeniedActions) != 1 || got.Policy.DeniedActions[0] != "shell" || got.Policy.MaxRisk != 5 {
		t.Errorf("policy round-trip mismatch: %+v", got.Policy)
	}
}

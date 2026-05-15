package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/plan"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// ---- aggregatePermissions ---------------------------------------------------

func TestAggregatePermissions_NilWhenNoSteps(t *testing.T) {
	p := &plan.Plan{Steps: []config.Step{}}
	got := aggregatePermissions(p)
	if got != nil {
		t.Errorf("expected nil for empty plan, got %+v", got)
	}
}

func TestAggregatePermissions_MergesUniqueBindaries(t *testing.T) {
	// Build a minimal plan inline to test the merge logic without hitting the
	// full handler registry. We test the appendUnique helper directly.
	got := appendUnique([]string{"git", "curl"}, []string{"curl", "jq"})
	want := []string{"git", "curl", "jq"}
	if len(got) != len(want) {
		t.Errorf("appendUnique = %v, want %v", got, want)
		return
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("appendUnique[%d] = %q, want %q", i, g, want[i])
		}
	}
}

// ---- aggregateCost ----------------------------------------------------------

func TestAggregateCost_NilWhenNoCosts(t *testing.T) {
	inspections := []plan.StepInspection{
		{StepID: "s1", WouldChange: true},
		{StepID: "s2", WouldChange: false},
	}
	got := aggregateCost(inspections)
	if got != nil {
		t.Errorf("expected nil cost summary for steps without Cost, got %v", got)
	}
}

func TestAggregateCost_MaxRiskAndBand(t *testing.T) {
	cases := []struct {
		risk     int
		wantBand string
	}{
		{2, "low"},
		{5, "medium"},
		{8, "high"},
		{10, "high"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.wantBand, func(t *testing.T) {
			inspections := []plan.StepInspection{
				{
					StepID:      "s1",
					WouldChange: true,
					Cost:        &actions.CostEstimate{Risk: tc.risk, Resources: 1},
				},
			}
			got := aggregateCost(inspections)
			if got == nil {
				t.Fatal("expected non-nil cost summary")
			}
			if got["risk_band"] != tc.wantBand {
				t.Errorf("risk_band = %q, want %q (risk=%d)", got["risk_band"], tc.wantBand, tc.risk)
			}
			if got["max_risk"] != tc.risk {
				t.Errorf("max_risk = %v, want %d", got["max_risk"], tc.risk)
			}
		})
	}
}

func TestAggregateCost_CountsWouldChange(t *testing.T) {
	inspections := []plan.StepInspection{
		{StepID: "s1", WouldChange: true, Cost: &actions.CostEstimate{Risk: 3, Resources: 1}},
		{StepID: "s2", WouldChange: false, Cost: &actions.CostEstimate{Risk: 2, Resources: 2}},
		{StepID: "s3", WouldChange: true, Cost: &actions.CostEstimate{Risk: 4, Resources: 0}},
	}
	got := aggregateCost(inspections)
	if got == nil {
		t.Fatal("expected non-nil cost summary")
	}
	if got["would_change_count"] != 2 {
		t.Errorf("would_change_count = %v, want 2", got["would_change_count"])
	}
	if got["total_resources"] != 3 {
		t.Errorf("total_resources = %v, want 3", got["total_resources"])
	}
}

// ---- HandleCheckPlan integration --------------------------------------------

func TestHandleCheckPlan_ReturnsCostSummaryAndRequires(t *testing.T) {
	// Write a minimal mooncake config that uses file.write to /tmp (not a
	// system path) so we get cost data without needing root.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mc.yml")
	target := filepath.Join(dir, "out.txt")
	content := "version: \"1.0\"\nsteps:\n  - name: write file\n    file.write:\n      path: " + target + "\n      content: hello\n      state: file\n"
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := HandleCheckPlan(nil, mustJSON(t, map[string]string{"config": cfg}))
	if err != nil {
		t.Fatalf("HandleCheckPlan: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, out)
	}

	// inspections must be present
	if _, ok := resp["inspections"]; !ok {
		t.Error("response missing 'inspections'")
	}

	// cost_summary should be present (file.write implements Coster)
	if _, ok := resp["cost_summary"]; !ok {
		t.Error("response missing 'cost_summary'")
	}

	// root_file must be present
	if resp["root_file"] == "" || resp["root_file"] == nil {
		t.Error("response missing 'root_file'")
	}
}

func TestHandleCheckPlan_InspectionsContainDiff(t *testing.T) {
	// Create a file first, then overwrite it — the file handler's Diff()
	// should produce a non-nil diff object when content changes.
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := filepath.Join(dir, "mc.yml")
	content := "version: \"1.0\"\nsteps:\n  - name: write file\n    file.write:\n      path: " + target + "\n      content: new content\n      state: file\n"
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := HandleCheckPlan(nil, mustJSON(t, map[string]string{"config": cfg}))
	if err != nil {
		t.Fatalf("HandleCheckPlan: %v", err)
	}

	var resp struct {
		Inspections []map[string]interface{} `json:"inspections"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Inspections) == 0 {
		t.Fatal("no inspections returned")
	}
	// The step should show WouldChange and have a diff
	insp := resp.Inspections[0]
	if insp["would_change"] != true {
		t.Errorf("expected would_change=true, got %v", insp["would_change"])
	}
	if insp["diff"] == nil {
		t.Error("expected diff to be non-nil for content-changed file step")
	}
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

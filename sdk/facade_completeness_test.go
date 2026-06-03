package mooncake_test

import (
	"context"
	"testing"
	"time"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// stubLLMClient is a facade-only implementation of mooncake.LLMClient. Its
// existence proves the LLMClient seam is nameable and implementable against
// the sdk package alone.
type stubLLMClient struct{}

func (stubLLMClient) GeneratePlan(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

// countingSubscriber is a facade-only mooncake.Subscriber. It proves the
// Subscriber/Event types are nameable and implementable against the sdk
// package alone (the live-stream behavior is asserted in events_test.go).
type countingSubscriber struct{ events int }

func (c *countingSubscriber) OnEvent(_ mooncake.Event) { c.events++ }
func (c *countingSubscriber) Close()                   {}

// TestRunOptions_EveryFieldIsFacadeNameable sets EVERY field of
// mooncake.RunOptions using only facade types. If a field's type leaked an
// internal/ package this file would not compile — that is the #120 proof.
// The run is never executed (no LLM, no filesystem mutation); compilation +
// assignment is the contract under test.
func TestRunOptions_EveryFieldIsFacadeNameable(t *testing.T) {
	approver := func(_ context.Context, planBytes []byte) (mooncake.ConfirmResult, error) {
		return mooncake.ConfirmResult{Outcome: mooncake.OutcomeApply, PlanBytes: planBytes}, nil
	}

	opts := mooncake.RunOptions{
		Goal:          "demo goal",
		PlanPath:      "plan.yml",
		UseStdin:      false,
		RepoRoot:      ".",
		Provider:      "claude",
		Endpoint:      "http://localhost:11434",
		Model:         "test-model",
		MaxIterations: 3,
		AutoApply:     true,
		Style:         mooncake.StyleStep,
		OutputFormat:  mooncake.OutputFormatJSON,
		Policy: &mooncake.Policy{
			AllowedActions: []string{"file.write"},
			DeniedActions:  []string{"shell", "cmd"},
			DenyNetwork:    true,
			MaxRisk:        5,
		},
		LLMTimeout:  30 * time.Second,
		Approver:    approver,
		Registry:    mooncake.DefaultRegistry(),
		LLMClient:   stubLLMClient{},
		Subscribers: []mooncake.Subscriber{&countingSubscriber{}},
		Logger:      mooncake.NewLogger(mooncake.LogLevelError),
		LoggerLevel: mooncake.LogLevelInfo,
	}

	// Read every field back through the facade so an unused-field never hides
	// a leak and the assignment types are exercised.
	if opts.Goal == "" ||
		opts.PlanPath == "" ||
		opts.UseStdin ||
		opts.RepoRoot == "" ||
		opts.Provider == "" ||
		opts.Endpoint == "" ||
		opts.Model == "" ||
		opts.MaxIterations == 0 ||
		!opts.AutoApply ||
		opts.Style != mooncake.StyleStep ||
		opts.OutputFormat != mooncake.OutputFormatJSON ||
		opts.Policy == nil ||
		opts.LLMTimeout == 0 ||
		opts.Approver == nil ||
		opts.Registry == nil ||
		opts.LLMClient == nil ||
		len(opts.Subscribers) == 0 ||
		opts.Logger == nil ||
		opts.LoggerLevel != mooncake.LogLevelInfo {
		t.Fatal("a RunOptions field round-trip read failed")
	}

	// Exercise the Policy fields a consumer must be able to name to build one.
	if len(opts.Policy.AllowedActions) == 0 ||
		len(opts.Policy.DeniedActions) == 0 ||
		!opts.Policy.DenyNetwork ||
		opts.Policy.MaxRisk == 0 {
		t.Fatal("Policy fields not facade-nameable")
	}
}

// TestResultTypes_EveryFieldIsFacadeReadable constructs each result type
// through the facade aliases and reads EVERY field. A facade-only consumer
// must be able to read a completed run's outcome without naming internal/ —
// this fails to compile if any result type or field leaked.
func TestResultTypes_EveryFieldIsFacadeReadable(t *testing.T) {
	diff := mooncake.DiffStat{Files: 2, Insertions: 10, Deletions: 3}
	if diff.Files != 2 || diff.Insertions != 10 || diff.Deletions != 3 {
		t.Fatal("DiffStat fields not facade-readable")
	}

	iter := mooncake.IterationLog{
		Iteration:        1,
		Goal:             "g",
		PlanHash:         "h",
		Status:           "success",
		ChangedFiles:     []string{"a.txt"},
		DiffStat:         diff,
		Artifacts:        []string{"plan.yml"},
		Provider:         "claude",
		Model:            "m",
		ValidationError:  "",
		ExecutionError:   "",
		AssertionsFailed: 0,
	}

	result := mooncake.LoopResult{
		Iterations: []mooncake.IterationLog{iter},
		StopReason: mooncake.StopSuccess,
		FinalLog:   &iter,
	}

	// Read every LoopResult + IterationLog field.
	if len(result.Iterations) == 0 ||
		result.StopReason != mooncake.StopSuccess ||
		result.FinalLog == nil {
		t.Fatal("LoopResult fields not facade-readable")
	}
	fl := result.FinalLog
	if fl.Iteration != 1 ||
		fl.Goal != "g" ||
		fl.PlanHash != "h" ||
		fl.Status != "success" ||
		len(fl.ChangedFiles) == 0 ||
		fl.DiffStat.Files != 2 ||
		len(fl.Artifacts) == 0 ||
		fl.Provider != "claude" ||
		fl.Model != "m" ||
		fl.ValidationError != "" ||
		fl.ExecutionError != "" ||
		fl.AssertionsFailed != 0 {
		t.Fatal("IterationLog fields not facade-readable")
	}

	// TerminalStatus is the method a consumer keys on for the worst outcome.
	if result.TerminalStatus() != "success" {
		t.Fatalf("TerminalStatus = %q, want success", result.TerminalStatus())
	}

	// AgentCompletedData is the JSON-mode terminal event payload.
	done := mooncake.AgentCompletedData{
		Iterations:   1,
		StopReason:   string(mooncake.StopSuccess),
		Status:       "success",
		DiffStat:     diff,
		ChangedFiles: []string{"a.txt"},
	}
	if done.Iterations != 1 ||
		done.StopReason == "" ||
		done.Status != "success" ||
		done.DiffStat.Files != 2 ||
		len(done.ChangedFiles) == 0 {
		t.Fatal("AgentCompletedData fields not facade-readable")
	}
}

// TestDifferCluster_FacadeNameable builds a mooncake.Diff using only facade
// types — the #120 Differ payload cluster (Diff / ResourceRef / ResourceKind /
// Operation / DiffLine / DiffOp). A consumer implementing mooncake.Differ
// returns exactly this shape.
func TestDifferCluster_FacadeNameable(t *testing.T) {
	d := mooncake.Diff{
		Resource: mooncake.ResourceRef{
			Kind:       mooncake.ResourceFile,
			Identifier: "/tmp/x",
			Attributes: map[string]string{"k": "v"},
		},
		Operation: mooncake.OpUpdate,
		Before:    nil,
		After:     nil,
		Lines: []mooncake.DiffLine{
			{Op: mooncake.DiffOpAdd, Text: "new line", LineNumber: 1},
			{Op: mooncake.DiffOpRemove, Text: "old line", LineNumber: 1},
			{Op: mooncake.DiffOpContext, Text: "ctx", LineNumber: 2},
		},
	}

	if d.Resource.Kind != mooncake.ResourceFile ||
		d.Resource.Identifier == "" ||
		d.Operation != mooncake.OpUpdate ||
		len(d.Lines) != 3 ||
		d.Lines[0].Op != mooncake.DiffOpAdd {
		t.Fatal("Diff cluster fields not facade-nameable")
	}

	// Name the remaining ResourceKind / Operation / DiffOp constants so a
	// leak in any of them fails to compile.
	_ = []mooncake.ResourceKind{
		mooncake.ResourcePackage, mooncake.ResourceService, mooncake.ResourceText,
		mooncake.ResourceShell, mooncake.ResourceVar, mooncake.ResourceGit,
		mooncake.ResourceOther,
	}
	_ = []mooncake.Operation{mooncake.OpCreate, mooncake.OpDelete, mooncake.OpNoop}
}

// TestCapabilityPayloads_FacadeNameable builds the Permitter/Coster payloads a
// consumer returns when implementing those capability interfaces.
func TestCapabilityPayloads_FacadeNameable(t *testing.T) {
	ps := mooncake.PermissionSet{
		Sudo:             true,
		Network:          true,
		RequiredBinaries: []string{"git"},
		FilesystemWrite:  []string{"/tmp"},
		Notes:            []string{"note"},
	}
	ce := mooncake.CostEstimate{Resources: 1, Bytes: 1024, Reversible: true, Risk: 5}
	if !ps.Sudo || !ps.Network || len(ps.RequiredBinaries) == 0 || ce.Risk != 5 {
		t.Fatal("capability payloads not facade-nameable")
	}
}

package kernel

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/alehatsman/mooncake/internal/agent"
	"github.com/alehatsman/mooncake/internal/events"
)

func TestResolveAgentOutputFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", agent.OutputFormatText, false},
		{"text", agent.OutputFormatText, false},
		{"json", agent.OutputFormatJSON, false},
		{"yaml", "", true},
		{"JSON", "", true}, // case-sensitive on purpose
	}
	for _, c := range cases {
		got, err := resolveAgentOutputFormat(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveAgentOutputFormat(%q): want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveAgentOutputFormat(%q): unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("resolveAgentOutputFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// decodeAgentCompleted parses the single NDJSON line an emit* helper wrote
// and asserts it's a well-formed agent.completed event, returning its Data.
func decodeAgentCompleted(t *testing.T, raw []byte) agent.AgentCompletedData {
	t.Helper()
	var env struct {
		Type events.Type              `json:"type"`
		Data agent.AgentCompletedData `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(raw), &env); err != nil {
		t.Fatalf("emitted line is not valid JSON: %v\nline: %s", err, raw)
	}
	if env.Type != events.EventAgentCompleted {
		t.Fatalf("event type = %q, want %q", env.Type, events.EventAgentCompleted)
	}
	return env.Data
}

func TestEmitAgentCompletedFromLoopResult(t *testing.T) {
	var buf bytes.Buffer
	emitAgentCompleted(&buf, &agent.LoopResult{
		Iterations: []agent.IterationLog{{Iteration: 1}, {Iteration: 2}},
		StopReason: agent.StopSuccess,
		FinalLog: &agent.IterationLog{
			Status:       "success",
			DiffStat:     agent.DiffStat{Files: 2, Insertions: 7, Deletions: 1},
			ChangedFiles: []string{"a.go", "b.go"},
		},
	})

	// Exactly one NDJSON line.
	if n := bytes.Count(buf.Bytes(), []byte{'\n'}); n != 1 {
		t.Fatalf("want exactly one NDJSON line, got %d:\n%s", n, buf.String())
	}
	data := decodeAgentCompleted(t, buf.Bytes())
	if data.Iterations != 2 {
		t.Errorf("iterations = %d, want 2", data.Iterations)
	}
	if data.StopReason != string(agent.StopSuccess) {
		t.Errorf("stop_reason = %q, want %q", data.StopReason, agent.StopSuccess)
	}
	if data.Status != "success" || data.DiffStat.Files != 2 || len(data.ChangedFiles) != 2 {
		t.Errorf("unexpected payload: %+v", data)
	}
}

func TestEmitAgentCompletedFromLog(t *testing.T) {
	var buf bytes.Buffer
	emitAgentCompletedFromLog(&buf, &agent.IterationLog{
		Status:       "success",
		DiffStat:     agent.DiffStat{Files: 1},
		ChangedFiles: []string{"only.txt"},
	})
	data := decodeAgentCompleted(t, buf.Bytes())
	if data.Iterations != 1 {
		t.Errorf("single-shot iterations = %d, want 1", data.Iterations)
	}
	if data.StopReason != string(agent.StopSuccess) {
		t.Errorf("stop_reason = %q, want success", data.StopReason)
	}
}

func TestEmitAgentCompletedNilFinalLog(t *testing.T) {
	var buf bytes.Buffer
	// A loop that produced no FinalLog (e.g. failed before any iteration
	// log) must still emit a parseable terminal event, not panic.
	emitAgentCompleted(&buf, &agent.LoopResult{StopReason: agent.StopFailed})
	data := decodeAgentCompleted(t, buf.Bytes())
	if data.Iterations != 0 || data.StopReason != string(agent.StopFailed) {
		t.Errorf("unexpected payload for empty result: %+v", data)
	}
}

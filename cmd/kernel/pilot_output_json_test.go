package kernel

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/pilot"
)

func TestResolvePilotOutputFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", pilot.OutputFormatText, false},
		{"text", pilot.OutputFormatText, false},
		{"json", pilot.OutputFormatJSON, false},
		{"yaml", "", true},
		{"JSON", "", true}, // case-sensitive on purpose
	}
	for _, c := range cases {
		got, err := resolvePilotOutputFormat(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolvePilotOutputFormat(%q): want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolvePilotOutputFormat(%q): unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("resolvePilotOutputFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// decodePilotCompleted parses the single NDJSON line an emit* helper wrote
// and asserts it's a well-formed pilot.completed event, returning its Data.
func decodePilotCompleted(t *testing.T, raw []byte) pilot.PilotCompletedData {
	t.Helper()
	var env struct {
		Type events.Type              `json:"type"`
		Data pilot.PilotCompletedData `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(raw), &env); err != nil {
		t.Fatalf("emitted line is not valid JSON: %v\nline: %s", err, raw)
	}
	if env.Type != events.EventPilotCompleted {
		t.Fatalf("event type = %q, want %q", env.Type, events.EventPilotCompleted)
	}
	return env.Data
}

func TestEmitPilotCompletedFromLoopResult(t *testing.T) {
	var buf bytes.Buffer
	emitPilotCompleted(&buf, &pilot.LoopResult{
		Iterations: []pilot.IterationLog{{Iteration: 1}, {Iteration: 2}},
		StopReason: pilot.StopSuccess,
		FinalLog: &pilot.IterationLog{
			Status:       "success",
			DiffStat:     pilot.DiffStat{Files: 2, Insertions: 7, Deletions: 1},
			ChangedFiles: []string{"a.go", "b.go"},
		},
	})

	// Exactly one NDJSON line.
	if n := bytes.Count(buf.Bytes(), []byte{'\n'}); n != 1 {
		t.Fatalf("want exactly one NDJSON line, got %d:\n%s", n, buf.String())
	}
	data := decodePilotCompleted(t, buf.Bytes())
	if data.Iterations != 2 {
		t.Errorf("iterations = %d, want 2", data.Iterations)
	}
	if data.StopReason != string(pilot.StopSuccess) {
		t.Errorf("stop_reason = %q, want %q", data.StopReason, pilot.StopSuccess)
	}
	if data.Status != "success" || data.DiffStat.Files != 2 || len(data.ChangedFiles) != 2 {
		t.Errorf("unexpected payload: %+v", data)
	}
}

func TestEmitPilotCompletedFromLog(t *testing.T) {
	var buf bytes.Buffer
	emitPilotCompletedFromLog(&buf, &pilot.IterationLog{
		Status:       "success",
		DiffStat:     pilot.DiffStat{Files: 1},
		ChangedFiles: []string{"only.txt"},
	})
	data := decodePilotCompleted(t, buf.Bytes())
	if data.Iterations != 1 {
		t.Errorf("single-shot iterations = %d, want 1", data.Iterations)
	}
	if data.StopReason != string(pilot.StopSuccess) {
		t.Errorf("stop_reason = %q, want success", data.StopReason)
	}
}

func TestEmitPilotCompletedNilFinalLog(t *testing.T) {
	var buf bytes.Buffer
	// A loop that produced no FinalLog (e.g. failed before any iteration
	// log) must still emit a parseable terminal event, not panic.
	emitPilotCompleted(&buf, &pilot.LoopResult{StopReason: pilot.StopFailed})
	data := decodePilotCompleted(t, buf.Bytes())
	if data.Iterations != 0 || data.StopReason != string(pilot.StopFailed) {
		t.Errorf("unexpected payload for empty result: %+v", data)
	}
}

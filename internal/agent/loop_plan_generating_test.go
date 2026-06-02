package agent

// #74 Phase 1: in --output-format json the agent loop must bracket the
// (buffered) plan phase with a plan.generating "started" event before the
// planner call, and plan.loaded must carry the flattened step list. This
// is what retires moongit's spinner-over-a-void: a real start event plus
// the whole plan to render on completion.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. ConsoleSubscriber.renderJSON encodes to
// os.Stdout at call time, so reassigning the package var before fn runs
// captures the NDJSON event stream.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.Bytes()
	}()

	fn()

	_ = w.Close()
	os.Stdout = old
	return string(<-done)
}

func TestRunLoop_JSON_EmitsPlanGeneratingAndStepList(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	// A single named step that changes no files: plan-style terminates
	// with StopSuccess after one iteration, so we get exactly one
	// plan.generating / plan.loaded pair.
	plan := "- name: greet\n  shell: echo hello\n"
	stub := &stubLLMClient{plans: []string{plan}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	out := captureStdout(t, func() {
		_, err := RunLoop(context.Background(), RunOptions{
			Goal:          "say hello",
			RepoRoot:      repo,
			MaxIterations: 2,
			AutoApply:     true, // skip TTY gate
			OutputFormat:  OutputFormatJSON,
		})
		if err != nil {
			t.Errorf("RunLoop: %v", err)
		}
	})

	genIdx, loadedIdx := -1, -1
	var loaded events.PlanLoadedData
	sc := bufio.NewScanner(bytes.NewReader([]byte(out)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		var ev struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			// Non-JSON lines shouldn't appear in JSON mode; fail loud so a
			// stream-corruption regression is caught here too.
			t.Fatalf("non-JSON line in --output-format json stream: %q (%v)", sc.Text(), err)
		}
		switch events.Type(ev.Type) {
		case events.EventPlanGenerating:
			if genIdx == -1 {
				genIdx = line
			}
		case events.EventPlanLoaded:
			if loadedIdx == -1 {
				loadedIdx = line
				if err := json.Unmarshal(ev.Data, &loaded); err != nil {
					t.Fatalf("decode plan.loaded data: %v", err)
				}
			}
		}
		line++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan stdout: %v", err)
	}

	if genIdx == -1 {
		t.Fatal("no plan.generating event in the JSON stream")
	}
	if loadedIdx == -1 {
		t.Fatal("no plan.loaded event in the JSON stream")
	}
	if genIdx >= loadedIdx {
		t.Errorf("plan.generating (line %d) must precede plan.loaded (line %d)", genIdx, loadedIdx)
	}

	// Option C: the flattened step list is present and names the real step
	// (not the synthetic transaction wrapper the agent loop adds).
	found := false
	for _, s := range loaded.Steps {
		if s == "greet" {
			found = true
		}
		if s == transactionWrapName {
			t.Errorf("plan.loaded.steps leaked the transaction wrapper %q; want flattened children", s)
		}
	}
	if !found {
		t.Errorf("plan.loaded.steps = %v, want it to include %q", loaded.Steps, "greet")
	}
}

func TestRunLoop_Text_NoPlanGeneratingNoise(t *testing.T) {
	// In text mode the console subscriber's renderText has no case for
	// plan.generating, so emitting it must produce no stdout noise — the
	// human watches the planning latency live and a bare event line would
	// be clutter. (run.started/plan.loaded are likewise silent in text.)
	repo := t.TempDir()
	initGitRepo(t, repo)
	plan := "- name: greet\n  shell: echo hello\n"
	stub := &stubLLMClient{plans: []string{plan}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	out := captureStdout(t, func() {
		_, err := RunLoop(context.Background(), RunOptions{
			Goal:          "say hello",
			RepoRoot:      repo,
			MaxIterations: 2,
			AutoApply:     true,
			OutputFormat:  "text",
		})
		if err != nil {
			t.Errorf("RunLoop: %v", err)
		}
	})

	if bytes.Contains([]byte(out), []byte("plan.generating")) {
		t.Errorf("text-mode stdout leaked a plan.generating line:\n%s", out)
	}
}

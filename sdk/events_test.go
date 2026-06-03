package mooncake_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// initGitRepo makes dir a minimal git repo with one commit, so the agent's
// snapshot collection (git branch/head) succeeds. GIT_* is scrubbed from the
// child env: under a pre-push hook GIT_DIR/GIT_WORK_TREE would otherwise
// redirect these commands at the real repo regardless of cmd.Dir.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	clean := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			clean = append(clean, e)
		}
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = clean
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// recordingSubscriber is a facade-only mooncake.Subscriber that records every
// event type it observes. It is the in-process tap a consumer wires into
// RunOptions.Subscribers (#121).
type recordingSubscriber struct {
	mu    sync.Mutex
	types []mooncake.EventType
}

func (r *recordingSubscriber) OnEvent(e mooncake.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types = append(r.types, e.Type)
}

func (r *recordingSubscriber) Close() {}

func (r *recordingSubscriber) saw(t mooncake.EventType) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.types, t)
}

// TestSubscriberReceivesLiveEvents drives a single-shot agent run against a
// trivial print plan with a fake subscriber attached via RunOptions.Subscribers
// and asserts the step lifecycle events arrive through OnEvent — proving #121
// wires a consumer's subscriber into the executor's publisher (not just the
// stdout NDJSON). No LLM is involved: Run executes a provided plan.
func TestSubscriberReceivesLiveEvents(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	planPath := filepath.Join(repo, "plan.yml")
	// A single log step: side-effect-free, no network, and the executor
	// emits step.started / step.completed for it.
	plan := "- name: say hello\n  log:\n    msg: \"hello from sdk test\"\n"
	if err := os.WriteFile(planPath, []byte(plan), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	sub := &recordingSubscriber{}

	_, err := mooncake.Run(context.Background(), mooncake.RunOptions{
		Goal:         "print hello",
		PlanPath:     planPath,
		RepoRoot:     repo,
		AutoApply:    true, // skip the interactive confirm gate
		OutputFormat: mooncake.OutputFormatJSON,
		Subscribers:  []mooncake.Subscriber{sub},
		Logger:       mooncake.NewDiscardLogger(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The executor's publisher is Closed (and thus drained) before Run
	// returns, so by here every event has been delivered to OnEvent.
	if !sub.saw(mooncake.EventStepStarted) {
		t.Errorf("subscriber never saw step.started; saw=%v", sub.types)
	}
	if !sub.saw(mooncake.EventStepCompleted) {
		t.Errorf("subscriber never saw step.completed; saw=%v", sub.types)
	}
	if !sub.saw(mooncake.EventRunCompleted) {
		t.Errorf("subscriber never saw run.completed; saw=%v", sub.types)
	}
}

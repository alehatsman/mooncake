package notify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/alehatsman/mooncake/examples/notify"
	mooncake "github.com/alehatsman/mooncake/sdk"
)

// stubLLM is a deterministic reasoning backend (the #106 LLMClient seam): it
// replays a fixed sequence of plans instead of calling a provider, so the test
// is offline and stable. Implemented against the facade's single-method
// interface.
type stubLLM struct {
	plans []string
	calls int
}

func (s *stubLLM) GeneratePlan(_ context.Context, _, _, _ string) (string, error) {
	i := s.calls
	s.calls++
	if i < len(s.plans) {
		return s.plans[i], nil
	}
	return "[]\n", nil // empty plans terminate the loop's no-change streak
}

// TestWebhookHandler_E2E proves the framework path end-to-end through the SDK
// facade alone: a consumer registers notify.WebhookHandler into a registry and
// injects a stub backend; a carrier-form plan (action:/with:, #111) dispatches
// to the handler, which POSTs to a live test server. Offline, deterministic, no
// internal/ import. Also asserts a credential passed through headers never
// leaks into the persisted loop result.
func TestWebhookHandler_E2E(t *testing.T) {
	var (
		mu        sync.Mutex
		hits      int
		gotMethod string
		gotAuth   string
		gotBody   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		hits++
		gotMethod, gotAuth, gotBody = r.Method, r.Header.Get("Authorization"), string(b)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := t.TempDir()
	initGitRepo(t, repo)

	const secret = "s3cr3t-token-xyz"
	// A carrier-form plan: no built-in field, dispatched purely by action name.
	plan := fmt.Sprintf(
		`[{"name":"notify deploy","action":"notify.webhook","with":{"url":%q,"method":"POST","headers":{"Authorization":"Bearer %s"},"body":"{\"event\":\"deploy\",\"ok\":true}"}}]`,
		srv.URL, secret,
	)
	backend := &stubLLM{plans: []string{plan}}

	reg := mooncake.DefaultRegistry()
	if err := reg.Register(notify.WebhookHandler{}); err != nil {
		t.Fatalf("register notify.webhook: %v", err)
	}

	result, err := mooncake.RunLoop(context.Background(), mooncake.RunOptions{
		Goal:          "notify the deploy webhook",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true, // skip the interactive approval gate
		LLMClient:     backend,
		Registry:      reg,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits == 0 {
		t.Fatalf("webhook never received a request — carrier did not dispatch (stop=%s, iters=%d)",
			result.StopReason, len(result.Iterations))
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if !strings.Contains(gotBody, `"event":"deploy"`) {
		t.Errorf("body not delivered: got %q", gotBody)
	}
	if gotAuth != "Bearer "+secret {
		t.Errorf("custom header not forwarded: got %q", gotAuth)
	}

	// Secret-non-leak: the credential reached the server but must not appear in
	// the persisted iteration record the agent writes to its runlog.
	blob, _ := json.Marshal(result)
	if strings.Contains(string(blob), secret) {
		t.Errorf("credential leaked into the loop result/runlog surface")
	}
}

// TestWebhookHandler_TypedKeyE2E proves the typed-key form (#115): a custom
// action written exactly like a built-in —
//
//   - name: notify deploy success
//     notify.webhook:
//     url: …
//     method: POST
//
// dispatches end-to-end with no carrier (`action:`/`with:`) in the plan. The
// config layer folds the typed key into the carrier against the run's registry
// before validation and execution, so the planner needs no special teaching.
// Same harness as TestWebhookHandler_E2E; only the plan syntax differs.
func TestWebhookHandler_TypedKeyE2E(t *testing.T) {
	var (
		mu        sync.Mutex
		hits      int
		gotMethod string
		gotAuth   string
		gotBody   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		hits++
		gotMethod, gotAuth, gotBody = r.Method, r.Header.Get("Authorization"), string(b)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := t.TempDir()
	initGitRepo(t, repo)

	const secret = "s3cr3t-token-xyz"
	// Typed-key YAML — notify.webhook as a step key, like shell:/file:. No
	// carrier anywhere; the body is single-quoted so the JSON survives YAML.
	plan := fmt.Sprintf(`- name: notify deploy success
  notify.webhook:
    url: %s
    method: POST
    headers:
      Authorization: Bearer %s
    body: '{"event":"deploy","ok":true}'
`, srv.URL, secret)
	backend := &stubLLM{plans: []string{plan}}

	reg := mooncake.DefaultRegistry()
	if err := reg.Register(notify.WebhookHandler{}); err != nil {
		t.Fatalf("register notify.webhook: %v", err)
	}

	result, err := mooncake.RunLoop(context.Background(), mooncake.RunOptions{
		Goal:          "notify the deploy webhook",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
		LLMClient:     backend,
		Registry:      reg,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits == 0 {
		t.Fatalf("webhook never received a request — typed-key did not dispatch (stop=%s, iters=%d)",
			result.StopReason, len(result.Iterations))
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if !strings.Contains(gotBody, `"event":"deploy"`) {
		t.Errorf("body not delivered: got %q", gotBody)
	}
	if gotAuth != "Bearer "+secret {
		t.Errorf("custom header not forwarded: got %q", gotAuth)
	}
}

// TestWebhookHandler_Validate covers the fail-fast config checks the executor
// runs before dispatch — no network, no loop.
func TestWebhookHandler_Validate(t *testing.T) {
	h := notify.WebhookHandler{}
	if err := h.Validate(&mooncake.Step{With: map[string]interface{}{}}); err == nil {
		t.Error("missing url: want error, got nil")
	}
	if err := h.Validate(&mooncake.Step{With: map[string]interface{}{"url": "https://x", "method": "DELETE"}}); err == nil {
		t.Error("bad method: want error, got nil")
	}
	if err := h.Validate(&mooncake.Step{With: map[string]interface{}{"url": "https://x"}}); err != nil {
		t.Errorf("valid step: want nil, got %v", err)
	}
}

// initGitRepo creates a minimal git repo the agent loop's snapshot step needs.
// GIT_* env vars are scrubbed so an exec'd git can't retarget the parent repo.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	clean := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			clean = append(clean, e)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = clean
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

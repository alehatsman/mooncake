package download

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
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
	return &executor.ExecutionContext{
		Variables:  map[string]interface{}{},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		CurrentMode: planMode(plan),
		Stats:      executor.NewExecutionStats(),
	}
}

func sha256OfBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// TestRun_DestMissing_WouldDownload: dest doesn't exist → plan reports
// would-download.
func TestRun_DestMissing_WouldDownload(t *testing.T) {
	step := &config.Step{
		Download: &config.Download{
			URL:  "https://example.com/x",
			Dest: filepath.Join(t.TempDir(), "x"),
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("should report would-download; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "missing") {
		t.Errorf("reason should mention missing dest; got %q", r.Reason)
	}
}

// TestRun_ExistingWithCorrectChecksum_AlreadyOk: dest exists, checksum
// matches → already-ok.
func TestRun_ExistingWithCorrectChecksum_AlreadyOk(t *testing.T) {
	body := []byte("the content")
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		Download: &config.Download{
			URL:      "https://example.com/x",
			Dest:     dest,
			Checksum: sha256OfBytes(body),
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("should be already-ok; reason=%q", r.Reason)
	}
}

// TestRun_ExistingWithBadChecksum_WouldDownload: dest exists but
// checksum mismatches → would-download.
func TestRun_ExistingWithBadChecksum_WouldDownload(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		Download: &config.Download{
			URL:      "https://example.com/x",
			Dest:     dest,
			Checksum: sha256OfBytes([]byte("expected")),
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("should report would-download on checksum mismatch")
	}
	if !strings.Contains(r.Reason, "mismatch") {
		t.Errorf("reason should mention mismatch; got %q", r.Reason)
	}
}

// TestRun_Force_AlwaysWouldDownload: force=true ignores dest state.
func TestRun_Force_AlwaysWouldDownload(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(dest, []byte("x"), 0o644)
	step := &config.Step{
		Download: &config.Download{
			URL:   "https://example.com/x",
			Dest:  dest,
			Force: true,
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("force=true should always report would-download")
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

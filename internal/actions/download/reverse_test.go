package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestReverse_CreateCycle: download to a fresh path, reverse should
// produce a state=absent step that — when applied via the file
// handler — deletes the downloaded file.
func TestReverse_CreateCycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("downloaded payload"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "fetched.bin")
	step := &config.Step{
		FileDownload: &config.Download{
			URL:  srv.URL,
			Dest: dest,
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Fatalf("apply: expected Changed=true; got result=%+v", r)
	}
	info := r.ReverseData.(*filehandler.FileReverseInfo)
	if info.Existed {
		t.Errorf("captured Existed=true for fresh dest; want false")
	}

	reverseStep, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite == nil || reverseStep.FileWrite.State != "absent" {
		t.Fatalf("reverse step bad: %+v", reverseStep.FileWrite)
	}

	fh := &filehandler.Handler{}
	if _, err := fh.Run(newCtx(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest still exists after reverse: %v", err)
	}
}

// TestReverse_OverwriteCycle: dest exists pre-apply, download with
// Force=true replaces it; reverse must restore pre-apply bytes +
// mode.
func TestReverse_OverwriteCycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("NEW downloaded content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "existing.bin")
	const original = "ORIGINAL bytes that must come back\n"
	if err := os.WriteFile(dest, []byte(original), 0o640); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	step := &config.Step{
		FileDownload: &config.Download{
			URL:   srv.URL,
			Dest:  dest,
			Force: true,
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r := res.(*executor.Result)
	info := r.ReverseData.(*filehandler.FileReverseInfo)
	if !info.Existed {
		t.Fatalf("captured Existed=false; want true (we seeded dest)")
	}
	if string(info.Content) != original {
		t.Fatalf("captured Content = %q, want %q", info.Content, original)
	}

	reverseStep, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}

	fh := &filehandler.Handler{}
	if _, err := fh.Run(newCtx(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != original {
		t.Errorf("after reverse: %q, want %q", got, original)
	}
}

func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}

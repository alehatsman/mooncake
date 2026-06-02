package effects

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_identical(t *testing.T) {
	result := unifiedDiff("/etc/foo", []byte("a\nb\nc\n"), []byte("a\nb\nc\n"))
	if result != "" {
		t.Errorf("expected empty diff for identical content, got: %q", result)
	}
}

func TestUnifiedDiff_basic(t *testing.T) {
	old := "worker_processes auto;\nworker_connections 512;\nevents {}\n"
	new := "worker_processes auto;\nworker_connections 1024;\nevents { worker_connections 1024; }\n"

	result := unifiedDiff("/etc/nginx/nginx.conf", []byte(old), []byte(new))

	if !strings.Contains(result, "--- /etc/nginx/nginx.conf") {
		t.Errorf("missing --- header: %s", result)
	}
	if !strings.Contains(result, "+++ /etc/nginx/nginx.conf (proposed)") {
		t.Errorf("missing +++ header: %s", result)
	}
	if !strings.Contains(result, "-worker_connections 512;") {
		t.Errorf("missing deletion line: %s", result)
	}
	if !strings.Contains(result, "+worker_connections 1024;") {
		t.Errorf("missing insertion line: %s", result)
	}
}

func TestUnifiedDiff_newFile(t *testing.T) {
	result := unifiedDiff("/etc/foo", []byte{}, []byte("new content\n"))
	// Empty old content: produces a diff adding all lines
	if !strings.Contains(result, "+new content") {
		t.Errorf("expected insertion of new content: %s", result)
	}
}

func TestUnifiedDiff_newFileHunkHeader(t *testing.T) {
	// Inserting into an empty 'a' side must emit a zero-length old
	// range as "-0,0" (unified-diff convention), not the off-by-one
	// "-1,0" that a start-clamped-to-1 header would produce.
	result := unifiedDiff("/etc/foo", []byte{}, []byte("new content\n"))
	if !strings.Contains(result, "@@ -0,0 +1,1 @@") {
		t.Errorf("expected zero-length old range header '@@ -0,0 +1,1 @@', got:\n%s", result)
	}
	if strings.Contains(result, "-1,0") {
		t.Errorf("off-by-one hunk header '-1,0' for empty original:\n%s", result)
	}
}

func TestUnifiedDiff_binary(t *testing.T) {
	result := unifiedDiff("/bin/foo", []byte("text"), []byte{0x00, 0x01, 0x02})
	if !strings.Contains(result, "Binary files") {
		t.Errorf("expected binary notice: %s", result)
	}
}

func TestUnifiedDiff_context(t *testing.T) {
	// Change in the middle of many lines — context lines should appear
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "unchanged line\n"
	}
	old := strings.Join(lines, "")
	lines[10] = "CHANGED line\n"
	new := strings.Join(lines, "")

	result := unifiedDiff("/etc/foo", []byte(old), []byte(new))
	if !strings.Contains(result, "@@") {
		t.Errorf("expected @@ hunk header: %s", result)
	}
	if !strings.Contains(result, "+CHANGED line") {
		t.Errorf("expected changed line: %s", result)
	}
}

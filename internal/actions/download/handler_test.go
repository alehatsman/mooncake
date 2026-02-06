package download

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// mockExecutionContext creates a minimal ExecutionContext for testing
func mockExecutionContext() *executor.ExecutionContext {
	ctx := testutil.NewMockContext()
	tmpl := template.NewPongo2Renderer()
	return &executor.ExecutionContext{
		Variables:      ctx.Variables,
		Template:       tmpl,
		Evaluator:      ctx.GetEvaluator(),
		Logger:         ctx.Log,
		EventPublisher: ctx.Publisher,
		CurrentStepID:  ctx.StepID,
		PathUtil:       pathutil.NewPathExpander(tmpl),
		CurrentDir:     "/tmp",
		DryRun:         false,
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "download" {
		t.Errorf("Name = %v, want 'download'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryNetwork {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryNetwork)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if !meta.SupportsBecome {
		t.Error("SupportsBecome should be true")
	}
	if len(meta.EmitsEvents) != 1 {
		t.Errorf("EmitsEvents length = %d, want 1", len(meta.EmitsEvents))
	}
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventFileDownloaded) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventFileDownloaded))
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %v, want '1.0.0'", meta.Version)
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid download action",
			step: &config.Step{
				Download: &config.Download{
					URL:  "https://example.com/file.txt",
					Dest: "/tmp/file.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "nil download action",
			step: &config.Step{
				Download: nil,
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			step: &config.Step{
				Download: &config.Download{
					Dest: "/tmp/file.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing dest",
			step: &config.Step{
				Download: &config.Download{
					URL: "https://example.com/file.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "empty URL",
			step: &config.Step{
				Download: &config.Download{
					URL:  "",
					Dest: "/tmp/file.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "empty dest",
			step: &config.Step{
				Download: &config.Download{
					URL:  "https://example.com/file.txt",
					Dest: "",
				},
			},
			wantErr: true,
		},
		{
			name: "with checksum",
			step: &config.Step{
				Download: &config.Download{
					URL:      "https://example.com/file.txt",
					Dest:     "/tmp/file.txt",
					Checksum: "md5:d8e8fca2dc0f896fd7cb4cb0031ba249",
				},
			},
			wantErr: false,
		},
		{
			name: "with headers",
			step: &config.Step{
				Download: &config.Download{
					URL:  "https://example.com/file.txt",
					Dest: "/tmp/file.txt",
					Headers: map[string]string{
						"Authorization": "Bearer token",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_BasicDownload(t *testing.T) {
	h := &Handler{}

	// Create test server
	testContent := "test file content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	// Create temp directory for test
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  server.URL,
			Dest: destPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check result
	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Execute() result is not *executor.Result")
	}

	if !execResult.Changed {
		t.Error("Result.Changed should be true for new download")
	}

	if execResult.Failed {
		t.Error("Result.Failed should be false")
	}

	// Verify file was created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Downloaded content = %q, want %q", string(content), testContent)
	}

	// Check event was published
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	event := pub.Events[0]
	if event.Type != events.EventFileDownloaded {
		t.Errorf("Event.Type = %v, want %v", event.Type, events.EventFileDownloaded)
	}

	downloadData, ok := event.Data.(events.FileDownloadedData)
	if !ok {
		t.Fatalf("Event.Data is not events.FileDownloadedData")
	}

	if downloadData.Dest != destPath {
		t.Errorf("FileDownloadedData.Dest = %v, want %v", downloadData.Dest, destPath)
	}

	if downloadData.SizeBytes != int64(len(testContent)) {
		t.Errorf("FileDownloadedData.SizeBytes = %v, want %v", downloadData.SizeBytes, len(testContent))
	}
}

func TestHandler_Execute_WithChecksum(t *testing.T) {
	h := &Handler{}

	testContent := "test content for checksum"

	// Calculate MD5 checksum (without prefix)
	hasher := md5.New()
	hasher.Write([]byte(testContent))
	md5sum := fmt.Sprintf("%x", hasher.Sum(nil))

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "checksum.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:      server.URL,
			Dest:     destPath,
			Checksum: md5sum,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true for new download")
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}
}

func TestHandler_Execute_ChecksumMismatch(t *testing.T) {
	h := &Handler{}

	testContent := "test content"
	wrongChecksum := "00000000000000000000000000000000" // 32 hex chars (MD5)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "mismatch.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:      server.URL,
			Dest:     destPath,
			Checksum: wrongChecksum,
		},
	}

	result, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on checksum mismatch")
	}

	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("Error should mention checksum mismatch, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on checksum mismatch")
	}
}

func TestHandler_Execute_IdempotencyWithChecksum(t *testing.T) {
	h := &Handler{}

	testContent := "idempotent content"

	// Calculate SHA256 checksum (without prefix)
	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	sha256sum := fmt.Sprintf("%x", hasher.Sum(nil))

	// Create file with correct content
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "idempotent.txt")
	err := os.WriteFile(destPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test server (should not be called if idempotent)
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:      server.URL,
			Dest:     destPath,
			Checksum: sha256sum,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when file exists with correct checksum")
	}

	if serverCalled {
		t.Error("Server should not be called when file exists with correct checksum")
	}
}

func TestHandler_Execute_ForceRedownload(t *testing.T) {
	h := &Handler{}

	testContent := "forced content"

	// Create existing file
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "force.txt")
	err := os.WriteFile(destPath, []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:   server.URL,
			Dest:  destPath,
			Force: true,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when force is enabled")
	}

	// Verify content was updated
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Content = %q, want %q", string(content), testContent)
	}
}

func TestHandler_Execute_WithCustomHeaders(t *testing.T) {
	h := &Handler{}

	testContent := "content with auth"
	expectedHeader := "Bearer secret-token"
	receivedHeader := ""

	// Create test server that checks headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "headers.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  server.URL,
			Dest: destPath,
			Headers: map[string]string{
				"Authorization": expectedHeader,
			},
		},
	}

	_, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if receivedHeader != expectedHeader {
		t.Errorf("Received header = %q, want %q", receivedHeader, expectedHeader)
	}
}

func TestHandler_Execute_HTTPError(t *testing.T) {
	h := &Handler{}

	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "error.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  server.URL,
			Dest: destPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on HTTP 404")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention 404, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on HTTP error")
	}
}

func TestHandler_Execute_WithTimeout(t *testing.T) {
	h := &Handler{}

	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "timeout.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:     server.URL,
			Dest:    destPath,
			Timeout: "50ms",
		},
	}

	result, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on timeout")
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on timeout")
	}
}

func TestHandler_Execute_WithRetry(t *testing.T) {
	h := &Handler{}

	attemptCount := 0
	testContent := "retry content"

	// Create server that fails first attempt, succeeds second
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "retry.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:     server.URL,
			Dest:    destPath,
			Retries: 2,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() should succeed on retry, got error: %v", err)
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}
}

func TestHandler_Execute_RetryExhausted(t *testing.T) {
	h := &Handler{}

	// Create server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "failed.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:     server.URL,
			Dest:    destPath,
			Retries: 2,
		},
		RetryDelay: "10ms",
	}

	result, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error after retries exhausted")
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true")
	}
}

func TestHandler_Execute_WithMode(t *testing.T) {
	h := &Handler{}

	testContent := "content with mode"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "mode.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  server.URL,
			Dest: destPath,
			Mode: "0600",
		},
	}

	_, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check file permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0600)
	if mode != expectedMode {
		t.Errorf("File mode = %o, want %o", mode, expectedMode)
	}
}

func TestHandler_Execute_SHA256Checksum(t *testing.T) {
	h := &Handler{}

	testContent := "sha256 test"

	// Calculate SHA256 checksum (without prefix)
	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	sha256sum := fmt.Sprintf("%x", hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "sha256.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:      server.URL,
			Dest:     destPath,
			Checksum: sha256sum,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}
}

func TestHandler_Execute_InvalidTimeout(t *testing.T) {
	h := &Handler{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "invalid-timeout.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:     server.URL,
			Dest:    destPath,
			Timeout: "invalid",
		},
	}

	_, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on invalid timeout")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Error should mention timeout, got: %v", err)
	}
}

func TestHandler_Execute_TemplateRendering(t *testing.T) {
	h := &Handler{}

	testContent := "templated content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that URL path contains the rendered variable
		if !strings.Contains(r.URL.Path, "/files/myfile.txt") {
			t.Errorf("Expected URL to contain /files/myfile.txt, got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "templated.txt")

	ec := mockExecutionContext()
	ec.Variables["filename"] = "myfile.txt"
	ec.Variables["destdir"] = tmpDir

	step := &config.Step{
		Download: &config.Download{
			URL:  server.URL + "/files/{{ filename }}",
			Dest: "{{ destdir }}/templated.txt",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file was created at correct path
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist at templated path")
	}
}

func TestHandler_Execute_NoPublisher(t *testing.T) {
	h := &Handler{}

	testContent := "no publisher"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "nopub.txt")

	ec := mockExecutionContext()
	ec.EventPublisher = nil

	step := &config.Step{
		Download: &config.Download{
			URL:  server.URL,
			Dest: destPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}
}

func TestHandler_DryRun(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		setup   func(*executor.ExecutionContext, string)
		wantErr bool
	}{
		{
			name: "basic dry-run",
			step: &config.Step{
				Download: &config.Download{
					URL:  "https://example.com/file.txt",
					Dest: "/tmp/file.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with checksum",
			step: &config.Step{
				Download: &config.Download{
					URL:      "https://example.com/file.txt",
					Dest:     "/tmp/file.txt",
					Checksum: "md5:abc123",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with existing file",
			step: &config.Step{
				Download: &config.Download{
					URL:  "https://example.com/file.txt",
					Dest: "/tmp/existing.txt",
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				destPath := filepath.Join(tmpDir, "existing.txt")
				os.WriteFile(destPath, []byte("existing"), 0644)
				ec.Variables["tmpdir"] = tmpDir
			},
			wantErr: false,
		},
		{
			name: "dry-run with headers",
			step: &config.Step{
				Download: &config.Download{
					URL:  "https://example.com/file.txt",
					Dest: "/tmp/file.txt",
					Headers: map[string]string{
						"Authorization": "Bearer token",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with timeout",
			step: &config.Step{
				Download: &config.Download{
					URL:     "https://example.com/file.txt",
					Dest:    "/tmp/file.txt",
					Timeout: "30s",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with retries",
			step: &config.Step{
				Download: &config.Download{
					URL:     "https://example.com/file.txt",
					Dest:    "/tmp/file.txt",
					Retries: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with backup",
			step: &config.Step{
				Download: &config.Download{
					URL:    "https://example.com/file.txt",
					Dest:   "/tmp/backup.txt",
					Backup: true,
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				destPath := filepath.Join(tmpDir, "backup.txt")
				os.WriteFile(destPath, []byte("old"), 0644)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			ec := mockExecutionContext()
			ec.CurrentDir = tmpDir

			if tt.setup != nil {
				tt.setup(ec, tmpDir)
			}

			// Update dest path to use temp dir if it's a template
			if tt.step.Download.Dest == "/tmp/existing.txt" && tt.setup != nil {
				tt.step.Download.Dest = filepath.Join(tmpDir, "existing.txt")
			}
			if tt.step.Download.Dest == "/tmp/backup.txt" && tt.setup != nil {
				tt.step.Download.Dest = filepath.Join(tmpDir, "backup.txt")
			}

			err := h.DryRun(ec, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check that something was logged
			log := ec.Logger.(*testutil.MockLogger)
			if len(log.Logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

func TestHandler_DryRun_IdempotentCheck(t *testing.T) {
	h := &Handler{}

	testContent := "existing content"

	// Calculate checksum
	hasher := md5.New()
	hasher.Write([]byte(testContent))
	md5sum := fmt.Sprintf("md5:%x", hasher.Sum(nil))

	// Create file with correct checksum
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "idempotent.txt")
	err := os.WriteFile(destPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:      "https://example.com/file.txt",
			Dest:     destPath,
			Checksum: md5sum,
		},
	}

	err = h.DryRun(ec, step)
	if err != nil {
		t.Errorf("DryRun() error = %v", err)
	}

	// Check log message mentions already downloaded
	log := ec.Logger.(*testutil.MockLogger)
	hasIdempotentMessage := false
	for _, msg := range log.Logs {
		if strings.Contains(msg, "already downloaded") || strings.Contains(msg, "correct checksum") {
			hasIdempotentMessage = true
			break
		}
	}

	if !hasIdempotentMessage {
		t.Error("DryRun() should log idempotent message when file exists with correct checksum")
	}
}

func TestHandler_parseFileMode(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name        string
		modeStr     string
		defaultMode os.FileMode
		want        os.FileMode
	}{
		{
			name:        "empty string uses default",
			modeStr:     "",
			defaultMode: 0644,
			want:        0644,
		},
		{
			name:        "valid octal mode",
			modeStr:     "0755",
			defaultMode: 0644,
			want:        0755,
		},
		{
			name:        "valid mode without leading zero",
			modeStr:     "644",
			defaultMode: 0600,
			want:        0644,
		},
		{
			name:        "invalid mode uses default",
			modeStr:     "invalid",
			defaultMode: 0644,
			want:        0644,
		},
		{
			name:        "restrictive mode",
			modeStr:     "0600",
			defaultMode: 0644,
			want:        0600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.parseFileMode(tt.modeStr, tt.defaultMode)
			if got != tt.want {
				t.Errorf("parseFileMode() = %o, want %o", got, tt.want)
			}
		})
	}
}

func TestHandler_formatMode(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{
			name: "standard file mode",
			mode: 0644,
			want: "0644",
		},
		{
			name: "executable mode",
			mode: 0755,
			want: "0755",
		},
		{
			name: "restrictive mode",
			mode: 0600,
			want: "0600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.formatMode(tt.mode)
			if got != tt.want {
				t.Errorf("formatMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHandler_Execute_RenderError tests error handling when template rendering fails
func TestHandler_Execute_RenderError(t *testing.T) {
	h := &Handler{}

	ec := mockExecutionContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  "{{ invalid template syntax",
			Dest: "/tmp/file.txt",
		},
	}

	_, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on invalid template")
	}

	if !strings.Contains(err.Error(), "render") {
		t.Errorf("Error should mention render failure, got: %v", err)
	}
}

// TestHandler_Execute_NotExecutionContext tests handling when context is not ExecutionContext
func TestHandler_Execute_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	ctx := testutil.NewMockContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  "https://example.com/file.txt",
			Dest: "/tmp/file.txt",
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when context is not ExecutionContext")
	}

	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

// TestHandler_DryRun_NotExecutionContext tests DryRun handling when context is not ExecutionContext
func TestHandler_DryRun_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	ctx := testutil.NewMockContext()
	step := &config.Step{
		Download: &config.Download{
			URL:  "https://example.com/file.txt",
			Dest: "/tmp/file.txt",
		},
	}

	err := h.DryRun(ctx, step)
	if err == nil {
		t.Error("DryRun() should error when context is not ExecutionContext")
	}

	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

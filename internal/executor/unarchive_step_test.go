package executor

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
)

// Test helper: create a tar archive with files
func createTestTar(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar file: %v", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}
}

// Test helper: create a tar.gz archive with files
func createTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar.gz file: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}
}

// Test helper: create a zip archive with files
func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write zip content: %v", err)
		}
	}
}

// Test helper: create a malicious tar with path traversal
func createMaliciousTar(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar file: %v", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	// Add entries with path traversal attempts
	maliciousPaths := []string{
		"../../../etc/passwd",
		"legit/../../sensitive",
		"/etc/passwd",
	}

	for _, name := range maliciousPaths {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len("malicious")),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte("malicious")); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}
}

func TestHandleUnarchive_MissingSrc(t *testing.T) {
	ec := newTestExecutionContext(t)
	step := config.Step{
		Unarchive: &config.Unarchive{
			Dest: "/tmp/dest",
		},
	}

	err := HandleUnarchive(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing src, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "src" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "src")
	}
}

func TestHandleUnarchive_MissingDest(t *testing.T) {
	ec := newTestExecutionContext(t)
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src: "/tmp/test.tar",
		},
	}

	err := HandleUnarchive(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing dest, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "dest" {
		t.Errorf("Field = %q, want %q", validationErr.Field, "dest")
	}
}

func TestHandleUnarchive_NonExistentSrc(t *testing.T) {
	ec := newTestExecutionContext(t)
	nonExistentPath := filepath.Join(ec.CurrentDir, "nonexistent.tar")

	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  nonExistentPath,
			Dest: filepath.Join(ec.CurrentDir, "dest"),
		},
	}

	err := HandleUnarchive(step, ec)
	if err == nil {
		t.Fatal("Expected error for non-existent src, got nil")
	}

	var fileErr *FileOperationError
	if !errors.As(err, &fileErr) {
		t.Fatalf("expected FileOperationError, got %T: %v", err, err)
	}
}

func TestHandleUnarchive_UnsupportedFormat(t *testing.T) {
	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "test.unknown")
	createTestFile(t, srcPath, "test content")

	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: filepath.Join(ec.CurrentDir, "dest"),
		},
	}

	err := HandleUnarchive(step, ec)
	if err == nil {
		t.Fatal("Expected error for unsupported format, got nil")
	}

	var validationErr *StepValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected StepValidationError, got %T: %v", err, err)
	}
	if !strings.Contains(validationErr.Message, "unsupported") {
		t.Errorf("Expected 'unsupported' in error message, got: %s", validationErr.Message)
	}
}

func TestHandleUnarchive_TarExtraction(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt":     "content1",
		"dir/file2.txt": "content2",
		"dir/file3.txt": "content3",
	}
	createTestTar(t, srcPath, files)

	// Extract
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	// Subscribe to events
	collector := &testEventCollector{}
	ec.EventPublisher.Subscribe(collector)

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify extracted files
	for name, expectedContent := range files {
		extractedPath := filepath.Join(destPath, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, string(content), expectedContent)
		}
	}

	// Verify event was emitted
	evts := collector.getEvents()
	found := false
	for _, e := range evts {
		if e.Type == events.EventArchiveExtracted {
			found = true
			data, ok := e.Data.(events.ArchiveExtractedData)
			if !ok {
				t.Errorf("Event data is not ArchiveExtractedData: %T", e.Data)
				continue
			}
			if data.Format != "tar" {
				t.Errorf("Format = %q, want %q", data.Format, "tar")
			}
			if data.FilesExtracted != 3 {
				t.Errorf("FilesExtracted = %d, want 3", data.FilesExtracted)
			}
		}
	}
	if !found {
		t.Error("archive.extracted event not emitted")
	}
}

func TestHandleUnarchive_TarGzExtraction(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar.gz
	srcPath := filepath.Join(ec.CurrentDir, "test.tar.gz")
	files := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}
	createTestTarGz(t, srcPath, files)

	// Extract
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify extracted files
	for name, expectedContent := range files {
		extractedPath := filepath.Join(destPath, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, string(content), expectedContent)
		}
	}
}

func TestHandleUnarchive_TgzExtraction(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test .tgz (should be treated same as .tar.gz)
	srcPath := filepath.Join(ec.CurrentDir, "test.tgz")
	files := map[string]string{
		"file1.txt": "content1",
	}
	createTestTarGz(t, srcPath, files)

	// Extract
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify format detected
	collector := &testEventCollector{}
	ec.EventPublisher.Subscribe(collector)

	// Run again to trigger event
	destPath2 := filepath.Join(ec.CurrentDir, "extracted2")
	step.Unarchive.Dest = destPath2
	err = HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	evts := collector.getEvents()
	for _, e := range evts {
		if e.Type == events.EventArchiveExtracted {
			data := e.Data.(events.ArchiveExtractedData)
			if data.Format != "tar.gz" {
				t.Errorf("Format = %q, want %q", data.Format, "tar.gz")
			}
		}
	}
}

func TestHandleUnarchive_ZipExtraction(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test zip
	srcPath := filepath.Join(ec.CurrentDir, "test.zip")
	files := map[string]string{
		"file1.txt":     "content1",
		"dir/file2.txt": "content2",
	}
	createTestZip(t, srcPath, files)

	// Extract
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify extracted files
	for name, expectedContent := range files {
		extractedPath := filepath.Join(destPath, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, string(content), expectedContent)
		}
	}
}

func TestHandleUnarchive_StripComponents(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar with nested structure
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"root/level1/level2/file1.txt": "content1",
		"root/level1/file2.txt":        "content2",
		"root/file3.txt":               "content3",
	}
	createTestTar(t, srcPath, files)

	// Extract with strip_components=1
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:             srcPath,
			Dest:            destPath,
			StripComponents: 1,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify files are stripped correctly
	expected := map[string]string{
		"level1/level2/file1.txt": "content1",
		"level1/file2.txt":        "content2",
		"file3.txt":               "content3",
	}

	for name, expectedContent := range expected {
		extractedPath := filepath.Join(destPath, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, string(content), expectedContent)
		}
	}
}

func TestHandleUnarchive_StripComponents_SkipAll(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar with shallow structure
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}
	createTestTar(t, srcPath, files)

	// Extract with strip_components=1 (should skip all files)
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:             srcPath,
			Dest:            destPath,
			StripComponents: 1,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify no files were extracted
	entries, err := os.ReadDir(destPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to read dest dir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("Expected no files extracted, got %d", len(entries))
	}
}

func TestHandleUnarchive_PathTraversal(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create malicious tar
	srcPath := filepath.Join(ec.CurrentDir, "malicious.tar")
	createMaliciousTar(t, srcPath)

	// Try to extract
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err == nil {
		t.Fatal("Expected error for path traversal, got nil")
	}

	// Error should mention path traversal
	if !strings.Contains(err.Error(), "traversal") && !strings.Contains(err.Error(), "absolute") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestHandleUnarchive_Idempotency(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt": "content1",
	}
	createTestTar(t, srcPath, files)

	// Create marker file
	markerPath := filepath.Join(ec.CurrentDir, "marker.txt")
	createTestFile(t, markerPath, "marker")

	// Extract with creates parameter
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:     srcPath,
			Dest:    destPath,
			Creates: markerPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify extraction was skipped (dest should not exist)
	if _, err := os.Stat(filepath.Join(destPath, "file1.txt")); err == nil {
		t.Error("Expected extraction to be skipped, but file exists")
	}
}

func TestHandleUnarchive_IdempotencyNoSkip(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt": "content1",
	}
	createTestTar(t, srcPath, files)

	// Don't create marker file
	markerPath := filepath.Join(ec.CurrentDir, "marker.txt")

	// Extract with creates parameter (marker doesn't exist)
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:     srcPath,
			Dest:    destPath,
			Creates: markerPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify extraction happened
	if _, err := os.Stat(filepath.Join(destPath, "file1.txt")); err != nil {
		t.Error("Expected extraction to happen, but file doesn't exist")
	}
}

func TestHandleUnarchive_DryRun(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt": "content1",
	}
	createTestTar(t, srcPath, files)

	// Extract
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	// Subscribe to events
	collector := &testEventCollector{}
	ec.EventPublisher.Subscribe(collector)

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify no files were extracted
	if _, err := os.Stat(filepath.Join(destPath, "file1.txt")); err == nil {
		t.Error("Expected no extraction in dry-run mode, but file exists")
	}

	// Verify event was still emitted
	evts := collector.getEvents()
	found := false
	for _, e := range evts {
		if e.Type == events.EventArchiveExtracted {
			found = true
			data := e.Data.(events.ArchiveExtractedData)
			if !data.DryRun {
				t.Error("Expected DryRun=true in event data")
			}
		}
	}
	if !found {
		t.Error("archive.extracted event not emitted in dry-run mode")
	}
}

func TestHandleUnarchive_Mode(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"dir/file1.txt": "content1",
	}
	createTestTar(t, srcPath, files)

	// Extract with custom mode
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
			Mode: "0700",
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify directory mode (at least that extraction succeeded)
	dirPath := filepath.Join(destPath, "dir")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory to exist")
	}
}

func TestHandleUnarchive_VariableRendering(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.Variables["archive_name"] = "test.tar"
	ec.Variables["dest_dir"] = "extracted"

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt": "content1",
	}
	createTestTar(t, srcPath, files)

	// Extract with variables in paths
	step := config.Step{
		Unarchive: &config.Unarchive{
			Src:  filepath.Join(ec.CurrentDir, "{{ archive_name }}"),
			Dest: filepath.Join(ec.CurrentDir, "{{ dest_dir }}"),
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify extraction happened
	destPath := filepath.Join(ec.CurrentDir, "extracted", "file1.txt")
	if _, err := os.Stat(destPath); err != nil {
		t.Error("Expected file to exist after variable rendering")
	}
}

func TestHandleUnarchive_Register(t *testing.T) {
	ec := newTestExecutionContext(t)

	// Create test tar
	srcPath := filepath.Join(ec.CurrentDir, "test.tar")
	files := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}
	createTestTar(t, srcPath, files)

	// Extract with register
	destPath := filepath.Join(ec.CurrentDir, "extracted")
	step := config.Step{
		Register: "extract_result",
		Unarchive: &config.Unarchive{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	err := HandleUnarchive(step, ec)
	if err != nil {
		t.Fatalf("HandleUnarchive failed: %v", err)
	}

	// Verify result was registered
	if _, ok := ec.Variables["extract_result"]; !ok {
		t.Error("Expected result to be registered")
	}
}

func TestStripPathComponents(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		stripComponents int
		wantPath        string
		wantExtract     bool
	}{
		{
			name:            "no strip",
			path:            "root/dir/file.txt",
			stripComponents: 0,
			wantPath:        "root/dir/file.txt",
			wantExtract:     true,
		},
		{
			name:            "strip one",
			path:            "root/dir/file.txt",
			stripComponents: 1,
			wantPath:        "dir/file.txt",
			wantExtract:     true,
		},
		{
			name:            "strip two",
			path:            "root/dir/file.txt",
			stripComponents: 2,
			wantPath:        "file.txt",
			wantExtract:     true,
		},
		{
			name:            "strip all",
			path:            "root/dir/file.txt",
			stripComponents: 3,
			wantPath:        "",
			wantExtract:     false,
		},
		{
			name:            "strip more than exists",
			path:            "file.txt",
			stripComponents: 1,
			wantPath:        "",
			wantExtract:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotExtract := stripPathComponents(tt.path, tt.stripComponents)
			if gotPath != tt.wantPath {
				t.Errorf("stripPathComponents() path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotExtract != tt.wantExtract {
				t.Errorf("stripPathComponents() extract = %v, want %v", gotExtract, tt.wantExtract)
			}
		})
	}
}

func TestDetectArchiveFormat(t *testing.T) {
	tests := []struct {
		path string
		want ArchiveFormat
	}{
		{"test.tar", ArchiveTar},
		{"test.tar.gz", ArchiveTarGz},
		{"test.tgz", ArchiveTarGz},
		{"test.zip", ArchiveZip},
		{"test.TAR", ArchiveTar},
		{"test.TAR.GZ", ArchiveTarGz},
		{"test.TGZ", ArchiveTarGz},
		{"test.ZIP", ArchiveZip},
		{"test.unknown", ArchiveUnknown},
		{"test.txt", ArchiveUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectArchiveFormat(tt.path)
			if got != tt.want {
				t.Errorf("detectArchiveFormat(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

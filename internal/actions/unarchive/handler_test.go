package unarchive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       tmpl,
			Evaluator:      ctx.GetEvaluator(),
			Logger:         ctx.Log,
			EventPublisher: ctx.Publisher,
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Mode:           actions.ModeApply,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: ctx.StepID,
		CurrentDir:    "/tmp",
	}
}

// createTestTarArchive creates a tar archive with test files
func createTestTarArchive(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar file: %v", err)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	// Add test files
	files := []struct {
		name    string
		content string
		mode    int64
	}{
		{"test.txt", "test content", 0644},
		{"subdir/file.txt", "nested content", 0644},
		{"executable.sh", "#!/bin/bash\necho hello", 0755},
	}

	for _, f := range files {
		// Create directory entry if needed
		if strings.Contains(f.name, "/") {
			dir := filepath.Dir(f.name)
			hdr := &tar.Header{
				Name:     dir + "/",
				Typeflag: tar.TypeDir,
				Mode:     0755,
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatalf("Failed to write dir header: %v", err)
			}
		}

		// Write file
		hdr := &tar.Header{
			Name:     f.name,
			Size:     int64(len(f.content)),
			Mode:     f.mode,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte(f.content)); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}
}

// createTestTarGzArchive creates a tar.gz archive with test files
func createTestTarGzArchive(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar.gz file: %v", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add test files
	files := []struct {
		name    string
		content string
	}{
		{"data.txt", "compressed data"},
		{"nested/file.json", `{"key": "value"}`},
	}

	for _, f := range files {
		// Create directory entry if needed
		if strings.Contains(f.name, "/") {
			dir := filepath.Dir(f.name)
			hdr := &tar.Header{
				Name:     dir + "/",
				Typeflag: tar.TypeDir,
				Mode:     0755,
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatalf("Failed to write dir header: %v", err)
			}
		}

		hdr := &tar.Header{
			Name:     f.name,
			Size:     int64(len(f.content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte(f.content)); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}
}

// createTestZipArchive creates a zip archive with test files
func createTestZipArchive(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	// Add test files
	files := []struct {
		name    string
		content string
	}{
		{"readme.txt", "zip archive readme"},
		{"lib/module.py", "import sys"},
	}

	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(f.content)); err != nil {
			t.Fatalf("Failed to write zip content: %v", err)
		}
	}
}

// createPathTraversalTar creates a tar with path traversal attempt
func createPathTraversalTar(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar file: %v", err)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	// Add file with path traversal
	hdr := &tar.Header{
		Name:     "../../../etc/malicious.txt",
		Size:     7,
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}
	if _, err := tw.Write([]byte("malware")); err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "file.unarchive" {
		t.Errorf("Name = %v, want 'unarchive'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryFile {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryFile)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if meta.SupportsBecome {
		t.Error("SupportsBecome should be false")
	}
	if len(meta.EmitsEvents) != 1 {
		t.Errorf("EmitsEvents length = %d, want 1", len(meta.EmitsEvents))
	}
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventArchiveExtracted) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventArchiveExtracted))
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
			name: "valid unarchive action",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Src:  "/tmp/test.tar.gz",
					Dest: "/tmp/extract",
				},
			},
			wantErr: false,
		},
		{
			name: "nil unarchive action",
			step: &config.Step{
				FileUnarchive: nil,
			},
			wantErr: true,
		},
		{
			name: "missing src",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Dest: "/tmp/extract",
				},
			},
			wantErr: true,
		},
		{
			name: "missing dest",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Src: "/tmp/test.tar.gz",
				},
			},
			wantErr: true,
		},
		{
			name: "empty src",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Src:  "",
					Dest: "/tmp/extract",
				},
			},
			wantErr: true,
		},
		{
			name: "empty dest",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Src:  "/tmp/test.tar.gz",
					Dest: "",
				},
			},
			wantErr: true,
		},
		{
			name: "with strip components",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Src:             "/tmp/test.tar.gz",
					Dest:            "/tmp/extract",
					StripComponents: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "with creates marker",
			step: &config.Step{
				FileUnarchive: &config.Unarchive{
					Src:     "/tmp/test.tar.gz",
					Dest:    "/tmp/extract",
					Creates: "/tmp/extract/marker",
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

func TestHandler_Run_TarArchive(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create test tar archive
	createTestTarArchive(t, tarPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Run() result is not *executor.Result")
	}

	if !execResult.Changed {
		t.Error("Result.Changed should be true for new extraction")
	}

	if execResult.Failed {
		t.Error("Result.Failed should be false")
	}

	// Verify files were extracted
	testFile := filepath.Join(extractDir, "test.txt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Extracted file does not exist")
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Content = %q, want %q", string(content), "test content")
	}

	// Verify nested file
	nestedFile := filepath.Join(extractDir, "subdir", "file.txt")
	if _, err := os.Stat(nestedFile); os.IsNotExist(err) {
		t.Error("Nested file does not exist")
	}

	// Verify executable permissions
	execFile := filepath.Join(extractDir, "executable.sh")
	info, err := os.Stat(execFile)
	if err != nil {
		t.Fatalf("Failed to stat executable: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("Executable file should have execute permission")
	}

	// Check event was published
	pub := ec.Svc.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	event := pub.Events[0]
	if event.Type != events.EventArchiveExtracted {
		t.Errorf("Event.Type = %v, want %v", event.Type, events.EventArchiveExtracted)
	}

	data, ok := event.Data.(events.ArchiveExtractedData)
	if !ok {
		t.Fatalf("Event.Data is not events.ArchiveExtractedData")
	}

	if data.Format != "tar" {
		t.Errorf("Format = %v, want 'tar'", data.Format)
	}
	if data.FilesExtracted != 3 {
		t.Errorf("FilesExtracted = %d, want 3", data.FilesExtracted)
	}
}

func TestHandler_Run_TarGzArchive(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarGzPath := filepath.Join(tmpDir, "test.tar.gz")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create test tar.gz archive
	createTestTarGzArchive(t, tarGzPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarGzPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify files were extracted
	dataFile := filepath.Join(extractDir, "data.txt")
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		t.Error("Extracted file does not exist")
	}

	content, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}
	if string(content) != "compressed data" {
		t.Errorf("Content = %q, want %q", string(content), "compressed data")
	}

	// Verify nested file
	jsonFile := filepath.Join(extractDir, "nested", "file.json")
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		t.Error("Nested file does not exist")
	}

	// Check event
	pub := ec.Svc.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	data, ok := pub.Events[0].Data.(events.ArchiveExtractedData)
	if !ok {
		t.Fatalf("Event.Data is not events.ArchiveExtractedData")
	}

	if data.Format != "tar.gz" {
		t.Errorf("Format = %v, want 'tar.gz'", data.Format)
	}
}

func TestHandler_Run_ZipArchive(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create test zip archive
	createTestZipArchive(t, zipPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  zipPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify files were extracted
	readmeFile := filepath.Join(extractDir, "readme.txt")
	if _, err := os.Stat(readmeFile); os.IsNotExist(err) {
		t.Error("Extracted file does not exist")
	}

	content, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}
	if string(content) != "zip archive readme" {
		t.Errorf("Content = %q, want %q", string(content), "zip archive readme")
	}

	// Verify nested file
	moduleFile := filepath.Join(extractDir, "lib", "module.py")
	if _, err := os.Stat(moduleFile); os.IsNotExist(err) {
		t.Error("Nested file does not exist")
	}

	// Check event
	pub := ec.Svc.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	data, ok := pub.Events[0].Data.(events.ArchiveExtractedData)
	if !ok {
		t.Fatalf("Event.Data is not events.ArchiveExtractedData")
	}

	if data.Format != "zip" {
		t.Errorf("Format = %v, want 'zip'", data.Format)
	}
}

func TestHandler_Run_StripComponents(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create tar with nested structure
	file, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("Failed to create tar: %v", err)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	// Add file with 2 leading components: top/middle/file.txt
	hdr := &tar.Header{
		Name:     "top/middle/file.txt",
		Size:     12,
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("stripped file"))

	tw.Close()
	file.Close()

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:             tarPath,
			Dest:            extractDir,
			StripComponents: 2, // Strip "top/middle"
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// File should be at extract/file.txt, not extract/top/middle/file.txt
	strippedFile := filepath.Join(extractDir, "file.txt")
	if _, err := os.Stat(strippedFile); os.IsNotExist(err) {
		t.Error("Stripped file does not exist at expected location")
	}

	// Original location should not exist
	originalFile := filepath.Join(extractDir, "top", "middle", "file.txt")
	if _, err := os.Stat(originalFile); !os.IsNotExist(err) {
		t.Error("File should not exist at original location after strip")
	}

	// Check event
	pub := ec.Svc.EventPublisher.(*testutil.MockPublisher)
	data := pub.Events[0].Data.(events.ArchiveExtractedData)
	if data.StripComponents != 2 {
		t.Errorf("StripComponents = %d, want 2", data.StripComponents)
	}
}

func TestHandler_Run_CreatesIdempotency(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")
	markerFile := filepath.Join(extractDir, "marker.txt")

	// Create test tar archive
	createTestTarArchive(t, tarPath)

	// Create marker file
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract dir: %v", err)
	}
	if err := os.WriteFile(markerFile, []byte("marker"), 0644); err != nil {
		t.Fatalf("Failed to create marker: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:     tarPath,
			Dest:    extractDir,
			Creates: markerFile,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when creates marker exists")
	}

	// Verify archive was not extracted (test file should not exist)
	testFile := filepath.Join(extractDir, "test.txt")
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Archive should not have been extracted when marker exists")
	}
}

// MT-46: file.unarchive should be idempotent without requiring an
// explicit creates: marker. Running the same archive against the
// same dest twice should report Changed=false on the second run.
func TestHandler_Run_ContentIdempotency_Tar(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")
	createTestTarArchive(t, tarPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	// First run: extract.
	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("First Run() error = %v", err)
	}
	if !result.(*executor.Result).Changed {
		t.Fatal("First run should report Changed=true (extracting)")
	}

	// Second run with fresh ec: should skip via content match.
	ec2 := mockExecutionContext()
	result2, err := h.Run(ec2, step)
	if err != nil {
		t.Fatalf("Second Run() error = %v", err)
	}
	if result2.(*executor.Result).Changed {
		t.Error("Second run should report Changed=false (archive contents already present)")
	}
}

// MT-46: same shape but tar.gz to confirm the gzip path also gates.
func TestHandler_Run_ContentIdempotency_TarGz(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarGzPath := filepath.Join(tmpDir, "test.tar.gz")
	extractDir := filepath.Join(tmpDir, "extract")
	createTestTarGzArchive(t, tarGzPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarGzPath,
			Dest: extractDir,
		},
	}

	if _, err := h.Run(ec, step); err != nil {
		t.Fatalf("First Run() error = %v", err)
	}

	ec2 := mockExecutionContext()
	result2, err := h.Run(ec2, step)
	if err != nil {
		t.Fatalf("Second Run() error = %v", err)
	}
	if result2.(*executor.Result).Changed {
		t.Error("Second tar.gz run should report Changed=false")
	}
}

// MT-46: if any extracted file diverges (truncated, modified size),
// the pre-check must fall through to extract — otherwise drift goes
// unnoticed.
func TestHandler_Run_ContentIdempotency_DivergedFile(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")
	createTestTarArchive(t, tarPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	// Extract once.
	if _, err := h.Run(ec, step); err != nil {
		t.Fatalf("First Run() error = %v", err)
	}

	// Mutate one extracted file so its size diverges.
	target := filepath.Join(extractDir, "test.txt")
	if err := os.WriteFile(target, []byte("DIFFERENT"), 0644); err != nil {
		t.Fatalf("Failed to mutate extracted file: %v", err)
	}

	// Second run must re-extract (restore content).
	ec2 := mockExecutionContext()
	result2, err := h.Run(ec2, step)
	if err != nil {
		t.Fatalf("Second Run() error = %v", err)
	}
	if !result2.(*executor.Result).Changed {
		t.Error("Second run should re-extract when a target file diverges (Changed=true)")
	}
}

func TestHandler_Run_PathTraversalProtection(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "malicious.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create tar with path traversal attempt
	createPathTraversalTar(t, tarPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Run() should error on path traversal attempt")
	}

	if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("Error should mention path traversal, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on path traversal")
	}

	// Verify malicious file was not created
	maliciousPath := filepath.Join(tmpDir, "..", "..", "..", "etc", "malicious.txt")
	if _, err := os.Stat(maliciousPath); !os.IsNotExist(err) {
		t.Error("Malicious file should not exist")
	}
}

func TestHandler_Run_SourceNotFound(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	extractDir := filepath.Join(tmpDir, "extract")

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  "/nonexistent/archive.tar.gz",
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Run() should error when source does not exist")
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true")
	}
}

func TestHandler_Run_SourceIsDirectory(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	extractDir := filepath.Join(tmpDir, "extract")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  srcDir,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Run() should error when source is a directory")
	}

	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Error should mention directory, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true")
	}
}

func TestHandler_Run_UnsupportedFormat(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	unsupportedFile := filepath.Join(tmpDir, "file.rar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create a file with unsupported extension
	if err := os.WriteFile(unsupportedFile, []byte("not an archive"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  unsupportedFile,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Run() should error on unsupported format")
	}

	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("Error should mention unsupported format, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true")
	}
}

func TestHandler_Run_WithCustomMode(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create test tar archive
	createTestTarArchive(t, tarPath)

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
			Mode: "0700",
		},
	}

	_, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify directory was created with correct mode
	info, err := os.Stat(extractDir)
	if err != nil {
		t.Fatalf("Failed to stat extract dir: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0700)
	if mode != expectedMode {
		t.Errorf("Directory mode = %o, want %o", mode, expectedMode)
	}
}

func TestHandler_Run_TemplateRendering(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create test tar archive
	createTestTarArchive(t, tarPath)

	ec := mockExecutionContext()
	ec.Scope.User["archive_name"] = "test.tar"
	ec.Scope.User["extract_base"] = tmpDir

	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  "{{ extract_base }}/{{ archive_name }}",
			Dest: "{{ extract_base }}/extract",
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify files were extracted
	testFile := filepath.Join(extractDir, "test.txt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Extracted file does not exist at templated path")
	}
}

func TestHandler_Run_NoPublisher(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create test tar archive
	createTestTarArchive(t, tarPath)

	ec := mockExecutionContext()
	ec.Svc.EventPublisher = nil

	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Errorf("Run() should not error when publisher is nil, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify files were extracted
	testFile := filepath.Join(extractDir, "test.txt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Extracted file does not exist")
	}
}

func TestHandler_detectArchiveFormat(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		path string
		want ArchiveFormat
	}{
		{
			name: "tar.gz file",
			path: "/path/to/file.tar.gz",
			want: ArchiveTarGz,
		},
		{
			name: "tgz file",
			path: "/path/to/file.tgz",
			want: ArchiveTarGz,
		},
		{
			name: "tar file",
			path: "/path/to/file.tar",
			want: ArchiveTar,
		},
		{
			name: "zip file",
			path: "/path/to/file.zip",
			want: ArchiveZip,
		},
		{
			name: "uppercase extension",
			path: "/path/to/FILE.TAR.GZ",
			want: ArchiveTarGz,
		},
		{
			name: "unsupported format",
			path: "/path/to/file.rar",
			want: ArchiveUnknown,
		},
		{
			name: "no extension",
			path: "/path/to/file",
			want: ArchiveUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.detectArchiveFormat(tt.path)
			if got != tt.want {
				t.Errorf("detectArchiveFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_stripPathComponents(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name      string
		path      string
		count     int
		wantPath  string
		wantStrip bool
	}{
		{
			name:      "no stripping",
			path:      "file.txt",
			count:     0,
			wantPath:  "file.txt",
			wantStrip: true,
		},
		{
			name:      "strip one component",
			path:      "dir/file.txt",
			count:     1,
			wantPath:  "file.txt",
			wantStrip: true,
		},
		{
			name:      "strip two components",
			path:      "top/middle/file.txt",
			count:     2,
			wantPath:  "file.txt",
			wantStrip: true,
		},
		{
			name:      "strip all components",
			path:      "top/middle/file.txt",
			count:     3,
			wantPath:  "",
			wantStrip: false,
		},
		{
			name:      "strip more than available",
			path:      "file.txt",
			count:     5,
			wantPath:  "",
			wantStrip: false,
		},
		{
			name:      "directory only gets stripped",
			path:      "top/",
			count:     1,
			wantPath:  "",
			wantStrip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotStrip := h.stripPathComponents(tt.path, tt.count)
			if gotPath != tt.wantPath {
				t.Errorf("stripPathComponents() path = %v, want %v", gotPath, tt.wantPath)
			}
			if gotStrip != tt.wantStrip {
				t.Errorf("stripPathComponents() strip = %v, want %v", gotStrip, tt.wantStrip)
			}
		})
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
			defaultMode: 0755,
			want:        0755,
		},
		{
			name:        "valid octal mode",
			modeStr:     "0700",
			defaultMode: 0755,
			want:        0700,
		},
		{
			name:        "valid mode without leading zero",
			modeStr:     "755",
			defaultMode: 0644,
			want:        0755,
		},
		{
			name:        "invalid mode uses default",
			modeStr:     "invalid",
			defaultMode: 0755,
			want:        0755,
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

func TestArchiveFormat_String(t *testing.T) {
	tests := []struct {
		format ArchiveFormat
		want   string
	}{
		{ArchiveTar, "tar"},
		{ArchiveTarGz, "tar.gz"},
		{ArchiveZip, "zip"},
		{ArchiveUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.format.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_Run_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	ctx := testutil.NewMockContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  "/tmp/test.tar",
			Dest: "/tmp/extract",
		},
	}

	_, err := h.Run(ctx, step)
	if err == nil {
		t.Error("Run() should error when context is not ExecutionContext")
	}

	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

func TestHandler_Run_CorruptedArchive(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "corrupted.tar.gz")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create a corrupted archive (not valid gzip)
	if err := os.WriteFile(tarPath, []byte("this is not a valid tar.gz file"), 0644); err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Run() should error on corrupted archive")
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on corrupted archive")
	}
}

func TestHandler_Run_TarWithSymlink(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "symlink.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create tar with symlink
	file, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("Failed to create tar: %v", err)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	// Add regular file
	hdr := &tar.Header{
		Name:     "target.txt",
		Size:     6,
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("target"))

	// Add symlink
	linkHdr := &tar.Header{
		Name:     "link.txt",
		Linkname: "target.txt",
		Mode:     0644,
		Typeflag: tar.TypeSymlink,
	}
	tw.WriteHeader(linkHdr)

	tw.Close()
	file.Close()

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify symlink was created
	linkPath := filepath.Join(extractDir, "link.txt")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Failed to stat symlink: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected a symlink")
	}
}

func TestHandler_Run_EmptyArchive(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "empty.tar")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create empty tar
	file, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("Failed to create tar: %v", err)
	}
	tw := tar.NewWriter(file)
	tw.Close()
	file.Close()

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  tarPath,
			Dest: extractDir,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false for empty archive")
	}

	// Check event
	pub := ec.Svc.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	data := pub.Events[0].Data.(events.ArchiveExtractedData)
	if data.FilesExtracted != 0 {
		t.Errorf("FilesExtracted = %d, want 0", data.FilesExtracted)
	}
}

func TestHandler_Run_RenderError(t *testing.T) {
	h := &Handler{}

	ec := mockExecutionContext()
	step := &config.Step{
		FileUnarchive: &config.Unarchive{
			Src:  "{{ invalid template syntax",
			Dest: "/tmp/extract",
		},
	}

	_, err := h.Run(ec, step)
	if err == nil {
		t.Error("Run() should error on invalid template")
	}

	if !strings.Contains(fmt.Sprintf("%v", err), "expand") {
		t.Errorf("Error should mention path expansion failure, got: %v", err)
	}
}

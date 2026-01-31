package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/template"
)

func TestPathExpander_ExpandPath(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	expander := NewPathExpander(renderer)

	home := os.Getenv("HOME")

	tests := []struct {
		name       string
		path       string
		currentDir string
		context    map[string]interface{}
		want       string
		wantErr    bool
	}{
		{
			name:       "absolute path",
			path:       "/tmp/test",
			currentDir: "/work",
			context:    nil,
			want:       "/tmp/test",
			wantErr:    false,
		},
		{
			name:       "home expansion",
			path:       "~/config",
			currentDir: "/work",
			context:    nil,
			want:       home + "/config",
			wantErr:    false,
		},
		{
			name:       "current directory",
			path:       "./test",
			currentDir: "/work",
			context:    nil,
			want:       "/work/test",
			wantErr:    false,
		},
		{
			name:       "parent directory",
			path:       "../test",
			currentDir: "/work/project",
			context:    nil,
			want:       "/work/test",
			wantErr:    false,
		},
		{
			name:       "with template variable",
			path:       "/tmp/{{ filename }}",
			currentDir: "/work",
			context:    map[string]interface{}{"filename": "test.txt"},
			want:       "/tmp/test.txt",
			wantErr:    false,
		},
		{
			name:       "with multiple variables",
			path:       "/tmp/{{ dir }}/{{ file }}",
			currentDir: "/work",
			context:    map[string]interface{}{"dir": "logs", "file": "app.log"},
			want:       "/tmp/logs/app.log",
			wantErr:    false,
		},
		{
			name:       "path with spaces",
			path:       "  /tmp/test  ",
			currentDir: "/work",
			context:    nil,
			want:       "/tmp/test",
			wantErr:    false,
		},
		{
			name:       "empty path",
			path:       "",
			currentDir: "/work",
			context:    nil,
			want:       "",
			wantErr:    false,
		},
		{
			name:       "relative with dot prefix",
			path:       ".config",
			currentDir: "/work",
			context:    nil,
			want:       "/work/config",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expander.ExpandPath(tt.path, tt.currentDir, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExpandPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePathWithinBase(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		baseDir    string
		wantErr    bool
	}{
		{
			name:       "valid path within base",
			targetPath: "/work/project/file.txt",
			baseDir:    "/work",
			wantErr:    false,
		},
		{
			name:       "path equals base",
			targetPath: "/work",
			baseDir:    "/work",
			wantErr:    false,
		},
		{
			name:       "path traversal attempt",
			targetPath: "/work/../etc/passwd",
			baseDir:    "/work",
			wantErr:    true,
		},
		{
			name:       "double dot traversal",
			targetPath: "/work/project/../../etc/passwd",
			baseDir:    "/work",
			wantErr:    true,
		},
		{
			name:       "empty base dir skips validation",
			targetPath: "/etc/passwd",
			baseDir:    "",
			wantErr:    false,
		},
		{
			name:       "nested valid path",
			targetPath: "/work/a/b/c/file.txt",
			baseDir:    "/work",
			wantErr:    false,
		},
		{
			name:       "sibling directory escape",
			targetPath: "/work2/file.txt",
			baseDir:    "/work",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathWithinBase(tt.targetPath, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathWithinBase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathExpander_SafeExpandPath(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	expander := NewPathExpander(renderer)

	tests := []struct {
		name       string
		path       string
		currentDir string
		context    map[string]interface{}
		baseDir    string
		wantErr    bool
		checkPath  bool
	}{
		{
			name:       "safe path within base",
			path:       "project/file.txt",
			currentDir: "/work",
			context:    nil,
			baseDir:    "/work",
			wantErr:    false,
			checkPath:  true,
		},
		{
			name:       "unsafe path escapes base",
			path:       "../etc/passwd",
			currentDir: "/work",
			context:    nil,
			baseDir:    "/work",
			wantErr:    true,
			checkPath:  false,
		},
		{
			name:       "no validation with empty base",
			path:       "../etc/passwd",
			currentDir: "/work",
			context:    nil,
			baseDir:    "",
			wantErr:    false,
			checkPath:  true,
		},
		{
			name:       "template variable safe",
			path:       "{{ dir }}/file.txt",
			currentDir: "/work",
			context:    map[string]interface{}{"dir": "logs"},
			baseDir:    "/work",
			wantErr:    false,
			checkPath:  true,
		},
		{
			name:       "template variable with traversal",
			path:       "{{ dir }}/file.txt",
			currentDir: "/work",
			context:    map[string]interface{}{"dir": "../etc"},
			baseDir:    "/work",
			wantErr:    true,
			checkPath:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expander.SafeExpandPath(tt.path, tt.currentDir, tt.context, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checkPath && got == "" {
				t.Error("SafeExpandPath() returned empty path unexpectedly")
			}
		})
	}
}

func TestGetDirectoryOfFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "file in directory",
			path: "/work/project/file.txt",
			want: "/work/project",
		},
		{
			name: "file in root",
			path: "/file.txt",
			want: "/",
		},
		{
			name: "relative path",
			path: "project/file.txt",
			want: "project",
		},
		{
			name: "directory path",
			path: "/work/project",
			want: "/work",
		},
		{
			name: "single file name",
			path: "file.txt",
			want: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDirectoryOfFile(tt.path)
			// Normalize paths for comparison
			wantClean := filepath.Clean(tt.want)
			gotClean := filepath.Clean(got)
			if gotClean != wantClean {
				t.Errorf("GetDirectoryOfFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewPathExpander(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	expander := NewPathExpander(renderer)

	if expander == nil {
		t.Error("NewPathExpander() returned nil")
	}

	// Test that it works
	path, err := expander.ExpandPath("/tmp/test", "/work", nil)
	if err != nil {
		t.Errorf("NewPathExpander() created non-functional expander: %v", err)
	}
	if path != "/tmp/test" {
		t.Errorf("NewPathExpander() expander returned wrong path: %v", path)
	}
}

func TestPathExpander_ExpandPathWithInvalidTemplate(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	expander := NewPathExpander(renderer)

	// Test with invalid template syntax
	_, err := expander.ExpandPath("/tmp/{{ unclosed", "/work", nil)
	if err == nil {
		t.Error("ExpandPath() should return error for invalid template")
	}
}

func TestPathExpander_SafeExpandPathWithInvalidTemplate(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	expander := NewPathExpander(renderer)

	// Test with invalid template syntax in SafeExpandPath
	_, err := expander.SafeExpandPath("/tmp/{{ unclosed", "/work", nil, "/work")
	if err == nil {
		t.Error("SafeExpandPath() should return error for invalid template")
	}
}

func TestValidatePathWithinBase_RelativePaths(t *testing.T) {
	// Get current directory for relative path tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Skip("Cannot get current directory")
	}

	tests := []struct {
		name       string
		targetPath string
		baseDir    string
		wantErr    bool
	}{
		{
			name:       "relative target within relative base",
			targetPath: "project/file.txt",
			baseDir:    ".",
			wantErr:    false,
		},
		{
			name:       "parent directory from relative",
			targetPath: "../outside",
			baseDir:    ".",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to absolute for consistent testing
			absTarget := filepath.Join(cwd, tt.targetPath)
			absBase := filepath.Join(cwd, tt.baseDir)

			err := ValidatePathWithinBase(absTarget, absBase)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathWithinBase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathExpander_ExpandPathEdgeCases(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	expander := NewPathExpander(renderer)

	// Test with nil context
	path, err := expander.ExpandPath("/tmp/test", "/work", nil)
	if err != nil {
		t.Errorf("ExpandPath() with nil context error = %v", err)
	}
	if path != "/tmp/test" {
		t.Errorf("ExpandPath() with nil context = %v, want /tmp/test", path)
	}

	// Test with empty context
	path, err = expander.ExpandPath("/tmp/test", "/work", map[string]interface{}{})
	if err != nil {
		t.Errorf("ExpandPath() with empty context error = %v", err)
	}
	if path != "/tmp/test" {
		t.Errorf("ExpandPath() with empty context = %v, want /tmp/test", path)
	}

	// Test home expansion when HOME is set
	if home := os.Getenv("HOME"); home != "" {
		path, err = expander.ExpandPath("~/test", "/work", nil)
		if err != nil {
			t.Errorf("ExpandPath() home expansion error = %v", err)
		}
		if !strings.HasPrefix(path, home) {
			t.Errorf("ExpandPath() home expansion = %v, should start with %v", path, home)
		}
	}
}

func TestValidatePathWithinBase_ExtremelyLongPaths(t *testing.T) {
	// Attempt to trigger filepath.Abs or filepath.Rel errors with extremely long paths
	// Note: These may not trigger errors on all systems

	// Create an extremely long path (> 4096 chars)
	longDir := "/tmp/" + strings.Repeat("a", 5000)
	longTarget := longDir + "/file.txt"

	// This should either succeed or return an error
	// We're just checking that it doesn't panic
	_ = ValidatePathWithinBase(longTarget, "/tmp")

	// Test with very long base directory
	longBase := "/tmp/" + strings.Repeat("b", 5000)
	_ = ValidatePathWithinBase("/tmp/test.txt", longBase)
}

func TestValidatePathWithinBase_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		baseDir    string
		shouldTest bool // Some tests might not be valid on all OS
	}{
		{
			name:       "path with unicode",
			targetPath: "/tmp/测试/file.txt",
			baseDir:    "/tmp",
			shouldTest: true,
		},
		{
			name:       "path with spaces",
			targetPath: "/tmp/my folder/file.txt",
			baseDir:    "/tmp",
			shouldTest: true,
		},
		{
			name:       "path with special chars",
			targetPath: "/tmp/test!@#$%/file.txt",
			baseDir:    "/tmp",
			shouldTest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.shouldTest {
				t.Skip("Test not applicable on this OS")
			}

			// Just verify it doesn't panic - error or success are both acceptable
			_ = ValidatePathWithinBase(tt.targetPath, tt.baseDir)
		})
	}
}

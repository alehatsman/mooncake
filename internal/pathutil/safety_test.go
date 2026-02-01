package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateRemovalPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "refusing to remove empty path",
		},
		{
			name:    "root directory",
			path:    "/",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "bin directory",
			path:    "/bin",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "sbin directory",
			path:    "/sbin",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "usr directory",
			path:    "/usr",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "usr/bin directory",
			path:    "/usr/bin",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "usr/sbin directory",
			path:    "/usr/sbin",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "etc directory",
			path:    "/etc",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "boot directory",
			path:    "/boot",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "sys directory",
			path:    "/sys",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "proc directory",
			path:    "/proc",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "dev directory",
			path:    "/dev",
			wantErr: true,
			errMsg:  "refusing to remove system path",
		},
		{
			name:    "safe path in tmp",
			path:    "/tmp/myfile.txt",
			wantErr: false,
		},
		{
			name:    "safe path in home",
			path:    "/home/user/test.txt",
			wantErr: false,
		},
		{
			name:    "safe relative path",
			path:    "test/file.txt",
			wantErr: false,
		},
		{
			name:    "path with .. that resolves to safe location",
			path:    "/tmp/test/../safe.txt",
			wantErr: false,
		},
		{
			name:    "nested system path should be safe",
			path:    "/usr/local/myapp",
			wantErr: false,
		},
	}

	// Add Windows-specific tests
	if runtime.GOOS == "windows" {
		windowsTests := []struct {
			name    string
			path    string
			wantErr bool
			errMsg  string
		}{
			{
				name:    "C drive root",
				path:    "C:\\",
				wantErr: true,
				errMsg:  "refusing to remove system path",
			},
			{
				name:    "Windows directory",
				path:    "C:\\Windows",
				wantErr: true,
				errMsg:  "refusing to remove system path",
			},
			{
				name:    "System32 directory",
				path:    "C:\\Windows\\System32",
				wantErr: true,
				errMsg:  "refusing to remove system path",
			},
			{
				name:    "Program Files",
				path:    "C:\\Program Files",
				wantErr: true,
				errMsg:  "refusing to remove system path",
			},
			{
				name:    "safe Windows path",
				path:    "C:\\Users\\test\\file.txt",
				wantErr: false,
			},
		}
		tests = append(tests, windowsTests...)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemovalPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRemovalPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateRemovalPath() error = %v, should contain %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateNoPathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple relative path",
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "nested relative path",
			path:    "dir/subdir/file.txt",
			wantErr: false,
		},
		{
			name:    "path with single dot",
			path:    "./file.txt",
			wantErr: false,
		},
		{
			name:    "path traversal with ..",
			path:    "../file.txt",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "path traversal in middle (cleans to safe)",
			path:    "dir/../file.txt",
			wantErr: false, // After cleaning, this becomes "file.txt" which is safe
		},
		{
			name:    "path traversal nested",
			path:    "dir/../../file.txt",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "multiple traversals",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "absolute path",
			path:    "/tmp/file.txt",
			wantErr: true,
			errMsg:  "absolute path not allowed",
		},
		{
			name:    "windows absolute path on unix",
			path:    "C:\\file.txt",
			wantErr: runtime.GOOS == "windows", // Only absolute on Windows
			errMsg:  "absolute path not allowed",
		},
		{
			name:    "path with encoded traversal gets cleaned",
			path:    "dir%2F..%2Ffile.txt",
			wantErr: false, // This doesn't decode, so it's treated as a literal filename
		},
		{
			name:    "current directory only",
			path:    ".",
			wantErr: false,
		},
		{
			name:    "parent directory only",
			path:    "..",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "hidden file",
			path:    ".hidden",
			wantErr: false,
		},
		{
			name:    "path that cleans to current dir",
			path:    "test/..",
			wantErr: false, // This cleans to "." which is safe
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoPathTraversal(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNoPathTraversal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateNoPathTraversal() error = %v, should contain %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestSafeJoin(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		elems   []string
		wantErr bool
		errMsg  string
		check   func(t *testing.T, result string)
	}{
		{
			name:    "simple join",
			base:    "/tmp",
			elems:   []string{"file.txt"},
			wantErr: false,
			check: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, filepath.Join("tmp", "file.txt")) {
					t.Errorf("SafeJoin() = %v, should end with tmp/file.txt", result)
				}
			},
		},
		{
			name:    "nested join",
			base:    "/tmp",
			elems:   []string{"dir", "subdir", "file.txt"},
			wantErr: false,
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "dir") || !strings.Contains(result, "subdir") {
					t.Errorf("SafeJoin() = %v, should contain dir and subdir", result)
				}
			},
		},
		{
			name:    "traversal attempt",
			base:    "/tmp",
			elems:   []string{"..", "etc", "passwd"},
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "multiple traversal attempts",
			base:    "/tmp/work",
			elems:   []string{"..", "..", "etc", "passwd"},
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "traversal in middle",
			base:    "/tmp",
			elems:   []string{"dir", "..", "..", "etc"},
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "relative navigation escapes base",
			base:    "/tmp/a/b",
			elems:   []string{"..", "c", "file.txt"},
			wantErr: true, // This resolves to /tmp/a/c/file.txt which is outside /tmp/a/b
			errMsg:  "path traversal detected",
		},
		{
			name:    "empty elements",
			base:    "/tmp",
			elems:   []string{"", "file.txt"},
			wantErr: false,
			check: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, filepath.Join("tmp", "file.txt")) {
					t.Errorf("SafeJoin() = %v, should end with tmp/file.txt", result)
				}
			},
		},
		{
			name:    "current directory references",
			base:    "/tmp",
			elems:   []string{".", "dir", ".", "file.txt"},
			wantErr: false,
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "dir") {
					t.Errorf("SafeJoin() = %v, should contain dir", result)
				}
			},
		},
		{
			name:    "no elements",
			base:    "/tmp",
			elems:   []string{},
			wantErr: false,
			check: func(t *testing.T, result string) {
				// Should just return cleaned base
				if !strings.Contains(result, "tmp") {
					t.Errorf("SafeJoin() = %v, should contain tmp", result)
				}
			},
		},
		{
			name:    "single element",
			base:    "/tmp",
			elems:   []string{"file.txt"},
			wantErr: false,
			check: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, filepath.Join("tmp", "file.txt")) {
					t.Errorf("SafeJoin() = %v, should end with tmp/file.txt", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeJoin(tt.base, tt.elems...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeJoin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SafeJoin() error = %v, should contain %v", err, tt.errMsg)
				}
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestSafeJoin_RelativeBase(t *testing.T) {
	// Test with relative base path
	got, err := SafeJoin(".", "test", "file.txt")
	if err != nil {
		t.Errorf("SafeJoin() with relative base error = %v", err)
		return
	}
	if got == "" {
		t.Error("SafeJoin() with relative base returned empty string")
	}

	// Test with relative base and traversal
	_, err = SafeJoin(".", "..", "etc", "passwd")
	if err == nil {
		t.Error("SafeJoin() should detect traversal with relative base")
	}
}

func TestValidateRemovalPath_WithDots(t *testing.T) {
	// Test paths that resolve to dangerous locations after cleaning
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "path that cleans to root",
			path:    "/tmp/../",
			wantErr: true,
		},
		{
			name:    "path that cleans to /etc",
			path:    "/tmp/../etc",
			wantErr: true,
		},
		{
			name:    "path that cleans to /usr",
			path:    "/tmp/../usr",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemovalPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRemovalPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafeJoin_EdgeCases(t *testing.T) {
	t.Run("base with trailing slash", func(t *testing.T) {
		got, err := SafeJoin("/tmp/", "file.txt")
		if err != nil {
			t.Errorf("SafeJoin() with trailing slash error = %v", err)
		}
		if !strings.Contains(got, "file.txt") {
			t.Errorf("SafeJoin() = %v, should contain file.txt", got)
		}
	})

	t.Run("elements with leading slash", func(t *testing.T) {
		got, err := SafeJoin("/tmp", "/file.txt")
		if err != nil {
			t.Errorf("SafeJoin() with leading slash error = %v", err)
		}
		// filepath.Join handles leading slashes in elements
		if got == "" {
			t.Error("SafeJoin() returned empty string")
		}
	})

	t.Run("complex nested path within base", func(t *testing.T) {
		got, err := SafeJoin("/tmp/project", "src", "..", "tests", "file.txt")
		if err != nil {
			t.Errorf("SafeJoin() complex nested path error = %v", err)
		}
		if !strings.Contains(got, "tmp") {
			t.Errorf("SafeJoin() = %v, should contain tmp", got)
		}
	})
}

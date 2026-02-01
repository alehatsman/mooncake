package executor

import (
	"os"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
)

func TestNewDryRunLogger(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	if dryRun == nil {
		t.Fatal("newDryRunLogger returned nil")
	}
}

func TestLogShellExecution_WithoutSudo(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogShellExecution("echo hello", false)
}

func TestLogShellExecution_WithSudo(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogShellExecution("systemctl restart nginx", true)
}

func TestLogTemplateRender(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogTemplateRender("/src/template.j2", "/dest/config.conf", 0644)
}

func TestLogVariableLoad(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogVariableLoad(5, "/path/to/vars.yml")
}

func TestLogVariableSet(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogVariableSet(3)
}

func TestLogRegister_WithRegister(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	step := config.Step{
		Register: "my_result",
	}

	// Should not panic
	dryRun.LogRegister(step)
}

func TestLogRegister_WithoutRegister(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	step := config.Step{}

	// Should not panic (and should not log anything)
	dryRun.LogRegister(step)
}

func TestLogFileCreate(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogFileCreate("/path/to/file.txt", 0644, 1024)
}

func TestLogFileUpdate(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogFileUpdate("/path/to/file.txt", 0644, 512, 1024)
}

func TestLogDirectoryCreate(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogDirectoryCreate("/path/to/dir", 0755)
}

func TestLogTemplateCreate(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogTemplateCreate("/src/template.j2", "/dest/file.conf", 0644, 2048)
}

func TestLogTemplateUpdate(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogTemplateUpdate("/src/template.j2", "/dest/file.conf", 0644, 1024, 2048)
}

func TestLogTemplateNoChange(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogTemplateNoChange("/src/template.j2", "/dest/file.conf")
}

func TestLogFileRemove(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogFileRemove("/path/to/file.txt", 4096)
}

func TestLogDirectoryRemove(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogDirectoryRemove("/path/to/dir")
}

func TestLogFileTouch(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogFileTouch("/path/to/file.txt")
}

func TestLogSymlinkCreate_WithoutForce(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogSymlinkCreate("/target", "/link", false)
}

func TestLogSymlinkCreate_WithForce(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogSymlinkCreate("/target", "/link", true)
}

func TestLogSymlinkNoChange(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogSymlinkNoChange("/target", "/link")
}

func TestLogHardlinkCreate_WithoutForce(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogHardlinkCreate("/target", "/link", false)
}

func TestLogHardlinkCreate_WithForce(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogHardlinkCreate("/target", "/link", true)
}

func TestLogHardlinkNoChange(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogHardlinkNoChange("/target", "/link")
}

func TestLogPermissionsChange_ModeOnly(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogPermissionsChange("/path/to/file", "0644", "", "", false)
}

func TestLogPermissionsChange_OwnerOnly(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogPermissionsChange("/path/to/file", "", "root", "", false)
}

func TestLogPermissionsChange_GroupOnly(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogPermissionsChange("/path/to/file", "", "", "wheel", false)
}

func TestLogPermissionsChange_AllParameters(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogPermissionsChange("/path/to/file", "0755", "root", "wheel", true)
}

func TestLogPermissionsChange_Recursive(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogPermissionsChange("/path/to/dir", "", "", "", true)
}

func TestLogPermissionsChange_NoParameters(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic even with all empty parameters
	dryRun.LogPermissionsChange("/path/to/file", "", "", "", false)
}

func TestLogPermissionsNoChange(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogPermissionsNoChange("/path/to/file")
}

func TestLogFileCopy(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogFileCopy("/src/file.txt", "/dest/file.txt", 0644, 8192)
}

func TestLogFileCopyNoChange(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Should not panic
	dryRun.LogFileCopyNoChange("/src/file.txt", "/dest/file.txt")
}

func TestDryRunLogger_MultipleOperations(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Perform multiple operations - should not panic
	dryRun.LogFileCreate("/file1.txt", 0644, 100)
	dryRun.LogFileUpdate("/file2.txt", 0644, 50, 150)
	dryRun.LogDirectoryCreate("/dir", 0755)
	dryRun.LogTemplateRender("/template.j2", "/output", 0644)
	dryRun.LogShellExecution("ls -la", false)
	dryRun.LogSymlinkCreate("/target", "/link", true)
	dryRun.LogHardlinkCreate("/file", "/link", false)
	dryRun.LogPermissionsChange("/path", "0755", "user", "group", true)
	dryRun.LogFileCopy("/src", "/dest", 0644, 1024)
}

func TestFormatModeVarious(t *testing.T) {
	testCases := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		{"standard file", 0644, "0644"},
		{"executable", 0755, "0755"},
		{"readonly", 0444, "0444"},
		{"owner only", 0600, "0600"},
		{"write all", 0666, "0666"},
		{"exec all", 0777, "0777"},
		{"owner exec", 0700, "0700"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatMode(tc.mode)
			if result != tc.expected {
				t.Errorf("formatMode(%v) = %s, want %s", tc.mode, result, tc.expected)
			}
		})
	}
}

func TestDryRunLogger_EdgeCases(t *testing.T) {
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	// Test with empty strings
	dryRun.LogShellExecution("", false)
	dryRun.LogTemplateRender("", "", 0644)
	dryRun.LogVariableLoad(0, "")
	dryRun.LogFileCreate("", 0644, 0)
	dryRun.LogDirectoryCreate("", 0755)
	dryRun.LogSymlinkCreate("", "", false)
	dryRun.LogHardlinkCreate("", "", false)
	dryRun.LogPermissionsChange("", "", "", "", false)
	dryRun.LogFileCopy("", "", 0644, 0)

	// Test with large values
	dryRun.LogFileCreate("/file", 0644, 999999999)
	dryRun.LogFileRemove("/file", 999999999)
	dryRun.LogVariableLoad(1000, "/vars")
	dryRun.LogVariableSet(1000)
}

func TestDryRunLogger_AllMethods(t *testing.T) {
	// Ensure all methods can be called without panicking
	log := logger.NewTestLogger()
	dryRun := newDryRunLogger(log)

	t.Run("shell", func(t *testing.T) {
		dryRun.LogShellExecution("cmd", false)
		dryRun.LogShellExecution("cmd", true)
	})

	t.Run("template", func(t *testing.T) {
		dryRun.LogTemplateRender("src", "dest", 0644)
		dryRun.LogTemplateCreate("src", "dest", 0644, 100)
		dryRun.LogTemplateUpdate("src", "dest", 0644, 100, 200)
		dryRun.LogTemplateNoChange("src", "dest")
	})

	t.Run("variables", func(t *testing.T) {
		dryRun.LogVariableLoad(5, "file")
		dryRun.LogVariableSet(3)
		dryRun.LogRegister(config.Step{Register: "var"})
		dryRun.LogRegister(config.Step{})
	})

	t.Run("files", func(t *testing.T) {
		dryRun.LogFileCreate("path", 0644, 100)
		dryRun.LogFileUpdate("path", 0644, 100, 200)
		dryRun.LogFileRemove("path", 100)
		dryRun.LogFileTouch("path")
		dryRun.LogFileCopy("src", "dest", 0644, 100)
		dryRun.LogFileCopyNoChange("src", "dest")
	})

	t.Run("directories", func(t *testing.T) {
		dryRun.LogDirectoryCreate("path", 0755)
		dryRun.LogDirectoryRemove("path")
	})

	t.Run("links", func(t *testing.T) {
		dryRun.LogSymlinkCreate("target", "link", false)
		dryRun.LogSymlinkCreate("target", "link", true)
		dryRun.LogSymlinkNoChange("target", "link")
		dryRun.LogHardlinkCreate("target", "link", false)
		dryRun.LogHardlinkCreate("target", "link", true)
		dryRun.LogHardlinkNoChange("target", "link")
	})

	t.Run("permissions", func(t *testing.T) {
		dryRun.LogPermissionsChange("path", "0644", "", "", false)
		dryRun.LogPermissionsChange("path", "", "user", "", false)
		dryRun.LogPermissionsChange("path", "", "", "group", false)
		dryRun.LogPermissionsChange("path", "0644", "user", "group", true)
		dryRun.LogPermissionsNoChange("path")
	})
}

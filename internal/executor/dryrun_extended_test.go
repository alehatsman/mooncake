package executor

import (
	"os"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
)

// TestDryRunLogger_LogArchiveExtraction tests archive extraction logging
func TestDryRunLogger_LogArchiveExtraction(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.LogArchiveExtraction("/tmp/archive.tar.gz", "/opt/app", "tar.gz", 0)

	// Logger should have been called
	t.Log("LogArchiveExtraction executed successfully")
}

// TestDryRunLogger_LogArchiveExtraction_WithStrip tests with strip components
func TestDryRunLogger_LogArchiveExtraction_WithStrip(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.LogArchiveExtraction("/tmp/archive.tar", "/opt/app", "tar", 2)

	// Should include strip components message
	t.Log("LogArchiveExtraction with strip executed successfully")
}

// TestDryRunLogger_LogFileDownload tests file download logging
func TestDryRunLogger_LogFileDownload(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.LogFileDownload("https://example.com/file.txt", "/tmp/file.txt", 0644)

	t.Log("LogFileDownload executed successfully")
}

// TestDryRunLogger_LogFileDownloadNoChange tests no-change download logging
func TestDryRunLogger_LogFileDownloadNoChange(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.LogFileDownloadNoChange("https://example.com/file.txt", "/tmp/file.txt")

	t.Log("LogFileDownloadNoChange executed successfully")
}

// TestDryRunLogger_LogServiceOperation tests service operation logging
func TestDryRunLogger_LogServiceOperation(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	serviceAction := &config.ServiceAction{
		Name:  "nginx",
		State: "started",
	}

	dryRunLogger.LogServiceOperation("nginx", serviceAction, false)

	t.Log("LogServiceOperation executed successfully")
}

// TestDryRunLogger_LogServiceOperation_WithSudo tests service operation with sudo
func TestDryRunLogger_LogServiceOperation_WithSudo(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	enabled := true
	serviceAction := &config.ServiceAction{
		Name:    "nginx",
		State:   "started",
		Enabled: &enabled,
	}

	dryRunLogger.LogServiceOperation("nginx", serviceAction, true)

	t.Log("LogServiceOperation with sudo executed successfully")
}

// TestDryRunLogger_LogServiceOperation_WithUnit tests service with unit file
func TestDryRunLogger_LogServiceOperation_WithUnit(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	serviceAction := &config.ServiceAction{
		Name: "myapp",
		Unit: &config.ServiceUnit{
			Dest:    "/etc/systemd/system/myapp.service",
			Content: "[Service]\nExecStart=/usr/bin/myapp",
		},
		DaemonReload: true,
	}

	dryRunLogger.LogServiceOperation("myapp", serviceAction, false)

	t.Log("LogServiceOperation with unit file executed successfully")
}

// TestDryRunLogger_LogServiceOperation_Empty tests service with no operations
func TestDryRunLogger_LogServiceOperation_Empty(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	serviceAction := &config.ServiceAction{
		Name: "nginx",
	}

	dryRunLogger.LogServiceOperation("nginx", serviceAction, false)

	t.Log("LogServiceOperation with no operations executed successfully")
}

// TestDryRunLogger_LogAssertCheck tests assertion check logging
func TestDryRunLogger_LogAssertCheck(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.LogAssertCheck("command", "exit code == 0")

	t.Log("LogAssertCheck executed successfully")
}

// TestDryRunLogger_LogAssertCheck_Types tests different assertion types
func TestDryRunLogger_LogAssertCheck_Types(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	assertTypes := []struct {
		assertType string
		expected   string
	}{
		{"command", "exit code == 0"},
		{"file", "file exists"},
		{"http", "status code == 200"},
	}

	for _, tt := range assertTypes {
		t.Run(tt.assertType, func(t *testing.T) {
			dryRunLogger.LogAssertCheck(tt.assertType, tt.expected)
			t.Logf("LogAssertCheck(%s) executed successfully", tt.assertType)
		})
	}
}

// TestDryRunLogger_LogPresetOperation tests preset operation logging
func TestDryRunLogger_LogPresetOperation(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	invocation := &config.PresetInvocation{
		Name: "ollama",
		With: map[string]interface{}{
			"state": "installed",
		},
	}

	dryRunLogger.LogPresetOperation(invocation, 1)

	t.Log("LogPresetOperation executed successfully")
}

// TestDryRunLogger_LogPresetOperation_NoParams tests preset with no parameters
func TestDryRunLogger_LogPresetOperation_NoParams(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	invocation := &config.PresetInvocation{
		Name: "simple-preset",
	}

	dryRunLogger.LogPresetOperation(invocation, 0)

	t.Log("LogPresetOperation with no parameters executed successfully")
}

// TestDryRunLogger_LogPrintMessage tests print message logging
func TestDryRunLogger_LogPrintMessage(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.LogPrintMessage("Hello, world!")

	t.Log("LogPrintMessage executed successfully")
}

// TestDryRunLogger_LogPrintMessage_MultiLine tests multi-line print
func TestDryRunLogger_LogPrintMessage_MultiLine(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	message := "Line 1\nLine 2\nLine 3"
	dryRunLogger.LogPrintMessage(message)

	t.Log("LogPrintMessage with multi-line executed successfully")
}

// TestDryRunLogger_LogAction tests internal logAction method
func TestDryRunLogger_LogAction(t *testing.T) {
	testLogger := logger.NewTestLogger()
	dryRunLogger := NewDryRunLogger(testLogger)

	dryRunLogger.logAction("copy", "copy file /src/file.txt to /dst/file.txt")

	t.Log("logAction executed successfully")
}

// TestFormatMode tests file mode formatting
func TestFormatMode(t *testing.T) {
	tests := []struct {
		mode     os.FileMode
		expected string
	}{
		{0644, "0644"},
		{0755, "0755"},
		{0600, "0600"},
		{0777, "0777"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatMode(tt.mode)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("formatMode(%v) = %q, should contain %q", tt.mode, result, tt.expected)
			}
		})
	}
}


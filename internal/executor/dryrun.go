package executor

import (
	"os"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
)

// dryRunLogger provides consistent dry-run message formatting across all handlers.
type dryRunLogger struct {
	logger logger.Logger
}

// newDryRunLogger creates a dry-run logger wrapper.
func newDryRunLogger(log logger.Logger) *dryRunLogger {
	return &dryRunLogger{logger: log}
}

// LogShellExecution logs a dry-run message for shell command execution.
func (d *dryRunLogger) LogShellExecution(command string, withSudo bool) {
	d.logger.Infof("  [DRY-RUN] Would execute: %s", command)
	if withSudo {
		d.logger.Infof("  [DRY-RUN] With sudo privileges")
	}
}

// LogFileOperation logs a dry-run message for file/directory operations.
func (d *dryRunLogger) LogFileOperation(state, path string, mode os.FileMode) {
	switch state {
	case "directory":
		d.logger.Infof("  [DRY-RUN] Would create directory: %s (mode: %04o)", path, mode)
	case "file":
		d.logger.Infof("  [DRY-RUN] Would create file: %s (mode: %04o)", path, mode)
	default:
		d.logger.Infof("  [DRY-RUN] Would create %s: %s (mode: %04o)", state, path, mode)
	}
}

// LogTemplateRender logs a dry-run message for template rendering.
func (d *dryRunLogger) LogTemplateRender(src, dest string, mode os.FileMode) {
	d.logger.Infof("  [DRY-RUN] Would template: %s -> %s (mode: %04o)", src, dest, mode)
}

// LogVariableLoad logs a dry-run message for loading variables.
func (d *dryRunLogger) LogVariableLoad(count int, source string) {
	d.logger.Infof("  [DRY-RUN] Would load %d variables from: %s", count, source)
}

// LogVariableSet logs a dry-run message for setting variables.
func (d *dryRunLogger) LogVariableSet(count int) {
	d.logger.Infof("  [DRY-RUN] Would set %d variables", count)
}

// LogInclude logs a dry-run message for including other config files.
func (d *dryRunLogger) LogInclude(stepCount int, path string) {
	d.logger.Debugf("  [DRY-RUN] Would include %d steps from: %s", stepCount, path)
}

// LogRegister logs a dry-run message for registering results.
func (d *dryRunLogger) LogRegister(step config.Step) {
	if step.Register != "" {
		d.logger.Debugf("  [DRY-RUN] Would register result as: %s", step.Register)
	}
}

package executor

import (
	"fmt"
	"os"
	"strings"

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

// LogTemplateRender logs a dry-run message for template rendering.
func (d *dryRunLogger) LogTemplateRender(src, dest string, mode os.FileMode) {
	d.logger.Infof("  [DRY-RUN] Would template: %s -> %s (mode: %s)", src, dest, formatMode(mode))
}

// LogVariableLoad logs a dry-run message for loading variables.
func (d *dryRunLogger) LogVariableLoad(count int, source string) {
	d.logger.Infof("  [DRY-RUN] Would load %d variables from: %s", count, source)
}

// LogVariableSet logs a dry-run message for setting variables.
func (d *dryRunLogger) LogVariableSet(count int) {
	d.logger.Infof("  [DRY-RUN] Would set %d variables", count)
}

// LogRegister logs a dry-run message for registering results.
func (d *dryRunLogger) LogRegister(step config.Step) {
	if step.Register != "" {
		d.logger.Debugf("  [DRY-RUN] Would register result as: %s", step.Register)
	}
}

// LogFileCreate logs a dry-run message for file creation.
func (d *dryRunLogger) LogFileCreate(path string, mode os.FileMode, size int) {
	d.logger.Infof("  [DRY-RUN] Would create file: %s (mode: %s, size: %d bytes)", path, formatMode(mode), size)
}

// LogFileUpdate logs a dry-run message for file update.
func (d *dryRunLogger) LogFileUpdate(path string, mode os.FileMode, oldSize, newSize int) {
	d.logger.Infof("  [DRY-RUN] Would update file: %s (mode: %s, %d -> %d bytes)", path, formatMode(mode), oldSize, newSize)
}

// LogDirectoryCreate logs a dry-run message for directory creation.
func (d *dryRunLogger) LogDirectoryCreate(path string, mode os.FileMode) {
	d.logger.Infof("  [DRY-RUN] Would create directory: %s (mode: %s)", path, formatMode(mode))
}

// LogTemplateCreate logs a dry-run message for template creation.
func (d *dryRunLogger) LogTemplateCreate(src, dest string, mode os.FileMode, size int) {
	d.logger.Infof("  [DRY-RUN] Would template: %s -> %s (mode: %s, size: %d bytes)", src, dest, formatMode(mode), size)
}

// LogTemplateUpdate logs a dry-run message for template update.
func (d *dryRunLogger) LogTemplateUpdate(src, dest string, mode os.FileMode, oldSize, newSize int) {
	d.logger.Infof("  [DRY-RUN] Would template: %s -> %s (mode: %s, %d -> %d bytes)", src, dest, formatMode(mode), oldSize, newSize)
}

// LogTemplateNoChange logs a dry-run message when template produces no changes.
func (d *dryRunLogger) LogTemplateNoChange(src, dest string) {
	d.logger.Infof("  [DRY-RUN] Template would produce no changes: %s -> %s", src, dest)
}

// LogFileRemove logs a dry-run message for file removal.
func (d *dryRunLogger) LogFileRemove(path string, size int64) {
	d.logger.Infof("  [DRY-RUN] Would remove file: %s (%d bytes)", path, size)
}

// LogDirectoryRemove logs a dry-run message for directory removal.
func (d *dryRunLogger) LogDirectoryRemove(path string) {
	d.logger.Infof("  [DRY-RUN] Would remove directory: %s", path)
}

// LogFileTouch logs a dry-run message for updating file timestamps.
func (d *dryRunLogger) LogFileTouch(path string) {
	d.logger.Infof("  [DRY-RUN] Would touch file (update timestamp): %s", path)
}

// LogSymlinkCreate logs a dry-run message for symlink creation.
func (d *dryRunLogger) LogSymlinkCreate(src, dest string, force bool) {
	if force {
		d.logger.Infof("  [DRY-RUN] Would create symlink (force): %s -> %s", dest, src)
	} else {
		d.logger.Infof("  [DRY-RUN] Would create symlink: %s -> %s", dest, src)
	}
}

// LogSymlinkNoChange logs a dry-run message when symlink already exists correctly.
func (d *dryRunLogger) LogSymlinkNoChange(src, dest string) {
	d.logger.Infof("  [DRY-RUN] Symlink already correct: %s -> %s", dest, src)
}

// LogHardlinkCreate logs a dry-run message for hardlink creation.
func (d *dryRunLogger) LogHardlinkCreate(src, dest string, force bool) {
	if force {
		d.logger.Infof("  [DRY-RUN] Would create hardlink (force): %s -> %s", dest, src)
	} else {
		d.logger.Infof("  [DRY-RUN] Would create hardlink: %s -> %s", dest, src)
	}
}

// LogHardlinkNoChange logs a dry-run message when hardlink already exists correctly.
func (d *dryRunLogger) LogHardlinkNoChange(src, dest string) {
	d.logger.Infof("  [DRY-RUN] Hardlink already correct: %s -> %s", dest, src)
}

// LogPermissionsChange logs a dry-run message for permission changes.
func (d *dryRunLogger) LogPermissionsChange(path, mode, owner, group string, recurse bool) {
	msg := fmt.Sprintf("  [DRY-RUN] Would change permissions: %s", path)
	details := []string{}
	if mode != "" {
		details = append(details, fmt.Sprintf("mode: %s", mode))
	}
	if owner != "" {
		details = append(details, fmt.Sprintf("owner: %s", owner))
	}
	if group != "" {
		details = append(details, fmt.Sprintf("group: %s", group))
	}
	if recurse {
		details = append(details, "recursive")
	}
	if len(details) > 0 {
		msg += " (" + strings.Join(details, ", ") + ")"
	}
	d.logger.Infof(msg)
}

// LogPermissionsNoChange logs a dry-run message when permissions are already correct.
func (d *dryRunLogger) LogPermissionsNoChange(path string) {
	d.logger.Infof("  [DRY-RUN] Permissions already correct: %s", path)
}

// LogFileCopy logs a dry-run message for file copy.
func (d *dryRunLogger) LogFileCopy(src, dest string, mode os.FileMode, size int64) {
	d.logger.Infof("  [DRY-RUN] Would copy file: %s -> %s (mode: %s, size: %d bytes)", src, dest, formatMode(mode), size)
}

// LogFileCopyNoChange logs a dry-run message when file copy is not needed.
func (d *dryRunLogger) LogFileCopyNoChange(src, dest string) {
	d.logger.Infof("  [DRY-RUN] File already up to date: %s -> %s", src, dest)
}

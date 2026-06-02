//go:build windows

package agentd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// defaultScheduledTaskName mirrors the task name baked into the dotfiles
// bootstrap (platforms/windows/bootstrap.yml). Operators using a
// different task name set MOONCAKE_AGENTD_SCHEDULED_TASK in agentd's
// environment.
const defaultScheduledTaskName = "Mooncake-Agentd-Autostart"

// scheduledTaskName resolves which scheduled task drives the agentd
// lifecycle on this Windows host. Env-overridable to keep the choice
// out of the binary.
func scheduledTaskName() string {
	if n := os.Getenv("MOONCAKE_AGENTD_SCHEDULED_TASK"); n != "" {
		return n
	}
	return defaultScheduledTaskName
}

// swapBinary on Windows can't simply os.Rename a running .exe — the
// destination is locked. Windows DOES allow moving the running .exe to
// a different name (Vista+), so we use that trick:
//
//  1. MoveFile dst → dst.pre-upgrade-<ts>   (running process keeps its
//     open handle to the renamed inode; nothing crashes)
//  2. MoveFile src → dst                    (staged binary now lives at
//     the canonical path)
//  3. When the scheduled task restarts agentd, it spawns from the
//     replaced binary at dst — fresh process image.
//
// On failure we best-effort restore dst from the backup so we don't
// leave the host in a no-binary state.
func swapBinary(src, dst string) error {
	backup := fmt.Sprintf("%s.pre-upgrade-%d", dst, time.Now().Unix())
	if err := os.Rename(dst, backup); err != nil {
		return fmt.Errorf("move running exe out of the way: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		// Restore so the scheduled task doesn't restart into nothing.
		if rerr := os.Rename(backup, dst); rerr != nil {
			return fmt.Errorf("move staged %s → %s failed (%w) AND restore failed (%v) — binary at %s",
				src, dst, err, rerr, backup)
		}
		return fmt.Errorf("move staged → %s: %w", dst, err)
	}
	return nil
}

// reExec on Windows can't replace the running process image in-place
// (no exec). Instead we spawn a *truly detached* PowerShell helper
// that outlives this daemon process, then it triggers a scheduled-task
// restart cycle:
//
//	Start-Sleep -Seconds 2                # let the 202 flush
//	Stop-ScheduledTask  $taskName          # kills this agentd
//	Start-Sleep -Seconds 1                 # let cleanup settle
//	Start-ScheduledTask $taskName          # spawns fresh agentd.exe
//
// The first attempt at this (Go's exec.Command with DETACHED_PROCESS
// flags) failed silently when agentd was running under a scheduled
// task with no console — child PowerShell never managed to do anything
// useful because it inherited unusable handles.
//
// The reliable pattern: write the helper to a .ps1 file, spawn it via
// `cmd.exe /c start "" /B powershell -File <ps1>`. cmd's `start /B`
// is the Windows-native way to background a child cleanly; the cmd
// process exits immediately, the PowerShell runs decoupled. The
// helper logs its own steps to %TEMP%\mooncake-upgrade-helper.log so
// failures aren't invisible.
//
// The new agentd gets a brand-new PID (scheduled-task spawn = fresh
// process), which is what the controller's waitForRestart watches for.
func reExec(binPath string) error {
	task := scheduledTaskName()

	logPath := filepath.Join(os.TempDir(), "mooncake-upgrade-helper.log")
	scriptPath := filepath.Join(os.TempDir(), "mooncake-upgrade-helper.ps1")

	// The script logs every step so a failed helper run is debuggable
	// from %TEMP%\mooncake-upgrade-helper.log rather than guessing.
	// Stop-ScheduledTask kills the agentd that wrote this file — that's
	// expected. Start-ScheduledTask runs after a brief pause so cleanup
	// of the killed process settles before the next spawn.
	script := fmt.Sprintf(`$ErrorActionPreference = 'Continue'
$log = '%s'
function Log($msg) { "[$(Get-Date -Format o)] $msg" | Out-File -FilePath $log -Append -Encoding utf8 }
Log "helper started; task='%s'"
Start-Sleep -Seconds 2
try {
  Stop-ScheduledTask -TaskName '%s' -ErrorAction Stop
  Log "Stop-ScheduledTask ok"
} catch {
  Log "Stop-ScheduledTask failed: $_"
}
Start-Sleep -Seconds 1
try {
  Start-ScheduledTask -TaskName '%s' -ErrorAction Stop
  Log "Start-ScheduledTask ok"
} catch {
  Log "Start-ScheduledTask failed: $_"
}
Log "helper done"
`, escapePSPath(logPath), escapePSPath(task), escapePSPath(task), escapePSPath(task))

	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write helper script %s: %w", scriptPath, err)
	}

	// `cmd /c start "" /B powershell -File <path>` — the canonical
	// Windows pattern for spawning a detached background process. The
	// empty "" is the (required) window title for start; /B means no
	// new window; -File runs the script. cmd.exe exits as soon as start
	// has launched the child, severing the parent/child relationship.
	cmd := exec.Command("cmd.exe", "/c", "start", "", "/B",
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-WindowStyle", "Hidden",
		"-File", scriptPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn restart helper for task %q: %w", task, err)
	}
	// Wait for cmd.exe to exit — it should finish near-instantly once
	// `start` returns. We deliberately do NOT Wait on the powershell
	// child; that's the whole point of `start /B`.
	_ = cmd.Wait()
	return nil
}

// escapePSPath quotes a Windows path for inclusion inside a PowerShell
// single-quoted string literal: backslashes don't need escaping inside
// single quotes, but single quotes do (doubled).
func escapePSPath(p string) string {
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		if p[i] == '\'' {
			out = append(out, '\'', '\'')
			continue
		}
		out = append(out, p[i])
	}
	return string(out)
}

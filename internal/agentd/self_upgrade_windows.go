//go:build windows

package agentd

import (
	"fmt"
	"os"
	"os/exec"
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
//   1. MoveFile dst → dst.pre-upgrade-<ts>   (running process keeps its
//      open handle to the renamed inode; nothing crashes)
//   2. MoveFile src → dst                    (staged binary now lives at
//      the canonical path)
//   3. When the scheduled task restarts agentd, it spawns from the
//      replaced binary at dst — fresh process image.
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

// reExec on Windows can't use syscall.Exec the way Unix does — there's
// no exec-in-place. Instead we spawn a detached PowerShell helper that
// outlives the current daemon process and triggers a scheduled-task
// restart cycle:
//
//	Start-Sleep -Seconds 2                    # let the 202 flush
//	Stop-ScheduledTask  $taskName              # kills this agentd
//	Start-Sleep -Seconds 1                    # let cleanup settle
//	Start-ScheduledTask $taskName              # spawns fresh agentd.exe
//
// The helper is fully detached so it survives Stop-ScheduledTask
// killing us. The new agentd gets a brand-new PID — the controller's
// waitForRestart detects this via daemon_pid != oldPID. binPath is
// accepted to keep the cross-OS signature stable; the new process is
// launched by the scheduler from the path the task was registered
// against (which we've already replaced via swapBinary).
func reExec(binPath string) error {
	task := scheduledTaskName()
	script := fmt.Sprintf(`Start-Sleep -Seconds 2
Stop-ScheduledTask -TaskName "%s" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1
Start-ScheduledTask -TaskName "%s"
`, task, task)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// DETACHED_PROCESS (0x8) | CREATE_NEW_PROCESS_GROUP (0x200).
		// Detaches from this process's console + job so the helper
		// survives the Stop-ScheduledTask that's about to kill us.
		CreationFlags: 0x00000008 | 0x00000200,
		HideWindow:    true,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn restart helper for task %q: %w", task, err)
	}
	// Don't Wait — we deliberately leave the helper running. Process
	// handle is closed when the parent exits via GC; the OS keeps the
	// detached child alive.
	return nil
}

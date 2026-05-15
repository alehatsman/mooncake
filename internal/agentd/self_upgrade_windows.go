//go:build windows

package agentd

// swapBinary on Windows needs the MoveFile-the-running-exe trick plus
// a scheduled-task restart handoff. Coming in a follow-up; v1 ships
// Linux-only and explicitly rejects Windows targets at the controller
// CLI layer so we never land here in practice.
func swapBinary(src, dst string) error {
	return errSelfUpgradeUnsupportedOS
}

// reExec is unreachable on Windows in v1 (swapBinary fails first), but
// keep the signature so the rest of the file compiles on every OS.
func reExec(binPath string) error {
	return errSelfUpgradeUnsupportedOS
}

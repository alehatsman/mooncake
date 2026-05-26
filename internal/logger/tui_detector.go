package logger

import (
	"os"

	"golang.org/x/term"
)

// TerminalInfo contains information about terminal capabilities.
type TerminalInfo struct {
	IsTerminal   bool
	SupportsANSI bool
	Width        int
	Height       int
}

// DetectTerminal detects terminal capabilities and returns terminal information.
func DetectTerminal() TerminalInfo {
	fd := int(os.Stdout.Fd())
	isTerminal := term.IsTerminal(fd)

	// NO_COLOR always wins — explicit user opt-out per
	// https://no-color.org, regardless of TTY state or downstream
	// force-color hints.
	if os.Getenv("NO_COLOR") != "" {
		return TerminalInfo{
			IsTerminal:   isTerminal,
			SupportsANSI: false,
		}
	}

	// FORCE_COLOR / CLICOLOR_FORCE flip ANSI on even when stdout is a
	// pipe — the standard convention used by ripgrep, fatih/color,
	// git, and the rest of the modern CLI ecosystem. `mooncake task`
	// injects FORCE_COLOR=1 into the child env when the outer process
	// is a TTY (see internal/actions/shell handler.go), so colors
	// flow through the outer task's `|`-prefixed stream wrap to the
	// user's terminal.
	if !isTerminal {
		if os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") != "" {
			return TerminalInfo{
				IsTerminal:   false,
				SupportsANSI: true,
			}
		}
		return TerminalInfo{
			IsTerminal:   false,
			SupportsANSI: false,
		}
	}

	// CI / dumb-term suppression. These imply "no interactive
	// features" rather than "no color", but mooncake conflates the
	// two through SupportsANSI today; preserve that bundling to avoid
	// regression.
	if os.Getenv("CI") == "true" || os.Getenv("TERM") == "dumb" {
		return TerminalInfo{
			IsTerminal:   true,
			SupportsANSI: false,
		}
	}

	width, height, err := term.GetSize(fd)
	if err != nil {
		// Default to standard terminal size if detection fails
		width, height = 80, 24
	}

	return TerminalInfo{
		IsTerminal:   true,
		SupportsANSI: true,
		Width:        width,
		Height:       height,
	}
}

// IsTUISupported checks if the terminal supports TUI mode.
// Returns true if terminal is detected, supports ANSI codes, and meets minimum size requirements.
func IsTUISupported() bool {
	info := DetectTerminal()

	if !info.IsTerminal || !info.SupportsANSI {
		return false
	}

	// Minimum terminal size requirements: 80x24
	if info.Width < 80 || info.Height < 24 {
		return false
	}

	return true
}

// GetTerminalSize returns the current terminal size.
// Returns default 80x24 if detection fails.
func GetTerminalSize() (width, height int) {
	info := DetectTerminal()

	if info.Width == 0 || info.Height == 0 {
		return 80, 24
	}

	return info.Width, info.Height
}

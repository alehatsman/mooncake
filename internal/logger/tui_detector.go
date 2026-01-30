package logger

import (
	"os"

	"golang.org/x/term"
)

// TerminalInfo contains information about terminal capabilities
type TerminalInfo struct {
	IsTerminal   bool
	SupportsANSI bool
	Width        int
	Height       int
}

// DetectTerminal detects terminal capabilities and returns terminal information
func DetectTerminal() TerminalInfo {
	fd := int(os.Stdout.Fd())
	isTerminal := term.IsTerminal(fd)

	if !isTerminal {
		return TerminalInfo{
			IsTerminal:   false,
			SupportsANSI: false,
			Width:        0,
			Height:       0,
		}
	}

	// Check environment variables that indicate non-interactive mode
	if os.Getenv("CI") == "true" || os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") != "" {
		return TerminalInfo{
			IsTerminal:   true,
			SupportsANSI: false,
			Width:        0,
			Height:       0,
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

// IsTUISupported checks if the terminal supports TUI mode
// Returns true if terminal is detected, supports ANSI codes, and meets minimum size requirements
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

// GetTerminalSize returns the current terminal size
// Returns default 80x24 if detection fails
func GetTerminalSize() (width, height int) {
	info := DetectTerminal()

	if info.Width == 0 || info.Height == 0 {
		return 80, 24
	}

	return info.Width, info.Height
}

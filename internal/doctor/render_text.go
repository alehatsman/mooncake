package doctor

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// sectionTitle is the long-form heading printed for each section. Order is
// the catalogue order; the renderer trusts the catalogue ordering and just
// groups consecutive results.
var sectionTitle = map[string]string{
	"install":  "Install",
	"system":   "System",
	"state":    "State",
	"presets":  "Preset search paths",
	"tools":    "Tools",
	"project":  "Project",
	"services": "Optional services",
}

// RenderText writes a scannable report to out. Colour is added on TTY
// unless noColor or the NO_COLOR env var is set (de-facto standard).
func RenderText(out io.Writer, rep *Report, noColor bool) error {
	useColor := !noColor && os.Getenv("NO_COLOR") == "" && isTerminal(out)

	fmt.Fprintln(out, "mooncake doctor — health check")
	fmt.Fprintln(out)

	var currentSection string
	for _, r := range rep.Results {
		if r.Section != currentSection {
			currentSection = r.Section
			title, ok := sectionTitle[currentSection]
			if !ok {
				title = currentSection
			}
			if r.Section == "project" {
				fmt.Fprintf(out, "Project (%s)\n", rep.Cwd)
			} else {
				fmt.Fprintln(out, title)
			}
		}
		writeResultLine(out, r, useColor)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Summary: %d ok, %d info, %d warning(s), %d error(s)",
		rep.Ok, rep.Infos, rep.Warnings, rep.Errors)
	if rep.HasErrors() {
		fmt.Fprintln(out, "  → exit 1")
	} else {
		fmt.Fprintln(out)
	}
	return nil
}

func writeResultLine(out io.Writer, r Result, useColor bool) {
	glyph := glyphFor(r.Status)
	if useColor {
		glyph = colorFor(r.Status) + glyph + colorReset
	}
	fmt.Fprintf(out, "  %s %s\n", glyph, r.Message)
	if r.Detail != "" {
		for _, line := range strings.Split(strings.TrimRight(r.Detail, "\n"), "\n") {
			fmt.Fprintf(out, "       %s\n", line)
		}
	}
	if r.Fix != "" {
		fmt.Fprintf(out, "       fix: %s\n", r.Fix)
	}
	if len(r.UsedBy) > 0 {
		fmt.Fprintf(out, "       used by: %s\n", strings.Join(r.UsedBy, ", "))
	}
}

func glyphFor(s Status) string {
	switch s {
	case StatusOK:
		return "✓"
	case StatusInfo:
		return "ℹ"
	case StatusWarning:
		return "⚠"
	case StatusError:
		return "✗"
	}
	return "?"
}

const (
	colorReset = "\033[0m"
)

func colorFor(s Status) string {
	switch s {
	case StatusOK:
		return "\033[32m" // green
	case StatusInfo:
		return "\033[34m" // blue
	case StatusWarning:
		return "\033[33m" // yellow
	case StatusError:
		return "\033[31m" // red
	}
	return ""
}

// isTerminal returns true when out is os.Stdout/os.Stderr AND the
// corresponding fd is a TTY. Avoids depending on golang.org/x/term;
// uses the OS-level check.
func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

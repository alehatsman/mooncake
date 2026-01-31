package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FormatDiagnosticsWithContext formats diagnostics with YAML context and step names
func FormatDiagnosticsWithContext(diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}

	// Filter to only user-friendly messages
	filtered := filterUserFriendlyDiagnostics(diagnostics)

	// Group by file
	fileGroups := make(map[string][]Diagnostic)
	for _, diag := range filtered {
		fileGroups[diag.FilePath] = append(fileGroups[diag.FilePath], diag)
	}

	var result strings.Builder

	for filePath, diags := range fileGroups {
		result.WriteString(fmt.Sprintf("\nError: %s\n\n", filePath))

		// Load file content for context
		fileLines := loadFileLines(filePath)

		for _, diag := range diags {
			// Show line reference
			result.WriteString(fmt.Sprintf("  Line %d: %s\n", diag.Line, diag.Message))

			// Show YAML context
			if len(fileLines) > 0 && diag.Line > 0 && diag.Line <= len(fileLines) {
				// Show the problematic line
				line := fileLines[diag.Line-1]
				result.WriteString(fmt.Sprintf("    %s\n", strings.TrimSpace(line)))

				// Try to extract step name if this is a step-level error
				stepName := extractStepName(fileLines, diag.Line)
				if stepName != "" {
					result.WriteString(fmt.Sprintf("    (in step: %s)\n", stepName))
				}
			}

			result.WriteString("\n")
		}
	}

	// Summary
	errorCount := 0
	warningCount := 0
	for _, diag := range filtered {
		if diag.Severity == "warning" {
			warningCount++
		} else {
			errorCount++
		}
	}

	if errorCount > 0 {
		result.WriteString(fmt.Sprintf("Found %d error(s)", errorCount))
		if warningCount > 0 {
			result.WriteString(fmt.Sprintf(" and %d warning(s)", warningCount))
		}
		result.WriteString("\n")
	}

	return result.String()
}

// filterUserFriendlyDiagnostics filters out technical schema validation messages
// and keeps only the actionable, user-friendly ones
func filterUserFriendlyDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	var filtered []Diagnostic

	for _, diag := range diagnostics {
		// Skip technical schema validation messages
		if strings.Contains(diag.Message, "doesn't validate with") {
			continue
		}
		if strings.Contains(diag.Message, "https://mooncake.dev/schemas") {
			continue
		}
		if strings.Contains(diag.Message, "/definitions/") && !strings.Contains(diag.Message, "step must have") {
			continue
		}

		filtered = append(filtered, diag)
	}

	return filtered
}

// loadFileLines loads a file and returns its lines
func loadFileLines(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

// extractStepName tries to find the step name near the given line
func extractStepName(lines []string, errorLine int) string {
	if errorLine < 1 || errorLine > len(lines) {
		return ""
	}

	// Search backwards from error line to find "name:" field
	for i := errorLine - 1; i >= 0 && i >= errorLine-10; i-- {
		line := strings.TrimSpace(lines[i])

		// Stop if we hit another step (starts with "- ")
		if i < errorLine-1 && strings.HasPrefix(line, "- ") {
			break
		}

		// Check if this line has a name
		if strings.HasPrefix(line, "name:") || strings.HasPrefix(line, "- name:") {
			// Extract the name value
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				// Remove quotes if present
				name = strings.Trim(name, "\"'")
				return name
			}
		}
	}

	return ""
}

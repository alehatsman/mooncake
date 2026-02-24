package agent

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func SanitizePlan(rawOutput string) ([]byte, error) {
	content := strings.TrimSpace(rawOutput)

	if content == "" {
		return nil, fmt.Errorf("empty plan output")
	}

	content = extractYAMLFromFences(content)
	content = strings.TrimSpace(content)

	if content == "" {
		return nil, fmt.Errorf("plan is empty after sanitization")
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &parsed); err == nil {
		if steps, ok := parsed["steps"]; ok {
			stepsBytes, err := yaml.Marshal(steps)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal steps array: %w", err)
			}
			return stepsBytes, nil
		}
	}

	return []byte(content), nil
}

func extractYAMLFromFences(content string) string {
	lines := strings.Split(content, "\n")

	start := 0
	end := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			start = i + 1
			break
		}
	}

	for i := len(lines) - 1; i >= start; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "```") {
			end = i
			break
		}
	}

	if start < end {
		return strings.Join(lines[start:end], "\n")
	}

	return strings.Join(lines, "\n")
}

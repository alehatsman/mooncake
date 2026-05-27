package pilot

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"gopkg.in/yaml.v3"
)

// SanitizePlan strips markdown fences from the LLM's raw response,
// validates the body parses as either YAML or JSON via
// config.DecodeAuto (the same auto-detect that powers the apply
// path, commit 93ee3c37), and returns the canonicalized YAML bytes.
//
// Re-encoding to YAML keeps the artifact on disk in a single,
// stable format regardless of what the model emitted — proposal-08
// §"Risks and non-issues": "The file on disk is still YAML."
// Downstream consumers (.mooncake/iterations/NNNNN.plan.yml,
// transaction_wrap.decodePlan, plan-confirm gate) keep working
// without format-sniffing.
//
// If the body is a structured RunConfig with a top-level `steps:`
// field, only the steps array is returned (preserving legacy
// SanitizePlan behavior so planHash stays comparable across the two
// equivalent representations).
func SanitizePlan(rawOutput string) ([]byte, error) {
	content := strings.TrimSpace(rawOutput)

	if content == "" {
		return nil, fmt.Errorf("empty plan output")
	}

	content = extractFromFences(content)
	content = strings.TrimSpace(content)

	if content == "" {
		return nil, fmt.Errorf("plan is empty after sanitization")
	}

	// Try to detect the "steps wrapper" shape and unwrap. We do this
	// via config.DecodeAuto so JSON and YAML both work — same code
	// path the apply CLI uses (commit 93ee3c37).
	var parsed map[string]interface{}
	if err := config.DecodeAuto([]byte(content), &parsed); err == nil {
		if steps, ok := parsed["steps"]; ok {
			stepsBytes, err := yaml.Marshal(steps)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal steps array: %w", err)
			}
			return stepsBytes, nil
		}
	}

	// Generic shape (bare list, scalar-shell, etc). Re-encode through
	// DecodeAuto + yaml.Marshal so the on-disk artifact is canonical
	// YAML regardless of whether the model emitted JSON or YAML.
	var generic any
	if err := config.DecodeAuto([]byte(content), &generic); err == nil && generic != nil {
		out, err := yaml.Marshal(generic)
		if err == nil {
			return out, nil
		}
	}

	// Last-resort fallback: emit the post-fence content as-is. This
	// preserves pre-proposal-08 behavior for inputs DecodeAuto can't
	// handle (so a regression here surfaces as an executor error, not
	// a silent sanitize swallow).
	return []byte(content), nil
}

// extractFromFences strips a leading and trailing markdown code fence
// if present. Recognizes ```yaml, ```yml, ```json, and bare ```
// fences — small local models are inconsistent about which language
// tag they emit, so the check is intentionally lenient.
func extractFromFences(content string) string {
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

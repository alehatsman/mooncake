package agent

import (
	"testing"
)

func TestSanitize_WithStepsWrapper(t *testing.T) {
	input := "```yaml\nname: Create test file\nsteps:\n  - name: Create test.txt\n    file:\n      path: test.txt\n      content: hello\n```"
	
	result, err := SanitizePlan(input)
	if err != nil {
		t.Fatalf("SanitizePlan failed: %v", err)
	}
	
	t.Logf("Result:\n%s", string(result))
	
	if !contains(string(result), "name:") {
		t.Error("Expected result to contain 'name:' field")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || containsAnywhere(s, substr))
}

func containsAnywhere(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

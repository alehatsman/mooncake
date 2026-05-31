package executor_test

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/executor"
)

// component_dir is overlaid into the execute-time variable scope from the
// running step's CurrentDir, so execute-time renders (when/creates/unless and
// path expansion) see the dir of the file/component that declared the step.
func TestVariables_OverlaysComponentDir(t *testing.T) {
	ec := &executor.ExecutionContext{
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/modules/cache/go-quality",
	}

	vars := ec.Variables()
	got, ok := vars["component_dir"].(string)
	if !ok || got != "/modules/cache/go-quality" {
		t.Errorf("component_dir = %v, want %q", vars["component_dir"], "/modules/cache/go-quality")
	}
}

// With no CurrentDir set, component_dir is omitted rather than set to an empty
// string — keeps it out of scope for contexts that have no declaring dir.
func TestVariables_NoComponentDirWhenUnset(t *testing.T) {
	ec := &executor.ExecutionContext{
		Scope: executor.NewVariableScope(),
	}

	if _, present := ec.Variables()["component_dir"]; present {
		t.Error("component_dir should be absent when CurrentDir is empty")
	}
}

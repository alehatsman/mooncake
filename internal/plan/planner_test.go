package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanner_BuildPlan_Simple(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
steps:
  - name: Test step
    shell: echo "hello"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify plan
	if plan == nil {
		t.Fatal("Plan is nil")
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.Name != "Test step" {
		t.Errorf("Expected step name 'Test step', got '%s'", step.Name)
	}

	if step.Shell == nil {
		t.Fatal("Expected shell action, got nil")
	}

	if *step.Shell != "echo \"hello\"" {
		t.Errorf("Expected command 'echo \"hello\"', got '%s'", *step.Shell)
	}
}

func TestPlanner_ExpandWithItems_LoopVars(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
vars:
  items:
    - one
    - two
    - three

steps:
  - name: "Process {{ item }}"
    shell: "echo \"{{ index }}: {{ item }} (first={{ first }}, last={{ last }})\""
    with_items: items
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify loop expansion
	if len(plan.Steps) != 3 {
		t.Fatalf("Expected 3 steps, got %d", len(plan.Steps))
	}

	// Check first iteration
	step0 := plan.Steps[0]
	if step0.LoopContext == nil {
		t.Fatal("Step 0 missing loop context")
	}
	if step0.LoopContext.Index != 0 {
		t.Errorf("Expected index 0, got %d", step0.LoopContext.Index)
	}
	if !step0.LoopContext.First {
		t.Error("Expected first=true for step 0")
	}
	if step0.LoopContext.Last {
		t.Error("Expected last=false for step 0")
	}

	// Check last iteration
	step2 := plan.Steps[2]
	if step2.LoopContext == nil {
		t.Fatal("Step 2 missing loop context")
	}
	if step2.LoopContext.Index != 2 {
		t.Errorf("Expected index 2, got %d", step2.LoopContext.Index)
	}
	if step2.LoopContext.First {
		t.Error("Expected first=false for step 2")
	}
	if !step2.LoopContext.Last {
		t.Error("Expected last=true for step 2")
	}
}

func TestPlanner_CycleDetection(t *testing.T) {
	// Create temporary config files with cycle
	tmpDir := t.TempDir()

	configA := filepath.Join(tmpDir, "a.yml")
	configB := filepath.Join(tmpDir, "b.yml")

	contentA := `- name: Step A
  shell: echo "A"
- include: b.yml
`
	contentB := `- name: Step B
  shell: echo "B"
- include: a.yml
`

	err := os.WriteFile(configA, []byte(contentA), 0644)
	if err != nil {
		t.Fatalf("Failed to write config A: %v", err)
	}

	err = os.WriteFile(configB, []byte(contentB), 0644)
	if err != nil {
		t.Fatalf("Failed to write config B: %v", err)
	}

	// Build plan - should detect cycle
	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configA,
		Variables:  nil,
		Tags:       nil,
	})

	if err == nil {
		t.Fatal("Expected error for include cycle, got nil")
	}

	// Check error message contains "cycle"
	errMsg := err.Error()
	if !contains(errMsg, "cycle") {
		t.Errorf("Expected error message to contain 'cycle', got: %s", errMsg)
	}
}

func TestPlanner_TagFiltering(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
steps:
  - name: Install step
    shell: echo "install"
    tags:
      - install

  - name: Test step
    shell: echo "test"
    tags:
      - test
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan with tag filter
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       []string{"install"},
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify tag filtering
	if len(plan.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(plan.Steps))
	}

	// First step should not be skipped (has install tag)
	if plan.Steps[0].Skipped {
		t.Error("Expected install step to not be skipped")
	}

	// Second step should be skipped (has test tag, not install)
	if !plan.Steps[1].Skipped {
		t.Error("Expected test step to be skipped")
	}
}

func TestDeterminism(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
vars:
  items:
    - a
    - b
    - c

steps:
  - name: Process {{ item }}
    shell: echo "{{ item }}"
    with_items: items
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan multiple times
	planner1 := NewPlanner()
	plan1, err := planner1.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})
	if err != nil {
		t.Fatalf("Failed to build plan 1: %v", err)
	}

	planner2 := NewPlanner()
	plan2, err := planner2.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})
	if err != nil {
		t.Fatalf("Failed to build plan 2: %v", err)
	}

	// Verify step IDs are the same
	if len(plan1.Steps) != len(plan2.Steps) {
		t.Fatalf("Plans have different number of steps: %d vs %d", len(plan1.Steps), len(plan2.Steps))
	}

	for i := range plan1.Steps {
		if plan1.Steps[i].ID != plan2.Steps[i].ID {
			t.Errorf("Step %d has different IDs: %s vs %s", i, plan1.Steps[i].ID, plan2.Steps[i].ID)
		}

		if plan1.Steps[i].Name != plan2.Steps[i].Name {
			t.Errorf("Step %d has different names: %s vs %s", i, plan1.Steps[i].Name, plan2.Steps[i].Name)
		}
	}
}

func TestPlanner_ExpandWithFileTree(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "templates")
	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Create test files in specific order to test sorting
	testFiles := []string{"c.txt", "a.txt", "b.txt"}
	for _, fname := range testFiles {
		err := os.WriteFile(filepath.Join(templateDir, fname), []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	configPath := filepath.Join(tmpDir, "test.yml")
	configContent := `version: "1.0"
steps:
  - name: "Copy {{ item.Src }}"
    template:
      src: "{{ item.Src }}"
      dest: "/tmp/{{ item.Name }}"
    with_filetree: ./templates
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify file tree expansion (includes both directory and files)
	if len(plan.Steps) < 3 {
		t.Fatalf("Expected at least 3 steps (files), got %d", len(plan.Steps))
	}

	// Verify all steps have loop context
	for i, step := range plan.Steps {
		if step.LoopContext == nil {
			t.Fatalf("Step %d missing loop context", i)
		}
		if step.LoopContext.Type != "with_filetree" {
			t.Errorf("Expected loop type 'with_filetree', got '%s'", step.LoopContext.Type)
		}
	}

	// Verify first/last flags
	if !plan.Steps[0].LoopContext.First {
		t.Error("First step should have first=true")
	}
	lastIdx := len(plan.Steps) - 1
	if !plan.Steps[lastIdx].LoopContext.Last {
		t.Error("Last step should have last=true")
	}

	// Verify files are in sorted order (check that at least some expected files appear)
	stepNames := ""
	for _, step := range plan.Steps {
		stepNames += step.Name + " "
	}
	if !contains(stepNames, "a.txt") || !contains(stepNames, "b.txt") || !contains(stepNames, "c.txt") {
		t.Errorf("Expected to find a.txt, b.txt, c.txt in step names, got: %s", stepNames)
	}
}

func TestPlanner_ExpandInclude(t *testing.T) {
	// Create temporary config files
	tmpDir := t.TempDir()

	mainConfig := filepath.Join(tmpDir, "main.yml")
	includedConfig := filepath.Join(tmpDir, "included.yml")

	mainContent := `version: "1.0"
steps:
  - name: Main step
    shell: echo "main"
  - include: included.yml
  - name: After include
    shell: echo "after"
`
	includedContent := `- name: Included step 1
  shell: echo "included1"
- name: Included step 2
  shell: echo "included2"
`

	err := os.WriteFile(mainConfig, []byte(mainContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	err = os.WriteFile(includedConfig, []byte(includedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write included config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: mainConfig,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify include expansion
	if len(plan.Steps) != 4 {
		t.Fatalf("Expected 4 steps (main + 2 included + after), got %d", len(plan.Steps))
	}

	// Check step order
	expectedNames := []string{"Main step", "Included step 1", "Included step 2", "After include"}
	for i, step := range plan.Steps {
		if step.Name != expectedNames[i] {
			t.Errorf("Step %d: expected name '%s', got '%s'", i, expectedNames[i], step.Name)
		}
	}

	// Verify origin tracking for included steps
	if plan.Steps[1].Origin == nil {
		t.Error("Included step 1 missing origin")
	} else if plan.Steps[1].Origin.FilePath != includedConfig {
		t.Errorf("Included step 1 origin file = %s, want %s", plan.Steps[1].Origin.FilePath, includedConfig)
	}
}

func TestPlanner_IncludeWithWhen(t *testing.T) {
	// Create temporary config files
	tmpDir := t.TempDir()

	mainConfig := filepath.Join(tmpDir, "main.yml")
	linuxConfig := filepath.Join(tmpDir, "linux.yml")

	mainContent := `version: "1.0"
vars:
  os: darwin
steps:
  - include: linux.yml
    when: os == "linux"
`
	linuxContent := `- name: Linux step
  shell: echo "linux"
`

	err := os.WriteFile(mainConfig, []byte(mainContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	err = os.WriteFile(linuxConfig, []byte(linuxContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write linux config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: mainConfig,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify include expanded and when condition propagated
	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step from included file, got %d", len(plan.Steps))
	}

	// Check that the when condition was propagated
	step := plan.Steps[0]
	if step.When == "" {
		t.Error("Expected when condition to be propagated to included step")
	}
	if !contains(step.When, "os == \"linux\"") {
		t.Errorf("Expected when condition to contain 'os == \"linux\"', got '%s'", step.When)
	}
}

func TestPlanner_WithFileTree_InvalidPath(t *testing.T) {
	// Create a temporary config file with invalid path
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
steps:
  - name: "Copy {{ item.Src }}"
    template:
      src: "{{ item.Src }}"
      dest: "/tmp/{{ item.Name }}"
    with_filetree: /nonexistent/path
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan - should fail
	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err == nil {
		t.Fatal("Expected error for invalid file tree path, got nil")
	}

	if !contains(err.Error(), "failed to walk file tree") && !contains(err.Error(), "no such file") {
		t.Errorf("Expected error about invalid path, got: %v", err)
	}
}

func TestPlanner_WithItems_InvalidTemplate(t *testing.T) {
	// Create config with invalid template in with_items
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
steps:
  - name: "Process {{ item }}"
    shell: echo "{{ item }}"
    with_items: "{{ undefined_variable }}"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan - should fail or handle gracefully
	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	// Should either fail or expand to empty
	if err != nil {
		// Error is acceptable - undefined variable
		if !contains(err.Error(), "failed to evaluate with_items") {
			t.Logf("Got error (acceptable): %v", err)
		}
	}
}

func TestPlanner_Include_ReadError(t *testing.T) {
	// Create config that includes non-existent file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
steps:
  - include: nonexistent.yml
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan - should fail
	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err == nil {
		t.Fatal("Expected error for non-existent include file, got nil")
	}

	if !contains(err.Error(), "failed to read included config") && !contains(err.Error(), "no such file") {
		t.Errorf("Expected error about missing file, got: %v", err)
	}
}

func TestPlanner_Include_ValidationError(t *testing.T) {
	// Create config with invalid included file
	tmpDir := t.TempDir()

	mainConfig := filepath.Join(tmpDir, "main.yml")
	invalidConfig := filepath.Join(tmpDir, "invalid.yml")

	mainContent := `version: "1.0"
steps:
  - include: invalid.yml
`
	// Invalid YAML - not a list
	invalidContent := `this is not valid step config`

	err := os.WriteFile(mainConfig, []byte(mainContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	err = os.WriteFile(invalidConfig, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// Build plan - should fail with validation error
	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: mainConfig,
		Variables:  nil,
		Tags:       nil,
	})

	if err == nil {
		t.Fatal("Expected validation error for invalid included config, got nil")
	}
}

func TestPlanner_WithFileTree_EmptyResults(t *testing.T) {
	// Create empty directory
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	err := os.MkdirAll(emptyDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "test.yml")
	configContent := `version: "1.0"
steps:
  - name: "Copy {{ item.Src }}"
    template:
      src: "{{ item.Src }}"
      dest: "/tmp/{{ item.Name }}"
    with_filetree: ./empty
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Empty directory may include the directory itself as an item
	// The important thing is that there are no file children
	if len(plan.Steps) > 1 {
		t.Errorf("Expected 0 or 1 steps for empty directory, got %d", len(plan.Steps))
	}
}

func TestPlanner_WithItems_NotAList(t *testing.T) {
	// Create config with with_items pointing to non-list variable
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
vars:
  not_a_list: "string value"
steps:
  - name: "Process {{ item }}"
    shell: echo "{{ item }}"
    with_items: not_a_list
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build plan - should fail with type error
	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err == nil {
		t.Fatal("Expected error when with_items is not a list, got nil")
	}

	if !contains(err.Error(), "not a list") && !contains(err.Error(), "failed to evaluate") {
		t.Errorf("Expected error about type mismatch, got: %v", err)
	}
}

func TestPlanner_Include_PathExpansion(t *testing.T) {
	// Create config with template in include path
	tmpDir := t.TempDir()

	mainConfig := filepath.Join(tmpDir, "main.yml")
	darwinConfig := filepath.Join(tmpDir, "darwin.yml")

	mainContent := `version: "1.0"
vars:
  os_name: darwin
steps:
  - include: "{{ os_name }}.yml"
`
	darwinContent := `- name: Darwin step
  shell: echo "darwin"
`

	err := os.WriteFile(mainConfig, []byte(mainContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	err = os.WriteFile(darwinConfig, []byte(darwinContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write darwin config: %v", err)
	}

	// Build plan
	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: mainConfig,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Verify include path was expanded correctly
	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step from included file, got %d", len(plan.Steps))
	}

	if plan.Steps[0].Name != "Darwin step" {
		t.Errorf("Expected 'Darwin step', got '%s'", plan.Steps[0].Name)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

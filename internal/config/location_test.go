package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLocationMap_SetAndGet(t *testing.T) {
	lm := NewLocationMap()

	lm.Set("/0/name", 10, 5)
	lm.Set("/0/shell", 11, 5)
	lm.Set("/1/template/src", 15, 7)

	tests := []struct {
		path string
		want Position
	}{
		{"/0/name", Position{Line: 10, Column: 5}},
		{"/0/shell", Position{Line: 11, Column: 5}},
		{"/1/template/src", Position{Line: 15, Column: 7}},
		{"/nonexistent", Position{Line: 0, Column: 0}}, // Default zero value
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := lm.Get(tt.path)
			if got != tt.want {
				t.Errorf("Get(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLocationMap_GetOrDefault(t *testing.T) {
	lm := NewLocationMap()
	lm.Set("/0/name", 10, 5)

	defaultPos := Position{Line: 100, Column: 100}

	tests := []struct {
		name string
		path string
		want Position
	}{
		{"existing path", "/0/name", Position{Line: 10, Column: 5}},
		{"nonexistent path", "/nonexistent", defaultPos},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lm.GetOrDefault(tt.path, defaultPos)
			if got != tt.want {
				t.Errorf("GetOrDefault(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestBuildLocationMap_SimpleArray(t *testing.T) {
	yamlContent := `- name: step1
  shell: echo hello
- name: step2
  shell: echo world`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	lm := buildLocationMap(&rootNode)

	// Check that we have location information for array elements
	pos0 := lm.Get("/0")
	if pos0.Line == 0 {
		t.Error("Expected location for /0, got zero position")
	}

	pos1 := lm.Get("/1")
	if pos1.Line == 0 {
		t.Error("Expected location for /1, got zero position")
	}

	// Check nested fields
	namePos := lm.Get("/0/name")
	if namePos.Line == 0 {
		t.Error("Expected location for /0/name, got zero position")
	}

	shellPos := lm.Get("/0/shell")
	if shellPos.Line == 0 {
		t.Error("Expected location for /0/shell, got zero position")
	}
}

func TestBuildLocationMap_NestedObject(t *testing.T) {
	yamlContent := `- name: template step
  template:
    src: /path/to/template
    dest: /path/to/dest
    mode: "0644"`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	lm := buildLocationMap(&rootNode)

	// Check nested template fields
	templatePos := lm.Get("/0/template")
	if templatePos.Line == 0 {
		t.Error("Expected location for /0/template, got zero position")
	}

	srcPos := lm.Get("/0/template/src")
	if srcPos.Line == 0 {
		t.Error("Expected location for /0/template/src, got zero position")
	}

	destPos := lm.Get("/0/template/dest")
	if destPos.Line == 0 {
		t.Error("Expected location for /0/template/dest, got zero position")
	}

	modePos := lm.Get("/0/template/mode")
	if modePos.Line == 0 {
		t.Error("Expected location for /0/template/mode, got zero position")
	}
}

func TestFormatArrayPath(t *testing.T) {
	tests := []struct {
		parentPath string
		index      int
		want       string
	}{
		{"", 0, "/0"},
		{"", 1, "/1"},
		{"/steps", 0, "/steps/0"},
		{"/steps/0/items", 2, "/steps/0/items/2"},
	}

	for _, tt := range tests {
		t.Run(tt.parentPath, func(t *testing.T) {
			got := formatArrayPath(tt.parentPath, tt.index)
			if got != tt.want {
				t.Errorf("formatArrayPath(%q, %d) = %q, want %q", tt.parentPath, tt.index, got, tt.want)
			}
		})
	}
}

func TestFormatObjectPath(t *testing.T) {
	tests := []struct {
		parentPath string
		fieldName  string
		want       string
	}{
		{"", "name", "/name"},
		{"", "shell", "/shell"},
		{"/0", "template", "/0/template"},
		{"/0/template", "src", "/0/template/src"},
	}

	for _, tt := range tests {
		t.Run(tt.parentPath+":"+tt.fieldName, func(t *testing.T) {
			got := formatObjectPath(tt.parentPath, tt.fieldName)
			if got != tt.want {
				t.Errorf("formatObjectPath(%q, %q) = %q, want %q", tt.parentPath, tt.fieldName, got, tt.want)
			}
		})
	}
}

func TestEscapeJSONPointer(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with~tilde", "with~0tilde"},
		{"with/slash", "with~1slash"},
		{"~both/chars~", "~0both~1chars~0"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeJSONPointer(tt.input)
			if got != tt.want {
				t.Errorf("escapeJSONPointer(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestBuildLocationMap_RealWorldExample tests with a realistic configuration
func TestBuildLocationMap_RealWorldExample(t *testing.T) {
	yamlContent := `- name: Create directory
  file:
    path: /tmp/test
    state: directory
    mode: "0755"

- name: Run command
  shell: echo "Hello World"
  when: os == "linux"
  tags:
    - setup
    - initial

- name: Render template
  template:
    src: /path/to/template.j2
    dest: /path/to/output.txt`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	lm := buildLocationMap(&rootNode)

	// Verify we can locate various fields
	paths := []string{
		"/0",
		"/0/name",
		"/0/file",
		"/0/file/path",
		"/0/file/state",
		"/0/file/mode",
		"/1",
		"/1/name",
		"/1/shell",
		"/1/when",
		"/1/tags",
		"/1/tags/0",
		"/1/tags/1",
		"/2",
		"/2/template",
		"/2/template/src",
		"/2/template/dest",
	}

	for _, path := range paths {
		pos := lm.Get(path)
		if pos.Line == 0 {
			t.Errorf("Expected location for %s, got zero position", path)
		}
	}
}

// TestBuildLocationMap_LineNumbers verifies that line numbers are reasonable
func TestBuildLocationMap_LineNumbers(t *testing.T) {
	yamlContent := strings.TrimSpace(`
- name: step1
  shell: echo hello

- name: step2
  shell: echo world
`)

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	lm := buildLocationMap(&rootNode)

	// First step should be on or near line 2
	pos0 := lm.Get("/0")
	if pos0.Line < 1 || pos0.Line > 3 {
		t.Errorf("Expected /0 to be around line 2, got line %d", pos0.Line)
	}

	// Second step should be after the first
	pos1 := lm.Get("/1")
	if pos1.Line <= pos0.Line {
		t.Errorf("Expected /1 (line %d) to be after /0 (line %d)", pos1.Line, pos0.Line)
	}
}

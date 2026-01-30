package template

import (
	"testing"
)

func TestPongo2Renderer_Render(t *testing.T) {
	renderer := NewPongo2Renderer()

	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			template: "Hello {{ name }}",
			vars:     map[string]interface{}{"name": "World"},
			want:     "Hello World",
			wantErr:  false,
		},
		{
			name:     "multiple variables",
			template: "{{ greeting }} {{ name }}!",
			vars:     map[string]interface{}{"greeting": "Hi", "name": "Alice"},
			want:     "Hi Alice!",
			wantErr:  false,
		},
		{
			name:     "no variables",
			template: "Static text",
			vars:     map[string]interface{}{},
			want:     "Static text",
			wantErr:  false,
		},
		{
			name:     "nil variables map",
			template: "Static text",
			vars:     nil,
			want:     "Static text",
			wantErr:  false,
		},
		{
			name:     "with loop",
			template: "{% for i in items %}{{ i }}{% endfor %}",
			vars:     map[string]interface{}{"items": []string{"a", "b", "c"}},
			want:     "abc",
			wantErr:  false,
		},
		{
			name:     "with conditional",
			template: "{% if show %}visible{% endif %}",
			vars:     map[string]interface{}{"show": true},
			want:     "visible",
			wantErr:  false,
		},
		{
			name:     "with conditional false",
			template: "{% if show %}visible{% endif %}",
			vars:     map[string]interface{}{"show": false},
			want:     "",
			wantErr:  false,
		},
		{
			name:     "nested variables",
			template: "{{ user.name }}",
			vars:     map[string]interface{}{"user": map[string]interface{}{"name": "Bob"}},
			want:     "Bob",
			wantErr:  false,
		},
		{
			name:     "missing variable",
			template: "{{ missing }}",
			vars:     map[string]interface{}{},
			want:     "",
			wantErr:  false,
		},
		{
			name:     "invalid syntax",
			template: "{{ unclosed",
			vars:     map[string]interface{}{},
			want:     "",
			wantErr:  true,
		},
		{
			name:     "with filter",
			template: "{{ name|upper }}",
			vars:     map[string]interface{}{"name": "test"},
			want:     "TEST",
			wantErr:  false,
		},
		{
			name:     "empty template",
			template: "",
			vars:     map[string]interface{}{},
			want:     "",
			wantErr:  false,
		},
		{
			name:     "path template",
			template: "/tmp/{{ filename }}.txt",
			vars:     map[string]interface{}{"filename": "test"},
			want:     "/tmp/test.txt",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderer.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPongo2Renderer_RenderComplex(t *testing.T) {
	renderer := NewPongo2Renderer()

	template := `
Name: {{ user.name }}
Age: {{ user.age }}
Items:
{% for item in user.items %}
  - {{ item }}
{% endfor %}
`

	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "Alice",
			"age":   30,
			"items": []string{"book", "pen", "laptop"},
		},
	}

	got, err := renderer.Render(template, vars)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Check that output contains expected parts
	expectedParts := []string{"Alice", "30", "book", "pen", "laptop"}
	for _, part := range expectedParts {
		if !contains(got, part) {
			t.Errorf("Render() output missing %q, got:\n%s", part, got)
		}
	}
}

func TestPongo2Renderer_Concurrent(t *testing.T) {
	renderer := NewPongo2Renderer()

	// Test that renderer is safe for concurrent use
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			template := "Hello {{ name }}"
			vars := map[string]interface{}{"name": "World"}
			_, err := renderer.Render(template, vars)
			if err != nil {
				t.Errorf("Concurrent render failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestNewPongo2Renderer(t *testing.T) {
	renderer := NewPongo2Renderer()
	if renderer == nil {
		t.Error("NewPongo2Renderer() returned nil")
	}

	// Verify it returns the interface
	var _ Renderer = renderer
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPongo2Renderer_ExpandUserFilter(t *testing.T) {
	renderer := NewPongo2Renderer()

	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "expanduser filter with path",
			template: "{{ path|expanduser }}",
			vars:     map[string]interface{}{"path": "~/test"},
			wantErr:  false,
		},
		{
			name:     "expanduser filter with regular path",
			template: "{{ path|expanduser }}",
			vars:     map[string]interface{}{"path": "/tmp/test"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderer.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() with expanduser filter error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPongo2Renderer_ErrorHandling(t *testing.T) {
	renderer := NewPongo2Renderer()

	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "unclosed variable",
			template: "{{ unclosed",
			vars:     map[string]interface{}{},
			wantErr:  true,
		},
		{
			name:     "unclosed tag",
			template: "{% if test %}",
			vars:     map[string]interface{}{},
			wantErr:  true,
		},
		{
			name:     "invalid filter",
			template: "{{ var|nonexistent }}",
			vars:     map[string]interface{}{"var": "test"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderer.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

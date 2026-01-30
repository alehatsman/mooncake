package config

import (
	"testing"
)

func TestStep_ValidateOneAction(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{
			name: "single shell action",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
			},
			wantErr: false,
		},
		{
			name: "single file action",
			step: Step{
				Name: "test",
				File: &File{Path: "/tmp/test"},
			},
			wantErr: false,
		},
		{
			name: "single template action",
			step: Step{
				Name:     "test",
				Template: &Template{Src: "src", Dest: "dest"},
			},
			wantErr: false,
		},
		{
			name: "single include action",
			step: Step{
				Name:    "test",
				Include: stringPtr("other.yml"),
			},
			wantErr: false,
		},
		{
			name: "single include_vars action",
			step: Step{
				Name:        "test",
				IncludeVars: stringPtr("vars.yml"),
			},
			wantErr: false,
		},
		{
			name: "single vars action",
			step: Step{
				Name: "test",
				Vars: &map[string]interface{}{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "multiple actions - shell and file",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
				File:  &File{Path: "/tmp/test"},
			},
			wantErr: true,
		},
		{
			name: "multiple actions - template and shell",
			step: Step{
				Name:     "test",
				Template: &Template{Src: "src", Dest: "dest"},
				Shell:    stringPtr("echo hello"),
			},
			wantErr: true,
		},
		{
			name: "multiple actions - include and vars",
			step: Step{
				Name:    "test",
				Include: stringPtr("other.yml"),
				Vars:    &map[string]interface{}{"key": "value"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.ValidateOneAction()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOneAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStep_ValidateHasAction(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{
			name: "has shell action",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
			},
			wantErr: false,
		},
		{
			name: "has file action",
			step: Step{
				Name: "test",
				File: &File{Path: "/tmp/test"},
			},
			wantErr: false,
		},
		{
			name: "no action",
			step: Step{
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "only name",
			step: Step{
				Name: "empty step",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.ValidateHasAction()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHasAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStep_Validate(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{
			name: "valid step with shell",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
			},
			wantErr: false,
		},
		{
			name: "valid step with file",
			step: Step{
				Name: "test",
				File: &File{Path: "/tmp/test", State: "file"},
			},
			wantErr: false,
		},
		{
			name: "invalid - no action",
			step: Step{
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "invalid - multiple actions",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
				File:  &File{Path: "/tmp/test"},
			},
			wantErr: true,
		},
		{
			name: "valid with when condition",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
				When:  "os == 'linux'",
			},
			wantErr: false,
		},
		{
			name: "valid with tags",
			step: Step{
				Name:  "test",
				Shell: stringPtr("echo hello"),
				Tags:  []string{"deploy", "production"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStep_Copy(t *testing.T) {
	original := Step{
		Name:    "test step",
		When:    "os == 'linux'",
		Shell:   stringPtr("echo hello"),
		Become:  true,
		Tags:    []string{"tag1", "tag2"},
		File:    &File{Path: "/tmp/test", State: "file"},
		Include: stringPtr("other.yml"),
		Vars:    &map[string]interface{}{"key": "value"},
	}

	copied := original.Copy()

	// Verify all fields are copied
	if copied.Name != original.Name {
		t.Errorf("Copy() Name = %v, want %v", copied.Name, original.Name)
	}
	if copied.When != original.When {
		t.Errorf("Copy() When = %v, want %v", copied.When, original.When)
	}
	if copied.Become != original.Become {
		t.Errorf("Copy() Become = %v, want %v", copied.Become, original.Become)
	}

	// Verify pointers are the same (shallow copy)
	if copied.Shell != original.Shell {
		t.Error("Copy() Shell pointer should be same")
	}
	if copied.File != original.File {
		t.Error("Copy() File pointer should be same")
	}
	if copied.Include != original.Include {
		t.Error("Copy() Include pointer should be same")
	}
	if copied.Vars != original.Vars {
		t.Error("Copy() Vars pointer should be same")
	}

	// Verify it's a different instance
	if &original == copied {
		t.Error("Copy() should return a different instance")
	}
}

func TestStep_CopyWithModification(t *testing.T) {
	original := Step{
		Name:  "test",
		Shell: stringPtr("echo hello"),
	}

	copied := original.Copy()
	copied.Name = "modified"

	if original.Name == copied.Name {
		t.Error("Modifying copy should not affect original")
	}

	// Verify original is unchanged
	if original.Name != "test" {
		t.Errorf("Original Name changed to %v", original.Name)
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

func TestStep_CountActions(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want int
	}{
		{
			name: "no actions",
			step: Step{Name: "test"},
			want: 0,
		},
		{
			name: "one action - shell",
			step: Step{Name: "test", Shell: strPtr("echo test")},
			want: 1,
		},
		{
			name: "one action - template",
			step: Step{Name: "test", Template: &Template{Src: "src", Dest: "dest"}},
			want: 1,
		},
		{
			name: "one action - file",
			step: Step{Name: "test", File: &File{Path: "/path"}},
			want: 1,
		},
		{
			name: "one action - include",
			step: Step{Name: "test", Include: strPtr("file.yml")},
			want: 1,
		},
		{
			name: "one action - includeVars",
			step: Step{Name: "test", IncludeVars: strPtr("vars.yml")},
			want: 1,
		},
		{
			name: "one action - vars",
			step: Step{Name: "test", Vars: &map[string]interface{}{"key": "value"}},
			want: 1,
		},
		{
			name: "two actions - shell and template",
			step: Step{
				Name:     "test",
				Shell:    strPtr("echo test"),
				Template: &Template{Src: "src", Dest: "dest"},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.step.countActions()
			if got != tt.want {
				t.Errorf("Step.countActions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

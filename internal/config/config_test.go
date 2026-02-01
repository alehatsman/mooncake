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

func TestStep_NewCommonFields(t *testing.T) {
	t.Run("all new fields can be set", func(t *testing.T) {
		step := Step{
			Name:        "test",
			Shell:       strPtr("echo test"),
			BecomeUser:  "postgres",
			Env:         map[string]string{"PATH": "/usr/bin", "HOME": "/home/user"},
			Cwd:         "/tmp",
			Timeout:     "30s",
			Retries:     3,
			RetryDelay:  "5s",
			ChangedWhen: "result.rc == 0",
			FailedWhen:  "result.rc != 0",
		}

		if step.BecomeUser != "postgres" {
			t.Errorf("BecomeUser = %s, want postgres", step.BecomeUser)
		}
		if step.Env["PATH"] != "/usr/bin" {
			t.Errorf("Env[PATH] = %s, want /usr/bin", step.Env["PATH"])
		}
		if step.Cwd != "/tmp" {
			t.Errorf("Cwd = %s, want /tmp", step.Cwd)
		}
		if step.Timeout != "30s" {
			t.Errorf("Timeout = %s, want 30s", step.Timeout)
		}
		if step.Retries != 3 {
			t.Errorf("Retries = %d, want 3", step.Retries)
		}
		if step.RetryDelay != "5s" {
			t.Errorf("RetryDelay = %s, want 5s", step.RetryDelay)
		}
		if step.ChangedWhen != "result.rc == 0" {
			t.Errorf("ChangedWhen = %s, want result.rc == 0", step.ChangedWhen)
		}
		if step.FailedWhen != "result.rc != 0" {
			t.Errorf("FailedWhen = %s, want result.rc != 0", step.FailedWhen)
		}
	})
}

func TestStep_CopyWithNewFields(t *testing.T) {
	original := Step{
		Name:        "test",
		Shell:       strPtr("echo test"),
		BecomeUser:  "postgres",
		Env:         map[string]string{"PATH": "/usr/bin"},
		Cwd:         "/tmp",
		Timeout:     "30s",
		Retries:     3,
		RetryDelay:  "5s",
		ChangedWhen: "result.rc == 0",
		FailedWhen:  "result.rc != 0",
	}

	copied := original.Copy()

	// Verify all new fields are copied
	if copied.BecomeUser != original.BecomeUser {
		t.Errorf("Copy() BecomeUser = %s, want %s", copied.BecomeUser, original.BecomeUser)
	}
	if copied.Env["PATH"] != original.Env["PATH"] {
		t.Errorf("Copy() Env not equal")
	}
	if copied.Cwd != original.Cwd {
		t.Errorf("Copy() Cwd = %s, want %s", copied.Cwd, original.Cwd)
	}
	if copied.Timeout != original.Timeout {
		t.Errorf("Copy() Timeout = %s, want %s", copied.Timeout, original.Timeout)
	}
	if copied.Retries != original.Retries {
		t.Errorf("Copy() Retries = %d, want %d", copied.Retries, original.Retries)
	}
	if copied.RetryDelay != original.RetryDelay {
		t.Errorf("Copy() RetryDelay = %s, want %s", copied.RetryDelay, original.RetryDelay)
	}
	if copied.ChangedWhen != original.ChangedWhen {
		t.Errorf("Copy() ChangedWhen = %s, want %s", copied.ChangedWhen, original.ChangedWhen)
	}
	if copied.FailedWhen != original.FailedWhen {
		t.Errorf("Copy() FailedWhen = %s, want %s", copied.FailedWhen, original.FailedWhen)
	}

	// Verify it's a shallow copy (map references are shared)
	// This is intentional behavior as documented in Copy()
	copied.Env["NEW"] = "value"
	if _, exists := original.Env["NEW"]; !exists {
		t.Error("Copy() is shallow copy, so map modifications should be visible in original")
	}
}

func TestRunConfig(t *testing.T) {
	t.Run("create RunConfig with all fields", func(t *testing.T) {
		rc := RunConfig{
			Version: "1.0",
			Vars: map[string]interface{}{
				"app_name": "myapp",
				"port":     8080,
			},
			Steps: []Step{
				{
					Name:  "step1",
					Shell: strPtr("echo test"),
				},
			},
		}

		if rc.Version != "1.0" {
			t.Errorf("Version = %s, want 1.0", rc.Version)
		}
		if rc.Vars["app_name"] != "myapp" {
			t.Errorf("Vars[app_name] = %v, want myapp", rc.Vars["app_name"])
		}
		if len(rc.Steps) != 1 {
			t.Errorf("len(Steps) = %d, want 1", len(rc.Steps))
		}
	})
}

package config

import (
	"testing"

	"gopkg.in/yaml.v3"
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
				Shell: shellActionPtr("echo hello"),
			},
			wantErr: false,
		},
		{
			name: "single file action",
			step: Step{
				Name: "test",
				FileWrite: &File{Path: "/tmp/test"},
			},
			wantErr: false,
		},
		{
			name: "single template action",
			step: Step{
				Name:     "test",
				FileTemplate: &Template{Src: "src", Dest: "dest"},
			},
			wantErr: false,
		},
		{
			name: "single include action",
			step: Step{
				Name:    "test",
				Import: stringPtr("other.yml"),
			},
			wantErr: false,
		},
		{
			name: "single include_vars action",
			step: Step{
				Name:        "test",
				VarsLoad: stringPtr("vars.yml"),
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
				Shell: shellActionPtr("echo hello"),
				FileWrite:  &File{Path: "/tmp/test"},
			},
			wantErr: true,
		},
		{
			name: "multiple actions - template and shell",
			step: Step{
				Name:     "test",
				FileTemplate: &Template{Src: "src", Dest: "dest"},
				Shell:    shellActionPtr("echo hello"),
			},
			wantErr: true,
		},
		{
			name: "multiple actions - include and vars",
			step: Step{
				Name:    "test",
				Import: stringPtr("other.yml"),
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
				Shell: shellActionPtr("echo hello"),
			},
			wantErr: false,
		},
		{
			name: "has file action",
			step: Step{
				Name: "test",
				FileWrite: &File{Path: "/tmp/test"},
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
				Shell: shellActionPtr("echo hello"),
			},
			wantErr: false,
		},
		{
			name: "valid step with file",
			step: Step{
				Name: "test",
				FileWrite: &File{Path: "/tmp/test", State: "file"},
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
				Shell: shellActionPtr("echo hello"),
				FileWrite:  &File{Path: "/tmp/test"},
			},
			wantErr: true,
		},
		{
			name: "valid with when condition",
			step: Step{
				Name:  "test",
				Shell: shellActionPtr("echo hello"),
				When:  "os == 'linux'",
			},
			wantErr: false,
		},
		{
			name: "valid with tags",
			step: Step{
				Name:  "test",
				Shell: shellActionPtr("echo hello"),
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
		Shell:   shellActionPtr("echo hello"),
		AsUser: "root",
		Tags:    []string{"tag1", "tag2"},
		FileWrite:    &File{Path: "/tmp/test", State: "file"},
		Import: stringPtr("other.yml"),
		Vars:    &map[string]interface{}{"key": "value"},
	}

	copied := original.Clone()

	// Verify all fields are copied
	if copied.Name != original.Name {
		t.Errorf("Copy() Name = %v, want %v", copied.Name, original.Name)
	}
	if copied.When != original.When {
		t.Errorf("Copy() When = %v, want %v", copied.When, original.When)
	}
	if copied.AsUser != original.AsUser {
		t.Errorf("Copy() AsUser = %v, want %v", copied.AsUser, original.AsUser)
	}

	// Verify pointers are the same (shallow copy)
	if copied.Shell != original.Shell {
		t.Error("Copy() Shell pointer should be same")
	}
	if copied.FileWrite != original.FileWrite {
		t.Error("Copy() File pointer should be same")
	}
	if copied.Import != original.Import {
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
		Shell: shellActionPtr("echo hello"),
	}

	copied := original.Clone()
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

func shellActionPtr(cmd string) *ShellAction {
	return &ShellAction{Cmd: cmd}
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
			step: Step{Name: "test", Shell: shellActionPtr("echo test")},
			want: 1,
		},
		{
			name: "one action - template",
			step: Step{Name: "test", FileTemplate: &Template{Src: "src", Dest: "dest"}},
			want: 1,
		},
		{
			name: "one action - file",
			step: Step{Name: "test", FileWrite: &File{Path: "/path"}},
			want: 1,
		},
		{
			name: "one action - include",
			step: Step{Name: "test", Import: strPtr("file.yml")},
			want: 1,
		},
		{
			name: "one action - includeVars",
			step: Step{Name: "test", VarsLoad: strPtr("vars.yml")},
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
				Shell:    shellActionPtr("echo test"),
				FileTemplate: &Template{Src: "src", Dest: "dest"},
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
			Shell:       shellActionPtr("echo test"),
			AsUser:  "postgres",
			Env:         map[string]string{"PATH": "/usr/bin", "HOME": "/home/user"},
			Cwd:         "/tmp",
			Timeout:     "30s",
			Retry: &RetryPolicy{Attempts: 3, Delay: "5s"},
			ChangedWhen: "result.rc == 0",
			FailedWhen:  "result.rc != 0",
		}

		if step.AsUser != "postgres" {
			t.Errorf("AsUser = %s, want postgres", step.AsUser)
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
		if step.RetryAttempts() != 3 {
			t.Errorf("Retries = %d, want 3", step.RetryAttempts())
		}
		if step.RetryDelayDuration() != "5s" {
			t.Errorf("RetryDelay = %s, want 5s", step.RetryDelayDuration())
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
		Shell:       shellActionPtr("echo test"),
		AsUser:  "postgres",
		Env:         map[string]string{"PATH": "/usr/bin"},
		Cwd:         "/tmp",
		Timeout:     "30s",
		Retry: &RetryPolicy{Attempts: 3, Delay: "5s"},
		ChangedWhen: "result.rc == 0",
		FailedWhen:  "result.rc != 0",
	}

	copied := original.Clone()

	// Verify all new fields are copied
	if copied.AsUser != original.AsUser {
		t.Errorf("Copy() AsUser = %s, want %s", copied.AsUser, original.AsUser)
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
	if copied.RetryAttempts() != original.RetryAttempts() {
		t.Errorf("Copy() Retry.Attempts = %d, want %d", copied.RetryAttempts(), original.RetryAttempts())
	}
	if copied.RetryDelayDuration() != original.RetryDelayDuration() {
		t.Errorf("Copy() Retry.Delay = %s, want %s", copied.RetryDelayDuration(), original.RetryDelayDuration())
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
					Shell: shellActionPtr("echo test"),
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

func TestShellAction_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  ShellAction
	}{
		{
			name:  "string form",
			input: `"echo hello"`,
			want:  ShellAction{Cmd: "echo hello"},
		},
		{
			name:  "object form with cmd only",
			input: `{cmd: "echo hello"}`,
			want:  ShellAction{Cmd: "echo hello"},
		},
		{
			name:  "object form with interpreter",
			input: `{cmd: "echo hello", interpreter: "/bin/bash"}`,
			want:  ShellAction{Cmd: "echo hello", Interpreter: "/bin/bash"},
		},
		{
			name:  "multiline string",
			input: `"echo line1\necho line2"`,
			want:  ShellAction{Cmd: "echo line1\necho line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ShellAction
			err := yaml.Unmarshal([]byte(tt.input), &got)
			if err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}
			if got.Cmd != tt.want.Cmd {
				t.Errorf("Cmd = %v, want %v", got.Cmd, tt.want.Cmd)
			}
			if got.Interpreter != tt.want.Interpreter {
				t.Errorf("Interpreter = %v, want %v", got.Interpreter, tt.want.Interpreter)
			}
		})
	}
}

func TestPresetInvocation_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  PresetInvocation
	}{
		{
			name:  "string form",
			input: `"ollama"`,
			want:  PresetInvocation{Name: "ollama"},
		},
		{
			name:  "object form with name only",
			input: `{name: "ollama"}`,
			want:  PresetInvocation{Name: "ollama"},
		},
		{
			name:  "object form with parameters",
			input: `{name: "ollama", with: {state: "present"}}`,
			want: PresetInvocation{
				Name: "ollama",
				With: map[string]interface{}{"state": "present"},
			},
		},
		{
			name:  "object form with multiple parameters",
			input: `{name: "deploy-webapp", with: {port: 8080, domain: "example.com"}}`,
			want: PresetInvocation{
				Name: "deploy-webapp",
				With: map[string]interface{}{"port": 8080, "domain": "example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PresetInvocation
			err := yaml.Unmarshal([]byte(tt.input), &got)
			if err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if tt.want.With != nil {
				if got.With == nil {
					t.Error("With should not be nil")
				} else {
					for k, v := range tt.want.With {
						if got.With[k] != v {
							t.Errorf("With[%s] = %v, want %v", k, got.With[k], v)
						}
					}
				}
			}
		})
	}
}

func TestPrintAction_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  PrintAction
	}{
		{
			name:  "string form",
			input: `"Hello World"`,
			want:  PrintAction{Msg: "Hello World"},
		},
		{
			name:  "object form",
			input: `{msg: "Hello World"}`,
			want:  PrintAction{Msg: "Hello World"},
		},
		{
			name:  "template string",
			input: `"Hello {{ name }}"`,
			want:  PrintAction{Msg: "Hello {{ name }}"},
		},
		{
			name:  "multiline string",
			input: `"Line 1\nLine 2\nLine 3"`,
			want:  PrintAction{Msg: "Line 1\nLine 2\nLine 3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PrintAction
			err := yaml.Unmarshal([]byte(tt.input), &got)
			if err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}
			if got.Msg != tt.want.Msg {
				t.Errorf("Msg = %v, want %v", got.Msg, tt.want.Msg)
			}
		})
	}
}

func TestStep_DetermineActionType(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want string
	}{
		{
			name: "shell action",
			step: Step{Shell: shellActionPtr("echo hello")},
			want: "shell",
		},
		{
			name: "cmd action",
			step: Step{Cmd: &CommandAction{Argv: []string{"ls", "-la"}}},
			want: "cmd",
		},
		{
			name: "file.write action",
			step: Step{FileWrite: &File{Path: "/tmp/test"}},
			want: "file.write",
		},
		{
			name: "file.template action",
			step: Step{FileTemplate: &Template{Src: "src", Dest: "dest"}},
			want: "file.template",
		},
		{
			name: "file.copy action",
			step: Step{FileCopy: &Copy{Src: "src", Dest: "dest"}},
			want: "file.copy",
		},
		{
			name: "file.unarchive action",
			step: Step{FileUnarchive: &Unarchive{Src: "file.tar.gz", Dest: "/tmp"}},
			want: "file.unarchive",
		},
		{
			name: "file.download action",
			step: Step{FileDownload: &Download{URL: "http://example.com", Dest: "/tmp/file"}},
			want: "file.download",
		},
		{
			name: "pkg action",
			step: Step{Pkg: &Package{Name: "nginx", State: "present"}},
			want: "pkg",
		},
		{
			name: "os.service action",
			step: Step{OsService: &ServiceAction{Name: "nginx", State: "started"}},
			want: "os.service",
		},
		{
			name: "assert action",
			step: Step{Assert: &Assert{Command: &AssertCommand{Cmd: "test -f /tmp/file"}}},
			want: "assert",
		},
		{
			name: "use (preset) action",
			step: Step{Use: &PresetInvocation{Name: "ollama"}},
			want: "use",
		},
		{
			name: "log (print) action",
			step: Step{Log: &PrintAction{Msg: "Hello"}},
			want: "log",
		},
		{
			name: "vars action",
			step: Step{Vars: &map[string]interface{}{"key": "value"}},
			want: "vars",
		},
		{
			name: "vars.load action",
			step: Step{VarsLoad: stringPtr("vars.yml")},
			want: "vars.load",
		},
		{
			name: "import action",
			step: Step{Import: stringPtr("other.yml")},
			want: "import",
		},
		{
			name: "loop for_each",
			step: Step{ForEach: &ForEachField{Expr: "['item1', 'item2']"}},
			want: "loop",
		},
		{
			name: "loop for_each_file",
			step: Step{ForEachFile: stringPtr("/tmp")},
			want: "loop",
		},
		{
			name: "no action",
			step: Step{Name: "empty"},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.step.DetermineActionType()
			if got != tt.want {
				t.Errorf("DetermineActionType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStep_DetermineActionType_AllFields verifies that every action field
// tagged with `action:` returns the correct type string and that countActions
// returns 1 for each. This is the compile-time-checked table of all 28 actions.
func TestStep_DetermineActionType_AllFields(t *testing.T) {
	strEmpty := ""
	emptyVars := map[string]interface{}{}

	cases := []struct {
		key  string
		step Step
	}{
		{"file.write", Step{FileWrite: &File{Path: "/tmp/x"}}},
		{"file.template", Step{FileTemplate: &Template{Src: "s", Dest: "d"}}},
		{"file.copy", Step{FileCopy: &Copy{Src: "s", Dest: "d"}}},
		{"file.download", Step{FileDownload: &Download{URL: "http://x", Dest: "/tmp/x"}}},
		{"file.unarchive", Step{FileUnarchive: &Unarchive{Src: "x.tar.gz", Dest: "/tmp"}}},
		{"text.replace", Step{TextReplace: &FileReplace{Path: "/f", Pattern: "a", Replace: "b"}}},
		{"text.insert", Step{TextInsert: &FileInsert{Path: "/f", Anchor: "x", Position: "after", Content: "y"}}},
		{"text.delete_range", Step{TextDeleteRange: &FileDeleteRange{Path: "/f", StartAnchor: "a", EndAnchor: "b"}}},
		{"text.patch", Step{TextPatch: &FilePatchApply{Path: "/f", Patch: "diff"}}},
		{"pkg", Step{Pkg: &Package{Name: "nginx"}}},
		{"tool", Step{Tool: &Tool{Name: "jq", Version: "1.7", Backend: "github-release"}}},
		{"os.service", Step{OsService: &ServiceAction{Name: "nginx"}}},
		{"container.image", Step{ContainerImage: &ContainerImage{Name: "ubuntu"}}},
		{"container", Step{Container: &Container{Image: "ubuntu"}}},
		{"cmd", Step{Cmd: &CommandAction{Argv: []string{"ls"}}}},
		{"repo.search", Step{RepoSearch: &RepoSearch{Pattern: "foo"}}},
		{"repo.tree", Step{RepoTree: &RepoTree{}}},
		{"repo.patch", Step{RepoPatch: &RepoApplyPatchset{Patchset: "diff"}}},
		{"artifact.capture", Step{ArtifactCapture: &ArtifactCapture{Name: "cap"}}},
		{"artifact.validate", Step{ArtifactValidate: &ArtifactValidate{ArtifactFile: "/f"}}},
		{"shell", Step{Shell: shellActionPtr("echo")}},
		{"assert", Step{Assert: &Assert{Command: &AssertCommand{Cmd: "true"}}}},
		{"wait.port", Step{WaitPort: &WaitPort{Port: 5432}}},
		{"wait.http", Step{WaitHTTP: &WaitHTTP{URL: "http://localhost/health"}}},
		{"wait.file", Step{WaitFile: &WaitFile{Path: "/tmp/ready"}}},
		{"wait.command", Step{WaitCommand: &WaitCommand{Cmd: "true"}}},
		{"log", Step{Log: &PrintAction{Msg: "hi"}}},
		{"use", Step{Use: &PresetInvocation{Name: "p"}}},
		{"import", Step{Import: &strEmpty}},
		{"vars.load", Step{VarsLoad: &strEmpty}},
		{"vars", Step{Vars: &emptyVars}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
			if got := c.step.DetermineActionType(); got != c.key {
				t.Errorf("DetermineActionType() = %q, want %q", got, c.key)
			}
			if got := c.step.countActions(); got != 1 {
				t.Errorf("countActions() = %d for %q, want 1", got, c.key)
			}
		})
	}
}

package executor

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestHandleOllama_Validation tests parameter validation
func TestHandleOllama_Validation(t *testing.T) {
	tests := []struct {
		name        string
		action      *config.OllamaAction
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil action",
			action:      nil,
			wantErr:     true,
			errContains: "ollama action is nil",
		},
		{
			name: "invalid state",
			action: &config.OllamaAction{
				State: "invalid",
			},
			wantErr:     true,
			errContains: "invalid state",
		},
		{
			name: "invalid method",
			action: &config.OllamaAction{
				State:  "present",
				Method: "invalid",
			},
			wantErr:     true,
			errContains: "invalid method",
		},
		{
			name: "valid present",
			action: &config.OllamaAction{
				State: "present",
			},
			wantErr: false,
		},
		{
			name: "valid absent",
			action: &config.OllamaAction{
				State: "absent",
			},
			wantErr: false,
		},
		{
			name: "valid with all options",
			action: &config.OllamaAction{
				State:     "present",
				Method:    "auto",
				Host:      "localhost:11434",
				ModelsDir: "/data/ollama",
				Pull:      []string{"llama3.1:8b"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name:   "test",
				Ollama: tt.action,
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true // Use dry-run to avoid actual operations

			err := HandleOllama(step, ec)

			if tt.wantErr {
				if err == nil {
					t.Errorf("HandleOllama() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("HandleOllama() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("HandleOllama() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestHandleOllama_DryRun tests dry-run mode
func TestHandleOllama_DryRun(t *testing.T) {
	tests := []struct {
		name   string
		action *config.OllamaAction
	}{
		{
			name: "install",
			action: &config.OllamaAction{
				State: "present",
			},
		},
		{
			name: "install with service",
			action: &config.OllamaAction{
				State:   "present",
				Service: boolPtr(true),
			},
		},
		{
			name: "install with models",
			action: &config.OllamaAction{
				State: "present",
				Pull:  []string{"llama3.1:8b", "mistral"},
			},
		},
		{
			name: "uninstall",
			action: &config.OllamaAction{
				State: "absent",
			},
		},
		{
			name: "uninstall with force",
			action: &config.OllamaAction{
				State: "absent",
				Force: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name:   "test",
				Ollama: tt.action,
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)
			if err != nil {
				t.Errorf("HandleOllama() in dry-run mode unexpected error = %v", err)
			}

			// Verify result exists
			if ec.CurrentResult == nil {
				t.Error("HandleOllama() did not set CurrentResult")
			}
		})
	}
}

// TestHandleOllama_ResultRegistration tests that results are properly registered
func TestHandleOllama_ResultRegistration(t *testing.T) {
	step := config.Step{
		Name: "test",
		Ollama: &config.OllamaAction{
			State: "present",
		},
		Register: "ollama_result",
	}

	ec := newTestExecutionContext(t)
	ec.DryRun = true

	err := HandleOllama(step, ec)
	if err != nil {
		t.Fatalf("HandleOllama() unexpected error = %v", err)
	}

	// Check if result was registered
	if _, ok := ec.Variables["ollama_result"]; !ok {
		t.Error("HandleOllama() did not register result to variables")
	}
}

// TestHandleOllama_States tests different state transitions
func TestHandleOllama_States(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		wantErr bool
	}{
		{
			name:    "present",
			state:   "present",
			wantErr: false,
		},
		{
			name:    "absent",
			state:   "absent",
			wantErr: false,
		},
		{
			name:    "invalid state",
			state:   "running",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test",
				Ollama: &config.OllamaAction{
					State: tt.state,
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)

			if tt.wantErr && err == nil {
				t.Error("HandleOllama() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_Methods tests different installation methods
func TestHandleOllama_Methods(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		wantErr bool
	}{
		{
			name:    "auto method",
			method:  "auto",
			wantErr: false,
		},
		{
			name:    "script method",
			method:  "script",
			wantErr: false,
		},
		{
			name:    "package method",
			method:  "package",
			wantErr: false,
		},
		{
			name:    "empty method (defaults to auto)",
			method:  "",
			wantErr: false,
		},
		{
			name:    "invalid method",
			method:  "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test",
				Ollama: &config.OllamaAction{
					State:  "present",
					Method: tt.method,
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)

			if tt.wantErr && err == nil {
				t.Error("HandleOllama() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_ServiceFlag tests service configuration flag
func TestHandleOllama_ServiceFlag(t *testing.T) {
	tests := []struct {
		name    string
		service *bool
	}{
		{
			name:    "service enabled",
			service: boolPtr(true),
		},
		{
			name:    "service disabled",
			service: boolPtr(false),
		},
		{
			name:    "service not specified",
			service: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test",
				Ollama: &config.OllamaAction{
					State:   "present",
					Service: tt.service,
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)
			if err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_ModelPull tests model pulling
func TestHandleOllama_ModelPull(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		force  bool
	}{
		{
			name:   "single model",
			models: []string{"llama3.1:8b"},
			force:  false,
		},
		{
			name:   "multiple models",
			models: []string{"llama3.1:8b", "mistral", "codellama:7b"},
			force:  false,
		},
		{
			name:   "with force flag",
			models: []string{"llama3.1:8b"},
			force:  true,
		},
		{
			name:   "no models",
			models: []string{},
			force:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test",
				Ollama: &config.OllamaAction{
					State: "present",
					Pull:  tt.models,
					Force: tt.force,
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)
			if err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_CustomConfiguration tests custom configuration options
func TestHandleOllama_CustomConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		modelsDir string
		env       map[string]string
	}{
		{
			name:      "custom host",
			host:      "0.0.0.0:11434",
			modelsDir: "",
			env:       nil,
		},
		{
			name:      "custom models directory",
			host:      "",
			modelsDir: "/data/ollama/models",
			env:       nil,
		},
		{
			name:      "custom environment variables",
			host:      "",
			modelsDir: "",
			env: map[string]string{
				"OLLAMA_DEBUG":   "1",
				"OLLAMA_ORIGINS": "*",
			},
		},
		{
			name:      "all custom options",
			host:      "localhost:8080",
			modelsDir: "/custom/models",
			env: map[string]string{
				"OLLAMA_DEBUG": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test",
				Ollama: &config.OllamaAction{
					State:     "present",
					Host:      tt.host,
					ModelsDir: tt.modelsDir,
					Env:       tt.env,
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)
			if err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_UninstallWithForce tests uninstall with force flag
func TestHandleOllama_UninstallWithForce(t *testing.T) {
	tests := []struct {
		name      string
		force     bool
		modelsDir string
	}{
		{
			name:      "uninstall without force",
			force:     false,
			modelsDir: "",
		},
		{
			name:      "uninstall with force",
			force:     true,
			modelsDir: "",
		},
		{
			name:      "uninstall with force and custom models dir",
			force:     true,
			modelsDir: "/custom/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "test",
				Ollama: &config.OllamaAction{
					State:     "absent",
					Force:     tt.force,
					ModelsDir: tt.modelsDir,
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true

			err := HandleOllama(step, ec)
			if err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_WithBecome tests sudo/become functionality
func TestHandleOllama_WithBecome(t *testing.T) {
	tests := []struct {
		name   string
		become bool
	}{
		{
			name:   "with become",
			become: true,
		},
		{
			name:   "without become",
			become: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name:   "test",
				Become: tt.become,
				Ollama: &config.OllamaAction{
					State: "present",
				},
			}

			ec := newTestExecutionContext(t)
			ec.DryRun = true
			if tt.become {
				ec.SudoPass = "test-password"
			}

			err := HandleOllama(step, ec)
			if err != nil {
				t.Errorf("HandleOllama() unexpected error = %v", err)
			}
		})
	}
}

// TestHandleOllama_CompleteWorkflow tests a complete installation workflow
func TestHandleOllama_CompleteWorkflow(t *testing.T) {
	step := config.Step{
		Name:   "Install Ollama with full configuration",
		Become: true,
		Ollama: &config.OllamaAction{
			State:     "present",
			Service:   boolPtr(true),
			Method:    "auto",
			Host:      "0.0.0.0:11434",
			ModelsDir: "/data/ollama",
			Pull:      []string{"llama3.1:8b", "mistral"},
			Env: map[string]string{
				"OLLAMA_DEBUG": "1",
			},
		},
		Register: "ollama_install",
	}

	ec := newTestExecutionContext(t)
	ec.DryRun = true
	ec.SudoPass = "test-password"

	err := HandleOllama(step, ec)
	if err != nil {
		t.Fatalf("HandleOllama() unexpected error = %v", err)
	}

	// Verify result was registered
	if _, ok := ec.Variables["ollama_install"]; !ok {
		t.Error("HandleOllama() did not register result")
	}

	// Verify result has expected structure
	result := ec.CurrentResult
	if result == nil {
		t.Fatal("HandleOllama() did not set CurrentResult")
	}

	if result.StartTime.IsZero() {
		t.Error("HandleOllama() result missing StartTime")
	}
	if result.EndTime.IsZero() {
		t.Error("HandleOllama() result missing EndTime")
	}
	if result.Duration == 0 {
		t.Error("HandleOllama() result has zero Duration")
	}
}

// TestHandleOllama_Idempotency tests idempotent behavior
func TestHandleOllama_Idempotency(t *testing.T) {
	// This test verifies that running the same action twice in dry-run
	// doesn't cause errors (actual idempotency would require real installation)
	step := config.Step{
		Name: "test",
		Ollama: &config.OllamaAction{
			State: "present",
			Pull:  []string{"llama3.1:8b"},
		},
	}

	ec := newTestExecutionContext(t)
	ec.DryRun = true

	// First run
	err := HandleOllama(step, ec)
	if err != nil {
		t.Fatalf("HandleOllama() first run unexpected error = %v", err)
	}

	// Second run (simulating idempotency)
	err = HandleOllama(step, ec)
	if err != nil {
		t.Errorf("HandleOllama() second run unexpected error = %v", err)
	}
}

// TestContains tests the contains helper function
func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		val   string
		want  bool
	}{
		{
			name:  "contains",
			slice: []string{"a", "b", "c"},
			val:   "b",
			want:  true,
		},
		{
			name:  "does not contain",
			slice: []string{"a", "b", "c"},
			val:   "d",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			val:   "a",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.val)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Note: Helper functions boolPtr and newTestExecutionContext are defined in other test files

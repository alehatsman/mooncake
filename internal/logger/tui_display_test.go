package logger

import (
	"strings"
	"testing"
	"time"
)

func TestNewTUIDisplay(t *testing.T) {
	animator, err := LoadEmbeddedFrames()
	if err != nil {
		t.Fatalf("LoadEmbeddedFrames error: %v", err)
	}

	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	if display == nil {
		t.Fatal("NewTUIDisplay returned nil")
	}
	if display.width != 80 {
		t.Errorf("width = %d, want 80", display.width)
	}
	if display.height != 24 {
		t.Errorf("height = %d, want 24", display.height)
	}
}

func TestTUIDisplay_Render(t *testing.T) {
	animator, err := LoadEmbeddedFrames()
	if err != nil {
		t.Fatalf("LoadEmbeddedFrames error: %v", err)
	}

	buffer := NewTUIBuffer(10)
	buffer.SetCurrentStep("Installing nginx", ProgressInfo{Current: 5, Total: 10})

	display := NewTUIDisplay(animator, buffer, 80, 24)
	output := display.Render()

	if !strings.Contains(output, "Installing nginx") {
		t.Error("Render() should contain current step")
	}

	// Should contain clear screen and home position
	if !strings.Contains(output, "\033[2J\033[H") {
		t.Error("Render() should contain clear screen codes")
	}

	// Should contain mooncake text
	if !strings.Contains(output, "Mooncake") {
		t.Error("Render() should contain Mooncake text")
	}
}

func TestTUIDisplay_RenderHeader(t *testing.T) {
	animator, err := LoadEmbeddedFrames()
	if err != nil {
		t.Fatalf("LoadEmbeddedFrames error: %v", err)
	}

	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	header := display.renderHeader()

	if !strings.Contains(header, "Mooncake Provisioning Tool") {
		t.Error("renderHeader() should contain 'Mooncake Provisioning Tool'")
	}

	// Should have multiple lines (animation frame)
	lines := strings.Split(header, "\n")
	if len(lines) < 3 {
		t.Errorf("renderHeader() should have at least 3 lines, got %d", len(lines))
	}
}

func TestTUIDisplay_RenderSeparator(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	separator := display.renderSeparator()

	// Should contain the separator character
	if !strings.Contains(separator, "─") {
		t.Error("renderSeparator() should contain '─' character")
	}

	// Count visible characters (80 separator chars)
	visibleCount := strings.Count(separator, "─")
	if visibleCount != 80 {
		t.Errorf("renderSeparator() visible chars = %d, want 80", visibleCount)
	}
}

func TestTUIDisplay_RenderCurrentStep(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name          string
		currentStep   string
		progress      ProgressInfo
		wantContains  []string
	}{
		{
			name:        "with current step and progress",
			currentStep: "Installing packages",
			progress:    ProgressInfo{Current: 3, Total: 10},
			wantContains: []string{
				"Current: Installing packages",
				"Progress:",
				"3/10",
			},
		},
		{
			name:        "with current step, no progress",
			currentStep: "Running tests",
			progress:    ProgressInfo{Current: 0, Total: 0},
			wantContains: []string{
				"Current: Running tests",
			},
		},
		{
			name:        "without current step",
			currentStep: "",
			progress:    ProgressInfo{Current: 0, Total: 0},
			wantContains: []string{
				"Current: Initializing...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer.SetCurrentStep(tt.currentStep, tt.progress)
			snapshot := buffer.GetSnapshot()

			output := display.renderCurrentStep(snapshot)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("renderCurrentStep() should contain %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestTUIDisplay_RenderProgressBar(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name         string
		current      int
		total        int
		wantContains []string
	}{
		{
			name:    "50% progress",
			current: 5,
			total:   10,
			wantContains: []string{
				"Progress:",
				"50%",
				"5/10",
				"█",
				"░",
			},
		},
		{
			name:    "0% progress",
			current: 0,
			total:   10,
			wantContains: []string{
				"Progress:",
				"0%",
				"0/10",
			},
		},
		{
			name:    "100% progress",
			current: 10,
			total:   10,
			wantContains: []string{
				"Progress:",
				"100%",
				"10/10",
			},
		},
		{
			name:    "no total (cumulative)",
			current: 5,
			total:   0,
			wantContains: []string{
				"Progress: 5 steps completed",
			},
		},
		{
			name:    "over 100%",
			current: 15,
			total:   10,
			wantContains: []string{
				"Progress:",
				"15/10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := display.renderProgressBar(tt.current, tt.total)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("renderProgressBar() should contain %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestTUIDisplay_RenderHistory(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(15)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name         string
		steps        []StepEntry
		wantContains []string
		wantCount    int
	}{
		{
			name:  "empty history",
			steps: []StepEntry{},
			wantContains: []string{
				"Recent Steps:",
				"No steps completed yet",
			},
		},
		{
			name: "single step",
			steps: []StepEntry{
				{Name: "Install nginx", Status: StatusSuccess, Level: 0},
			},
			wantContains: []string{
				"Recent Steps:",
				"Install nginx",
				"✓",
			},
		},
		{
			name: "multiple steps with different statuses",
			steps: []StepEntry{
				{Name: "Step 1", Status: StatusSuccess, Level: 0},
				{Name: "Step 2", Status: StatusError, Level: 1},
				{Name: "Step 3", Status: StatusSkipped, Level: 0},
				{Name: "Step 4", Status: StatusRunning, Level: 2},
			},
			wantContains: []string{
				"Recent Steps:",
				"Step 1",
				"Step 2",
				"Step 3",
				"Step 4",
			},
		},
		{
			name: "more than 10 steps (should limit)",
			steps: func() []StepEntry {
				steps := make([]StepEntry, 15)
				for i := 0; i < 15; i++ {
					steps[i] = StepEntry{
						Name:   "Step",
						Status: StatusSuccess,
						Level:  0,
					}
				}
				return steps
			}(),
			wantCount: 10, // Should only show last 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset buffer
			buffer = NewTUIBuffer(15)
			display.buffer = buffer

			// Add steps
			for _, step := range tt.steps {
				buffer.AddStep(step)
			}

			snapshot := buffer.GetSnapshot()
			output := display.renderHistory(snapshot)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("renderHistory() should contain %q\nGot: %s", want, output)
				}
			}

			if tt.wantCount > 0 {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				// Subtract header line
				actualCount := len(lines) - 1
				if actualCount > tt.wantCount {
					t.Errorf("renderHistory() should show max %d steps, got %d", tt.wantCount, actualCount)
				}
			}
		})
	}
}

func TestTUIDisplay_RenderMessages(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name         string
		debugMsgs    []string
		errorMsgs    []string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "no messages",
			debugMsgs: []string{},
			errorMsgs: []string{},
			wantEmpty: true,
		},
		{
			name:      "debug messages only",
			debugMsgs: []string{"Debug info 1", "Debug info 2"},
			errorMsgs: []string{},
			wantContains: []string{
				"Messages:",
				"[DEBUG]",
				"Debug info 1",
				"Debug info 2",
			},
		},
		{
			name:      "error messages only",
			debugMsgs: []string{},
			errorMsgs: []string{"Error occurred", "Another error"},
			wantContains: []string{
				"Messages:",
				"[ERROR]",
				"Error occurred",
				"Another error",
			},
		},
		{
			name:      "both debug and error messages",
			debugMsgs: []string{"Debug message"},
			errorMsgs: []string{"Error message"},
			wantContains: []string{
				"Messages:",
				"[DEBUG]",
				"Debug message",
				"[ERROR]",
				"Error message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset buffer
			buffer = NewTUIBuffer(10)
			display.buffer = buffer

			// Add messages
			for _, msg := range tt.debugMsgs {
				buffer.AddDebug(msg)
			}
			for _, msg := range tt.errorMsgs {
				buffer.AddError(msg)
			}

			snapshot := buffer.GetSnapshot()
			output := display.renderMessages(snapshot)

			if tt.wantEmpty {
				if output != "" {
					t.Errorf("renderMessages() should be empty, got: %s", output)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("renderMessages() should contain %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestTUIDisplay_GetStatusIndicator(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"success status", StatusSuccess, "✓"},
		{"error status", StatusError, "✗"},
		{"skipped status", StatusSkipped, "⊘"},
		{"running status", StatusRunning, "⊙"},
		{"unknown status", "unknown", "•"},
		{"empty status", "", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := display.getStatusIndicator(tt.status)
			// Remove ANSI color codes for comparison
			if !strings.Contains(got, tt.want) {
				t.Errorf("getStatusIndicator(%q) should contain %q, got %q", tt.status, tt.want, got)
			}
		})
	}
}

func TestTUIDisplay_Truncate(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "no truncation needed",
			input:    "short text",
			maxWidth: 20,
			want:     "short text",
		},
		{
			name:     "truncate with ellipsis",
			input:    "this is a very long text that needs truncation",
			maxWidth: 20,
			want:     "this is a very lo...",
		},
		{
			name:     "exact fit",
			input:    "exactly 20 chars txt",
			maxWidth: 20,
			want:     "exactly 20 chars txt",
		},
		{
			name:     "very short width",
			input:    "text",
			maxWidth: 2,
			want:     "te",
		},
		{
			name:     "width less than 4",
			input:    "text",
			maxWidth: 3,
			want:     "tex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := display.truncate(tt.input, tt.maxWidth)
			if len(got) > tt.maxWidth {
				t.Errorf("truncate() length = %d, should be <= %d", len(got), tt.maxWidth)
			}
			if !strings.HasPrefix(got, tt.want[:min(len(tt.want), len(got))]) {
				t.Errorf("truncate() = %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestTUIDisplay_RenderCompletion(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	tests := []struct {
		name         string
		stats        *ExecutionStats
		wantContains []string
	}{
		{
			name: "successful execution",
			stats: &ExecutionStats{
				Duration: 1 * time.Second,
				Executed: 10,
				Skipped:  0,
				Failed:   0,
			},
			wantContains: []string{
				"Execution completed successfully",
				"Executed: 10",
				"Duration:",
			},
		},
		{
			name: "failed execution",
			stats: &ExecutionStats{
				Duration: 500 * time.Millisecond,
				Executed: 5,
				Skipped:  2,
				Failed:   3,
			},
			wantContains: []string{
				"Execution failed",
				"Executed: 5",
				"Skipped:  2",
				"Failed:   3",
				"Duration:",
			},
		},
		{
			name: "execution with skipped steps",
			stats: &ExecutionStats{
				Duration: 2 * time.Second,
				Executed: 8,
				Skipped:  3,
				Failed:   0,
			},
			wantContains: []string{
				"Execution completed successfully",
				"Executed: 8",
				"Skipped:  3",
				"Duration:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := display.renderCompletion(tt.stats)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("renderCompletion() should contain %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestTUIDisplay_RenderWithCompletion(t *testing.T) {
	animator, _ := LoadEmbeddedFrames()
	buffer := NewTUIBuffer(10)
	display := NewTUIDisplay(animator, buffer, 80, 24)

	// Set completion stats
	stats := ExecutionStats{
		Duration: 1 * time.Second,
		Executed: 5,
		Skipped:  1,
		Failed:   0,
	}
	buffer.SetCompletion(stats)

	output := display.Render()

	// Should contain completion section
	if !strings.Contains(output, "Execution completed successfully") {
		t.Error("Render() with completion should contain completion message")
	}
	if !strings.Contains(output, "Duration:") {
		t.Error("Render() with completion should contain duration")
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

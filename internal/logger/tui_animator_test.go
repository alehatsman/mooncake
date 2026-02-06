package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEmbeddedFrames(t *testing.T) {
	frames, err := LoadEmbeddedFrames()
	if err != nil {
		t.Fatalf("LoadEmbeddedFrames() error = %v", err)
	}

	if frames == nil {
		t.Fatal("LoadEmbeddedFrames() returned nil")
	}

	if frames.FrameCount() == 0 {
		t.Error("LoadEmbeddedFrames() returned zero frames")
	}

	t.Logf("Loaded %d embedded frames", frames.FrameCount())
}

func TestLoadFramesFromString(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantFrames int
		wantErr    bool
	}{
		{
			name: "valid single frame",
			content: `line 1
line 2
line 3
`,
			wantFrames: 1,
			wantErr:    false,
		},
		{
			name: "valid multiple frames",
			content: `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3
`,
			wantFrames: 2,
			wantErr:    false,
		},
		{
			name: "valid frame without trailing newline",
			content: `line 1
line 2
line 3`,
			wantFrames: 1,
			wantErr:    false,
		},
		{
			name: "invalid - less than 3 lines",
			content: `line 1
line 2
`,
			wantFrames: 0,
			wantErr:    true,
		},
		{
			name: "invalid - more than 3 lines per frame",
			content: `line 1
line 2
line 3
line 4
`,
			wantFrames: 0,
			wantErr:    true,
		},
		{
			name:       "empty content",
			content:    "",
			wantFrames: 0,
			wantErr:    true,
		},
		{
			name: "valid multiple frames with extra blank lines",
			content: `frame 1 line 1
frame 1 line 2
frame 1 line 3


frame 2 line 1
frame 2 line 2
frame 2 line 3
`,
			wantFrames: 2,
			wantErr:    false,
		},
		{
			name: "mixed valid and invalid frames",
			content: `valid line 1
valid line 2
valid line 3

invalid line 1
invalid line 2

another line 1
another line 2
another line 3
`,
			wantFrames: 2, // Only valid frames should be loaded
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames, err := LoadFramesFromString(tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFramesFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if frames.FrameCount() != tt.wantFrames {
				t.Errorf("LoadFramesFromString() frame count = %d, want %d", frames.FrameCount(), tt.wantFrames)
			}
		})
	}
}

func TestLoadFramesFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		content    string
		wantFrames int
		wantErr    bool
	}{
		{
			name: "valid animation file",
			content: `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3
`,
			wantFrames: 2,
			wantErr:    false,
		},
		{
			name: "single frame file",
			content: `line 1
line 2
line 3
`,
			wantFrames: 1,
			wantErr:    false,
		},
		{
			name: "invalid frame file",
			content: `line 1
line 2
`,
			wantFrames: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, "test_animation.txt")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			frames, err := LoadFramesFromFile(testFile)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFramesFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if frames.FrameCount() != tt.wantFrames {
				t.Errorf("LoadFramesFromFile() frame count = %d, want %d", frames.FrameCount(), tt.wantFrames)
			}
		})
	}
}

func TestLoadFramesFromFile_NotFound(t *testing.T) {
	_, err := LoadFramesFromFile("/nonexistent/path/to/file.txt")
	if err == nil {
		t.Error("LoadFramesFromFile() should return error for nonexistent file")
	}
}

func TestAnimationFrames_Current(t *testing.T) {
	content := `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3
`
	frames, err := LoadFramesFromString(content)
	if err != nil {
		t.Fatalf("LoadFramesFromString() error = %v", err)
	}

	// Current should return first frame initially
	current := frames.Current()
	if len(current) != 3 {
		t.Errorf("Current() returned %d lines, want 3", len(current))
	}

	if !strings.Contains(current[0], "frame 1") {
		t.Errorf("Current() should return first frame initially, got: %v", current)
	}
}

func TestAnimationFrames_Next(t *testing.T) {
	content := `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3

frame 3 line 1
frame 3 line 2
frame 3 line 3
`
	frames, err := LoadFramesFromString(content)
	if err != nil {
		t.Fatalf("LoadFramesFromString() error = %v", err)
	}

	// Verify initial frame
	current := frames.Current()
	if !strings.Contains(current[0], "frame 1") {
		t.Errorf("Initial frame should be frame 1, got: %v", current)
	}

	// Advance to next frame
	next := frames.Next()
	if len(next) != 3 {
		t.Errorf("Next() returned %d lines, want 3", len(next))
	}
	if !strings.Contains(next[0], "frame 2") {
		t.Errorf("Next() should return frame 2, got: %v", next)
	}

	// Advance again
	next = frames.Next()
	if !strings.Contains(next[0], "frame 3") {
		t.Errorf("Next() should return frame 3, got: %v", next)
	}

	// Advance again (should wrap to first frame)
	next = frames.Next()
	if !strings.Contains(next[0], "frame 1") {
		t.Errorf("Next() should wrap to frame 1, got: %v", next)
	}
}

func TestAnimationFrames_FrameCount(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantFrames int
	}{
		{
			name: "single frame",
			content: `line 1
line 2
line 3
`,
			wantFrames: 1,
		},
		{
			name: "three frames",
			content: `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3

frame 3 line 1
frame 3 line 2
frame 3 line 3
`,
			wantFrames: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames, err := LoadFramesFromString(tt.content)
			if err != nil {
				t.Fatalf("LoadFramesFromString() error = %v", err)
			}

			got := frames.FrameCount()
			if got != tt.wantFrames {
				t.Errorf("FrameCount() = %d, want %d", got, tt.wantFrames)
			}
		})
	}
}

func TestAnimationFrames_ConcurrentAccess(t *testing.T) {
	frames, err := LoadEmbeddedFrames()
	if err != nil {
		t.Fatalf("LoadEmbeddedFrames() error = %v", err)
	}

	// Test concurrent access to frames (mutex protection)
	done := make(chan bool)

	// Goroutine 1: Advance frames
	go func() {
		for i := 0; i < 50; i++ {
			frames.Next()
		}
		done <- true
	}()

	// Goroutine 2: Read current frame
	go func() {
		for i := 0; i < 50; i++ {
			_ = frames.Current()
		}
		done <- true
	}()

	// Goroutine 3: Read frame count
	go func() {
		for i := 0; i < 50; i++ {
			_ = frames.FrameCount()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should not panic
}

func TestAnimationFrames_NextWraparound(t *testing.T) {
	content := `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3
`
	frames, err := LoadFramesFromString(content)
	if err != nil {
		t.Fatalf("LoadFramesFromString() error = %v", err)
	}

	frameCount := frames.FrameCount()

	// Advance through all frames multiple times
	for cycle := 0; cycle < 3; cycle++ {
		for i := 0; i < frameCount; i++ {
			frame := frames.Next()
			if len(frame) != 3 {
				t.Errorf("Cycle %d, frame %d: got %d lines, want 3", cycle, i, len(frame))
			}
		}
	}

	// Verify it wrapped correctly (should be back at frame 1)
	current := frames.Current()
	if !strings.Contains(current[0], "frame 1") {
		t.Errorf("After wraparound, should be at frame 1, got: %v", current)
	}
}

func TestEmbeddedAnimationFrames_Content(t *testing.T) {
	// Test that the embedded animation content is valid
	if embeddedAnimationFrames == "" {
		t.Fatal("embeddedAnimationFrames is empty")
	}

	frames, err := LoadFramesFromString(embeddedAnimationFrames)
	if err != nil {
		t.Fatalf("Embedded animation frames are invalid: %v", err)
	}

	if frames.FrameCount() < 5 {
		t.Errorf("Embedded animation should have multiple frames, got %d", frames.FrameCount())
	}

	// Verify each frame has exactly 3 lines
	for i := 0; i < frames.FrameCount(); i++ {
		frame := frames.Next()
		if len(frame) != 3 {
			t.Errorf("Frame %d has %d lines, want 3", i, len(frame))
		}
	}
}

func TestAnimationFrames_CurrentDoesNotAdvance(t *testing.T) {
	content := `frame 1 line 1
frame 1 line 2
frame 1 line 3

frame 2 line 1
frame 2 line 2
frame 2 line 3
`
	frames, err := LoadFramesFromString(content)
	if err != nil {
		t.Fatalf("LoadFramesFromString() error = %v", err)
	}

	// Get current frame multiple times
	frame1 := frames.Current()
	frame2 := frames.Current()
	frame3 := frames.Current()

	// All should be the same (frame 1)
	if !strings.Contains(frame1[0], "frame 1") {
		t.Errorf("First Current() should return frame 1, got: %v", frame1)
	}
	if !strings.Contains(frame2[0], "frame 1") {
		t.Errorf("Second Current() should return frame 1, got: %v", frame2)
	}
	if !strings.Contains(frame3[0], "frame 1") {
		t.Errorf("Third Current() should return frame 1, got: %v", frame3)
	}
}

package logger

import (
	"fmt"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

// TUISubscriber implements event-based TUI display.
type TUISubscriber struct {
	buffer   *TUIBuffer
	display  *TUIDisplay
	animator *AnimationFrames
	ticker   *time.Ticker
	done     chan bool
	logLevel int
	redactor Redactor
	mu       sync.Mutex

	// Track current state
	currentStep *StepInfo
	isRunning   bool
}

// NewTUISubscriber creates a new TUI subscriber.
func NewTUISubscriber(logLevel int) (*TUISubscriber, error) {
	// Load animation frames
	animator, err := LoadEmbeddedFrames()
	if err != nil {
		return nil, fmt.Errorf("failed to load animation: %w", err)
	}

	// Create buffer
	buffer := NewTUIBuffer(10)

	// Get terminal size
	width, height := GetTerminalSize()

	// Create display
	display := NewTUIDisplay(animator, buffer, width, height)

	return &TUISubscriber{
		buffer:    buffer,
		display:   display,
		animator:  animator,
		done:      make(chan bool),
		logLevel:  logLevel,
		isRunning: false,
	}, nil
}

// Start begins the animation and rendering loop.
func (t *TUISubscriber) Start() {
	t.mu.Lock()
	if t.isRunning {
		t.mu.Unlock()
		return
	}
	t.isRunning = true
	t.mu.Unlock()

	t.ticker = time.NewTicker(150 * time.Millisecond)
	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.animator.Next()
				output := t.display.Render()
				fmt.Print(output)
			case <-t.done:
				return
			}
		}
	}()
}

// Stop stops the animation and shows final render.
func (t *TUISubscriber) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning {
		return
	}

	if t.ticker != nil {
		t.ticker.Stop()
	}

	select {
	case t.done <- true:
	default:
	}

	// Final render
	output := t.display.Render()
	fmt.Print(output)
	fmt.Println()

	t.isRunning = false
}

// SetRedactor sets the redactor for sensitive data.
func (t *TUISubscriber) SetRedactor(r Redactor) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.redactor = r
}

// OnEvent handles incoming events.
func (t *TUISubscriber) OnEvent(event events.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch event.Type {
	case events.EventStepStarted:
		if data, ok := event.Data.(events.StepStartedData); ok {
			t.handleStepStarted(data)
		}

	case events.EventStepCompleted:
		if data, ok := event.Data.(events.StepCompletedData); ok {
			t.handleStepCompleted(data)
		}

	case events.EventStepFailed:
		if data, ok := event.Data.(events.StepFailedData); ok {
			t.handleStepFailed(data)
		}

	case events.EventStepSkipped:
		if data, ok := event.Data.(events.StepSkippedData); ok {
			t.handleStepSkipped(data)
		}

	case events.EventRunCompleted:
		if data, ok := event.Data.(events.RunCompletedData); ok {
			t.handleRunCompleted(data)
		}
	}
}

// Close implements the Subscriber interface.
func (t *TUISubscriber) Close() {
	t.Stop()
}

// handleStepStarted processes step.started events.
func (t *TUISubscriber) handleStepStarted(data events.StepStartedData) {
	if t.logLevel > InfoLevel {
		return
	}

	// Update current step display
	t.currentStep = &StepInfo{
		Name:       data.Name,
		Level:      data.Level,
		GlobalStep: data.GlobalStep,
		Status:     StatusRunning,
	}

	t.buffer.SetCurrentStep(data.Name, ProgressInfo{
		Current: data.GlobalStep,
	})
}

// handleStepCompleted processes step.completed events.
func (t *TUISubscriber) handleStepCompleted(data events.StepCompletedData) {
	if t.logLevel > InfoLevel {
		return
	}

	// Add to history as success
	t.buffer.AddStep(StepEntry{
		Name:   data.Name,
		Status: StatusSuccess,
		Level:  data.Level,
	})

	// Clear current step
	t.currentStep = nil
}

// handleStepFailed processes step.failed events.
func (t *TUISubscriber) handleStepFailed(data events.StepFailedData) {
	if t.logLevel > InfoLevel {
		return
	}

	// Add to history as error
	t.buffer.AddStep(StepEntry{
		Name:   data.Name,
		Status: StatusError,
		Level:  data.Level,
	})

	// Clear current step
	t.currentStep = nil
}

// handleStepSkipped processes step.skipped events.
func (t *TUISubscriber) handleStepSkipped(data events.StepSkippedData) {
	if t.logLevel > InfoLevel {
		return
	}

	// Add to history as skipped
	t.buffer.AddStep(StepEntry{
		Name:   data.Name,
		Status: StatusSkipped,
		Level:  data.Level,
	})
}

// handleRunCompleted processes run.completed events.
func (t *TUISubscriber) handleRunCompleted(data events.RunCompletedData) {
	// Clear current step on completion
	t.currentStep = nil
	// Set completion stats
	t.buffer.SetCompletion(ExecutionStats{
		Duration: time.Duration(data.DurationMs) * time.Millisecond,
		Executed: data.SuccessSteps,
		Skipped:  data.SkippedSteps,
		Failed:   data.FailedSteps,
	})
}

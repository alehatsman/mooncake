package logger

import (
	"sync"
	"time"
)

// StepEntry represents a single step in the execution history
type StepEntry struct {
	Name      string
	Status    string // "success", "error", "skipped", "running"
	Level     int    // Nesting level for indentation
	Timestamp time.Time
}

// ProgressInfo tracks overall execution progress
type ProgressInfo struct {
	Current int
	Total   int
}

// BufferSnapshot is an atomic snapshot of the buffer state for rendering
type BufferSnapshot struct {
	StepHistory   []StepEntry
	CurrentStep   string
	Progress      ProgressInfo
	DebugMessages []string
	ErrorMessages []string
	Completion    *ExecutionStats
}

// TUIBuffer manages step history and message buffering
type TUIBuffer struct {
	stepHistory   []StepEntry // Circular buffer
	historySize   int
	historyStart  int // Start index for circular buffer
	historyCount  int // Number of items in buffer

	currentStep string
	progress    ProgressInfo

	debugMessages []string
	errorMessages []string
	maxMessages   int

	completion *ExecutionStats

	mu sync.RWMutex
}

// NewTUIBuffer creates a new TUI buffer with specified history size
func NewTUIBuffer(historySize int) *TUIBuffer {
	return &TUIBuffer{
		stepHistory:   make([]StepEntry, historySize),
		historySize:   historySize,
		historyStart:  0,
		historyCount:  0,
		debugMessages: make([]string, 0, 5),
		errorMessages: make([]string, 0, 5),
		maxMessages:   5,
	}
}

// AddStep adds a step to the history (circular buffer)
func (b *TUIBuffer) AddStep(entry StepEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.historyCount < b.historySize {
		// Buffer not full yet, just append
		b.stepHistory[b.historyCount] = entry
		b.historyCount++
	} else {
		// Buffer full, overwrite oldest entry
		b.stepHistory[b.historyStart] = entry
		b.historyStart = (b.historyStart + 1) % b.historySize
	}
}

// SetCurrentStep sets the currently executing step
func (b *TUIBuffer) SetCurrentStep(name string, progress ProgressInfo) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.currentStep = name
	b.progress = progress
}

// SetCompletion sets execution completion statistics
func (b *TUIBuffer) SetCompletion(stats ExecutionStats) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.completion = &stats
}

// AddDebug adds a debug message to the buffer
func (b *TUIBuffer) AddDebug(message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.debugMessages = append(b.debugMessages, message)
	// Keep only last maxMessages
	if len(b.debugMessages) > b.maxMessages {
		b.debugMessages = b.debugMessages[len(b.debugMessages)-b.maxMessages:]
	}
}

// AddError adds an error message to the buffer
func (b *TUIBuffer) AddError(message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.errorMessages = append(b.errorMessages, message)
	// Keep only last maxMessages
	if len(b.errorMessages) > b.maxMessages {
		b.errorMessages = b.errorMessages[len(b.errorMessages)-b.maxMessages:]
	}
}

// GetSnapshot returns an atomic snapshot of the buffer state
func (b *TUIBuffer) GetSnapshot() BufferSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Copy step history in correct order
	history := make([]StepEntry, b.historyCount)
	for i := 0; i < b.historyCount; i++ {
		idx := (b.historyStart + i) % b.historySize
		history[i] = b.stepHistory[idx]
	}

	// Copy message slices
	debug := make([]string, len(b.debugMessages))
	copy(debug, b.debugMessages)

	errors := make([]string, len(b.errorMessages))
	copy(errors, b.errorMessages)

	return BufferSnapshot{
		StepHistory:   history,
		CurrentStep:   b.currentStep,
		Progress:      b.progress,
		DebugMessages: debug,
		ErrorMessages: errors,
		Completion:    b.completion,
	}
}

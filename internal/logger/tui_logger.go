package logger

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TUILogger implements Logger interface with animated TUI display
type TUILogger struct {
	buffer   *TUIBuffer
	display  *TUIDisplay
	animator *AnimationFrames
	ticker   *time.Ticker
	done     chan bool
	logLevel int
	padLevel int
	mu       sync.Mutex

	// Track previous step for history
	lastStepInfo *StepInfo
}

// ansiPattern matches ANSI escape codes
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// NewTUILogger creates a new TUI logger
func NewTUILogger(logLevel int) (*TUILogger, error) {
	// Load animation frames from embedded content
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

	return &TUILogger{
		buffer:   buffer,
		display:  display,
		animator: animator,
		done:     make(chan bool),
		logLevel: logLevel,
		padLevel: 0,
	}, nil
}

// Start begins the animation and rendering loop
func (l *TUILogger) Start() {
	l.ticker = time.NewTicker(150 * time.Millisecond)
	go func() {
		for {
			select {
			case <-l.ticker.C:
				l.animator.Next()
				output := l.display.Render()
				fmt.Print(output)
			case <-l.done:
				return
			}
		}
	}()
}

// Stop stops the animation and shows final render
func (l *TUILogger) Stop() {
	if l.ticker != nil {
		l.ticker.Stop()
	}
	select {
	case l.done <- true:
	default:
	}

	// Final render to show completion
	output := l.display.Render()
	fmt.Print(output)
	fmt.Println() // Add newline for shell prompt
}

// stripANSI removes ANSI color codes from a string
func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// LogStep handles structured step logging
func (l *TUILogger) LogStep(info StepInfo) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logLevel > InfoLevel {
		return
	}

	// Handle different statuses
	switch info.Status {
	case "skipped":
		// Add directly to history as skipped
		l.buffer.AddStep(StepEntry{
			Name:      info.Name,
			Status:    "skipped",
			Level:     info.Level,
			Timestamp: time.Now(),
		})
		l.lastStepInfo = nil
	case "error":
		// Add directly to history as error
		l.buffer.AddStep(StepEntry{
			Name:      info.Name,
			Status:    "error",
			Level:     info.Level,
			Timestamp: time.Now(),
		})
		l.lastStepInfo = nil
	case "success":
		// Add directly to history as success
		l.buffer.AddStep(StepEntry{
			Name:      info.Name,
			Status:    "success",
			Level:     info.Level,
			Timestamp: time.Now(),
		})
		l.lastStepInfo = nil
	case "running":
		// Set as current step, store for later completion
		l.buffer.SetCurrentStep(info.Name, ProgressInfo{
			Current: info.GlobalStep,
			Total:   0, // Will display as "X steps completed"
		})
		l.lastStepInfo = &info
	}
}

// Infof logs an info message (for non-step messages)
func (l *TUILogger) Infof(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logLevel > InfoLevel {
		return
	}

	// Just add as debug message - steps should use LogStep now
	message := fmt.Sprintf(format, v...)
	l.buffer.AddDebug(message)
}

// Debugf logs a debug message
func (l *TUILogger) Debugf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logLevel > DebugLevel {
		return
	}

	message := fmt.Sprintf(format, v...)
	l.buffer.AddDebug(message)
}

// Errorf logs an error message
func (l *TUILogger) Errorf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	message := fmt.Sprintf(format, v...)
	l.buffer.AddError(message)

	// Mark last step as error
	if l.lastStepInfo != nil {
		l.buffer.AddStep(StepEntry{
			Name:      l.lastStepInfo.Name,
			Status:    "error",
			Level:     l.lastStepInfo.Level,
			Timestamp: time.Now(),
		})
		l.lastStepInfo = nil
	}
}

// Codef logs formatted code
func (l *TUILogger) Codef(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logLevel > DebugLevel {
		return
	}

	message := fmt.Sprintf(format, v...)
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			l.buffer.AddDebug(line)
		}
	}
}

// Textf logs plain text
func (l *TUILogger) Textf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logLevel > InfoLevel {
		return
	}

	message := fmt.Sprintf(format, v...)
	l.buffer.AddDebug(message)
}

// Mooncake displays the mooncake banner (initializes display)
func (l *TUILogger) Mooncake() {
	// In TUI mode, the animation is always running
	// No special action needed for banner
}

// SetLogLevel sets the log level
func (l *TUILogger) SetLogLevel(logLevel int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logLevel = logLevel
}

// SetLogLevelStr sets the log level from a string
func (l *TUILogger) SetLogLevelStr(logLevel string) error {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return err
	}

	l.SetLogLevel(level)
	return nil
}

// WithPadLevel creates a new logger with the specified padding level
func (l *TUILogger) WithPadLevel(padLevel int) Logger {
	// Create a new TUILogger that shares the same buffer and display
	newLogger := &TUILogger{
		buffer:       l.buffer,
		display:      l.display,
		animator:     l.animator,
		ticker:       l.ticker,
		done:         l.done,
		logLevel:     l.logLevel,
		padLevel:     padLevel,
		lastStepInfo: nil,
	}

	return newLogger
}

// parseLogLevel parses a log level string
func parseLogLevel(level string) (int, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "error":
		return ErrorLevel, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s (valid: debug, info, error)", level)
	}
}

// atoi converts string to int, returns 0 on error
func atoi(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

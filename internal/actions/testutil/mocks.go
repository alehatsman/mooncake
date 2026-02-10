// Package testutil provides shared testing utilities for action handlers.
package testutil

import (
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// MockContext implements actions.Context for testing
type MockContext struct {
	Variables map[string]interface{}
	Tmpl      template.Renderer
	Publisher *MockPublisher
	Log       logger.Logger
	StepID    string
	DryRun    bool
}

func (m *MockContext) GetVariables() map[string]interface{} {
	return m.Variables
}

func (m *MockContext) SetVariable(key string, value interface{}) {
	m.Variables[key] = value
}

func (m *MockContext) GetTemplate() template.Renderer {
	return m.Tmpl
}

func (m *MockContext) GetEventPublisher() events.Publisher {
	return m.Publisher
}

func (m *MockContext) GetLogger() logger.Logger {
	return m.Log
}

func (m *MockContext) GetCurrentStepID() string {
	if m.StepID == "" {
		return "step-1"
	}
	return m.StepID
}

func (m *MockContext) GetEvaluator() expression.Evaluator {
	return expression.NewExprEvaluator()
}

func (m *MockContext) IsDryRun() bool {
	return m.DryRun
}

// MockPublisher implements events.Publisher for testing
type MockPublisher struct {
	Events []events.Event
}

func (m *MockPublisher) Publish(event events.Event) {
	if m == nil {
		return
	}
	m.Events = append(m.Events, event)
}

func (m *MockPublisher) Subscribe(_ events.Subscriber) int {
	return 0
}

func (m *MockPublisher) Unsubscribe(_ int) {}

func (m *MockPublisher) Flush() {}

func (m *MockPublisher) Close() {}

// MockLogger implements logger.Logger for testing
type MockLogger struct {
	Logs []string
}

func (m *MockLogger) Infof(format string, _ ...interface{}) {
	m.Logs = append(m.Logs, format)
}

func (m *MockLogger) Debugf(format string, _ ...interface{}) {
	m.Logs = append(m.Logs, format)
}

func (m *MockLogger) Errorf(format string, _ ...interface{}) {
	m.Logs = append(m.Logs, format)
}

func (m *MockLogger) Codef(format string, _ ...interface{}) {
	m.Logs = append(m.Logs, format)
}

func (m *MockLogger) Textf(format string, _ ...interface{}) {
	m.Logs = append(m.Logs, format)
}

func (m *MockLogger) Mooncake() {
	m.Logs = append(m.Logs, "mooncake")
}

func (m *MockLogger) SetLogLevel(_ int) {}

func (m *MockLogger) SetLogLevelStr(_ string) error {
	return nil
}

func (m *MockLogger) WithPadLevel(_ int) logger.Logger {
	return m
}

func (m *MockLogger) LogStep(info logger.StepInfo) {
	m.Logs = append(m.Logs, info.Name)
}

func (m *MockLogger) Complete(_ logger.ExecutionStats) {
	m.Logs = append(m.Logs, "complete")
}

func (m *MockLogger) SetRedactor(_ logger.Redactor) {}

// NewMockContext creates a new mock context with sensible defaults
func NewMockContext() *MockContext {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create mock renderer: " + err.Error())
	}
	return &MockContext{
		Variables: make(map[string]interface{}),
		Tmpl:      renderer,
		Publisher: &MockPublisher{Events: []events.Event{}},
		Log:       &MockLogger{Logs: []string{}},
		StepID:    "step-1",
		DryRun:    false,
	}
}

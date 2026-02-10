package print

import (
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// mustNewRenderer creates a renderer or panics
func mustNewRenderer() template.Renderer {
	r, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return r
}


// mockContext implements actions.Context for testing
type mockContext struct {
	variables map[string]interface{}
	tmpl      template.Renderer
	publisher *mockPublisher
	log       logger.Logger
	stepID    string
}

func (m *mockContext) GetVariables() map[string]interface{} {
	return m.variables
}

func (m *mockContext) SetVariable(key string, value interface{}) {
	m.variables[key] = value
}

func (m *mockContext) GetTemplate() template.Renderer {
	return m.tmpl
}

func (m *mockContext) GetEventPublisher() events.Publisher {
	return m.publisher
}

func (m *mockContext) GetLogger() logger.Logger {
	return m.log
}

func (m *mockContext) GetCurrentStepID() string {
	if m.stepID == "" {
		return "step-1"
	}
	return m.stepID
}

func (m *mockContext) GetEvaluator() expression.Evaluator {
	return expression.NewExprEvaluator()
}

func (m *mockContext) IsDryRun() bool {
	return false
}

// mockPublisher implements events.Publisher for testing
type mockPublisher struct {
	events []events.Event
}

func (m *mockPublisher) Publish(event events.Event) {
	if m == nil {
		return
	}
	m.events = append(m.events, event)
}

func (m *mockPublisher) Subscribe(subscriber events.Subscriber) int {
	return 0
}

func (m *mockPublisher) Unsubscribe(id int) {}

func (m *mockPublisher) Flush() {}

func (m *mockPublisher) Close() {}

// mockLogger implements logger.Logger for testing
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Infof(format string, args ...interface{}) {
	m.logs = append(m.logs, format)
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	m.logs = append(m.logs, format)
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	m.logs = append(m.logs, format)
}

func (m *mockLogger) Codef(format string, args ...interface{}) {
	m.logs = append(m.logs, format)
}

func (m *mockLogger) Textf(format string, args ...interface{}) {
	m.logs = append(m.logs, format)
}

func (m *mockLogger) Mooncake() {
	m.logs = append(m.logs, "mooncake")
}

func (m *mockLogger) SetLogLevel(logLevel int) {}

func (m *mockLogger) SetLogLevelStr(logLevel string) error {
	return nil
}

func (m *mockLogger) WithPadLevel(padLevel int) logger.Logger {
	return m
}

func (m *mockLogger) LogStep(info logger.StepInfo) {
	m.logs = append(m.logs, info.Name)
}

func (m *mockLogger) Complete(stats logger.ExecutionStats) {
	m.logs = append(m.logs, "complete")
}

func (m *mockLogger) SetRedactor(redactor logger.Redactor) {}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "print" {
		t.Errorf("Name = %v, want 'print'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryOutput {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryOutput)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if meta.SupportsBecome {
		t.Error("SupportsBecome should be false")
	}
	if len(meta.EmitsEvents) != 1 {
		t.Errorf("EmitsEvents length = %d, want 1", len(meta.EmitsEvents))
	}
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventPrintMessage) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventPrintMessage))
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %v, want '1.0.0'", meta.Version)
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid print action",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, World!",
				},
			},
			wantErr: false,
		},
		{
			name: "nil print action",
			step: &config.Step{
				Print: nil,
			},
			wantErr: true,
		},
		{
			name: "empty message",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "",
				},
			},
			wantErr: true,
		},
		{
			name: "message with template",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, {{ name }}!",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name      string
		step      *config.Step
		variables map[string]interface{}
		wantMsg   string
		wantErr   bool
	}{
		{
			name: "simple message",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, World!",
				},
			},
			variables: map[string]interface{}{},
			wantMsg:   "Hello, World!",
			wantErr:   false,
		},
		{
			name: "message with template variable",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, {{ name }}!",
				},
			},
			variables: map[string]interface{}{
				"name": "Alice",
			},
			wantMsg: "Hello, Alice!",
			wantErr: false,
		},
		{
			name: "message with multiple variables",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "{{ greeting }}, {{ name }}!",
				},
			},
			variables: map[string]interface{}{
				"greeting": "Hi",
				"name":     "Bob",
			},
			wantMsg: "Hi, Bob!",
			wantErr: false,
		},
		{
			name: "message with missing variable renders empty",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, {{ missing_var }}!",
				},
			},
			variables: map[string]interface{}{},
			wantMsg:   "Hello, !",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := mustNewRenderer()
			pub := &mockPublisher{events: []events.Event{}}
			log := &mockLogger{logs: []string{}}

			ctx := &mockContext{
				variables: tt.variables,
				tmpl:      tmpl,
				publisher: pub,
				log:       log,
			}

			result, err := h.Execute(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check result properties
			execResult, ok := result.(*executor.Result)
			if !ok {
				t.Fatalf("Execute() result is not *executor.Result")
			}

			if execResult.Changed {
				t.Error("Result.Changed should be false for print action")
			}

			if execResult.Stdout != tt.wantMsg {
				t.Errorf("Result.Stdout = %v, want %v", execResult.Stdout, tt.wantMsg)
			}

			// Check event was published
			if len(pub.events) != 1 {
				t.Errorf("Expected 1 event to be published, got %d", len(pub.events))
				return
			}

			event := pub.events[0]
			if event.Type != events.EventPrintMessage {
				t.Errorf("Event.Type = %v, want %v", event.Type, events.EventPrintMessage)
			}

			printData, ok := event.Data.(events.PrintData)
			if !ok {
				t.Fatalf("Event.Data is not events.PrintData")
			}

			if printData.Message != tt.wantMsg {
				t.Errorf("PrintData.Message = %v, want %v", printData.Message, tt.wantMsg)
			}

			// Check timestamp is reasonable
			if event.Timestamp.IsZero() {
				t.Error("Event.Timestamp is zero")
			}
			if time.Since(event.Timestamp) > time.Second {
				t.Error("Event.Timestamp is too old")
			}
		})
	}
}

func TestHandler_Execute_NoPublisher(t *testing.T) {
	h := &Handler{}

	tmpl := mustNewRenderer()
	log := &mockLogger{logs: []string{}}

	ctx := &mockContext{
		variables: map[string]interface{}{},
		tmpl:      tmpl,
		publisher: nil, // No publisher
		log:       log,
	}

	step := &config.Step{
		Print: &config.PrintAction{
			Msg: "Hello, World!",
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Execute() result is not *executor.Result")
	}

	if execResult.Stdout != "Hello, World!" {
		t.Errorf("Result.Stdout = %v, want 'Hello, World!'", execResult.Stdout)
	}
}

func TestHandler_DryRun(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name      string
		step      *config.Step
		variables map[string]interface{}
		wantErr   bool
	}{
		{
			name: "simple message",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, World!",
				},
			},
			variables: map[string]interface{}{},
			wantErr:   false,
		},
		{
			name: "message with template variable",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, {{ name }}!",
				},
			},
			variables: map[string]interface{}{
				"name": "Alice",
			},
			wantErr: false,
		},
		{
			name: "message with missing variable - should not error",
			step: &config.Step{
				Print: &config.PrintAction{
					Msg: "Hello, {{ missing_var }}!",
				},
			},
			variables: map[string]interface{}{},
			wantErr:   false, // DryRun should not error on template failures
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := mustNewRenderer()
			log := &mockLogger{logs: []string{}}

			ctx := &mockContext{
				variables: tt.variables,
				tmpl:      tmpl,
				publisher: nil,
				log:       log,
			}

			err := h.DryRun(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check that something was logged
			if len(log.logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

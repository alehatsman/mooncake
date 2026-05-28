// Package testutil provides shared testing utilities for action handlers.
package testutil

import (
	"context"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// MockContext implements actions.Context for testing
type MockContext struct {
	Variables   map[string]interface{}
	Tmpl        template.Renderer
	Publisher   *MockPublisher
	Log         logger.Logger
	StepID      string
	CurrentMode actions.Mode
	// Performer, if set, is returned by Effects(). Tests exercising the
	// Spec-16 Run path inject a fake or real Performer here. When nil,
	// Effects() returns a no-op Performer that records nothing.
	Performer actions.Performer
	// CurrentCtx, if set, is returned by Ctx(). Tests exercising the
	// F2 ctx-threading paths inject a cancellable ctx here. When nil,
	// Ctx() falls back to context.Background().
	CurrentCtx context.Context
}

func (m *MockContext) GetVariables() map[string]interface{} {
	return m.Variables
}

func (m *MockContext) SetVariable(key string, value interface{}) {
	m.Variables[key] = value
}

func (m *MockContext) MergeUserVars(vars map[string]interface{}) {
	for k, v := range vars {
		m.Variables[k] = v
	}
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

// Mode reports the configured CurrentMode.
func (m *MockContext) Mode() actions.Mode {
	return m.CurrentMode
}

// Ctx returns the test-configured CurrentCtx, or context.Background()
// when the test didn't bother to set one. Tests that exercise F2
// ctx-cancellation paths must populate CurrentCtx with a cancellable
// ctx (typically context.WithCancel or context.WithCancelCause).
func (m *MockContext) Ctx() context.Context {
	if m.CurrentCtx != nil {
		return m.CurrentCtx
	}
	return context.Background()
}

// Effects returns a no-op Performer that records calls but does not
// touch the filesystem. Tests that exercise effect calls in handlers
// should replace this with a real or fake Performer as needed; the
// default just exists to satisfy the Context interface.
func (m *MockContext) Effects() actions.Performer {
	if m.Performer != nil {
		return m.Performer
	}
	return noopPerformer{mode: m.Mode()}
}

// Privileged returns a no-op PrivilegedRunner. Handlers under test
// that need to inspect sudo-exec interactions should swap in their
// own fake; the default satisfies the actions.Context contract added
// by spec-69 without shelling out to real sudo.
// Privileged returns a Privileged primitive bound to AvailableRoot
// — i.e. "this process is already root, no sudo wrap needed." Tests
// that need to assert against sudo-exec interactions construct
// their own *security.Privileged with the AsUser / Escalation
// values they want to drive.
func (m *MockContext) Privileged() *security.Privileged {
	return &security.Privileged{
		Escalation: security.EscalationReport{
			Available: true,
			Reason:    security.EscalationAvailableRoot,
		},
	}
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

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Logs = append(m.Logs, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Logs = append(m.Logs, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Logs = append(m.Logs, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Codef(format string, args ...interface{}) {
	m.Logs = append(m.Logs, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Textf(format string, args ...interface{}) {
	m.Logs = append(m.Logs, fmt.Sprintf(format, args...))
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
		Variables:   make(map[string]interface{}),
		Tmpl:        renderer,
		Publisher:   &MockPublisher{Events: []events.Event{}},
		Log:         &MockLogger{Logs: []string{}},
		StepID:      "step-1",
		CurrentMode: actions.ModeApply,
	}
}

// noopPerformer satisfies actions.Performer without touching the
// filesystem. Used by MockContext.Effects() when no real Performer is
// injected. Every call returns an Effect with no Performed / WouldChange
// / Err — the test should set MockContext.Performer to something more
// useful when it needs realistic behavior.
type noopPerformer struct{ mode actions.Mode }

func (p noopPerformer) Mode() actions.Mode { return p.mode }

func (p noopPerformer) Mkdir(path string, _ os.FileMode, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionMkdir, Path: path}
}
func (p noopPerformer) WriteFile(path string, _ []byte, _ os.FileMode, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionWriteFile, Path: path}
}
func (p noopPerformer) CopyFile(_, dest string, _ os.FileMode, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionCopyFile, Path: dest}
}
func (p noopPerformer) Symlink(_, path string, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionSymlink, Path: path}
}
func (p noopPerformer) Hardlink(_, path string, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionHardlink, Path: path}
}
func (p noopPerformer) Touch(path string, _ os.FileMode, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionTouch, Path: path}
}
func (p noopPerformer) Remove(path string, _ bool, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionRemove, Path: path}
}
func (p noopPerformer) Chmod(path string, _ os.FileMode, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionChmod, Path: path}
}
func (p noopPerformer) Chown(path, _, _ string, _ actions.PerformerOpts) actions.Effect {
	return actions.Effect{Action: actions.ActionChown, Path: path}
}

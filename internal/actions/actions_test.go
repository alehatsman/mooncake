package actions

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// Mock handler for testing
type mockHandler struct {
	metadata ActionMetadata
	validateFunc func(*config.Step) error
	executeFunc func(Context, *config.Step) (Result, error)
	dryRunFunc func(Context, *config.Step) error
}

func (m *mockHandler) Metadata() ActionMetadata {
	return m.metadata
}

func (m *mockHandler) Validate(step *config.Step) error {
	if m.validateFunc != nil {
		return m.validateFunc(step)
	}
	return nil
}

func (m *mockHandler) Execute(ctx Context, step *config.Step) (Result, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, step)
	}
	return nil, nil
}

func (m *mockHandler) DryRun(ctx Context, step *config.Step) error {
	if m.dryRunFunc != nil {
		return m.dryRunFunc(ctx, step)
	}
	return nil
}

// Mock context for testing
type mockContext struct {
	variables map[string]interface{}
	tmpl      template.Renderer
	publisher *mockPublisher
	log       logger.Logger
	stepID    string
	dryRun    bool
}

func (m *mockContext) GetVariables() map[string]interface{} {
	if m.variables == nil {
		return make(map[string]interface{})
	}
	return m.variables
}

func (m *mockContext) GetTemplate() template.Renderer {
	if m.tmpl == nil {
		return template.NewPongo2Renderer()
	}
	return m.tmpl
}

func (m *mockContext) GetEventPublisher() events.Publisher {
	if m.publisher == nil {
		return &mockPublisher{}
	}
	return m.publisher
}

func (m *mockContext) GetLogger() logger.Logger {
	if m.log == nil {
		return &mockLogger{}
	}
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
	return m.dryRun
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

// Mock logger for testing
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Infof(format string, args ...interface{}) {
	if m != nil {
		m.logs = append(m.logs, fmt.Sprintf(format, args...))
	}
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	if m != nil {
		m.logs = append(m.logs, fmt.Sprintf(format, args...))
	}
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	if m != nil {
		m.logs = append(m.logs, fmt.Sprintf(format, args...))
	}
}

func (m *mockLogger) Codef(format string, args ...interface{}) {
	if m != nil {
		m.logs = append(m.logs, fmt.Sprintf(format, args...))
	}
}

func (m *mockLogger) Textf(format string, args ...interface{}) {
	if m != nil {
		m.logs = append(m.logs, fmt.Sprintf(format, args...))
	}
}

func (m *mockLogger) Mooncake() {
	if m != nil {
		m.logs = append(m.logs, "mooncake")
	}
}

func (m *mockLogger) SetLogLevel(logLevel int) {}

func (m *mockLogger) SetLogLevelStr(logLevel string) error {
	return nil
}

func (m *mockLogger) WithPadLevel(padLevel int) logger.Logger {
	return m
}

func (m *mockLogger) LogStep(info logger.StepInfo) {
	if m != nil {
		m.logs = append(m.logs, info.Name)
	}
}

func (m *mockLogger) Complete(stats logger.ExecutionStats) {
	if m != nil {
		m.logs = append(m.logs, "complete")
	}
}

func (m *mockLogger) SetRedactor(redactor logger.Redactor) {}

// TestRegistry_NewRegistry tests registry creation
func TestRegistry_NewRegistry(t *testing.T) {
	reg := NewRegistry()

	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if reg.handlers == nil {
		t.Error("Registry handlers map is nil")
	}

	if reg.Count() != 0 {
		t.Errorf("New registry should have 0 handlers, got %d", reg.Count())
	}
}

// TestRegistry_Register tests handler registration
func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()

	handler := &mockHandler{
		metadata: ActionMetadata{
			Name: "test",
			Description: "Test action",
			Category: CategoryCommand,
		},
	}

	reg.Register(handler)

	if reg.Count() != 1 {
		t.Errorf("Expected 1 handler, got %d", reg.Count())
	}

	retrieved, ok := reg.Get("test")
	if !ok {
		t.Error("Failed to retrieve registered handler")
	}

	if retrieved.Metadata().Name != "test" {
		t.Errorf("Retrieved handler has wrong name: %s", retrieved.Metadata().Name)
	}
}

// TestRegistry_Register_Duplicate tests duplicate registration panics
func TestRegistry_Register_Duplicate(t *testing.T) {
	reg := NewRegistry()

	handler1 := &mockHandler{
		metadata: ActionMetadata{Name: "duplicate"},
	}
	handler2 := &mockHandler{
		metadata: ActionMetadata{Name: "duplicate"},
	}

	reg.Register(handler1)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on duplicate registration")
		} else {
			expected := "action handler already registered: duplicate"
			if r != expected {
				t.Errorf("Expected panic message %q, got %q", expected, r)
			}
		}
	}()

	reg.Register(handler2)
}

// TestRegistry_Get tests handler retrieval
func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()

	handler := &mockHandler{
		metadata: ActionMetadata{Name: "get_test"},
	}

	reg.Register(handler)

	// Test successful retrieval
	retrieved, ok := reg.Get("get_test")
	if !ok {
		t.Error("Failed to get registered handler")
	}
	if retrieved == nil {
		t.Error("Retrieved handler is nil")
	}

	// Test non-existent handler
	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent handler")
	}
}

// TestRegistry_List tests listing all handlers
func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()

	handlers := []*mockHandler{
		{metadata: ActionMetadata{Name: "handler1", Category: CategoryCommand}},
		{metadata: ActionMetadata{Name: "handler2", Category: CategoryFile}},
		{metadata: ActionMetadata{Name: "handler3", Category: CategorySystem}},
	}

	for _, h := range handlers {
		reg.Register(h)
	}

	list := reg.List()

	if len(list) != 3 {
		t.Errorf("Expected 3 handlers in list, got %d", len(list))
	}

	// Check that all registered handlers are in the list
	names := make(map[string]bool)
	for _, meta := range list {
		names[meta.Name] = true
	}

	for _, h := range handlers {
		if !names[h.metadata.Name] {
			t.Errorf("Handler %s not found in list", h.metadata.Name)
		}
	}
}

// TestRegistry_Has tests handler existence check
func TestRegistry_Has(t *testing.T) {
	reg := NewRegistry()

	handler := &mockHandler{
		metadata: ActionMetadata{Name: "exists"},
	}

	reg.Register(handler)

	if !reg.Has("exists") {
		t.Error("Has should return true for registered handler")
	}

	if reg.Has("nonexistent") {
		t.Error("Has should return false for non-existent handler")
	}
}

// TestRegistry_Count tests handler count
func TestRegistry_Count(t *testing.T) {
	reg := NewRegistry()

	if reg.Count() != 0 {
		t.Errorf("Empty registry should have count 0, got %d", reg.Count())
	}

	reg.Register(&mockHandler{metadata: ActionMetadata{Name: "h1"}})
	if reg.Count() != 1 {
		t.Errorf("Expected count 1, got %d", reg.Count())
	}

	reg.Register(&mockHandler{metadata: ActionMetadata{Name: "h2"}})
	if reg.Count() != 2 {
		t.Errorf("Expected count 2, got %d", reg.Count())
	}
}

// TestRegistry_ThreadSafety tests concurrent access to registry
func TestRegistry_ThreadSafety(t *testing.T) {
	reg := NewRegistry()

	// Pre-register one handler for Get operations
	reg.Register(&mockHandler{metadata: ActionMetadata{Name: "existing"}})

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.Get("existing")
			reg.Has("existing")
			reg.List()
			reg.Count()
		}()
	}

	// Concurrent writes (with unique names to avoid panics)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			handler := &mockHandler{
				metadata: ActionMetadata{Name: fmt.Sprintf("concurrent_%d", idx)},
			}
			reg.Register(handler)
		}(i)
	}

	wg.Wait()

	// Verify all handlers were registered
	if reg.Count() != 11 { // 1 existing + 10 concurrent
		t.Errorf("Expected 11 handlers after concurrent registration, got %d", reg.Count())
	}
}

// TestGlobalRegistry_Functions tests global registry convenience functions
func TestGlobalRegistry_Functions(t *testing.T) {
	// Note: This test might interfere with other tests if they use the global registry
	// In a real scenario, you'd want to reset the global registry or use a separate test package

	// Test that global functions delegate to global registry
	initialCount := Count()

	handler := &mockHandler{
		metadata: ActionMetadata{Name: "global_test_handler"},
	}

	// Clean up in case this test ran before
	if Has("global_test_handler") {
		t.Log("Handler already exists in global registry")
		return
	}

	Register(handler)

	if !Has("global_test_handler") {
		t.Error("Global Has() should return true for registered handler")
	}

	retrieved, ok := Get("global_test_handler")
	if !ok {
		t.Error("Global Get() should retrieve registered handler")
	}
	if retrieved.Metadata().Name != "global_test_handler" {
		t.Error("Retrieved handler from global registry has wrong name")
	}

	if Count() <= initialCount {
		t.Error("Global Count() should increase after registration")
	}

	list := List()
	found := false
	for _, meta := range list {
		if meta.Name == "global_test_handler" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Global List() should include registered handler")
	}
}

// TestActionMetadata_Fields tests ActionMetadata structure
func TestActionMetadata_Fields(t *testing.T) {
	meta := ActionMetadata{
		Name: "test_action",
		Description: "A test action",
		Category: CategoryCommand,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{"test.started", "test.completed"},
		Version: "1.0.0",
	}

	if meta.Name != "test_action" {
		t.Errorf("Name mismatch: %s", meta.Name)
	}
	if meta.Description != "A test action" {
		t.Errorf("Description mismatch: %s", meta.Description)
	}
	if meta.Category != CategoryCommand {
		t.Errorf("Category mismatch: %s", meta.Category)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if !meta.SupportsBecome {
		t.Error("SupportsBecome should be true")
	}
	if len(meta.EmitsEvents) != 2 {
		t.Errorf("Expected 2 events, got %d", len(meta.EmitsEvents))
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version mismatch: %s", meta.Version)
	}
}

// TestActionCategory_Constants tests all category constants
func TestActionCategory_Constants(t *testing.T) {
	categories := map[ActionCategory]string{
		CategoryCommand: "command",
		CategoryFile: "file",
		CategorySystem: "system",
		CategoryData: "data",
		CategoryNetwork: "network",
		CategoryOutput: "output",
	}

	for category, expected := range categories {
		if string(category) != expected {
			t.Errorf("Category %s has wrong value: %s", expected, category)
		}
	}
}

// TestHandlerFunc_NewHandlerFunc tests HandlerFunc creation
func TestHandlerFunc_NewHandlerFunc(t *testing.T) {
	metadata := ActionMetadata{
		Name: "func_test",
		Description: "Test function handler",
		Category: CategoryCommand,
	}

	validateCalled := false
	executeCalled := false
	dryRunCalled := false

	handler := NewHandlerFunc(
		metadata,
		func(step *config.Step) error {
			validateCalled = true
			return nil
		},
		func(ctx Context, step *config.Step) (Result, error) {
			executeCalled = true
			return nil, nil
		},
		func(ctx Context, step *config.Step) error {
			dryRunCalled = true
			return nil
		},
	)

	if handler == nil {
		t.Fatal("NewHandlerFunc returned nil")
	}

	// Test Metadata
	meta := handler.Metadata()
	if meta.Name != "func_test" {
		t.Errorf("Metadata name mismatch: %s", meta.Name)
	}

	// Test Validate
	err := handler.Validate(&config.Step{})
	if err != nil {
		t.Errorf("Validate returned error: %v", err)
	}
	if !validateCalled {
		t.Error("Validate function was not called")
	}

	// Test Execute
	ctx := &mockContext{}
	_, err = handler.Execute(ctx, &config.Step{})
	if err != nil {
		t.Errorf("Execute returned error: %v", err)
	}
	if !executeCalled {
		t.Error("Execute function was not called")
	}

	// Test DryRun
	err = handler.DryRun(ctx, &config.Step{})
	if err != nil {
		t.Errorf("DryRun returned error: %v", err)
	}
	if !dryRunCalled {
		t.Error("DryRun function was not called")
	}
}

// TestHandlerFunc_NilValidate tests HandlerFunc with nil validate function
func TestHandlerFunc_NilValidate(t *testing.T) {
	handler := NewHandlerFunc(
		ActionMetadata{Name: "test"},
		nil, // nil validate
		func(ctx Context, step *config.Step) (Result, error) { return nil, nil },
		nil,
	)

	err := handler.Validate(&config.Step{})
	if err != nil {
		t.Errorf("Validate with nil function should return nil, got: %v", err)
	}
}

// TestHandlerFunc_NilDryRun tests HandlerFunc with nil dry-run function
func TestHandlerFunc_NilDryRun(t *testing.T) {
	handler := NewHandlerFunc(
		ActionMetadata{Name: "test"},
		nil,
		func(ctx Context, step *config.Step) (Result, error) { return nil, nil },
		nil, // nil dryRun
	)

	ctx := &mockContext{}

	err := handler.DryRun(ctx, &config.Step{})
	if err != nil {
		t.Errorf("DryRun with nil function should return nil, got: %v", err)
	}

	// The default dry-run calls GetLogger().Infof, which should not panic
}

// TestHandlerFunc_ValidateError tests HandlerFunc returning validation error
func TestHandlerFunc_ValidateError(t *testing.T) {
	expectedErr := errors.New("validation failed")

	handler := NewHandlerFunc(
		ActionMetadata{Name: "test"},
		func(step *config.Step) error {
			return expectedErr
		},
		func(ctx Context, step *config.Step) (Result, error) { return nil, nil },
		nil,
	)

	err := handler.Validate(&config.Step{})
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestHandlerFunc_ExecuteError tests HandlerFunc returning execution error
func TestHandlerFunc_ExecuteError(t *testing.T) {
	expectedErr := errors.New("execution failed")

	handler := NewHandlerFunc(
		ActionMetadata{Name: "test"},
		nil,
		func(ctx Context, step *config.Step) (Result, error) {
			return nil, expectedErr
		},
		nil,
	)

	ctx := &mockContext{}
	_, err := handler.Execute(ctx, &config.Step{})
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestHandlerFunc_DryRunError tests HandlerFunc returning dry-run error
func TestHandlerFunc_DryRunError(t *testing.T) {
	expectedErr := errors.New("dry-run failed")

	handler := NewHandlerFunc(
		ActionMetadata{Name: "test"},
		nil,
		func(ctx Context, step *config.Step) (Result, error) { return nil, nil },
		func(ctx Context, step *config.Step) error {
			return expectedErr
		},
	)

	ctx := &mockContext{}
	err := handler.DryRun(ctx, &config.Step{})
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestRegistry_List_EmptyRegistry tests listing handlers in empty registry
func TestRegistry_List_EmptyRegistry(t *testing.T) {
	reg := NewRegistry()

	list := reg.List()

	if list == nil {
		t.Error("List should not return nil for empty registry")
	}

	if len(list) != 0 {
		t.Errorf("Empty registry should return empty list, got %d items", len(list))
	}
}

// TestRegistry_MultipleCategories tests handlers with different categories
func TestRegistry_MultipleCategories(t *testing.T) {
	reg := NewRegistry()

	categories := []ActionCategory{
		CategoryCommand,
		CategoryFile,
		CategorySystem,
		CategoryData,
		CategoryNetwork,
		CategoryOutput,
	}

	for i, cat := range categories {
		handler := &mockHandler{
			metadata: ActionMetadata{
				Name: fmt.Sprintf("handler_%d", i),
				Category: cat,
			},
		}
		reg.Register(handler)
	}

	if reg.Count() != len(categories) {
		t.Errorf("Expected %d handlers, got %d", len(categories), reg.Count())
	}

	list := reg.List()
	categoriesFound := make(map[ActionCategory]bool)
	for _, meta := range list {
		categoriesFound[meta.Category] = true
	}

	if len(categoriesFound) != len(categories) {
		t.Errorf("Expected %d different categories, found %d", len(categories), len(categoriesFound))
	}
}

// TestHandlerFunc_ComplexMetadata tests HandlerFunc with full metadata
func TestHandlerFunc_ComplexMetadata(t *testing.T) {
	metadata := ActionMetadata{
		Name: "complex",
		Description: "A complex handler with all metadata fields",
		Category: CategorySystem,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{"complex.start", "complex.progress", "complex.complete"},
		Version: "2.1.0",
	}

	handler := NewHandlerFunc(
		metadata,
		nil,
		func(ctx Context, step *config.Step) (Result, error) { return nil, nil },
		nil,
	)

	meta := handler.Metadata()

	if meta.Name != metadata.Name {
		t.Errorf("Name mismatch: expected %s, got %s", metadata.Name, meta.Name)
	}
	if meta.Description != metadata.Description {
		t.Errorf("Description mismatch")
	}
	if meta.Category != metadata.Category {
		t.Errorf("Category mismatch")
	}
	if meta.SupportsDryRun != metadata.SupportsDryRun {
		t.Errorf("SupportsDryRun mismatch")
	}
	if meta.SupportsBecome != metadata.SupportsBecome {
		t.Errorf("SupportsBecome mismatch")
	}
	if len(meta.EmitsEvents) != len(metadata.EmitsEvents) {
		t.Errorf("EmitsEvents length mismatch: expected %d, got %d",
			len(metadata.EmitsEvents), len(meta.EmitsEvents))
	}
	if meta.Version != metadata.Version {
		t.Errorf("Version mismatch: expected %s, got %s", metadata.Version, meta.Version)
	}
}

// TestRegistry_StressTest tests registry under high load
func TestRegistry_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	reg := NewRegistry()

	// Register many handlers
	numHandlers := 1000
	for i := 0; i < numHandlers; i++ {
		handler := &mockHandler{
			metadata: ActionMetadata{
				Name: fmt.Sprintf("stress_handler_%d", i),
				Category: CategoryCommand,
			},
		}
		reg.Register(handler)
	}

	if reg.Count() != numHandlers {
		t.Errorf("Expected %d handlers, got %d", numHandlers, reg.Count())
	}

	// Concurrent reads while listing
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Random operations
			if idx%3 == 0 {
				reg.Get(fmt.Sprintf("stress_handler_%d", idx%numHandlers))
			} else if idx%3 == 1 {
				reg.Has(fmt.Sprintf("stress_handler_%d", idx%numHandlers))
			} else {
				reg.List()
			}
		}(i)
	}

	wg.Wait()
}

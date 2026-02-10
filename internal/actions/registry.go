package actions

import (
	"fmt"
	"sync"
)

// Registry manages registered action handlers through a thread-safe map.
//
// The registry pattern enables:
//  1. Dynamic action discovery - handlers register themselves via init()
//  2. Loose coupling - executor doesn't import all action packages
//  3. Extensibility - new actions added without changing executor
//  4. Thread safety - concurrent access from multiple goroutines
//
// Registration flow:
//   1. Action package imports actions: import "github.com/.../internal/actions"
//   2. Action package defines handler: type Handler struct{}
//   3. Action package registers in init(): func init() { actions.Register(&Handler{}) }
//   4. Main imports register package: import _ "github.com/.../internal/register"
//   5. Register package imports all actions: import _ ".../actions/shell"
//   6. All handlers automatically registered before main() runs
//
// Lookup flow:
//   1. Executor determines action type from step: actionType := step.DetermineActionType()
//   2. Executor queries registry: handler, ok := actions.Get(actionType)
//   3. If found, executor calls: handler.Validate(step), handler.Execute(ctx, step)
//   4. If not found, executor falls back to legacy implementation
//
// This avoids circular imports because:
//   - actions package defines Handler interface
//   - action implementations (shell, file, etc.) import actions
//   - executor imports actions but NOT action implementations
//   - register package imports action implementations (triggers init())
//   - cmd imports register (triggers all registrations)
type Registry struct {
	mu       sync.RWMutex        // Protects concurrent access to handlers map
	handlers map[string]Handler  // Maps action names ("shell", "file") to handlers
}

// NewRegistry creates a new action registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler to the registry.
// This is typically called from init() functions in action packages.
// Returns an error if a handler with the same name is already registered.
func (r *Registry) Register(handler Handler) error {
	meta := handler.Metadata()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[meta.Name]; exists {
		return fmt.Errorf("action handler already registered: %s", meta.Name)
	}

	r.handlers[meta.Name] = handler
	return nil
}

// Get retrieves a handler by action type name.
// Returns the handler and true if found, nil and false otherwise.
func (r *Registry) Get(actionType string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.handlers[actionType]
	return handler, ok
}

// List returns metadata for all registered handlers.
// Useful for introspection and documentation generation.
func (r *Registry) List() []ActionMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ActionMetadata, 0, len(r.handlers))
	for _, handler := range r.handlers {
		result = append(result, handler.Metadata())
	}
	return result
}

// Has checks if a handler is registered for the given action type.
func (r *Registry) Has(actionType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.handlers[actionType]
	return ok
}

// Count returns the number of registered handlers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.handlers)
}

// Global registry instance used by default.
var globalRegistry = NewRegistry()

// Register registers a handler in the global registry.
// This is the most common way to register handlers from init() functions.
// Panics if registration fails (e.g., duplicate handler name).
//
// Example:
//
//	func init() {
//	    actions.Register(&MyHandler{})
//	}
func Register(handler Handler) {
	if err := globalRegistry.Register(handler); err != nil {
		panic(err)
	}
}

// Get retrieves a handler from the global registry.
func Get(actionType string) (Handler, bool) {
	return globalRegistry.Get(actionType)
}

// List returns all handlers from the global registry.
func List() []ActionMetadata {
	return globalRegistry.List()
}

// Has checks if a handler exists in the global registry.
func Has(actionType string) bool {
	return globalRegistry.Has(actionType)
}

// Count returns the number of handlers in the global registry.
func Count() int {
	return globalRegistry.Count()
}

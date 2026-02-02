package actions

import (
	"fmt"
	"sync"
)

// Registry manages registered action handlers.
// It provides thread-safe registration and lookup of handlers by action type.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry creates a new action registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler to the registry.
// This is typically called from init() functions in action packages.
// Panics if a handler with the same name is already registered.
func (r *Registry) Register(handler Handler) {
	meta := handler.Metadata()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[meta.Name]; exists {
		panic(fmt.Sprintf("action handler already registered: %s", meta.Name))
	}

	r.handlers[meta.Name] = handler
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
//
// Example:
//
//	func init() {
//	    actions.Register(&MyHandler{})
//	}
func Register(handler Handler) {
	globalRegistry.Register(handler)
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

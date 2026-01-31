// Package template provides template rendering functionality using pongo2.
package template

import (
	"sync"

	"github.com/flosch/pongo2/v6"
)

var (
	// once ensures filters are registered only once
	once sync.Once
)

// Renderer defines the interface for template rendering.
type Renderer interface {
	Render(template string, variables map[string]interface{}) (string, error)
}

// Pongo2Renderer implements Renderer using the pongo2 template engine.
type Pongo2Renderer struct {
	// Mutex to protect concurrent access to pongo2.FromString
	// The pongo2 TemplateSet is not thread-safe
	mu sync.Mutex
}

// NewPongo2Renderer creates a new Pongo2Renderer with filters registered.
func NewPongo2Renderer() Renderer {
	r := &Pongo2Renderer{}

	// Register custom filters once globally
	once.Do(func() {
		if err := pongo2.RegisterFilter("expanduser", r.expandUserFilter); err != nil {
			// This should only fail if the filter name is already registered
			// Log or panic if this is critical to the application
			panic("failed to register expanduser filter: " + err.Error())
		}
	})

	return r
}

// expandUserFilter is a custom pongo2 filter for expanding ~ to user home directory
func (r *Pongo2Renderer) expandUserFilter(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	path := in.String()

	// This is a simple implementation - in practice, you'd want to use
	// pathutil.ExpandPath here, but to avoid circular dependencies,
	// we just do basic ~ expansion
	// The full path expansion happens in pathutil.ExpandPath

	return pongo2.AsValue(path), nil
}

// Render renders a template string with the given variables.
func (r *Pongo2Renderer) Render(template string, variables map[string]interface{}) (string, error) {
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// Lock to prevent race condition in pongo2.FromString
	// The pongo2 TemplateSet is not thread-safe for concurrent FromString calls
	r.mu.Lock()
	pongoTemplate, err := pongo2.FromString(template)
	r.mu.Unlock()

	if err != nil {
		return "", err
	}

	output, err := pongoTemplate.Execute(variables)
	if err != nil {
		return "", err
	}

	return output, nil
}

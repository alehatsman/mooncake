// Package template provides template rendering functionality using pongo2.
package template

import (
	"fmt"
	"os"
	"sync"

	"github.com/flosch/pongo2/v6"
)

var (
	// once ensures filters are registered only once
	once sync.Once
	// filterRegisterError stores any error from filter registration
	filterRegisterError error
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
// Returns an error if filter registration fails (e.g., filter name already registered).
func NewPongo2Renderer() (Renderer, error) {
	r := &Pongo2Renderer{}

	// Register custom filters once globally
	once.Do(func() {
		if err := pongo2.RegisterFilter("expanduser", r.expandUserFilter); err != nil {
			// Store error for later return
			filterRegisterError = fmt.Errorf("failed to register expanduser filter: %w", err)
		}
	})

	// Check if filter registration failed
	if filterRegisterError != nil {
		return nil, filterRegisterError
	}

	return r, nil
}

// expandUserFilter is a custom pongo2 filter for expanding ~ to user home directory
func (r *Pongo2Renderer) expandUserFilter(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	path := in.String()

	// Expand ~ to $HOME
	// Note: Full path expansion (relative paths, etc.) happens in pathutil.ExpandPath
	// This filter only handles the ~ prefix to avoid circular dependencies
	if len(path) > 0 && path[0] == '~' {
		home := os.Getenv("HOME")
		if home == "" {
			return nil, &pongo2.Error{
				Sender:    "filter:expanduser",
				OrigError: fmt.Errorf("HOME environment variable not set"),
			}
		}

		if len(path) == 1 {
			// Just "~" expands to $HOME
			return pongo2.AsValue(home), nil
		} else if path[1] == '/' {
			// "~/foo" expands to "$HOME/foo"
			return pongo2.AsValue(home + path[1:]), nil
		}
		// "~foo" is left as-is (user home directory expansion not supported)
	}

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

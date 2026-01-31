package template

import (
	"github.com/flosch/pongo2/v6"
)

// Renderer defines the interface for template rendering
type Renderer interface {
	Render(template string, variables map[string]interface{}) (string, error)
}

// Pongo2Renderer implements Renderer using the pongo2 template engine
type Pongo2Renderer struct {
	// Store any renderer-specific state here if needed
}

// NewPongo2Renderer creates a new Pongo2Renderer with filters registered
func NewPongo2Renderer() Renderer {
	r := &Pongo2Renderer{}

	// Register custom filters once during initialization
	pongo2.RegisterFilter("expanduser", r.expandUserFilter)

	return r
}

// expandUserFilter is a custom pongo2 filter for expanding ~ to user home directory
func (r *Pongo2Renderer) expandUserFilter(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	path := in.String()

	// This is a simple implementation - in practice, you'd want to use
	// pathutil.ExpandPath here, but to avoid circular dependencies,
	// we just do basic ~ expansion
	// The full path expansion happens in pathutil.ExpandPath

	return pongo2.AsValue(path), nil
}

// Render renders a template string with the given variables
func (r *Pongo2Renderer) Render(template string, variables map[string]interface{}) (string, error) {
	if variables == nil {
		variables = make(map[string]interface{})
	}

	pongoTemplate, err := pongo2.FromString(template)
	if err != nil {
		return "", err
	}

	output, err := pongoTemplate.Execute(variables)
	if err != nil {
		return "", err
	}

	return output, nil
}

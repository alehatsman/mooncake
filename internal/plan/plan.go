package plan

import (
	"time"

	"github.com/alehatsman/mooncake/internal/config"
)

// Plan represents a fully expanded, deterministic execution plan
type Plan struct {
	Version     string                 `json:"version" yaml:"version"`
	GeneratedAt time.Time              `json:"generated_at" yaml:"generated_at"`
	RootFile    string                 `json:"root_file" yaml:"root_file"`
	Steps       []config.Step          `json:"steps" yaml:"steps"`
	InitialVars map[string]interface{} `json:"initial_vars,omitempty" yaml:"initial_vars,omitempty"`
	Tags        []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
}

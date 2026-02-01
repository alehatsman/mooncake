// Package config provides data structures and validation for mooncake configuration files.
package config

import (
	"fmt"
)

// RunConfig represents the root configuration structure.
// This can be either:
// - A simple array of steps (for backward compatibility)
// - A structured config with version, global settings, and steps
type RunConfig struct {
	// Version specifies the config schema version (e.g., "1.0")
	Version string `yaml:"version" json:"version,omitempty"`

	// Vars defines global variables available to all steps
	Vars map[string]interface{} `yaml:"vars" json:"vars,omitempty"`

	// Steps contains the configuration steps to execute
	Steps []Step `yaml:"steps" json:"steps"`
}

// File represents a file or directory operation in a configuration step.
type File struct {
	Path    string `yaml:"path" json:"path"`
	State   string `yaml:"state" json:"state,omitempty"`
	Content string `yaml:"content" json:"content,omitempty"`
	Mode    string `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}

// Template represents a template rendering operation in a configuration step.
type Template struct {
	Src  string                  `yaml:"src" json:"src"`
	Dest string                  `yaml:"dest" json:"dest"`
	Vars *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`
	Mode string                  `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}

// Shell represents a shell command execution in a configuration step.
type Shell struct {
	Command string `yaml:"command"`
}

// Step represents a single configuration step that can perform various actions.
type Step struct {
	// Identification
	Name string `yaml:"name" json:"name,omitempty"`

	// Conditionals
	When string `yaml:"when" json:"when,omitempty"`

	// Idempotency controls
	Creates *string `yaml:"creates" json:"creates,omitempty"` // Skip if path exists
	Unless  *string `yaml:"unless" json:"unless,omitempty"`   // Skip if command succeeds

	// Actions (exactly one required)
	Template    *Template `yaml:"template" json:"template,omitempty"`
	File        *File     `yaml:"file" json:"file,omitempty"`
	Shell       *string   `yaml:"shell" json:"shell,omitempty"`
	Include     *string   `yaml:"include" json:"include,omitempty"`
	IncludeVars *string   `yaml:"include_vars" json:"include_vars,omitempty"`
	Vars        *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`

	// Privilege escalation
	Become     bool   `yaml:"become" json:"become,omitempty"`
	BecomeUser string `yaml:"become_user" json:"become_user,omitempty"`

	// Environment
	Env map[string]string `yaml:"env" json:"env,omitempty"`
	Cwd string            `yaml:"cwd" json:"cwd,omitempty"`

	// Execution control
	Timeout    string `yaml:"timeout" json:"timeout,omitempty"`
	Retries    int    `yaml:"retries" json:"retries,omitempty"`
	RetryDelay string `yaml:"retry_delay" json:"retry_delay,omitempty"`

	// Result overrides
	ChangedWhen string `yaml:"changed_when" json:"changed_when,omitempty"`
	FailedWhen  string `yaml:"failed_when" json:"failed_when,omitempty"`

	// Loops
	WithFileTree *string `yaml:"with_filetree" json:"with_filetree,omitempty"`
	WithItems    *string `yaml:"with_items" json:"with_items,omitempty"`

	// Tags and registration
	Tags     []string `yaml:"tags" json:"tags,omitempty"`
	Register string   `yaml:"register" json:"register,omitempty"`

	// Plan metadata (populated during plan expansion, omitted in config files)
	ID          string        `yaml:"id,omitempty" json:"id,omitempty"`
	ActionType  string        `yaml:"action_type,omitempty" json:"action_type,omitempty"`
	Origin      *Origin       `yaml:"origin,omitempty" json:"origin,omitempty"`
	Skipped     bool          `yaml:"skipped,omitempty" json:"skipped,omitempty"`
	LoopContext *LoopContext  `yaml:"loop_context,omitempty" json:"loop_context,omitempty"`
}

// Origin tracks source location and include chain for plan traceability
type Origin struct {
	FilePath     string   `yaml:"file" json:"file"`
	Line         int      `yaml:"line" json:"line"`
	Column       int      `yaml:"column" json:"column"`
	IncludeChain []string `yaml:"include_chain,omitempty" json:"include_chain,omitempty"` // "file:line" entries
}

// LoopContext captures loop iteration metadata
type LoopContext struct {
	Type           string      `yaml:"type" json:"type"` // "with_items" or "with_filetree"
	Item           interface{} `yaml:"item" json:"item"`
	Index          int         `yaml:"index" json:"index"`
	First          bool        `yaml:"first" json:"first"`
	Last           bool        `yaml:"last" json:"last"`
	LoopExpression string      `yaml:"loop_expression,omitempty" json:"loop_expression,omitempty"`
	Depth          int         `yaml:"depth,omitempty" json:"depth,omitempty"` // Directory depth for filetree items
}

// countActions returns the number of non-nil action fields in this step.
func (s *Step) countActions() int {
	count := 0
	if s.Template != nil {
		count++
	}
	if s.File != nil {
		count++
	}
	if s.Shell != nil {
		count++
	}
	if s.Include != nil {
		count++
	}
	if s.IncludeVars != nil {
		count++
	}
	if s.Vars != nil {
		count++
	}
	return count
}

// DetermineActionType returns the action type for this step based on which action field is populated.
func (s *Step) DetermineActionType() string {
	if s.Shell != nil {
		return "shell"
	}
	if s.File != nil {
		return "file"
	}
	if s.Template != nil {
		return "template"
	}
	if s.Vars != nil {
		return "vars"
	}
	if s.IncludeVars != nil {
		return "include_vars"
	}
	if s.Include != nil {
		return "include"
	}
	if s.WithItems != nil || s.WithFileTree != nil {
		return "loop"
	}
	return "unknown"
}

// ValidateOneAction checks that the step has at most one action defined.
func (s *Step) ValidateOneAction() error {
	if s.countActions() > 1 {
		return fmt.Errorf("Step %s has more than one action", s.Name)
	}
	return nil
}

// ValidateHasAction checks that the step has at least one action defined.
func (s *Step) ValidateHasAction() error {
	if s.countActions() == 0 {
		return fmt.Errorf("Step %s has no action", s.Name)
	}
	return nil
}

// Validate checks that the step configuration is valid.
func (s *Step) Validate() error {
	err := s.ValidateHasAction()
	if err != nil {
		return err
	}

	err = s.ValidateOneAction()
	if err != nil {
		return err
	}

	return nil
}

// Copy creates a shallow copy of the step.
func (s *Step) Copy() *Step {
	return &Step{
		Name:         s.Name,
		When:         s.When,
		Creates:      s.Creates,
		Unless:       s.Unless,
		Template:     s.Template,
		File:         s.File,
		Shell:        s.Shell,
		Include:      s.Include,
		IncludeVars:  s.IncludeVars,
		Vars:         s.Vars,
		Become:       s.Become,
		BecomeUser:   s.BecomeUser,
		Env:          s.Env,
		Cwd:          s.Cwd,
		Timeout:      s.Timeout,
		Retries:      s.Retries,
		RetryDelay:   s.RetryDelay,
		ChangedWhen:  s.ChangedWhen,
		FailedWhen:   s.FailedWhen,
		WithFileTree: s.WithFileTree,
		WithItems:    s.WithItems,
		Tags:         s.Tags,
		Register:     s.Register,
		ID:           s.ID,
		Origin:       s.Origin,
		Skipped:      s.Skipped,
		LoopContext:  s.LoopContext,
	}
}

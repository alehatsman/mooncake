package config

import (
	"errors"
	"fmt"
)

type File struct {
	Path    string `yaml:"path" json:"path"`
	State   string `yaml:"state" json:"state,omitempty"`
	Content string `yaml:"content" json:"content,omitempty"`
	Mode    string `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}

type Template struct {
	Src  string                  `yaml:"src" json:"src"`
	Dest string                  `yaml:"dest" json:"dest"`
	Vars *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`
	Mode string                  `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}

type Shell struct {
	Command string `yaml:"command"`
}

type Step struct {
	Name         string                  `yaml:"name" json:"name,omitempty"`
	When         string                  `yaml:"when" json:"when,omitempty"`
	Template     *Template               `yaml:"template" json:"template,omitempty"`
	File         *File                   `yaml:"file" json:"file,omitempty"`
	Shell        *string                 `yaml:"shell" json:"shell,omitempty"`
	Include      *string                 `yaml:"include" json:"include,omitempty"`
	IncludeVars  *string                 `yaml:"include_vars" json:"include_vars,omitempty"`
	Become       bool                    `yaml:"become" json:"become,omitempty"`
	Vars         *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`
	Tags         []string                `yaml:"tags" json:"tags,omitempty"`
	WithFileTree *string                 `yaml:"with_filetree" json:"with_filetree,omitempty"`
	WithItems    *string                 `yaml:"with_items" json:"with_items,omitempty"`
	Register     string                  `yaml:"register" json:"register,omitempty"`
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

func (s *Step) ValidateOneAction() error {
	if s.countActions() > 1 {
		return errors.New(fmt.Sprintf("Step %s has more than one action", s.Name))
	}
	return nil
}

func (s *Step) ValidateHasAction() error {
	if s.countActions() == 0 {
		return fmt.Errorf("Step %s has no action", s.Name)
	}
	return nil
}

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

func (s *Step) Copy() *Step {
	return &Step{
		Name:         s.Name,
		When:         s.When,
		Template:     s.Template,
		File:         s.File,
		Shell:        s.Shell,
		Include:      s.Include,
		IncludeVars:  s.IncludeVars,
		Become:       s.Become,
		Vars:         s.Vars,
		Tags:         s.Tags,
		WithFileTree: s.WithFileTree,
		WithItems:    s.WithItems,
		Register:     s.Register,
	}
}

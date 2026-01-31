package config

import (
	"errors"
	"fmt"
)

type File struct {
	Path    string `yaml:"path"`
	State   string `yaml:"state"`
	Content string `yaml:"content"`
}

type Template struct {
	Src  string                  `yaml:"src"`
	Dest string                  `yaml:"dest"`
	Vars *map[string]interface{} `yaml:"vars"`
}

type Shell struct {
	Command string `yaml:"command"`
}

type Step struct {
	Name        string                  `yaml:"name"`
	When        string                  `yaml:"when"`
	Template    *Template               `yaml:"template"`
	File        *File                   `yaml:"file"`
	Shell       *string                 `yaml:"shell"`
	Include     *string                 `yaml:"include"`
	IncludeVars *string                 `yaml:"include_vars"`
	Become      bool                    `yaml:"become"`
	Vars        *map[string]interface{} `yaml:"vars"`
	Tags        []string                `yaml:"tags"`
}

func (s *Step) ValidateOneAction() error {
	actionsCount := 0
	if s.Template != nil {
		actionsCount++
	}
	if s.File != nil {
		actionsCount++
	}
	if s.Shell != nil {
		actionsCount++
	}
	if s.Include != nil {
		actionsCount++
	}
	if s.IncludeVars != nil {
		actionsCount++
	}
	if s.Vars != nil {
		actionsCount++
	}

	if actionsCount > 1 {
		return errors.New(fmt.Sprintf("Step %s has more than one action", s.Name))
	}

	return nil
}

func (s *Step) ValidateHasAction() error {
	if s.Template == nil && s.File == nil && s.Shell == nil &&
		s.Include == nil && s.IncludeVars == nil && s.Vars == nil {
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

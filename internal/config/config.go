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
	State   string `yaml:"state" json:"state,omitempty"`      // file|directory|absent|link|hardlink|touch|perms
	Content string `yaml:"content" json:"content,omitempty"`
	Mode    string `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")

	// Ownership
	Owner string `yaml:"owner" json:"owner,omitempty"` // Username or UID
	Group string `yaml:"group" json:"group,omitempty"` // Groupname or GID

	// Link operations
	Src string `yaml:"src" json:"src,omitempty"` // Source path for link/copy operations

	// Behavior flags
	Force   bool `yaml:"force" json:"force,omitempty"`     // Overwrite existing files
	Recurse bool `yaml:"recurse" json:"recurse,omitempty"` // Apply permissions recursively
	Backup  bool `yaml:"backup" json:"backup,omitempty"`   // Create .bak before overwrite
}

// Template represents a template rendering operation in a configuration step.
type Template struct {
	Src  string                  `yaml:"src" json:"src"`
	Dest string                  `yaml:"dest" json:"dest"`
	Vars *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`
	Mode string                  `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}

// ShellAction represents a structured shell command execution in a configuration step.
// Supports both simple string form and structured object form for backward compatibility.
type ShellAction struct {
	// Cmd is the command to execute (required)
	Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty"`

	// Interpreter specifies the shell interpreter to use
	// Supported values: "bash", "sh", "pwsh", "cmd"
	// Default: "bash" on Unix, "pwsh" on Windows
	Interpreter string `yaml:"interpreter,omitempty" json:"interpreter,omitempty"`

	// Stdin provides input to the command
	Stdin string `yaml:"stdin,omitempty" json:"stdin,omitempty"`

	// Capture controls whether to capture command output
	// When false, output is only streamed (not stored in result)
	// Default: true
	Capture *bool `yaml:"capture,omitempty" json:"capture,omitempty"`

	// Note: env, cwd, timeout are in Step-level fields for consistency
	// Note: this enables reuse across shell/command actions
}

// UnmarshalYAML implements custom YAML unmarshaling to support both string and object forms.
// Supports: shell: "command" AND shell: { cmd: "command", interpreter: "bash", ... }
func (s *ShellAction) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try unmarshaling as string first (backward compatibility)
	var str string
	if err := unmarshal(&str); err == nil {
		s.Cmd = str
		return nil
	}

	// Try unmarshaling as structured object
	type rawShell ShellAction
	var raw rawShell
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*s = ShellAction(raw)
	return nil
}

// Shell represents a shell command execution in a configuration step.
//
// Deprecated: Use ShellAction instead.
type Shell struct {
	Command string `yaml:"command"`
}

// CommandAction represents a direct command execution without shell interpolation.
// This is safer than shell when you have a known command with arguments.
type CommandAction struct {
	// Argv is the command and arguments as a list (required)
	// Example: ["git", "clone", "https://..."]
	Argv []string `yaml:"argv" json:"argv"`

	// Stdin provides input to the command
	Stdin string `yaml:"stdin,omitempty" json:"stdin,omitempty"`

	// Capture controls whether to capture command output
	// When false, output is only streamed (not stored in result)
	// Default: true
	Capture *bool `yaml:"capture,omitempty" json:"capture,omitempty"`

	// Note: env, cwd, timeout are in Step-level fields for consistency
}

// Copy represents a file copy operation in a configuration step.
type Copy struct {
	Src      string `yaml:"src" json:"src"`                       // Source file path
	Dest     string `yaml:"dest" json:"dest"`                     // Destination file path
	Mode     string `yaml:"mode" json:"mode,omitempty"`           // Octal file permissions (e.g., "0644", "0755")
	Owner    string `yaml:"owner" json:"owner,omitempty"`         // Username or UID
	Group    string `yaml:"group" json:"group,omitempty"`         // Groupname or GID
	Backup   bool   `yaml:"backup" json:"backup,omitempty"`       // Create .bak before overwrite
	Force    bool   `yaml:"force" json:"force,omitempty"`         // Overwrite if exists
	Checksum string `yaml:"checksum" json:"checksum,omitempty"`   // Expected SHA256 or MD5 checksum
}

// Unarchive represents an archive extraction operation in a configuration step.
type Unarchive struct {
	Src             string `yaml:"src" json:"src"`                                     // Source archive path
	Dest            string `yaml:"dest" json:"dest"`                                   // Destination directory
	StripComponents int    `yaml:"strip_components" json:"strip_components,omitempty"` // Number of leading path components to strip
	Creates         string `yaml:"creates" json:"creates,omitempty"`                   // Skip if this path exists (idempotency marker)
	Mode            string `yaml:"mode" json:"mode,omitempty"`                         // Octal directory permissions (e.g., "0755")
}

// Download represents a file download operation in a configuration step.
type Download struct {
	URL      string            `yaml:"url" json:"url"`                         // Remote URL (required)
	Dest     string            `yaml:"dest" json:"dest"`                       // Destination path (required)
	Checksum string            `yaml:"checksum" json:"checksum,omitempty"`     // Expected SHA256 or MD5 checksum
	Mode     string            `yaml:"mode" json:"mode,omitempty"`             // Octal file permissions (e.g., "0644")
	Timeout  string            `yaml:"timeout" json:"timeout,omitempty"`       // Maximum download time (e.g., "30s", "5m")
	Force    bool              `yaml:"force" json:"force,omitempty"`           // Force re-download if destination exists
	Backup   bool              `yaml:"backup" json:"backup,omitempty"`         // Create .bak backup before overwriting
	Headers  map[string]string `yaml:"headers" json:"headers,omitempty"`       // Custom HTTP headers
	Retries  int               `yaml:"retries" json:"retries,omitempty"`       // Number of retry attempts
}

// ServiceAction represents a service management operation in a configuration step.
// Supports systemd (Linux), launchd (macOS), and Windows services.
type ServiceAction struct {
	Name         string         `yaml:"name" json:"name"`                                 // Service name (required)
	State        string         `yaml:"state" json:"state,omitempty"`                     // started|stopped|restarted|reloaded
	Enabled      *bool          `yaml:"enabled" json:"enabled,omitempty"`                 // Enable service on boot
	DaemonReload bool           `yaml:"daemon_reload" json:"daemon_reload,omitempty"`     // Run daemon-reload after unit changes (systemd)
	Unit         *ServiceUnit   `yaml:"unit" json:"unit,omitempty"`                       // Unit file management
	Dropin       *ServiceDropin `yaml:"dropin" json:"dropin,omitempty"`                   // Drop-in configuration file
}

// ServiceUnit represents a systemd unit file or launchd plist configuration.
type ServiceUnit struct {
	Dest        string `yaml:"dest" json:"dest,omitempty"`                         // Unit file path (auto-detected if empty)
	Content     string `yaml:"content" json:"content,omitempty"`                   // Inline content
	SrcTemplate string `yaml:"src_template" json:"src_template,omitempty"`         // Template file path
	Mode        string `yaml:"mode" json:"mode,omitempty"`                         // File permissions (default: "0644")
}

// ServiceDropin represents a systemd drop-in configuration file.
// Drop-in files are placed in /etc/systemd/system/<service>.service.d/<name>.conf
type ServiceDropin struct {
	Name        string `yaml:"name" json:"name"`                                   // Drop-in file name (e.g., "10-mooncake.conf")
	Content     string `yaml:"content" json:"content,omitempty"`                   // Inline content
	SrcTemplate string `yaml:"src_template" json:"src_template,omitempty"`         // Template file path
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
	Template    *Template       `yaml:"template" json:"template,omitempty"`
	File        *File           `yaml:"file" json:"file,omitempty"`
	Shell       *ShellAction    `yaml:"shell" json:"shell,omitempty"`
	Command     *CommandAction  `yaml:"command" json:"command,omitempty"`
	Copy        *Copy           `yaml:"copy" json:"copy,omitempty"`
	Unarchive   *Unarchive      `yaml:"unarchive" json:"unarchive,omitempty"`
	Download    *Download       `yaml:"download" json:"download,omitempty"`
	Service     *ServiceAction  `yaml:"service" json:"service,omitempty"`
	Include     *string         `yaml:"include" json:"include,omitempty"`
	IncludeVars *string         `yaml:"include_vars" json:"include_vars,omitempty"`
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
	if s.Command != nil {
		count++
	}
	if s.Copy != nil {
		count++
	}
	if s.Unarchive != nil {
		count++
	}
	if s.Download != nil {
		count++
	}
	if s.Service != nil {
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
	if s.Command != nil {
		return "command"
	}
	if s.File != nil {
		return "file"
	}
	if s.Template != nil {
		return "template"
	}
	if s.Copy != nil {
		return "copy"
	}
	if s.Unarchive != nil {
		return "unarchive"
	}
	if s.Download != nil {
		return "download"
	}
	if s.Service != nil {
		return "service"
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

// Clone creates a shallow copy of the step.
func (s *Step) Clone() *Step {
	return &Step{
		Name:         s.Name,
		When:         s.When,
		Creates:      s.Creates,
		Unless:       s.Unless,
		Template:     s.Template,
		File:         s.File,
		Shell:        s.Shell,
		Command:      s.Command,
		Copy:         s.Copy,
		Unarchive:    s.Unarchive,
		Download:     s.Download,
		Service:      s.Service,
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
		ActionType:   s.ActionType,
		Origin:       s.Origin,
		Skipped:      s.Skipped,
		LoopContext:  s.LoopContext,
	}
}

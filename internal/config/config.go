// Package config provides data structures and validation for mooncake configuration files.
//
// This package defines the complete YAML schema for mooncake plans, including:
//   - Step structure and universal fields
//   - Action-specific configuration structs
//   - Validation logic and error reporting
//   - YAML unmarshaling with custom behavior
//   - JSON schema validation
//
// # Configuration Structure
//
// A mooncake configuration file is a YAML document containing an array of steps:
//
//	- name: Install nginx
//	  package:
//	    name: nginx
//	    state: present
//	  become: true
//	  when: os == "linux"
//
//	- name: Start nginx
//	  service:
//	    name: nginx
//	    state: started
//	  become: true
//
// Each step consists of:
//   - Universal fields: name, when, register, tags, become, env, cwd, timeout, etc.
//   - Exactly one action: shell, file, template, package, service, assert, etc.
//   - Optional control flow: with_items, with_filetree
//
// # Step Structure
//
// The Step struct represents a single configuration step. Key fields:
//
//	type Step struct {
//	    // Universal fields (apply to all actions)
//	    Name     string   // Human-readable step name
//	    When     string   // Conditional expression (e.g., "os == 'linux'")
//	    Register string   // Variable name to store result
//	    Tags     []string // Tag filter for selective execution
//	    Become   bool     // Run with sudo/privilege escalation
//
//	    // Action fields (exactly one must be set)
//	    Shell    *ShellAction
//	    File     *File
//	    Template *Template
//	    Package  *Package
//	    Service  *ServiceAction
//	    Assert   *Assert
//	    // ... other actions
//	}
//
// # Action Types
//
// Each action type has its own struct defining required and optional fields:
//
//   - ShellAction: Execute shell commands (cmd, interpreter, stdin, capture)
//   - File: Manage files/directories (path, state, content, mode, owner, group)
//   - Template: Render Jinja2 templates (src, dest, vars, mode)
//   - Package: Install/remove packages (name/names, state, manager, update_cache)
//   - ServiceAction: Manage services (name, state, enabled, unit, daemon_reload)
//   - Assert: Verify state (command, file, http assertions)
//   - Copy: Copy files (src, dest, mode, owner, group, backup, checksum)
//   - Download: Download files (url, dest, checksum, timeout, retries)
//   - Unarchive: Extract archives (src, dest, format, strip_components)
//   - PrintAction: Output messages (msg)
//   - PresetInvocation: Invoke presets (name, with parameters)
//
// # Validation
//
// Configuration is validated at multiple levels:
//
//  1. YAML syntax: Parser errors caught during ReadConfig()
//  2. Schema validation: JSON schema enforces structure (SchemaValidator)
//  3. Template syntax: Jinja2 templates validated (TemplateValidator)
//  4. Step validation: Each step must have exactly one action
//  5. Action validation: Handler-specific validation before execution
//
// Validation produces Diagnostic objects with:
//   - Severity: error, warning, info
//   - Message: Human-readable error description
//   - Path: YAML path to the error (e.g., "steps[0].when")
//   - Position: Line and column in source file
//   - Context: Surrounding YAML for better error messages
//
// # Custom Unmarshaling
//
// Some actions support multiple YAML forms for convenience:
//
//	# Simple string form
//	shell: "apt install nginx"
//
//	# Structured object form
//	shell:
//	  cmd: "apt install nginx"
//	  interpreter: bash
//	  capture: true
//
// This is implemented via UnmarshalYAML() methods that try string first,
// then fall back to struct unmarshaling.
//
// # Usage Example
//
//	// Read and validate configuration
//	steps, diagnostics, err := config.ReadConfigWithValidation("config.yml")
//	if err != nil {
//	    return fmt.Errorf("failed to read config: %w", err)
//	}
//
//	// Check for validation errors
//	if config.HasErrors(diagnostics) {
//	    fmt.Println(config.FormatDiagnosticsWithContext(diagnostics))
//	    return fmt.Errorf("configuration validation failed")
//	}
//
//	// Validate each step has one action
//	for i, step := range steps {
//	    if err := step.ValidateOneAction(); err != nil {
//	        return fmt.Errorf("step %d: %w", i, err)
//	    }
//	}
//
// # Thread Safety
//
// Config structures are designed to be read-only after parsing. The executor
// clones steps when expanding loops to avoid modifying shared structures.
// Use step.Clone() to create independent copies.
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

// ParsedConfig holds the result of parsing a configuration file.
// It includes both the steps to execute and any global variables defined.
type ParsedConfig struct {
	// Steps are the configuration steps to execute
	Steps []Step

	// GlobalVars are variables defined at the config level, available to all steps
	GlobalVars map[string]interface{}

	// Version is the config schema version (e.g., "1.0")
	Version string
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

// Package represents a package management operation (install/remove/update packages).
// Supports apt, dnf, yum, pacman, zypper, apk (Linux), brew, port (macOS), choco, scoop (Windows).
type Package struct {
	Name         string   `yaml:"name" json:"name,omitempty"`                     // Package name (single package)
	Names        []string `yaml:"names" json:"names,omitempty"`                   // Multiple packages
	State        string   `yaml:"state" json:"state,omitempty"`                   // present|absent|latest (default: present)
	Manager      string   `yaml:"manager" json:"manager,omitempty"`               // Package manager to use (auto-detected if empty)
	UpdateCache  bool     `yaml:"update_cache" json:"update_cache,omitempty"`     // Update package cache before operation
	Upgrade      bool     `yaml:"upgrade" json:"upgrade,omitempty"`               // Upgrade all packages (ignores name/names)
	Extra        []string `yaml:"extra" json:"extra,omitempty"`                   // Extra arguments to pass to package manager
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

// Assert represents an assertion/verification operation in a configuration step.
// Assertions always have changed: false and fail if the assertion doesn't pass.
// Supports three types: command (exit code), file (content/existence), and http (response).
type Assert struct {
	Command *AssertCommand `yaml:"command" json:"command,omitempty"` // Command assertion
	File    *AssertFile    `yaml:"file" json:"file,omitempty"`       // File assertion
	HTTP    *AssertHTTP    `yaml:"http" json:"http,omitempty"`       // HTTP assertion
}

// AssertCommand verifies a command exits with the expected code.
type AssertCommand struct {
	Cmd      string `yaml:"cmd" json:"cmd"`                           // Command to execute (required)
	ExitCode int    `yaml:"exit_code" json:"exit_code,omitempty"`     // Expected exit code (default: 0)
}

// AssertFile verifies file existence, content, or properties.
type AssertFile struct {
	Path     string  `yaml:"path" json:"path"`                         // File path (required)
	Exists   *bool   `yaml:"exists" json:"exists,omitempty"`           // Verify existence (true) or non-existence (false)
	Content  *string `yaml:"content" json:"content,omitempty"`         // Expected exact content
	Contains *string `yaml:"contains" json:"contains,omitempty"`       // Expected substring
	Mode     *string `yaml:"mode" json:"mode,omitempty"`               // Expected file permissions (e.g., "0644")
	Owner    *string `yaml:"owner" json:"owner,omitempty"`             // Expected owner (username or UID)
	Group    *string `yaml:"group" json:"group,omitempty"`             // Expected group (groupname or GID)
}

// AssertHTTP verifies HTTP response status, headers, or body content.
type AssertHTTP struct {
	URL        string            `yaml:"url" json:"url"`                               // URL to request (required)
	Method     string            `yaml:"method" json:"method,omitempty"`               // HTTP method (default: GET)
	Status     int               `yaml:"status" json:"status,omitempty"`               // Expected status code (default: 200)
	Headers    map[string]string `yaml:"headers" json:"headers,omitempty"`             // Request headers
	Body       *string           `yaml:"body" json:"body,omitempty"`                   // Request body
	Contains   *string           `yaml:"contains" json:"contains,omitempty"`           // Expected response body substring
	BodyEquals *string           `yaml:"body_equals" json:"body_equals,omitempty"`     // Expected exact response body
	JSONPath   *string           `yaml:"jsonpath" json:"jsonpath,omitempty"`           // JSONPath expression for response body
	JSONValue  interface{}       `yaml:"jsonpath_value" json:"jsonpath_value,omitempty"` // Expected value at JSONPath
	Timeout    string            `yaml:"timeout" json:"timeout,omitempty"`             // Request timeout (e.g., "30s")
}

// PresetDefinition represents a reusable preset loaded from a YAML file.
// Presets are parameterized collections of steps that can be invoked as a single action.
type PresetDefinition struct {
	Name        string                     `yaml:"name" json:"name"`                                 // Preset name (required)
	Description string                     `yaml:"description" json:"description,omitempty"`         // Human-readable description
	Version     string                     `yaml:"version" json:"version,omitempty"`                 // Semantic version
	Parameters  map[string]PresetParameter `yaml:"parameters" json:"parameters,omitempty"`           // Parameter definitions
	Steps       []Step                     `yaml:"steps" json:"steps"`                               // Steps to execute
	BaseDir     string                     `yaml:"-" json:"-"`                                       // Base directory for relative paths (set by loader)
}

// PresetParameter defines a parameter that can be passed to a preset.
type PresetParameter struct {
	Type        string        `yaml:"type" json:"type"`                               // string|bool|array|object
	Required    bool          `yaml:"required" json:"required,omitempty"`             // Whether parameter is required
	Default     interface{}   `yaml:"default" json:"default,omitempty"`               // Default value if not provided
	Enum        []interface{} `yaml:"enum" json:"enum,omitempty"`                     // Valid values (if restricted)
	Description string        `yaml:"description" json:"description,omitempty"`       // Human-readable description
}

// PresetInvocation represents a user's invocation of a preset in their playbook.
type PresetInvocation struct {
	Name string                 `yaml:"name" json:"name"`                       // Preset name (required)
	With map[string]interface{} `yaml:"with" json:"with,omitempty"`             // Parameter values
}

// UnmarshalYAML implements custom YAML unmarshaling to support string form.
// Supports: preset: "ollama" AND preset: { name: "ollama", with: {...} }
func (p *PresetInvocation) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try unmarshaling as string first (simple form)
	var str string
	if err := unmarshal(&str); err == nil {
		p.Name = str
		return nil
	}

	// Try unmarshaling as structured object
	type rawPreset PresetInvocation
	var raw rawPreset
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*p = PresetInvocation(raw)
	return nil
}

// PrintAction represents a print/output action for displaying messages.
type PrintAction struct {
	Msg string `yaml:"msg,omitempty" json:"msg,omitempty"` // Message to print (supports templates)
}

// UnmarshalYAML implements custom YAML unmarshaling to support both string and object forms.
// Supports: print: "message" AND print: { msg: "message" }
func (p *PrintAction) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try unmarshaling as string first (simple form)
	var str string
	if err := unmarshal(&str); err == nil {
		p.Msg = str
		return nil
	}

	// Try unmarshaling as structured object
	type rawPrint PrintAction
	var raw rawPrint
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*p = PrintAction(raw)
	return nil
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
	Template    *Template          `yaml:"template" json:"template,omitempty"`
	File        *File              `yaml:"file" json:"file,omitempty"`
	Shell       *ShellAction       `yaml:"shell" json:"shell,omitempty"`
	Command     *CommandAction     `yaml:"command" json:"command,omitempty"`
	Copy        *Copy              `yaml:"copy" json:"copy,omitempty"`
	Unarchive   *Unarchive         `yaml:"unarchive" json:"unarchive,omitempty"`
	Download    *Download          `yaml:"download" json:"download,omitempty"`
	Package     *Package           `yaml:"package" json:"package,omitempty"`
	Service     *ServiceAction     `yaml:"service" json:"service,omitempty"`
	Assert      *Assert            `yaml:"assert" json:"assert,omitempty"`
	Preset      *PresetInvocation  `yaml:"preset" json:"preset,omitempty"`
	Print       *PrintAction       `yaml:"print" json:"print,omitempty"`
	Include     *string            `yaml:"include" json:"include,omitempty"`
	IncludeVars *string            `yaml:"include_vars" json:"include_vars,omitempty"`
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
	if s.Package != nil {
		count++
	}
	if s.Service != nil {
		count++
	}
	if s.Assert != nil {
		count++
	}
	if s.Preset != nil {
		count++
	}
	if s.Print != nil {
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
	if s.Package != nil {
		return "package"
	}
	if s.Service != nil {
		return "service"
	}
	if s.Assert != nil {
		return "assert"
	}
	if s.Preset != nil {
		return "preset"
	}
	if s.Print != nil {
		return "print"
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
		Package:      s.Package,
		Service:      s.Service,
		Assert:       s.Assert,
		Preset:       s.Preset,
		Print:        s.Print,
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
		Tags:         append([]string(nil), s.Tags...),
		Register:     s.Register,
		ID:           s.ID,
		ActionType:   s.ActionType,
		Origin:       s.Origin,
		Skipped:      s.Skipped,
		LoopContext:  s.LoopContext,
	}
}

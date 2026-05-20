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
//   - name: Install nginx
//     package:
//     name: nginx
//     state: present
//     become: true
//     when: os == "linux"
//
//   - name: Start nginx
//     service:
//     name: nginx
//     state: started
//     become: true
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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
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

	// Modules maps alias names to module references of the form
	// "<host>/<owner>/<repo>[/<subpath>]@<version>". Populated by `mooncake mod add`
	// and consumed by the resolver when a step's `use:` field names an alias.
	Modules map[string]string `yaml:"modules" json:"modules,omitempty"`

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

	// Modules carries the parsed `modules:` alias map from the playbook.
	Modules map[string]string

	// Version is the config schema version (e.g., "1.0")
	Version string
}

// File represents a file or directory operation in a configuration step.
type File struct {
	Path    string `yaml:"path" json:"path" plan:"path"`
	State   string `yaml:"state" json:"state,omitempty"` // file|directory|absent|link|hardlink|touch|perms
	Content string `yaml:"content" json:"content,omitempty"`
	Mode    string `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")

	// Ownership
	Owner string `yaml:"owner" json:"owner,omitempty"` // Username or UID
	Group string `yaml:"group" json:"group,omitempty"` // Groupname or GID

	// Link operations
	Src string `yaml:"src" json:"src,omitempty" plan:"path"` // Source path for link/copy operations

	// Behavior flags
	Force   bool `yaml:"force" json:"force,omitempty"`     // Overwrite existing files
	Recurse bool `yaml:"recurse" json:"recurse,omitempty"` // Apply permissions recursively
	Backup  bool `yaml:"backup" json:"backup,omitempty"`   // Create .bak before overwrite
}

// Template represents a template rendering operation in a configuration step.
type Template struct {
	Src  string                  `yaml:"src" json:"src" plan:"path"`
	Dest string                  `yaml:"dest" json:"dest" plan:"path"`
	Vars *map[string]interface{} `yaml:"vars" json:"vars,omitempty"`
	Mode string                  `yaml:"mode" json:"mode,omitempty"` // Octal file permissions (e.g., "0644", "0755")
}

// ShellAction represents a structured shell command execution in a configuration step.
// Supports both simple string form and structured object form.
type ShellAction struct {
	// Cmd is the command to execute (required)
	Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty"`

	// Interpreter specifies the shell interpreter binary to invoke.
	// Default: "bash" on Unix, "powershell" on Windows.
	// Any executable on PATH is accepted (bash, sh, zsh, pwsh, powershell, cmd, nu, ...).
	// On Windows, "cmd" dispatches with /c; everything else uses -Command.
	Interpreter string `yaml:"interpreter,omitempty" json:"interpreter,omitempty"`

	// Stdin provides input to the command
	Stdin string `yaml:"stdin,omitempty" json:"stdin,omitempty"`

	// Capture controls whether to capture command output
	// When false, output is only streamed (not stored in result)
	// Default: true
	Capture *bool `yaml:"capture,omitempty" json:"capture,omitempty"`

	// RunAsAdmin asserts that the current process is running elevated on Windows.
	// If true and the process is not elevated, the step fails before exec.
	// The handler does NOT attempt to elevate (no programmatic UAC prompt).
	// Ignored on non-Windows platforms — use `become: true` for sudo.
	RunAsAdmin bool `yaml:"run_as_admin,omitempty" json:"run_as_admin,omitempty"`

	// ErrorAction sets $ErrorActionPreference for PowerShell interpreters
	// (powershell, pwsh). Common values: Stop, Continue, SilentlyContinue.
	// Default: "Stop" (any non-zero error aborts the script).
	// Ignored for non-PowerShell interpreters.
	ErrorAction string `yaml:"error_action,omitempty" json:"error_action,omitempty"`

	// Creates is an idempotency marker path. If the path exists on the host
	// when the step runs, the command is not executed and the result is
	// reported as skipped. Mirrors the Ansible/Puppet/unarchive convention.
	Creates string `yaml:"creates,omitempty" json:"creates,omitempty" plan:"path"`

	// Unless is an idempotency probe command. The command is executed (via
	// the default shell, not the configured interpreter) and the main shell
	// step runs only when it exits non-zero. If it exits zero the main step
	// is reported as skipped without running.
	Unless string `yaml:"unless,omitempty" json:"unless,omitempty"`

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
	Src      string `yaml:"src" json:"src" plan:"path"`         // Source file path
	Dest     string `yaml:"dest" json:"dest" plan:"path"`       // Destination file path
	Mode     string `yaml:"mode" json:"mode,omitempty"`         // Octal file permissions (e.g., "0644", "0755")
	Owner    string `yaml:"owner" json:"owner,omitempty"`       // Username or UID
	Group    string `yaml:"group" json:"group,omitempty"`       // Groupname or GID
	Backup   bool   `yaml:"backup" json:"backup,omitempty"`     // Create .bak before overwrite
	Force    bool   `yaml:"force" json:"force,omitempty"`       // Overwrite if exists
	Checksum string `yaml:"checksum" json:"checksum,omitempty"` // Expected SHA256 or MD5 checksum
	// FollowSymlinks controls how symbolic links at the source path are
	// handled (MT-51). Default (nil or true): the link is dereferenced
	// and the target's content is copied to dest as a regular file.
	// When set to false: the link itself is preserved — dest becomes
	// a symlink with the same target string. Pointer so unset is
	// distinguishable from explicit false; back-compat default is
	// "follow".
	FollowSymlinks *bool `yaml:"follow_symlinks,omitempty" json:"follow_symlinks,omitempty"`
}

// Unarchive represents an archive extraction operation in a configuration step.
type Unarchive struct {
	Src             string `yaml:"src" json:"src" plan:"path"`                         // Source archive path
	Dest            string `yaml:"dest" json:"dest" plan:"path"`                       // Destination directory
	StripComponents int    `yaml:"strip_components" json:"strip_components,omitempty"` // Number of leading path components to strip
	Creates         string `yaml:"creates" json:"creates,omitempty" plan:"path"`       // Skip if this path exists (idempotency marker)
	Mode            string `yaml:"mode" json:"mode,omitempty"`                         // Octal directory permissions (e.g., "0755")
}

// Tool represents a declarative tool install via a backend (archive-url,
// github-release, mise). Spec 19.
type Tool struct {
	Name    string `yaml:"name" json:"name"`       // Tool name (required)
	Version string `yaml:"version" json:"version"` // Concrete version (required)
	Backend string `yaml:"backend" json:"backend"` // archive-url | github-release | mise

	// archive-url
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// github-release
	Repo  string `yaml:"repo,omitempty" json:"repo,omitempty"`
	Asset string `yaml:"asset,omitempty" json:"asset,omitempty"`
	Tag   string `yaml:"tag,omitempty" json:"tag,omitempty"`

	// mise
	MiseTool string            `yaml:"mise_tool,omitempty" json:"mise_tool,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Common (URL-based)
	Checksum          string `yaml:"checksum,omitempty" json:"checksum,omitempty"`
	StripComponents   int    `yaml:"strip_components,omitempty" json:"strip_components,omitempty"`
	Bin               string `yaml:"bin,omitempty" json:"bin,omitempty"`
	WriteToolVersions bool   `yaml:"write_tool_versions,omitempty" json:"write_tool_versions,omitempty"`
}

// Download represents a file download operation in a configuration step.
type Download struct {
	URL      string            `yaml:"url" json:"url"`                     // Remote URL (required)
	Dest     string            `yaml:"dest" json:"dest" plan:"path"`       // Destination path (required)
	Checksum string            `yaml:"checksum" json:"checksum,omitempty"` // Expected SHA256 or MD5 checksum
	Mode     string            `yaml:"mode" json:"mode,omitempty"`         // Octal file permissions (e.g., "0644")
	Timeout  string            `yaml:"timeout" json:"timeout,omitempty"`   // Maximum download time (e.g., "30s", "5m")
	Force    bool              `yaml:"force" json:"force,omitempty"`       // Force re-download if destination exists
	Backup   bool              `yaml:"backup" json:"backup,omitempty"`     // Create .bak backup before overwriting
	Headers  map[string]string `yaml:"headers" json:"headers,omitempty"`   // Custom HTTP headers
}

// GitClone represents a git clone-or-update operation in a configuration step.
// Idempotent: if dest is already a git repo at the requested ref, no change.
type GitClone struct {
	Repo              string          `yaml:"repo" json:"repo"`                                       // Remote URL (required)
	Dest              string          `yaml:"dest" json:"dest" plan:"path"`                           // Local destination path (required)
	Ref               string          `yaml:"ref" json:"ref,omitempty"`                               // Branch, tag, or commit SHA (optional; default: remote HEAD)
	Depth             int             `yaml:"depth" json:"depth,omitempty"`                           // Shallow clone depth (0 = full)
	RecurseSubmodules bool            `yaml:"recurse_submodules" json:"recurse_submodules,omitempty"` // Init + update submodules
	Update            bool            `yaml:"update" json:"update,omitempty"`                         // If dest exists: fetch + checkout ref (default false → noop)
	Force             bool            `yaml:"force" json:"force,omitempty"`                           // Discard local changes when updating (git reset --hard)
	Credentials       *GitCredentials `yaml:"credentials" json:"credentials,omitempty"`               // Auth for the remote; values are redacted in logs
	// URL is an alias for Repo (MT-33). Authors familiar with `git clone <url>`
	// or with file.download's url: field naturally reach for url:. Handler
	// precedence: Repo wins when both are set so the canonical name stays
	// authoritative.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
}

// GitCredentials carries optional authentication for git network
// operations. Username + password drive HTTPS auth via a temporary
// GIT_ASKPASS helper; ssh_key drives SSH auth via GIT_SSH_COMMAND with
// a tmpfile (mode 0600) for the key. Inline key content (starting with
// `-----BEGIN`) is written to a tempfile; otherwise the value is
// treated as a filesystem path to an existing key.
type GitCredentials struct {
	Username   string `yaml:"username" json:"username,omitempty"`       // HTTPS username
	Password   string `yaml:"password" json:"password,omitempty"`       // HTTPS password / personal-access token; redacted
	SSHKey     string `yaml:"ssh_key" json:"ssh_key,omitempty"`         // SSH private key — path or inline PEM; redacted when inline
	SSHOptions string `yaml:"ssh_options" json:"ssh_options,omitempty"` // Extra ssh options appended to GIT_SSH_COMMAND (e.g. "-o StrictHostKeyChecking=no")
}

// GitCheckout represents a git checkout operation against an existing repo.
// Idempotent: if HEAD already resolves to the requested ref's sha, no change.
type GitCheckout struct {
	Dest  string `yaml:"dest" json:"dest" plan:"path"` // Local git working tree (required, must already be a git repo)
	Ref   string `yaml:"ref" json:"ref"`               // Branch, tag, or commit SHA to check out (required)
	Force bool   `yaml:"force" json:"force,omitempty"` // Discard local changes if working tree is dirty
}

// GitConfig manages git config keys at local/global/system scope.
// Idempotent: only set/unset keys whose current value differs from the desired state.
type GitConfig struct {
	Scope string            `yaml:"scope" json:"scope"`                               // local | global | system (required)
	Repo  string            `yaml:"repo" json:"repo,omitempty" plan:"path"`           // Working tree path (required iff scope=local; `dest:` accepted as alias)
	Dest  string            `yaml:"dest,omitempty" json:"dest,omitempty" plan:"path"` // Alias for `repo:` — matches git.clone/git.checkout's path-naming
	Set   map[string]string `yaml:"set" json:"set,omitempty"`                         // Key → desired value
	Unset []string          `yaml:"unset" json:"unset,omitempty"`                     // Keys to remove
}

// WorkingTree returns the configured local repo path, reading from
// either `repo:` (canonical) or `dest:` (alias). Errors when both
// are set to different values. Callers should use this rather than
// reading Repo directly so the alias propagates outside Validate.
func (g *GitConfig) WorkingTree() (string, error) {
	if g.Repo != "" && g.Dest != "" && g.Repo != g.Dest {
		return "", fmt.Errorf("git.config: both `repo:` and `dest:` set with different values (use one)")
	}
	if g.Repo != "" {
		return g.Repo, nil
	}
	return g.Dest, nil
}

// Package represents a package management operation (install/remove/update packages).
// Supports apt, dnf, yum, pacman, zypper, apk (Linux), brew, port (macOS), choco, scoop (Windows).
type Package struct {
	Name        string   `yaml:"name" json:"name,omitempty"`                 // Package name (single package)
	Names       []string `yaml:"-" json:"names,omitempty"`                   // Multiple packages (literal list)
	NamesExpr   string   `yaml:"-" json:"-"`                                 // Templated names expression (set when YAML `names:` is a scalar)
	State       string   `yaml:"state" json:"state,omitempty"`               // present|absent|latest (default: present)
	Manager     string   `yaml:"manager" json:"manager,omitempty"`           // Package manager to use (auto-detected if empty)
	UpdateCache bool     `yaml:"update_cache" json:"update_cache,omitempty"` // Update package cache before operation
	Upgrade     bool     `yaml:"upgrade" json:"upgrade,omitempty"`           // Upgrade all packages (ignores name/names)
	Cask        bool     `yaml:"cask" json:"cask,omitempty"`                 // Install as Homebrew cask (macOS only; requires manager: brew)
	Extra       []string `yaml:"extra" json:"extra,omitempty"`               // Extra arguments to pass to package manager
}

// PkgRepo declares a third-party package repository. The action picks
// the matching nested block (apt/dnf/brew) for the active manager.
// v1 implements apt only; dnf/brew blocks are accepted by the schema
// but raise a clear "not yet implemented" error at run time.
type PkgRepo struct {
	Name  string       `yaml:"name" json:"name"`             // Human-readable label, used as the source-list filename
	State string       `yaml:"state" json:"state,omitempty"` // present|absent (default: present)
	Apt   *PkgRepoApt  `yaml:"apt" json:"apt,omitempty"`     // Apt-specific config
	Dnf   *PkgRepoDnf  `yaml:"dnf" json:"dnf,omitempty"`     // DNF/YUM-specific config (deferred)
	Brew  *PkgRepoBrew `yaml:"brew" json:"brew,omitempty"`   // Homebrew-specific config (deferred)
}

// PkgRepoApt is the apt driver block for pkg.repo. Written as DEB822
// to /etc/apt/sources.list.d/<name>.sources with the keyring (if any)
// at /etc/apt/keyrings/<name>.gpg.
type PkgRepoApt struct {
	URI               string   `yaml:"uri" json:"uri"`                                           // Required: repository URI
	Suites            []string `yaml:"suites" json:"suites,omitempty"`                           // Required: suite names (e.g. [nodistro] or [jammy])
	Components        []string `yaml:"components" json:"components,omitempty"`                   // Default: [main]
	Architectures     []string `yaml:"architectures" json:"architectures,omitempty"`             // Default: host arch
	GPGKeyURL         string   `yaml:"gpg_key_url" json:"gpg_key_url,omitempty"`                 // URL to .gpg or .asc public key
	GPGKeyFingerprint string   `yaml:"gpg_key_fingerprint" json:"gpg_key_fingerprint,omitempty"` // Required when gpg_check is true
	GPGCheck          *bool    `yaml:"gpg_check" json:"gpg_check,omitempty"`                     // Default: true
	UpdateCache       *bool    `yaml:"update_cache" json:"update_cache,omitempty"`               // Run apt-get update after change (default: true)
}

// PkgRepoDnf is the dnf/yum driver block for pkg.repo. Written as an
// INI-style .repo file to /etc/yum.repos.d/<name>.repo with the
// optional keyring at /etc/pki/rpm-gpg/RPM-GPG-KEY-<name>.
//
// Exactly one of baseurl/metalink/mirrorlist must be set when
// state=present. Mirroring the apt driver's security posture,
// `gpg_key_fingerprint` is required when `gpg_check` is true (the
// default); set `gpg_check: false` to opt out of key pinning.
type PkgRepoDnf struct {
	BaseURL           string `yaml:"baseurl" json:"baseurl,omitempty"`                         // Direct mirror URL (mutually exclusive with metalink/mirrorlist)
	Metalink          string `yaml:"metalink" json:"metalink,omitempty"`                       // Fedora-style metalink URL
	Mirrorlist        string `yaml:"mirrorlist" json:"mirrorlist,omitempty"`                   // Text mirrorlist URL
	Description       string `yaml:"description" json:"description,omitempty"`                 // Human-readable `name=` field (defaults to repo name)
	Enabled           *bool  `yaml:"enabled" json:"enabled,omitempty"`                         // Default: true
	GPGKeyURL         string `yaml:"gpg_key_url" json:"gpg_key_url,omitempty"`                 // URL to .gpg/.asc public key (fetched and pinned locally)
	GPGKeyFingerprint string `yaml:"gpg_key_fingerprint" json:"gpg_key_fingerprint,omitempty"` // Required when gpg_check is true
	GPGCheck          *bool  `yaml:"gpg_check" json:"gpg_check,omitempty"`                     // Default: true
	UpdateCache       *bool  `yaml:"update_cache" json:"update_cache,omitempty"`               // Run dnf clean expire-cache after change (default: true)
}

// PkgRepoBrew is the homebrew driver block. Reserved for a future phase.
type PkgRepoBrew struct {
	Tap string `yaml:"tap" json:"tap"`
}

// PkgHold declares one or more packages as held/unheld so the package
// manager will refuse to upgrade or remove them automatically. v1
// implements apt only (apt-mark hold/unhold); other managers raise a
// clear "only apt is supported in v1" error.
type PkgHold struct {
	Name    string   `yaml:"name" json:"name,omitempty"`       // Single package (mutually exclusive with names)
	Names   []string `yaml:"names" json:"names,omitempty"`     // Multiple packages (mutually exclusive with name)
	State   string   `yaml:"state" json:"state,omitempty"`     // held|unheld (default: held)
	Manager string   `yaml:"manager" json:"manager,omitempty"` // Package manager (auto-detected if empty; only apt in v1)
}

// PkgUpgrade requests an upgrade of named packages, or all packages
// when names is empty. v1 implements apt only. Upgrade is declared
// "partially idempotent" by the spec: there's no clean way to predict
// whether a real upgrade would happen without simulating it, so this
// action always invokes apt and reports Changed=true on success.
type PkgUpgrade struct {
	Names      []string `yaml:"names" json:"names,omitempty"`           // Optional: subset of packages to upgrade
	Autoremove bool     `yaml:"autoremove" json:"autoremove,omitempty"` // Run apt-get autoremove -y after upgrade
	Manager    string   `yaml:"manager" json:"manager,omitempty"`       // Package manager (auto-detected if empty; only apt in v1)
}

// PkgList is the read-only query action that returns the currently
// installed packages and versions. No side effects in any mode; Changed
// is always false. v1 implements apt (via dpkg-query) only.
type PkgList struct {
	Manager string `yaml:"manager" json:"manager,omitempty"` // Package manager (auto-detected if empty; only apt in v1)
}

// UnmarshalYAML decodes Package and supports `names:` as either a list of
// strings or a single scalar (template expression). The scalar form is
// stashed in NamesExpr for late resolution at execute time.
func (p *Package) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawPackage struct {
		Name        string      `yaml:"name"`
		Names       interface{} `yaml:"names"`
		State       string      `yaml:"state"`
		Manager     string      `yaml:"manager"`
		UpdateCache bool        `yaml:"update_cache"`
		Upgrade     bool        `yaml:"upgrade"`
		Cask        bool        `yaml:"cask"`
		Extra       []string    `yaml:"extra"`
	}
	var raw rawPackage
	if err := unmarshal(&raw); err != nil {
		return err
	}

	p.Name = raw.Name
	p.State = raw.State
	p.Manager = raw.Manager
	p.UpdateCache = raw.UpdateCache
	p.Upgrade = raw.Upgrade
	p.Cask = raw.Cask
	p.Extra = raw.Extra

	switch v := raw.Names.(type) {
	case nil:
		// no names
	case string:
		p.NamesExpr = v
	case []interface{}:
		names := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("package.names[%d]: expected string, got %T", i, item)
			}
			names = append(names, s)
		}
		p.Names = names
	default:
		return fmt.Errorf("package.names: expected list or string, got %T", raw.Names)
	}
	return nil
}

// ServiceAction represents a service management operation in a configuration step.
// Supports systemd (Linux), launchd (macOS), and Windows services.
type ServiceAction struct {
	Name         string         `yaml:"name" json:"name"`                             // Service name (required)
	State        string         `yaml:"state" json:"state,omitempty"`                 // started|stopped|restarted|reloaded
	Enabled      *bool          `yaml:"enabled" json:"enabled,omitempty"`             // Enable service on boot
	DaemonReload bool           `yaml:"daemon_reload" json:"daemon_reload,omitempty"` // Run daemon-reload after unit changes (systemd)
	Unit         *ServiceUnit   `yaml:"unit" json:"unit,omitempty"`                   // Unit file management
	Dropin       *ServiceDropin `yaml:"dropin" json:"dropin,omitempty"`               // Drop-in configuration file
}

// ServiceUnit represents a systemd unit file or launchd plist configuration.
type ServiceUnit struct {
	Dest        string `yaml:"dest" json:"dest,omitempty" plan:"path"`                 // Unit file path (auto-detected if empty)
	Content     string `yaml:"content" json:"content,omitempty"`                       // Inline content
	SrcTemplate string `yaml:"src_template" json:"src_template,omitempty" plan:"path"` // Template file path
	Mode        string `yaml:"mode" json:"mode,omitempty"`                             // File permissions (default: "0644")
}

// ServiceDropin represents a systemd drop-in configuration file.
// Drop-in files are placed in /etc/systemd/system/<service>.service.d/<name>.conf
type ServiceDropin struct {
	Name        string `yaml:"name" json:"name"`                                       // Drop-in file name (e.g., "10-mooncake.conf")
	Content     string `yaml:"content" json:"content,omitempty"`                       // Inline content
	SrcTemplate string `yaml:"src_template" json:"src_template,omitempty" plan:"path"` // Template file path
}

// OsUser represents a declarative OS user account. Idempotent at the
// field level: each setting is compared with current state and only
// drifting fields are modified.
type OsUser struct {
	Name         string   `yaml:"name" json:"name"`                             // Username (required)
	State        string   `yaml:"state" json:"state,omitempty"`                 // present|absent (default: present)
	UID          *int     `yaml:"uid" json:"uid,omitempty"`                     // Numeric UID (optional)
	GID          *int     `yaml:"gid" json:"gid,omitempty"`                     // Primary GID (numeric); takes precedence over group name
	Group        string   `yaml:"group" json:"group,omitempty"`                 // Primary group by name (alternative to gid)
	Shell        string   `yaml:"shell" json:"shell,omitempty"`                 // Login shell
	Home         string   `yaml:"home" json:"home,omitempty" plan:"path"`       // Home directory
	CreateHome   *bool    `yaml:"create_home" json:"create_home,omitempty"`     // Create home dir on create (default: true unless system=true)
	Groups       []string `yaml:"groups" json:"groups,omitempty"`               // Supplementary groups
	AppendGroups *bool    `yaml:"append_groups" json:"append_groups,omitempty"` // Append to existing supplementary groups (default: true). False replaces.
	Comment      string   `yaml:"comment" json:"comment,omitempty"`             // GECOS field
	System       bool     `yaml:"system" json:"system,omitempty"`               // Create as system user (uid < UID_MIN, no home by default)
	RemoveHome   bool     `yaml:"remove_home" json:"remove_home,omitempty"`     // When state=absent, also remove home directory
}

// OsGroup represents a declarative Unix group. Idempotency is keyed by
// `name`; the action refuses to renumber an existing group's GID (that
// would silently change file ownership on disk) and refuses to remove a
// group that still has members.
type OsGroup struct {
	Name   string `yaml:"name" json:"name"`               // Group name (required)
	State  string `yaml:"state" json:"state,omitempty"`   // present|absent (default: present)
	GID    *int   `yaml:"gid" json:"gid,omitempty"`       // Numeric GID; required for new groups when set, errors on drift
	System bool   `yaml:"system" json:"system,omitempty"` // Create as system group (uid < SYS_GID_MAX, typically <1000)
}

// OsSSHKey represents authorized_keys management for a user.
// Idempotency is per-key by algorithm + base64-encoded public material;
// the comment is descriptive and doesn't participate in identity.
type OsSSHKey struct {
	User      string   `yaml:"user" json:"user"`                       // Target user (required); home dir is resolved via getent
	Key       string   `yaml:"key" json:"key,omitempty"`               // Single public key (key or keys required)
	Keys      []string `yaml:"keys" json:"keys,omitempty"`             // Multiple public keys
	State     string   `yaml:"state" json:"state,omitempty"`           // present|absent (default: present)
	Options   []string `yaml:"options" json:"options,omitempty"`       // Optional per-line options (e.g. "no-port-forwarding")
	Path      string   `yaml:"path" json:"path,omitempty" plan:"path"` // Override authorized_keys location (default: ~user/.ssh/authorized_keys)
	Exclusive bool     `yaml:"exclusive" json:"exclusive,omitempty"`   // When state=present and keys is set: remove any keys not in the supplied list
}

// OsCron declares a cron entry written to /etc/cron.d/<name>. The
// `name` is the identity for idempotency; one file per action. v1
// supports the cron.d form only (no per-user crontab via crontab -u).
type OsCron struct {
	Name     string            `yaml:"name" json:"name"`                   // Identity; used as filename in /etc/cron.d
	State    string            `yaml:"state" json:"state,omitempty"`       // present|absent (default: present)
	User     string            `yaml:"user" json:"user,omitempty"`         // Account that runs the command (default: root)
	Minute   string            `yaml:"minute" json:"minute,omitempty"`     // Cron field; default: *
	Hour     string            `yaml:"hour" json:"hour,omitempty"`         // Cron field; default: *
	Day      string            `yaml:"day" json:"day,omitempty"`           // Cron field; default: *
	Month    string            `yaml:"month" json:"month,omitempty"`       // Cron field; default: *
	Weekday  string            `yaml:"weekday" json:"weekday,omitempty"`   // Cron field; default: *
	Schedule string            `yaml:"schedule" json:"schedule,omitempty"` // Whole schedule string; mutually exclusive with the individual fields
	Command  string            `yaml:"command" json:"command,omitempty"`   // Command line to run; required when state=present
	Env      map[string]string `yaml:"env" json:"env,omitempty"`           // Environment variables (e.g. MAILTO, PATH)
}

// OsSysctl manages a single Linux kernel parameter. The persist file
// is a shared `/etc/sysctl.d/99-mooncake.conf`; each call owns one
// line keyed by `name`.
type OsSysctl struct {
	Name    string      `yaml:"name" json:"name"`                 // Sysctl key, e.g. net.ipv4.ip_forward (required)
	Value   interface{} `yaml:"value" json:"value,omitempty"`     // Desired value (string or int); required when state=present
	State   string      `yaml:"state" json:"state,omitempty"`     // present|absent (default: present)
	Persist *bool       `yaml:"persist" json:"persist,omitempty"` // Write to /etc/sysctl.d/99-mooncake.conf (default: true)
	Reload  *bool       `yaml:"reload" json:"reload,omitempty"`   // Apply via `sysctl key=value` when changed (default: true)
}

// OsSystemd manages a systemd unit file plus its lifecycle (daemon-reload
// after content change, enable/disable, start/stop). The unit `name`
// includes the suffix (e.g. "myapp.service", "backup.timer"). Section
// values may be scalars or lists; list values emit one `Key=value` line
// per element, matching systemd's handling of repeated directives like
// ExecStartPre.
type OsSystemd struct {
	Name           string                 `yaml:"name" json:"name"`                                   // Unit filename with suffix, e.g. myapp.service
	State          string                 `yaml:"state" json:"state,omitempty"`                       // present|absent (default: present)
	Unit           map[string]interface{} `yaml:"unit" json:"unit,omitempty"`                         // [Unit] section
	Service        map[string]interface{} `yaml:"service" json:"service,omitempty"`                   // [Service] section (for .service)
	Timer          map[string]interface{} `yaml:"timer" json:"timer,omitempty"`                       // [Timer] section (for .timer)
	Socket         map[string]interface{} `yaml:"socket" json:"socket,omitempty"`                     // [Socket] section (for .socket)
	Install        map[string]interface{} `yaml:"install" json:"install,omitempty"`                   // [Install] section
	Enabled        *bool                  `yaml:"enabled" json:"enabled,omitempty"`                   // ensure enabled state (default: unmanaged)
	Started        *bool                  `yaml:"started" json:"started,omitempty"`                   // ensure started state (default: unmanaged)
	ReloadOnChange *bool                  `yaml:"reload_on_change" json:"reload_on_change,omitempty"` // daemon-reload on unit content drift (default: true)
	Path           string                 `yaml:"path" json:"path,omitempty"`                         // override unit dir (default: /etc/systemd/system)
}

// OsFirewall manages host firewall rules. v1 ships a ufw driver only;
// `backend: auto` resolves to ufw when present and errors otherwise so
// nftables / firewalld can be added later without changing the user
// surface. Idempotency is computed by parsing the live rule set
// (`ufw status numbered`) and applying only the deltas.
type OsFirewall struct {
	Backend string         `yaml:"backend" json:"backend,omitempty"` // ufw|auto (default: auto; v1 supports ufw only)
	State   string         `yaml:"state" json:"state,omitempty"`     // present|absent (default: present)
	Rule    *FirewallRule  `yaml:"rule" json:"rule,omitempty"`       // Single-rule shape (mutually exclusive with rules)
	Rules   []FirewallRule `yaml:"rules" json:"rules,omitempty"`     // Multi-rule shape
}

// FirewallRule describes one inbound rule. Port-based rules are the
// common case; `from` defaults to "any" to mean any source address.
type FirewallRule struct {
	Port     int    `yaml:"port" json:"port"`                   // Destination port (required for port-based rules)
	Protocol string `yaml:"protocol" json:"protocol,omitempty"` // tcp|udp (default: tcp)
	Action   string `yaml:"action" json:"action,omitempty"`     // allow|deny|reject (default: allow)
	From     string `yaml:"from" json:"from,omitempty"`         // Source CIDR or "any" (default: any)
	Comment  string `yaml:"comment" json:"comment,omitempty"`   // Human-readable note appended to the rule
}

// WindowsFirewallRule declares an inbound/outbound Windows Firewall
// rule. spec-57. Identity is `name` (Windows Firewall's DisplayName).
// Idempotent: re-applying with the same fields is a no-op; changing
// any non-identity field updates the existing rule rather than
// creating a duplicate.
type WindowsFirewallRule struct {
	Name        string   `yaml:"name" json:"name"`                         // DisplayName (identity, required)
	State       string   `yaml:"state" json:"state,omitempty"`             // present|absent (default: present)
	Description string   `yaml:"description" json:"description,omitempty"` // Optional free-form metadata
	Direction   string   `yaml:"direction" json:"direction,omitempty"`     // inbound|outbound (default: inbound)
	Protocol    string   `yaml:"protocol" json:"protocol,omitempty"`       // tcp|udp|icmpv4|icmpv6|any (default: tcp)
	LocalPort   []string `yaml:"local_port" json:"local_port,omitempty"`   // List of ports / ranges ("7878" / "8000-8010")
	RemotePort  []string `yaml:"remote_port" json:"remote_port,omitempty"` // Same shape as local_port; default any
	Action      string   `yaml:"action" json:"action,omitempty"`           // allow|block (default: allow)
	Profile     []string `yaml:"profile" json:"profile,omitempty"`         // any|domain|private|public; default [domain, private]
	Enabled     *bool    `yaml:"enabled" json:"enabled,omitempty"`         // Default true
}

// WindowsScheduledTask declares a Task Scheduler entry. spec-57.
// Identity is `name` (TaskName). The trigger shape is the union of
// what spec-57 promises (boot, logon, repetition); fields not
// relevant to the chosen trigger type are ignored.
type WindowsScheduledTask struct {
	Name        string                         `yaml:"name" json:"name"`                         // TaskName (identity, required)
	State       string                         `yaml:"state" json:"state,omitempty"`             // present|absent (default: present)
	Description string                         `yaml:"description" json:"description,omitempty"` // Optional
	Trigger     *WindowsScheduledTaskTrigger   `yaml:"trigger" json:"trigger,omitempty"`         // Single-trigger shape (mutually exclusive with triggers)
	Triggers    []WindowsScheduledTaskTrigger  `yaml:"triggers" json:"triggers,omitempty"`       // Multi-trigger shape
	Actions     []WindowsScheduledTaskAction   `yaml:"actions" json:"actions"`                   // Required, at least one
	Principal   *WindowsScheduledTaskPrincipal `yaml:"principal" json:"principal,omitempty"`     // Optional; defaults to S4U + HighestAvailable
	Settings    *WindowsScheduledTaskSettings  `yaml:"settings" json:"settings,omitempty"`       // Optional
}

// WindowsScheduledTaskTrigger is one entry in the <Triggers> block.
type WindowsScheduledTaskTrigger struct {
	Type          string `yaml:"type" json:"type"`                               // boot|logon|repetition
	UserID        string `yaml:"user_id" json:"user_id,omitempty"`               // For type=logon: limits to a specific user; empty = any user
	Interval      string `yaml:"interval" json:"interval,omitempty"`             // For type=repetition: Go-style duration ("5m") or ISO8601 ("PT5M")
	Duration      string `yaml:"duration" json:"duration,omitempty"`             // Optional total window; same format as interval
	StartBoundary string `yaml:"start_boundary" json:"start_boundary,omitempty"` // ISO8601 timestamp; defaults to "now" for repetition triggers
}

// WindowsScheduledTaskAction is one entry in the <Actions> block.
type WindowsScheduledTaskAction struct {
	Execute          string `yaml:"execute" json:"execute"`                               // Path to the binary (required)
	Arguments        string `yaml:"arguments" json:"arguments,omitempty"`                 // Command-line arguments
	WorkingDirectory string `yaml:"working_directory" json:"working_directory,omitempty"` // Optional cwd
}

// WindowsScheduledTaskPrincipal is the user identity the task runs as.
type WindowsScheduledTaskPrincipal struct {
	User      string `yaml:"user" json:"user,omitempty"`             // e.g. "DESKTOP-X\\aleh"; default: current user
	LogonType string `yaml:"logon_type" json:"logon_type,omitempty"` // s4u|interactive|password|service_account (default: s4u)
	RunLevel  string `yaml:"run_level" json:"run_level,omitempty"`   // highest|limited (default: highest)
}

// WindowsScheduledTaskSettings maps to the <Settings> block.
type WindowsScheduledTaskSettings struct {
	StartWhenAvailable         *bool  `yaml:"start_when_available" json:"start_when_available,omitempty"`
	AllowStartIfOnBatteries    *bool  `yaml:"allow_start_if_on_batteries" json:"allow_start_if_on_batteries,omitempty"`
	DontStopIfGoingOnBatteries *bool  `yaml:"dont_stop_if_going_on_batteries" json:"dont_stop_if_going_on_batteries,omitempty"`
	RunOnlyIfNetworkAvailable  *bool  `yaml:"run_only_if_network_available" json:"run_only_if_network_available,omitempty"`
	MultipleInstances          string `yaml:"multiple_instances" json:"multiple_instances,omitempty"`     // parallel|ignore_new|queue|stop_existing
	ExecutionTimeLimit         string `yaml:"execution_time_limit" json:"execution_time_limit,omitempty"` // Go duration or ISO8601; 0 = unbounded
	RestartCount               int    `yaml:"restart_count" json:"restart_count,omitempty"`
	RestartInterval            string `yaml:"restart_interval" json:"restart_interval,omitempty"`
	Hidden                     *bool  `yaml:"hidden" json:"hidden,omitempty"`
}

// OsMount declares a filesystem mount: an `/etc/fstab` entry plus the
// matching live mount state. Identity is the destination mount point
// (one fstab entry per dest). Linux-only for v1.
type OsMount struct {
	Src     string   `yaml:"src" json:"src,omitempty"`         // Device, UUID=..., LABEL=..., tmpfs, overlay, etc. Required when state != absent
	Dest    string   `yaml:"dest" json:"dest" plan:"path"`     // Mount point (identity, required)
	FSType  string   `yaml:"fstype" json:"fstype,omitempty"`   // Filesystem type; required when adding/updating fstab entry
	Options []string `yaml:"options" json:"options,omitempty"` // Mount options; default: [defaults]
	State   string   `yaml:"state" json:"state,omitempty"`     // mounted|unmounted|fstab_only|absent (default: mounted)
	Dump    *int     `yaml:"dump" json:"dump,omitempty"`       // fstab dump field; default 0
	Pass    *int     `yaml:"pass" json:"pass,omitempty"`       // fstab pass field; default 0
	Backup  *bool    `yaml:"backup" json:"backup,omitempty"`   // Snapshot /etc/fstab to /etc/fstab.bak.<ts> before write; default true
}

// ContainerImage represents a container image management operation.
// Ensures an image reference is present (or absent) in local storage of
// the selected container runtime (podman/docker).
type ContainerImage struct {
	Name      string `yaml:"name" json:"name"`                       // Image ref (required), e.g. "alpine:3.20" or "ghcr.io/x/y@sha256:..."
	State     string `yaml:"state" json:"state,omitempty"`           // present|absent (default: present)
	ForcePull bool   `yaml:"force_pull" json:"force_pull,omitempty"` // When state=present, pull even if image is already local
	Runtime   string `yaml:"runtime" json:"runtime,omitempty"`       // podman|docker (auto-detected if empty; podman preferred)
}

// Container represents a container lifecycle operation.
// Idempotency is keyed by container name: if the named container is
// already in the desired state, the action is a no-op. Image and spec
// drift triggers recreation when state is running/stopped.
type Container struct {
	Name     string            `yaml:"name" json:"name"`                   // Container name (required, idempotency key)
	Image    string            `yaml:"image" json:"image,omitempty"`       // Image ref; required for state=running|stopped
	State    string            `yaml:"state" json:"state,omitempty"`       // running|stopped|absent (default: running)
	Command  []string          `yaml:"command" json:"command,omitempty"`   // Override image CMD
	Env      map[string]string `yaml:"env" json:"env,omitempty"`           // Environment variables
	Ports    []string          `yaml:"ports" json:"ports,omitempty"`       // "host:container[/proto]" entries
	Volumes  []string          `yaml:"volumes" json:"volumes,omitempty"`   // "host:container[:opts]" entries
	Network  string            `yaml:"network" json:"network,omitempty"`   // Engine network name
	Restart  string            `yaml:"restart" json:"restart,omitempty"`   // no|on-failure|always|unless-stopped
	Recreate bool              `yaml:"recreate" json:"recreate,omitempty"` // Force recreate even when spec matches
	Runtime  string            `yaml:"runtime" json:"runtime,omitempty"`   // podman|docker (auto-detected if empty)
	Extra    []string          `yaml:"extra" json:"extra,omitempty"`       // Extra args appended before image
}

// Assert represents an assertion/verification operation in a configuration step.
// Assertions always have changed: false and fail if the assertion doesn't pass.
// Supports three types: command (exit code), file (content/existence), and http (response).
type Assert struct {
	Command    *AssertCommand    `yaml:"command" json:"command,omitempty"`         // Command assertion
	File       *AssertFile       `yaml:"file" json:"file,omitempty"`               // File assertion
	HTTP       *AssertHTTP       `yaml:"http" json:"http,omitempty"`               // HTTP assertion
	FileSHA256 *AssertFileSHA256 `yaml:"file_sha256" json:"file_sha256,omitempty"` // File checksum assertion
	GitClean   *AssertGitClean   `yaml:"git_clean" json:"git_clean,omitempty"`     // Git clean working tree assertion
	GitDiff    *AssertGitDiff    `yaml:"git_diff" json:"git_diff,omitempty"`       // Git diff assertion
}

// AssertCommand verifies a command exits with the expected code.
type AssertCommand struct {
	Cmd      string `yaml:"cmd" json:"cmd"`                       // Command to execute (required)
	ExitCode int    `yaml:"exit_code" json:"exit_code,omitempty"` // Expected exit code (default: 0)
}

// AssertFile verifies file existence, content, or properties.
type AssertFile struct {
	Path     string  `yaml:"path" json:"path" plan:"path"`       // File path (required)
	Exists   *bool   `yaml:"exists" json:"exists,omitempty"`     // Verify existence (true) or non-existence (false)
	Content  *string `yaml:"content" json:"content,omitempty"`   // Expected exact content
	Contains *string `yaml:"contains" json:"contains,omitempty"` // Expected substring
	Mode     *string `yaml:"mode" json:"mode,omitempty"`         // Expected file permissions (e.g., "0644")
	Owner    *string `yaml:"owner" json:"owner,omitempty"`       // Expected owner (username or UID)
	Group    *string `yaml:"group" json:"group,omitempty"`       // Expected group (groupname or GID)
}

// AssertHTTP verifies HTTP response status, headers, or body content.
type AssertHTTP struct {
	URL        string            `yaml:"url" json:"url"`                                 // URL to request (required)
	Method     string            `yaml:"method" json:"method,omitempty"`                 // HTTP method (default: GET)
	Status     int               `yaml:"status" json:"status,omitempty"`                 // Expected status code (default: 200)
	Headers    map[string]string `yaml:"headers" json:"headers,omitempty"`               // Request headers
	Body       *string           `yaml:"body" json:"body,omitempty"`                     // Request body
	Contains   *string           `yaml:"contains" json:"contains,omitempty"`             // Expected response body substring
	BodyEquals *string           `yaml:"body_equals" json:"body_equals,omitempty"`       // Expected exact response body
	JSONPath   *string           `yaml:"jsonpath" json:"jsonpath,omitempty"`             // JSONPath expression for response body
	JSONValue  interface{}       `yaml:"jsonpath_value" json:"jsonpath_value,omitempty"` // Expected value at JSONPath
	Timeout    string            `yaml:"timeout" json:"timeout,omitempty"`               // Request timeout (e.g., "30s")
}

// AssertFileSHA256 verifies a file's SHA256 checksum matches the expected value.
type AssertFileSHA256 struct {
	Path     string `yaml:"path" json:"path" plan:"path"` // File path (required)
	Checksum string `yaml:"checksum" json:"checksum"`     // Expected SHA256 checksum (required)
}

// AssertGitClean verifies the git working tree is clean (no uncommitted changes).
type AssertGitClean struct {
	AllowUntracked bool `yaml:"allow_untracked" json:"allow_untracked,omitempty"` // Allow untracked files (default: false)
}

// AssertGitDiff verifies the git diff matches the expected unified diff.
type AssertGitDiff struct {
	ExpectedDiff string  `yaml:"expected_diff" json:"expected_diff"` // Expected diff content (required)
	Cached       bool    `yaml:"cached" json:"cached,omitempty"`     // Check staged changes (default: false, checks working tree)
	Files        *string `yaml:"files" json:"files,omitempty"`       // Limit diff to specific files/paths (optional)
}

// PresetDefinition represents a reusable preset loaded from a YAML file.
// Presets are parameterized collections of steps that can be invoked as a single action.
//
// Either `props:` (preferred, spec-67) or `parameters:` (deprecated) may declare the
// inputs. Both unmarshal into the Parameters field; UsedParametersKey records which
// form the source file used so the loader can emit a deprecation warning.
type PresetDefinition struct {
	Name        string                     `yaml:"name" json:"name"`                         // Preset name (required)
	Description string                     `yaml:"description" json:"description,omitempty"` // Human-readable description
	Version     string                     `yaml:"version" json:"version,omitempty"`         // Semantic version
	Parameters  map[string]PresetParameter `yaml:"-" json:"parameters,omitempty"`            // Parameter/prop definitions (parsed from `props:` or `parameters:`)
	Steps       []Step                     `yaml:"steps" json:"steps"`                       // Steps to execute
	BaseDir     string                     `yaml:"-" json:"-"`                               // Base directory for relative paths (set by loader)
	// UsedParametersKey is true when the source file declared inputs under
	// `parameters:` rather than `props:`. Set by UnmarshalYAML; the loader
	// uses it to emit a one-time deprecation warning.
	UsedParametersKey bool `yaml:"-" json:"-"`
}

// UnmarshalYAML accepts both `props:` (preferred) and `parameters:` (deprecated)
// keys for the parameter map. If both are present, `props:` wins and the conflict
// is reported as an error.
func (p *PresetDefinition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type raw struct {
		Name        string                     `yaml:"name"`
		Description string                     `yaml:"description"`
		Version     string                     `yaml:"version"`
		Props       map[string]PresetParameter `yaml:"props"`
		Parameters  map[string]PresetParameter `yaml:"parameters"`
		Steps       []Step                     `yaml:"steps"`
	}
	var r raw
	if err := unmarshal(&r); err != nil {
		return err
	}
	p.Name = r.Name
	p.Description = r.Description
	p.Version = r.Version
	p.Steps = r.Steps
	switch {
	case r.Props != nil && r.Parameters != nil:
		return fmt.Errorf("preset declares both `props:` and `parameters:` — use `props:` only (`parameters:` is deprecated)")
	case r.Props != nil:
		p.Parameters = r.Props
	case r.Parameters != nil:
		p.Parameters = r.Parameters
		p.UsedParametersKey = true
	}
	return nil
}

// PresetParameter defines a parameter that can be passed to a preset.
type PresetParameter struct {
	Type        string        `yaml:"type" json:"type"`                         // string|bool|array|object
	Required    bool          `yaml:"required" json:"required,omitempty"`       // Whether parameter is required
	Default     interface{}   `yaml:"default" json:"default,omitempty"`         // Default value if not provided
	Enum        []interface{} `yaml:"enum" json:"enum,omitempty"`               // Valid values (if restricted)
	Description string        `yaml:"description" json:"description,omitempty"` // Human-readable description
}

// PresetInvocation represents a user's invocation of a reusable component via
// `use:` in their playbook. Originally only handled `use: <preset-name>`; spec-67
// generalises it so Name may also be a local path (`./foo.yml`), an alias from
// the playbook's `modules:` block (`postgres` or `postgres/backup`), or an inline
// remote module reference (`github.com/owner/repo@v1.0.0`).
//
// ComponentRef is the spec-67 name for this type and is kept as an alias.
type PresetInvocation struct {
	Name string                 `yaml:"name" json:"name"`           // Preset/component reference (required)
	With map[string]interface{} `yaml:"with" json:"with,omitempty"` // Parameter / prop values
}

// ComponentRef is the spec-67 name for PresetInvocation. New code should
// prefer this alias; the old name is kept to avoid a wide rename.
type ComponentRef = PresetInvocation

// ComponentRefKind identifies which dispatch path a use: reference takes.
type ComponentRefKind int

const (
	// ComponentRefPreset is a bare name with no slash and no scheme — looked up
	// in the preset search paths (legacy behavior).
	ComponentRefPreset ComponentRefKind = iota
	// ComponentRefLocalPath is a filesystem path: starts with "./", "../", or "/".
	ComponentRefLocalPath
	// ComponentRefRemote is a fully qualified module reference: contains "@".
	ComponentRefRemote
	// ComponentRefAlias is a module alias from the modules: block, optionally
	// with an export name after a slash: "postgres" or "postgres/backup".
	ComponentRefAlias
)

// Kind classifies the reference form. The classification is purely syntactic;
// alias-vs-preset disambiguation requires the playbook's modules: map and is
// done by the executor, not here.
func (p *PresetInvocation) Kind() ComponentRefKind {
	if p == nil || p.Name == "" {
		return ComponentRefPreset
	}
	if strings.Contains(p.Name, "@") {
		return ComponentRefRemote
	}
	if strings.HasPrefix(p.Name, "./") || strings.HasPrefix(p.Name, "../") || strings.HasPrefix(p.Name, "/") {
		return ComponentRefLocalPath
	}
	if strings.Contains(p.Name, "/") {
		return ComponentRefAlias
	}
	return ComponentRefPreset
}

// SplitAlias decomposes an alias-form reference into (alias, export).
// "postgres" → ("postgres", "default"); "postgres/backup" → ("postgres", "backup").
// Returns ("", "") if the reference is empty or not in alias form
// (local paths and remote refs are excluded).
func (p *PresetInvocation) SplitAlias() (alias, export string) {
	if p == nil || p.Name == "" {
		return "", ""
	}
	k := p.Kind()
	if k != ComponentRefAlias && k != ComponentRefPreset {
		return "", ""
	}
	name := p.Name
	if i := strings.Index(name, "/"); i >= 0 {
		return name[:i], name[i+1:]
	}
	return name, "default"
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
//
// Three forms are supported. Each is independently usable; combinations
// render in a consistent order (Title → Msg → Data):
//
//   - Msg  (string): free-text line, supports template rendering. The
//     historical form; behaves exactly as before when Data is unset.
//   - Data (any):    structured payload — map, slice, scalar. Rendered
//     by Format ("kv" default, "json" for pretty-printed JSON).
//   - Title (string): optional header line shown above Data in kv mode.
//
// Budget bounds total rendered output size (bytes). Defaults to 4096
// when unset; 0 disables truncation. Larger payloads truncate at a UTF-8
// boundary with a "… (truncated)" suffix so agents don't blow context.
type PrintAction struct {
	Msg    string `yaml:"msg,omitempty"    json:"msg,omitempty"    schema:"description=Free-text message (supports templates)"`
	Title  string `yaml:"title,omitempty"  json:"title,omitempty"  schema:"description=Optional header above Data (kv mode only)"`
	Data   any    `yaml:"data,omitempty"   json:"data,omitempty"   schema:"description=Structured payload to render"`
	Format string `yaml:"format,omitempty" json:"format,omitempty" schema:"description=Render format for Data,enum=kv|json,default=kv"`
	Budget int    `yaml:"budget,omitempty" json:"budget,omitempty" schema:"description=Max bytes of rendered output; 0 disables truncation"`
}

// FileReplace represents an in-place text replacement operation in a file.
// Supports both literal and regex-based search-and-replace patterns.
type FileReplace struct {
	Path         string        `yaml:"path" json:"path" plan:"path"`                   // Target file path (required)
	Pattern      string        `yaml:"pattern" json:"pattern"`                         // Search pattern (required)
	Replace      string        `yaml:"replace" json:"replace"`                         // Replacement text (required)
	Count        *int          `yaml:"count" json:"count,omitempty"`                   // Max replacements (null = all)
	Flags        *ReplaceFlags `yaml:"flags" json:"flags,omitempty"`                   // Regex flags
	AllowNoMatch bool          `yaml:"allow_no_match" json:"allow_no_match,omitempty"` // Don't fail if no matches found
	Backup       bool          `yaml:"backup" json:"backup,omitempty"`                 // Create .bak before modify
}

// ReplaceFlags configures text replacement behavior.
type ReplaceFlags struct {
	Regex           bool `yaml:"regex" json:"regex,omitempty"`                       // Enable regex mode (default: true)
	Multiline       bool `yaml:"multiline" json:"multiline,omitempty"`               // ^ and $ match line boundaries
	CaseInsensitive bool `yaml:"case_insensitive" json:"case_insensitive,omitempty"` // Case-insensitive matching
}

// FileInsert represents an anchor-based text insertion operation in a file.
// Inserts content before or after a matched anchor pattern.
type FileInsert struct {
	Path          string `yaml:"path" json:"path" plan:"path"`                   // Target file path (required)
	Anchor        string `yaml:"anchor" json:"anchor"`                           // Anchor pattern to match (required)
	Position      string `yaml:"position" json:"position"`                       // Insert position: "before" or "after" (required)
	Content       string `yaml:"content" json:"content"`                         // Content to insert (required)
	Regex         bool   `yaml:"regex" json:"regex,omitempty"`                   // Treat anchor as regex (default: false)
	AllowMultiple bool   `yaml:"allow_multiple" json:"allow_multiple,omitempty"` // Insert at all matches (default: false)
	Backup        bool   `yaml:"backup" json:"backup,omitempty"`                 // Create .bak before modify
}

// FileDeleteRange represents a range deletion operation between two anchor patterns.
// Deletes all lines between (and optionally including) start and end anchors.
type FileDeleteRange struct {
	Path        string `yaml:"path" json:"path" plan:"path"`         // Target file path (required)
	StartAnchor string `yaml:"start_anchor" json:"start_anchor"`     // Start anchor pattern (required)
	EndAnchor   string `yaml:"end_anchor" json:"end_anchor"`         // End anchor pattern (required)
	Regex       bool   `yaml:"regex" json:"regex,omitempty"`         // Treat anchors as regex (default: false)
	Inclusive   bool   `yaml:"inclusive" json:"inclusive,omitempty"` // Include anchor lines in deletion (default: false)
	Backup      bool   `yaml:"backup" json:"backup,omitempty"`       // Create .bak before modify
}

// TextLine represents an "ensure this line is present/absent in this file"
// operation, optionally anchored by regex and/or positioned with
// insert_after / insert_before. Idempotent: a second run produces a
// byte-identical file.
type TextLine struct {
	Path         string `yaml:"path" json:"path" plan:"path"`                 // Target file path (required)
	Line         string `yaml:"line" json:"line,omitempty"`                   // The line content (required for state=present; optional for state=absent if regexp given)
	State        string `yaml:"state" json:"state,omitempty"`                 // present|absent (default: present)
	Regexp       string `yaml:"regexp" json:"regexp,omitempty"`               // Anchor regex: matched line is replaced (present) or removed (absent)
	InsertAfter  string `yaml:"insert_after" json:"insert_after,omitempty"`   // Regex; if regexp doesn't match, insert `line` after this anchor
	InsertBefore string `yaml:"insert_before" json:"insert_before,omitempty"` // Regex; if regexp doesn't match, insert `line` before this anchor
	Backup       bool   `yaml:"backup" json:"backup,omitempty"`               // Create .bak before modify
}

// TextPatchINI represents structural section/key edits to an INI-style
// configuration file (php.ini, systemd unit files, ssh_config, ...).
// Keys take the form "Section.key" for `[Section]` files; a bare key
// (no dot) targets the top level for sectionless variants. Set values
// are written as-is; delete removes matching key lines. Comments,
// blank lines, section ordering, indentation of untouched keys, and
// line endings (LF vs CRLF) are preserved across edits, so a second
// run with the same desired state is byte-identical.
type TextPatchINI struct {
	Path   string            `yaml:"path" json:"path" plan:"path"`   // Target file path (required)
	Set    map[string]string `yaml:"set" json:"set,omitempty"`       // Map of "Section.key" → value (literal; not quoted)
	Delete []string          `yaml:"delete" json:"delete,omitempty"` // List of "Section.key" entries to remove
	Backup bool              `yaml:"backup" json:"backup,omitempty"` // Create .bak before modify
}

// TextPatchJSON represents structural edits to a JSON file via a
// JSONPath-lite subset (`a.b.c`, `a[0]`, `a.b[3].c`). Three operations
// are supported and applied in order: `set` upserts a value at a path;
// `delete` removes keys/elements at the listed paths; `merge` applies
// object/array merges (arrays follow the chosen `merge_strategy`).
// Key order, indentation, and the trailing newline of the source file
// are preserved so a second run with byte-identical desired state is a
// true no-op (Changed=false, no write).
type TextPatchJSON struct {
	Path          string                 `yaml:"path" json:"path" plan:"path"`                   // Target file path (required)
	Set           map[string]interface{} `yaml:"set" json:"set,omitempty"`                       // Path → value upserts
	Delete        []string               `yaml:"delete" json:"delete,omitempty"`                 // Paths to remove
	Merge         map[string]interface{} `yaml:"merge" json:"merge,omitempty"`                   // Path → value merges (object deep-set; array per strategy)
	MergeStrategy string                 `yaml:"merge_strategy" json:"merge_strategy,omitempty"` // append_unique|replace|append (default: append_unique)
	Backup        bool                   `yaml:"backup" json:"backup,omitempty"`                 // Create .bak before modify
}

// TextPatchYAML represents structural edits to a YAML file via a small
// dotted + indexed path subset (`a.b.c`, `a[0]`, `a.b[3].c`). Three
// operations are supported and applied in order: `set` upserts a value
// at a path; `delete` removes keys/elements at the listed paths;
// `merge` deep-merges objects (non-destructive on existing keys) or
// arrays (per `merge_strategy`). Edits go through gopkg.in/yaml.v3's
// node API to preserve key order and comments adjacent to unchanged
// nodes. Idempotent: a second run with byte-identical desired state
// writes nothing.
type TextPatchYAML struct {
	Path          string                 `yaml:"path" json:"path" plan:"path"`                   // Target file path (required)
	Set           map[string]interface{} `yaml:"set" json:"set,omitempty"`                       // Path → value upserts
	Delete        []string               `yaml:"delete" json:"delete,omitempty"`                 // Paths to remove
	Merge         map[string]interface{} `yaml:"merge" json:"merge,omitempty"`                   // Path → value merges
	MergeStrategy string                 `yaml:"merge_strategy" json:"merge_strategy,omitempty"` // append_unique|replace|append (default: append_unique)
	Backup        bool                   `yaml:"backup" json:"backup,omitempty"`                 // Create .bak before modify
}

// FilePatchApply represents a unified diff patch application operation.
// Applies a unified diff patch to a file with validation and safety checks.
type FilePatchApply struct {
	Path         string `yaml:"path" json:"path" plan:"path"`                       // Target file path (required)
	Patch        string `yaml:"patch" json:"patch,omitempty"`                       // Inline patch content (patch or patch_file required)
	PatchFile    string `yaml:"patch_file" json:"patch_file,omitempty" plan:"path"` // Path to patch file (patch or patch_file required)
	Backup       bool   `yaml:"backup" json:"backup,omitempty"`                     // Create .bak before modify
	ContextLines *int   `yaml:"context_lines" json:"context_lines,omitempty"`       // Required matching context lines (default: 3)
	Strict       bool   `yaml:"strict" json:"strict,omitempty"`                     // Strict mode: fail if any hunk fails (default: true)
	DryRun       bool   `yaml:"dry_run" json:"dry_run,omitempty"`                   // Test patch without applying
}

// ReadFile is the shared shape for `read.json` and `read.yaml` (spec-38).
// Read-only by contract: parses the file, optionally extracts a value by
// pathquery path, optionally applies redaction patterns to string leaves
// in the parsed value before publishing it.
type ReadFile struct {
	Path     string   `yaml:"path"      json:"path"                  plan:"path"` // required
	Query    string   `yaml:"query"     json:"query,omitempty"`                   // pathquery syntax (dotted + integer indices)
	MaxBytes *int64   `yaml:"max_bytes" json:"max_bytes,omitempty"`               // default 4 MiB when nil
	Redact   []string `yaml:"redact"    json:"redact,omitempty"`                  // regex patterns applied to string leaves
}

// RepoSearch represents a codebase search operation.
// Searches files for patterns and outputs results in JSON format.
type RepoSearch struct {
	Pattern    string   `yaml:"pattern" json:"pattern"`                               // Search pattern (required)
	Regex      bool     `yaml:"regex" json:"regex,omitempty"`                         // Use regex mode (default: true)
	Glob       string   `yaml:"glob" json:"glob,omitempty"`                           // File glob pattern (e.g., "**/*.{ts,js}")
	Path       string   `yaml:"path" json:"path,omitempty" plan:"path"`               // Search path (default: current directory)
	OutputFile string   `yaml:"output_file" json:"output_file,omitempty" plan:"path"` // Output JSON file path
	MaxResults *int     `yaml:"max_results" json:"max_results,omitempty"`             // Maximum number of results (null = unlimited)
	IgnoreDirs []string `yaml:"ignore_dirs" json:"ignore_dirs,omitempty"`             // Directories to ignore (e.g., [".git", "node_modules"])
}

// RepoTree represents a repository tree generation operation.
// Generates a JSON representation of the directory structure.
type RepoTree struct {
	Path         string   `yaml:"path" json:"path,omitempty" plan:"path"`               // Root path (default: current directory)
	MaxDepth     *int     `yaml:"max_depth" json:"max_depth,omitempty"`                 // Maximum directory depth (null = unlimited)
	ExcludeDirs  []string `yaml:"exclude_dirs" json:"exclude_dirs,omitempty"`           // Directories to exclude (e.g., ["node_modules", ".git"])
	OutputFile   string   `yaml:"output_file" json:"output_file,omitempty" plan:"path"` // Output JSON file path
	IncludeFiles *bool    `yaml:"include_files" json:"include_files,omitempty"`         // Include files in tree (default: true; explicit false opts out)
}

// RepoApplyPatchset represents a multi-file patch application operation.
// Applies multiple patches to multiple files in a single atomic operation.
type RepoApplyPatchset struct {
	Patchset     string `yaml:"patchset" json:"patchset,omitempty"`                       // Inline patchset content (patchset or patchset_file required)
	PatchsetFile string `yaml:"patchset_file" json:"patchset_file,omitempty" plan:"path"` // Path to patchset file (patchset or patchset_file required)
	BaseDir      string `yaml:"base_dir" json:"base_dir,omitempty" plan:"path"`           // Base directory for relative paths (default: current directory)
	Backup       bool   `yaml:"backup" json:"backup,omitempty"`                           // Create .bak files before modifications
	Strict       bool   `yaml:"strict" json:"strict,omitempty"`                           // Strict mode: rollback all if any file fails (default: true)
	DryRun       bool   `yaml:"dry_run" json:"dry_run,omitempty"`                         // Test patchset without applying
	OutputFile   string `yaml:"output_file" json:"output_file,omitempty" plan:"path"`     // Output JSON file with results
}

// ArtifactCapture wraps steps and captures all file changes with enhanced metadata.
// Designed for LLM agent loops to provide structured output for decision-making.
type ArtifactCapture struct {
	Name             string `yaml:"name" json:"name"`                                     // Artifact name (required)
	OutputDir        string `yaml:"output_dir" json:"output_dir,omitempty" plan:"path"`   // Output directory (default: "./artifacts")
	Format           string `yaml:"format" json:"format,omitempty"`                       // Output format: "json", "markdown", "both" (default: "both")
	CaptureContent   bool   `yaml:"capture_content" json:"capture_content,omitempty"`     // Include before/after file content (default: false)
	MaxDiffSize      int    `yaml:"max_diff_size" json:"max_diff_size,omitempty"`         // Max diff size in bytes (default: 1MB)
	IncludeChecksums bool   `yaml:"include_checksums" json:"include_checksums,omitempty"` // Include file checksums (default: true)
	EmbedPlan        *bool  `yaml:"embed_plan" json:"embed_plan,omitempty"`               // Embed full plan in artifact (default: true if steps <= max_plan_steps)
	MaxPlanSteps     int    `yaml:"max_plan_steps" json:"max_plan_steps,omitempty"`       // Don't embed if plan exceeds this many steps (default: 20)
	Steps            []Step `yaml:"steps" json:"steps"`                                   // Steps to execute and capture (required)
}

// ObservePort is the spec-59 single-shot read of TCP/UDP port state.
// The polling cousin is wait.port; observe.port returns the current
// state once and lets the next step branch on it via spec-37 `as:`
// capture. Read-only by contract — Changed=false, empty Diff, nil
// Reverse, Cost{Risk:1, Reversible:true}.
type ObservePort struct {
	Host     string `yaml:"host" json:"host,omitempty"`         // Host to probe (default: "localhost")
	Port     int    `yaml:"port" json:"port"`                   // Port (required)
	Protocol string `yaml:"protocol" json:"protocol,omitempty"` // "tcp" (default) | "udp"
	Timeout  string `yaml:"timeout" json:"timeout,omitempty"`   // Dial timeout (default: "2s")
}

// ObserveProcess is the spec-59 single-shot read of process state.
// Selector is either Name (exact match against process basename) or
// Pattern (regex against full argv). At least one must be set.
type ObserveProcess struct {
	Name    string `yaml:"name" json:"name,omitempty"`       // Exact match against process basename
	Pattern string `yaml:"pattern" json:"pattern,omitempty"` // Regex against full argv (alternative to name)
}

// ObserveHTTP is the spec-59 single-shot HTTP GET observation.
// Network-flagged via Permissions{Network:true}. Body sample is
// capped at 2048 bytes; headers are filtered to CaptureHeaders.
type ObserveHTTP struct {
	URL            string   `yaml:"url" json:"url"`                                   // Target URL (required)
	Method         string   `yaml:"method" json:"method,omitempty"`                   // HTTP method (default: "GET")
	Timeout        string   `yaml:"timeout" json:"timeout,omitempty"`                 // Request timeout (default: "3s")
	ExpectStatus   int      `yaml:"expect_status" json:"expect_status,omitempty"`     // If set, Found=false when StatusCode != ExpectStatus
	CaptureHeaders []string `yaml:"capture_headers" json:"capture_headers,omitempty"` // Headers to expose in HTTPObservation.Headers
	SkipTLSVerify  bool     `yaml:"skip_tls_verify" json:"skip_tls_verify,omitempty"` // Disable cert verification (use with care)

	// FollowRedirects bounds how many redirects the client will follow
	// before giving up. Pointer so the zero value (no follow) is
	// distinguishable from "unset" (default = 10, matches Go's
	// http.Client default). Set to 0 to probe the redirect itself
	// (canonical use: pair with `expect_status: 301` to verify an
	// HTTP→HTTPS redirect is still in place). Issue #18.
	FollowRedirects *int `yaml:"follow_redirects,omitempty" json:"follow_redirects,omitempty"`
}

// ObserveService is the spec-59 single-shot read of init-system
// service state. systemd on Linux, launchd on macOS, sysv fallback
// elsewhere. Manager defaults to "auto" (detect from facts).
type ObserveService struct {
	Name    string `yaml:"name" json:"name"`                 // Service unit name (required)
	Manager string `yaml:"manager" json:"manager,omitempty"` // "systemd" | "launchd" | "auto"
}

// ObserveCPU is the spec-60 single-shot read of CPU utilization +
// load averages. Pulls from the shared internal/metrics collector.
type ObserveCPU struct{}

// ObserveMemory is the spec-60 single-shot read of RAM / swap state.
// Total + Used + Free + Available + Swap fields are read directly
// from /proc/meminfo on Linux, sysctl on macOS.
type ObserveMemory struct{}

// ObserveDisk is the spec-60 single-shot read of a filesystem path.
// Path defaults to "/" if unset. ReadOnly and inode counts are
// best-effort (platform-dependent).
type ObserveDisk struct {
	Path string `yaml:"path" json:"path,omitempty"` // Filesystem path (default: "/")
}

// ObserveGPU is the spec-62 single-shot read of GPU utilization +
// memory. Wraps the shared internal/metrics collector so /v1/metrics
// and observe.gpu share one nvidia-smi/powermetrics sample. Index
// selects one GPU; unset returns all detected with an aggregate view.
type ObserveGPU struct {
	Index *int `yaml:"index" json:"index,omitempty"` // Specific GPU index (default: all)
}

// ObserveLogs is the spec-61 single-shot read of a log source within
// a time / line window. Exactly one of Path / JournalUnit / Container
// must be set. Patterns are regexes evaluated line-by-line; per-pattern
// match counts + sample lines (capped) are returned in the typed
// LogObservation.
type ObserveLogs struct {
	// Source selectors — exactly one must be set.
	Path        string `yaml:"path" json:"path,omitempty"`                 // Log file path (tail mode)
	JournalUnit string `yaml:"journal_unit" json:"journal_unit,omitempty"` // systemd unit (Linux)
	Container   string `yaml:"container" json:"container,omitempty"`       // Docker/Podman container name

	// Window — relative duration (e.g. "60s", "5m"). Default "60s".
	Since string `yaml:"since" json:"since,omitempty"`

	// Patterns are regexes matched against each line. Required.
	Patterns []string `yaml:"patterns" json:"patterns"`

	// SampleLines caps the per-pattern sample lines retained in the
	// LogMatchGroup. Default 5.
	SampleLines int `yaml:"sample_lines" json:"sample_lines,omitempty"`

	// MaxBytes caps the total bytes read. Default 1 MiB.
	MaxBytes int64 `yaml:"max_bytes" json:"max_bytes,omitempty"`

	// MaxLines caps the total lines scanned. Default 10000.
	MaxLines int `yaml:"max_lines" json:"max_lines,omitempty"`
}

// WaitPort waits for a TCP port to accept connections.
// Useful for orchestrating service start → port open → next step.
type WaitPort struct {
	Host         string `yaml:"host" json:"host,omitempty"`                   // Host to dial (default: "localhost")
	Port         int    `yaml:"port" json:"port"`                             // TCP port (required)
	Timeout      string `yaml:"timeout" json:"timeout,omitempty"`             // Total timeout duration (default: "60s")
	PollInterval string `yaml:"poll_interval" json:"poll_interval,omitempty"` // Time between dial attempts (default: "1s")
	// Interval is an alias for PollInterval (MT-42). Authors instinctively
	// write `interval:`; without this the field is silently dropped and
	// the default 1s is used. Handler precedence: PollInterval wins if
	// both are set.
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// WaitHTTP waits for an HTTP endpoint to return one of the accepted
// status codes, optionally with a substring match on the body.
type WaitHTTP struct {
	URL          string `yaml:"url" json:"url"`                               // Target URL (required)
	Method       string `yaml:"method" json:"method,omitempty"`               // HTTP method (default: "GET")
	Status       []int  `yaml:"status" json:"status,omitempty"`               // Accepted status codes (default: [200])
	BodyContains string `yaml:"body_contains" json:"body_contains,omitempty"` // Optional substring required in body
	// Body is the optional request payload sent with every poll —
	// proposal-10's motivating case is "poll POST /v1/embeddings
	// with a JSON payload because the service has no GET /healthz."
	// Goes through the template renderer so `{{ var }}` interpolation
	// works the same way it does for URL / Headers / BodyContains.
	// Raw-string shape (vs structured) matches what download:/
	// pkg.repo: do for inline content and keeps the user in charge
	// of JSON escaping. Caller is responsible for setting an
	// appropriate Content-Type header — this action is the polling
	// primitive, not a JSON client.
	//
	// Side-effect note: polling a non-GET endpoint hits the handler
	// every iteration. The embedding case re-runs inference each
	// poll. The user is expected to know what they're poking.
	Body         string            `yaml:"body,omitempty" json:"body,omitempty"`
	Headers      map[string]string `yaml:"headers" json:"headers,omitempty"`             // Optional request headers
	Timeout      string            `yaml:"timeout" json:"timeout,omitempty"`             // Total timeout duration (default: "60s")
	PollInterval string            `yaml:"poll_interval" json:"poll_interval,omitempty"` // Time between requests (default: "1s")
	// Interval is an alias for PollInterval (MT-42). See WaitPort.
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// HTTPRequest is the proposal-16 first-class HTTP action. Unlike
// `file.download` (URL→file+checksum), `observe.http` (single-shot
// probe), and `wait.http` (poll until ready), HTTPRequest is the
// general "call an endpoint, capture the response as a fact" primitive
// — the action `notify` and `llm` will sit on top of.
//
// Idempotency contract: POST and PATCH calls are unsafe by default
// (the server may create or mutate state on each call). The handler's
// Validate() refuses to ship a POST/PATCH unless the user supplies
// EXACTLY ONE of:
//   - IdempotencyKey (sent as an `Idempotency-Key` header — server-side
//     dedup),
//   - CreatesWhen (template predicate evaluated against existing facts
//     — Wave 2 wires this into plan-mode probes; Wave 1 honors it as a
//     gate inside Run),
//   - Risk = "high" (explicit operator ack).
//
// GET/HEAD/OPTIONS/PUT/DELETE are exempt: GET/HEAD/OPTIONS are
// read-only by convention; PUT/DELETE are idempotent by HTTP spec.
//
// See docs-working/streams/core/proposals/proposal-16-http-request-action.md
// for full design notes and the deferred Wave-2/3 fields (probe:,
// reverse:, expect_json_schema:, save_to:).
type HTTPRequest struct {
	URL     string            `yaml:"url" json:"url"`                           // Target URL (required, template-rendered)
	Method  string            `yaml:"method,omitempty" json:"method,omitempty"` // HTTP method (default: "GET")
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Auth supplies credentials. Bearer/Basic/Header are mutually
	// exclusive — set at most one. All values flow through the
	// secret-redaction filter for logs/diffs/events.
	Auth *HTTPAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Body forms — set AT MOST ONE. Validate() rejects multiple.
	//
	//   Body — raw string (template-rendered). Caller sets Content-Type.
	//   JSON — structured value. Marshalled to JSON; Content-Type defaults
	//          to application/json. Templates inside JSON values are NOT
	//          rendered in Wave 1 (use Body with the | tojson filter for
	//          templated JSON).
	//   Form — flat string map. URL-encoded; Content-Type defaults to
	//          application/x-www-form-urlencoded.
	//   File — path to a file (template-rendered + expanded). Raw bytes
	//          sent verbatim; caller sets Content-Type.
	Body string            `yaml:"body,omitempty" json:"body,omitempty"`
	JSON interface{}       `yaml:"json,omitempty" json:"json,omitempty"`
	Form map[string]string `yaml:"form,omitempty" json:"form,omitempty"`
	File string            `yaml:"file,omitempty" json:"file,omitempty"`

	// Idempotency contract for POST/PATCH — set EXACTLY ONE. Validate()
	// refuses to ship POST/PATCH that has none of these. GET/HEAD/
	// OPTIONS/PUT/DELETE ignore these fields.
	IdempotencyKey string `yaml:"idempotency_key,omitempty" json:"idempotency_key,omitempty"`
	CreatesWhen    string `yaml:"creates_when,omitempty" json:"creates_when,omitempty"`

	// Risk overrides the per-method default (low for read methods,
	// medium for PUT/DELETE, high for POST/PATCH). Setting Risk="high"
	// also serves as the explicit ack that satisfies the idempotency
	// contract for POST/PATCH without IdempotencyKey/CreatesWhen.
	// Accepted values: "low", "medium", "high".
	Risk string `yaml:"risk,omitempty" json:"risk,omitempty"`

	// ExpectStatus lists acceptable HTTP status codes. If unset,
	// defaults to 2xx (200-299). A response outside this list fails the
	// step (after retries are exhausted).
	ExpectStatus []int `yaml:"expect_status,omitempty" json:"expect_status,omitempty"`

	// RetryOn lists HTTP-specific conditions that classify a response
	// as retryable. Attempts and delay come from the step-level
	// retry: { attempts, delay, backoff } block — the executor owns
	// the loop and the backoff curve. Accepted tokens: "5xx", "4xx",
	// "429", "timeout", "connection_error".
	RetryOn []string `yaml:"retry_on,omitempty" json:"retry_on,omitempty"`

	// Timeout bounds the per-request transfer (default "30s"). The
	// httputil DefaultTransport's dial / TLS / response-headers
	// timeouts also apply.
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// SkipTLSVerify disables certificate verification. Strongly
	// discouraged outside test fixtures.
	SkipTLSVerify bool `yaml:"skip_tls_verify,omitempty" json:"skip_tls_verify,omitempty"`

	// RedactBody = true filters request + response bodies out of logs,
	// diffs, and events. Auth headers are ALWAYS redacted regardless.
	RedactBody bool `yaml:"redact_body,omitempty" json:"redact_body,omitempty"`

	// FollowRedirects bounds how many redirects the client will follow.
	// Pointer so the zero value (no follow) is distinguishable from
	// "unset" (default = 10, matches Go's http.Client default).
	FollowRedirects *int `yaml:"follow_redirects,omitempty" json:"follow_redirects,omitempty"`

	// MaxResponseBytes caps how much of the response body is captured
	// in the registered fact. Default 1 MiB. Larger responses are
	// truncated; the body field is suffixed with "...(truncated)".
	MaxResponseBytes int64 `yaml:"max_response_bytes,omitempty" json:"max_response_bytes,omitempty"`

	// Probe is the proposal-16 Wave 2 plan-mode inspection sub-request.
	// In plan mode the handler executes the probe (single attempt, no
	// retry), exposes the response under the variable name `probe`, and
	// evaluates CreatesWhen against the merged vars to predict whether
	// this action would change state. probe.method must be GET / HEAD /
	// OPTIONS; nested probe / reverse are rejected by Validate().
	//
	// Idempotency: a probe-evaluated CreatesWhen=false in plan mode tells
	// the operator "this POST is a no-op against current state" — the
	// closest you can get to read-style idempotency for non-idempotent
	// methods without actually issuing the write.
	Probe *HTTPRequest `yaml:"probe,omitempty" json:"probe,omitempty"`

	// Reverse is the proposal-16 Wave 2 compensating sub-request. After
	// a successful apply, its templates are rendered against the
	// response fact (exposed as `response` plus, when step.As is set,
	// as `<as>.json`/`.body`/`.status_code`) and the rendered request
	// is stashed on Result.ReverseData. When a transaction layer calls
	// Reverse(), the handler returns a Step wrapping that rendered
	// request — so rollback re-issues the compensating call with the
	// exact resource ID returned by the original POST.
	//
	// Without Reverse:, Reverse() returns an explicit "not reversible"
	// error so transaction failures fail loudly rather than skip
	// rollback silently. Nested probe / reverse are rejected.
	Reverse *HTTPRequest `yaml:"reverse,omitempty" json:"reverse,omitempty"`

	// SaveTo is the proposal-16 Wave 3 response-persistence sink.
	// When set, the handler writes the response body bytes to this
	// path after a successful apply. The path is template-rendered
	// against vars (typical use: `save_to: "{{ .state_dir }}/last-
	// hook.json"`); parent directories are created as needed (mkdir
	// -p semantics). On plan-mode runs the write is skipped, only
	// the intent is reported.
	//
	// Permissions: when SaveTo is set, the handler's Permissions
	// adds FilesystemWrite=[save_to] so spec-44 doctor can flag a
	// missing parent dir or unwritable path BEFORE the network call
	// runs. Rejected inside the probe sub-block — probes are
	// read-only inspection and persisting their body confuses the
	// audit story; declare save_to on the top-level request
	// instead.
	SaveTo string `yaml:"save_to,omitempty" json:"save_to,omitempty"`

	// ExpectJSONKeys lists top-level keys that the auto-parsed
	// response JSON object must contain. After a successful response
	// (status accepted), the handler asserts response.json is a
	// JSON object AND each listed key is present (regardless of
	// value type). A missing key fails the step with a clear error
	// listing what's missing.
	//
	// This is deliberately a narrower contract than the full JSON-
	// schema validation deferred under expect_json_schema (file
	// path → draft-07 validator). The narrow check covers the most
	// common "I called POST /hooks, prove the server returned an id
	// and url" assertion without taking on a JSONSchema dependency.
	// Empty/whitespace-only entries are rejected by Validate so
	// `expect_json_keys: [""]` doesn't silently no-op.
	ExpectJSONKeys []string `yaml:"expect_json_keys,omitempty" json:"expect_json_keys,omitempty"`
}

// HTTPAuth is the one-of credential block for HTTPRequest. Set at
// most one of Bearer/Basic/Header.
type HTTPAuth struct {
	// Bearer sets "Authorization: Bearer <value>".
	Bearer string `yaml:"bearer,omitempty" json:"bearer,omitempty"`
	// Basic sets "Authorization: Basic <base64(user:pass)>".
	Basic *HTTPBasicAuth `yaml:"basic,omitempty" json:"basic,omitempty"`
	// Header sets an arbitrary header (e.g. {name: "X-Api-Key", value: "..."}).
	Header *HTTPAuthHeader `yaml:"header,omitempty" json:"header,omitempty"`
}

// HTTPBasicAuth is the user/pass pair for HTTPAuth.Basic.
type HTTPBasicAuth struct {
	User string `yaml:"user" json:"user"`
	Pass string `yaml:"pass" json:"pass"`
}

// HTTPAuthHeader is an arbitrary auth header.
type HTTPAuthHeader struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// WaitFile waits for a filesystem path to exist, optionally containing
// a substring in its contents.
type WaitFile struct {
	Path         string `yaml:"path" json:"path" plan:"path"`                 // File or directory path (required)
	Contains     string `yaml:"contains" json:"contains,omitempty"`           // Optional substring required in file contents
	Timeout      string `yaml:"timeout" json:"timeout,omitempty"`             // Total timeout duration (default: "60s")
	PollInterval string `yaml:"poll_interval" json:"poll_interval,omitempty"` // Time between checks (default: "1s")
	// Interval is an alias for PollInterval (MT-42). See WaitPort.
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// WaitCommand waits for a shell command to exit with the expected code.
type WaitCommand struct {
	Cmd          string `yaml:"cmd" json:"cmd"`                               // Shell command (required)
	ExpectExit   int    `yaml:"expect_exit" json:"expect_exit,omitempty"`     // Expected exit code (default: 0)
	Timeout      string `yaml:"timeout" json:"timeout,omitempty"`             // Total timeout duration (default: "60s")
	PollInterval string `yaml:"poll_interval" json:"poll_interval,omitempty"` // Time between attempts (default: "1s")
	// Interval is an alias for PollInterval (MT-42). See WaitPort.
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// ArtifactValidate validates artifacts against constraints (change budgets).
// Designed for LLM agent loops to enforce guardrails on file modifications.
type ArtifactValidate struct {
	ArtifactFile    string   `yaml:"artifact_file" json:"artifact_file" plan:"path"`       // Path to artifact JSON file (required)
	MaxFiles        *int     `yaml:"max_files" json:"max_files,omitempty"`                 // Maximum number of files changed
	MaxLinesChanged *int     `yaml:"max_lines_changed" json:"max_lines_changed,omitempty"` // Maximum total lines changed
	MaxFileSize     *int     `yaml:"max_file_size" json:"max_file_size,omitempty"`         // Maximum individual file size in bytes
	RequireTests    bool     `yaml:"require_tests" json:"require_tests,omitempty"`         // Require test file changes if code files changed
	AllowedPaths    []string `yaml:"allowed_paths" json:"allowed_paths,omitempty"`         // Glob patterns for allowed paths
	ForbiddenPaths  []string `yaml:"forbidden_paths" json:"forbidden_paths,omitempty"`     // Glob patterns for forbidden paths
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
//
// # Field categories
//
// USER-INPUT FIELDS — set by the operator in config YAML, never modified by
// the planner or executor. Validated by Validate(). These are the fields a
// human or generator writes when authoring a plan file.
//
// COMPOUND-STEP CONTAINERS — also user-input, but structurally exclusive with
// action fields: a step with Transaction, Try, or both is a compound wrapper
// node and must have no action pointer set. Validated by Validate(). The
// planner expands the children as siblings; the compound step itself becomes
// a structural marker (no-op in ExecuteStep).
//
// PLANNER-INTERNAL FIELDS — populated by plan expansion, never present in
// config YAML (all tagged yaml:"-" or omitempty). Carry context the executor
// needs to route compound-step children correctly (TryRole, TxnRole) or to
// correlate events (ID, TriggeredBy). Treat as read-only after the planner
// runs; writing to them in handler code is a bug.
//
// MT-75: every YAML tag carries `,omitempty` so `plan -o plan.yaml`
// doesn't print 60+ `null` action members for each step. The JSON tags
// already had it, but `gopkg.in/yaml.v3` honors omitempty independently
// — without it, every nil pointer / zero string serializes as `null` /
// `""` and a 1-step plan blows up to ~580 lines.
type Step struct {
	// Identification
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Conditionals
	When string `yaml:"when,omitempty" json:"when,omitempty"`

	// Idempotency controls (4 fields total; UnlessExists/UnlessCommand are
	// the canonical names and Creates/Unless are friendly aliases — see
	// F013 in docs-working/code-review for the policy rationale).
	UnlessExists  *string `yaml:"unless_exists,omitempty" json:"unless_exists,omitempty"`   // Skip if path exists
	UnlessCommand *string `yaml:"unless_command,omitempty" json:"unless_command,omitempty"` // Skip if command succeeds

	// Creates / Unless are the friendly step-level aliases for
	// UnlessExists / UnlessCommand (MT-15). They produce the same skip
	// semantics; the names match the colocated action-level shell
	// guards (ShellAction.Creates / ShellAction.Unless) so an operator
	// who learned the verb on one action doesn't have to relearn it as
	// `unless_exists:` at step level. Both forms remain supported; the
	// guards are unioned in checkIdempotencyConditions.
	Creates *string `yaml:"creates,omitempty" json:"creates,omitempty" plan:"path"` // Alias: skip if path exists
	Unless  *string `yaml:"unless,omitempty"  json:"unless,omitempty"`              // Alias: skip if command succeeds

	// ── USER-INPUT ────────────────────────────────────────────────────────────

	// Actions (exactly one required — enforced at runtime by Validate()).
	// The Go type system cannot express this as a compile-time union:
	// all action pointers live on the struct (run `make budget-status`
	// for the current count) and exactly one must be non-nil. Use
	// DetermineActionType() to recover the action name; never switch on
	// the fields directly.
	// Action keys are dot-namespaced by domain (spec-21):
	//   file.*    — file & content management
	//   text.*    — structured text editing
	//   os.*      — OS-level resource management
	//   pkg       — package manager (declarative; state: present/absent)
	//   cmd       — typed command execution
	//   repo.*    — repository operations
	//   artifact.* — artifact capture / validation
	// Flat keys (shell, assert, wait, vars, log, use, import) are foundational
	// or control-flow primitives that do not warrant a namespace.
	FileWrite            *File                   `yaml:"file.write,omitempty"        json:"file.write,omitempty"        action:"file.write"`
	FileTemplate         *Template               `yaml:"file.template,omitempty"     json:"file.template,omitempty"     action:"file.template"`
	FileCopy             *Copy                   `yaml:"file.copy,omitempty"         json:"file.copy,omitempty"         action:"file.copy"`
	FileDownload         *Download               `yaml:"file.download,omitempty"     json:"file.download,omitempty"     action:"file.download"`
	FileUnarchive        *Unarchive              `yaml:"file.unarchive,omitempty"    json:"file.unarchive,omitempty"    action:"file.unarchive"`
	TextLine             *TextLine               `yaml:"text.line,omitempty"         json:"text.line,omitempty"         action:"text.line"`
	TextReplace          *FileReplace            `yaml:"text.replace,omitempty"      json:"text.replace,omitempty"      action:"text.replace"`
	TextInsert           *FileInsert             `yaml:"text.insert,omitempty"       json:"text.insert,omitempty"       action:"text.insert"`
	TextDeleteRange      *FileDeleteRange        `yaml:"text.delete_range,omitempty" json:"text.delete_range,omitempty" action:"text.delete_range"`
	TextPatch            *FilePatchApply         `yaml:"text.patch,omitempty"        json:"text.patch,omitempty"        action:"text.patch"`
	TextPatchINI         *TextPatchINI           `yaml:"text.patch.ini,omitempty"    json:"text.patch.ini,omitempty"    action:"text.patch.ini"`
	TextPatchJSON        *TextPatchJSON          `yaml:"text.patch.json,omitempty"   json:"text.patch.json,omitempty"   action:"text.patch.json"`
	TextPatchYAML        *TextPatchYAML          `yaml:"text.patch.yaml,omitempty"   json:"text.patch.yaml,omitempty"   action:"text.patch.yaml"`
	Pkg                  *Package                `yaml:"pkg,omitempty"               json:"pkg,omitempty"               action:"pkg"`
	PkgRepo              *PkgRepo                `yaml:"pkg.repo,omitempty"          json:"pkg.repo,omitempty"          action:"pkg.repo"`
	PkgHold              *PkgHold                `yaml:"pkg.hold,omitempty"          json:"pkg.hold,omitempty"          action:"pkg.hold"`
	PkgUpgrade           *PkgUpgrade             `yaml:"pkg.upgrade,omitempty"       json:"pkg.upgrade,omitempty"       action:"pkg.upgrade"`
	PkgList              *PkgList                `yaml:"pkg.list,omitempty"          json:"pkg.list,omitempty"          action:"pkg.list"`
	Tool                 *Tool                   `yaml:"tool,omitempty"              json:"tool,omitempty"              action:"tool"`
	OsService            *ServiceAction          `yaml:"os.service,omitempty"        json:"os.service,omitempty"        action:"os.service"`
	OsUser               *OsUser                 `yaml:"os.user,omitempty"           json:"os.user,omitempty"           action:"os.user"`
	OsGroup              *OsGroup                `yaml:"os.group,omitempty"          json:"os.group,omitempty"          action:"os.group"`
	OsSSHKey             *OsSSHKey               `yaml:"os.ssh_key,omitempty"        json:"os.ssh_key,omitempty"        action:"os.ssh_key"`
	OsCron               *OsCron                 `yaml:"os.cron,omitempty"           json:"os.cron,omitempty"           action:"os.cron"`
	OsSysctl             *OsSysctl               `yaml:"os.sysctl,omitempty"         json:"os.sysctl,omitempty"         action:"os.sysctl"`
	OsSystemd            *OsSystemd              `yaml:"os.systemd,omitempty"        json:"os.systemd,omitempty"        action:"os.systemd"`
	OsMount              *OsMount                `yaml:"os.mount,omitempty"          json:"os.mount,omitempty"          action:"os.mount"`
	OsFirewall           *OsFirewall             `yaml:"os.firewall,omitempty"       json:"os.firewall,omitempty"       action:"os.firewall"`
	WindowsFirewallRule  *WindowsFirewallRule    `yaml:"windows.firewall_rule,omitempty"   json:"windows.firewall_rule,omitempty"   action:"windows.firewall_rule"`
	WindowsScheduledTask *WindowsScheduledTask   `yaml:"windows.scheduled_task,omitempty"  json:"windows.scheduled_task,omitempty"  action:"windows.scheduled_task"`
	ContainerImage       *ContainerImage         `yaml:"container.image,omitempty"   json:"container.image,omitempty"   action:"container.image"`
	Container            *Container              `yaml:"container,omitempty"         json:"container,omitempty"         action:"container"`
	Cmd                  *CommandAction          `yaml:"cmd,omitempty"               json:"cmd,omitempty"               action:"cmd"`
	RepoSearch           *RepoSearch             `yaml:"repo.search,omitempty"       json:"repo.search,omitempty"       action:"repo.search"`
	RepoTree             *RepoTree               `yaml:"repo.tree,omitempty"         json:"repo.tree,omitempty"         action:"repo.tree"`
	ReadJSON             *ReadFile               `yaml:"read.json,omitempty"         json:"read.json,omitempty"         action:"read.json"`
	ReadYAML             *ReadFile               `yaml:"read.yaml,omitempty"         json:"read.yaml,omitempty"         action:"read.yaml"`
	RepoPatch            *RepoApplyPatchset      `yaml:"repo.patch,omitempty"        json:"repo.patch,omitempty"        action:"repo.patch"`
	GitClone             *GitClone               `yaml:"git.clone,omitempty"         json:"git.clone,omitempty"         action:"git.clone"`
	GitCheckout          *GitCheckout            `yaml:"git.checkout,omitempty"      json:"git.checkout,omitempty"      action:"git.checkout"`
	GitConfig            *GitConfig              `yaml:"git.config,omitempty"        json:"git.config,omitempty"        action:"git.config"`
	ArtifactCapture      *ArtifactCapture        `yaml:"artifact.capture,omitempty"  json:"artifact.capture,omitempty"  action:"artifact.capture"`
	ArtifactValidate     *ArtifactValidate       `yaml:"artifact.validate,omitempty" json:"artifact.validate,omitempty" action:"artifact.validate"`
	Shell                *ShellAction            `yaml:"shell,omitempty"             json:"shell,omitempty"             action:"shell"`
	Assert               *Assert                 `yaml:"assert,omitempty"            json:"assert,omitempty"            action:"assert"`
	ObservePort          *ObservePort            `yaml:"observe.port,omitempty"      json:"observe.port,omitempty"      action:"observe.port"`
	ObserveProcess       *ObserveProcess         `yaml:"observe.process,omitempty"   json:"observe.process,omitempty"   action:"observe.process"`
	ObserveHTTP          *ObserveHTTP            `yaml:"observe.http,omitempty"      json:"observe.http,omitempty"      action:"observe.http"`
	ObserveService       *ObserveService         `yaml:"observe.service,omitempty"   json:"observe.service,omitempty"   action:"observe.service"`
	ObserveCPU           *ObserveCPU             `yaml:"observe.cpu,omitempty"       json:"observe.cpu,omitempty"       action:"observe.cpu"`
	ObserveMemory        *ObserveMemory          `yaml:"observe.memory,omitempty"    json:"observe.memory,omitempty"    action:"observe.memory"`
	ObserveDisk          *ObserveDisk            `yaml:"observe.disk,omitempty"      json:"observe.disk,omitempty"      action:"observe.disk"`
	ObserveGPU           *ObserveGPU             `yaml:"observe.gpu,omitempty"       json:"observe.gpu,omitempty"       action:"observe.gpu"`
	ObserveLogs          *ObserveLogs            `yaml:"observe.logs,omitempty"      json:"observe.logs,omitempty"      action:"observe.logs"`
	WaitPort             *WaitPort               `yaml:"wait.port,omitempty"         json:"wait.port,omitempty"         action:"wait.port"`
	WaitHTTP             *WaitHTTP               `yaml:"wait.http,omitempty"         json:"wait.http,omitempty"         action:"wait.http"`
	WaitFile             *WaitFile               `yaml:"wait.file,omitempty"         json:"wait.file,omitempty"         action:"wait.file"`
	WaitCommand          *WaitCommand            `yaml:"wait.command,omitempty"      json:"wait.command,omitempty"      action:"wait.command"`
	HTTPRequest          *HTTPRequest            `yaml:"http.request,omitempty"      json:"http.request,omitempty"      action:"http.request"`
	Log                  *PrintAction            `yaml:"log,omitempty"               json:"log,omitempty"               action:"log"`
	Use                  *PresetInvocation       `yaml:"use,omitempty"               json:"use,omitempty"               action:"use"`
	Import               *string                 `yaml:"import,omitempty"            json:"import,omitempty"            action:"import"`
	VarsLoad             *string                 `yaml:"vars.load,omitempty"         json:"vars.load,omitempty"         action:"vars.load"`
	Vars                 *map[string]interface{} `yaml:"vars,omitempty"              json:"vars,omitempty"              action:"vars"`

	// Privilege escalation (spec-21: collapsed from become/become_user).
	// Empty = current user; "root" = sudo to root; "<name>" = sudo to <name>.
	AsUser string `yaml:"as_user,omitempty" json:"as_user,omitempty"`

	// Environment
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Cwd string            `yaml:"cwd,omitempty" json:"cwd,omitempty"`

	// Execution control
	Timeout         string       `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retry           *RetryPolicy `yaml:"retry,omitempty" json:"retry,omitempty"`
	ContinueOnError bool         `yaml:"continue_on_error,omitempty" json:"continue_on_error,omitempty"`

	// Result overrides
	ChangedWhen string `yaml:"changed_when,omitempty" json:"changed_when,omitempty"`
	FailedWhen  string `yaml:"failed_when,omitempty" json:"failed_when,omitempty"`

	// Loops
	ForEach     *ForEachField `yaml:"for_each,omitempty" json:"for_each,omitempty"`
	ForEachFile *string       `yaml:"for_each_file,omitempty" json:"for_each_file,omitempty"`

	// Tags and outputs
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	As   string   `yaml:"as,omitempty" json:"as,omitempty"`

	// ── COMPOUND-STEP CONTAINERS ─────────────────────────────────────────────

	// Reactive triggers (spec-23 §1). Children run iff the parent step's
	// outputs reported `changed: true`. Otherwise skipped with reason
	// "parent didn't change". Children execute in declaration order; a
	// child's failure surfaces normally and may itself have on_change.
	// Children DO NOT inherit the parent's outputs scope.
	OnChange []Step `yaml:"on_change,omitempty" json:"on_change,omitempty"`

	// Transaction (spec-30) declares a sequence of steps that apply
	// all-or-nothing. If any child fails during apply, the executor
	// runs Reverse() on each previously-applied child in LIFO order,
	// leaving the system at pre-transaction state. Plan-time check
	// refuses to plan a transaction containing an irreversible step
	// unless AllowIrreversible is true.
	//
	// If Transaction is set, no action field on this Step may be set —
	// the Step is a compound transaction node, not a leaf action.
	Transaction []Step `yaml:"transaction,omitempty" json:"transaction,omitempty"`

	// Try / Catch / Finally (spec-23 §2) declares user-authored error
	// recovery: Try runs sequentially; on the first error, Catch runs
	// (steps can inspect the failure); Finally always runs at the end.
	// The Step is a compound node — no action field may be set
	// alongside Try, and Catch/Finally without Try is invalid.
	//
	// Distinct from Transaction: this is user-authored rollback, not
	// ABI-automated. Even when Catch handles the failure, the compound
	// Step's outcome is failure (exit code non-zero) — Catch is for
	// notification / cleanup, not for swallowing the error.
	Try     []Step `yaml:"try,omitempty" json:"try,omitempty"`
	Catch   []Step `yaml:"catch,omitempty" json:"catch,omitempty"`
	Finally []Step `yaml:"finally,omitempty" json:"finally,omitempty"`

	// OnRollback (spec-30) — sibling to OnChange. Steps that run after
	// the transaction's rollback finishes (successful or partial), for
	// notification / cleanup. Must be empty if Transaction is empty.
	OnRollback []Step `yaml:"on_rollback,omitempty" json:"on_rollback,omitempty"`

	// AllowIrreversible (spec-30) opts the transaction in to including
	// steps whose handler does not implement actions.Reverser, or that
	// declare themselves irreversible at apply time. Default false so
	// plans that mix in a `shell:` step fail loudly at plan time.
	AllowIrreversible bool `yaml:"allow_irreversible,omitempty" json:"allow_irreversible,omitempty"`

	// ── PLANNER-INTERNAL ─────────────────────────────────────────────────────

	// Plan metadata (populated during plan expansion, omitted in config files)
	ID             string       `yaml:"id,omitempty" json:"id,omitempty"`
	ActionType     string       `yaml:"action_type,omitempty" json:"action_type,omitempty"`
	Origin         *Origin      `yaml:"origin,omitempty" json:"origin,omitempty"`
	Skipped        bool         `yaml:"skipped,omitempty" json:"skipped,omitempty"`
	LoopContext    *LoopContext `yaml:"loop_context,omitempty" json:"loop_context,omitempty"`
	SourceLocation *Position    `yaml:"-" json:"-"` // Source location from YAML parsing (set by Reader)

	// TriggeredBy carries the ID of the parent step when this Step was
	// expanded from an on_change child. Populated during plan expansion;
	// empty for top-level steps. Used by the executor to gate execution
	// on the parent's `changed` outcome, and by events/runlog to surface
	// the parent→child relationship to consumers.
	TriggeredBy string `yaml:"triggered_by,omitempty" json:"triggered_by,omitempty"`

	// TxnParent carries the ID of the parent transaction Step when this
	// Step was expanded from a Transaction child. Mirrors TriggeredBy's
	// shape; populated by the planner. Used by the executor to track
	// which steps belong to which open transaction.
	TxnParent string `yaml:"txn_parent,omitempty" json:"txn_parent,omitempty"`

	// TxnRole tags an expanded child with its role inside the parent
	// transaction. Populated by the planner; one of:
	//   - ""          — regular step (not part of a transaction)
	//   - "body"      — forward-apply child. On failure, the executor
	//                    runs Reverse() on previously-completed body
	//                    children of the same TxnParent in LIFO order.
	//   - "rollback"  — on_rollback child. Skipped when the transaction
	//                    committed successfully; runs when it rolled
	//                    back (regardless of whether rollback itself
	//                    completed or only partially reverted).
	TxnRole string `yaml:"txn_role,omitempty" json:"txn_role,omitempty"`

	// TryParent carries the ID of the parent compound Step when this
	// Step was expanded from a Try / Catch / Finally branch. Mirrors
	// TxnParent's shape. Populated by the planner; empty for steps
	// outside a try-block.
	TryParent string `yaml:"try_parent,omitempty" json:"try_parent,omitempty"`

	// TryRole tags an expanded child with its role inside the parent
	// compound. Populated by the planner; one of:
	//   - ""        — regular step (not part of a try-block)
	//   - "try"     — body branch. Skipped if a prior try child of the
	//                 same TryParent failed.
	//   - "catch"   — error-recovery branch. Skipped when try ran to
	//                 completion without error.
	//   - "finally" — always-run branch. Never gated.
	TryRole string `yaml:"try_role,omitempty" json:"try_role,omitempty"`
}

// ForEachField holds the value of a Step's `for_each` keyword. It supports
// two YAML forms:
//
//	for_each: items_var          # scalar — variable reference / template
//	for_each: [a, b, c]          # inline sequence — literal list
//	for_each:                     # block sequence — literal list
//	  - a
//	  - b
//
// The scalar form is rendered through the template engine and resolved
// via the variable scope at plan time; the sequence form is used as a
// literal list of items.
type ForEachField struct {
	// Expr is set when YAML is a scalar (string). Rendered and then
	// resolved by template.ResolveList at plan expansion time.
	Expr string
	// Items is set when YAML is a sequence (inline or block).
	Items []interface{}
}

// UnmarshalYAML accepts scalar or sequence forms.
func (f *ForEachField) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		f.Expr = s
		return nil
	}
	var items []interface{}
	if err := unmarshal(&items); err == nil {
		f.Items = items
		return nil
	}
	return fmt.Errorf("for_each: expected scalar (variable expression) or sequence (literal list)")
}

// MarshalYAML emits whichever form is populated (scalar or sequence).
func (f ForEachField) MarshalYAML() (interface{}, error) {
	if f.Items != nil {
		return f.Items, nil
	}
	return f.Expr, nil
}

// MarshalJSON ensures Validate's json.Marshal → unmarshal → schema-check
// round-trip emits the scalar/sequence form rather than a struct shape.
func (f ForEachField) MarshalJSON() ([]byte, error) {
	if f.Items != nil {
		return json.Marshal(f.Items)
	}
	return json.Marshal(f.Expr)
}

// UnmarshalJSON parses either a scalar (string) or sequence (array) form.
func (f *ForEachField) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		f.Expr = s
		return nil
	}
	var items []interface{}
	if err := json.Unmarshal(data, &items); err == nil {
		f.Items = items
		return nil
	}
	return fmt.Errorf("for_each: expected JSON string or array")
}

// RetryPolicy controls per-step retry behavior (spec-21).
// Replaces the legacy flat Retries + RetryDelay fields with a single
// structured block; future-compat for backoff strategies.
//
// Semantics (MT-49):
//   - `attempts:` is the number of *retries* after the initial try.
//   - Total executions therefore = attempts + 1 (e.g. attempts:3 →
//     up to 4 executions: 1 initial + 3 retries).
//   - Matches Ansible's `until: … retries:` convention, but call it
//     out here because the field name is otherwise ambiguous and
//     operators sometimes read "attempts: 3" as "exactly 3 tries".
type RetryPolicy struct {
	// Attempts is the number of retries after the initial try.
	// Total executions = Attempts + 1. Set to 0 to disable retry.
	Attempts int    `yaml:"attempts" json:"attempts,omitempty"`
	Delay    string `yaml:"delay" json:"delay,omitempty"`
	Backoff  string `yaml:"backoff" json:"backoff,omitempty"` // "fixed", "linear", "exponential"
}

// RetryAttempts returns the configured retry-attempt count, or 0 if no
// retry policy is set. Helper for the post-spec-21 Retry struct.
func (s *Step) RetryAttempts() int {
	if s.Retry == nil {
		return 0
	}
	return s.Retry.Attempts
}

// RetryDelayDuration returns the configured retry delay string, or "" if no
// retry policy is set. Helper for the post-spec-21 Retry struct.
func (s *Step) RetryDelayDuration() string {
	if s.Retry == nil {
		return ""
	}
	return s.Retry.Delay
}

// RetryBackoffStrategy returns the configured backoff strategy, or
// "fixed" (the default) when unset. The Retry.Backoff field was
// declared in the schema but never read — `linear` and `exponential`
// were silently ignored and every retry slept for the bare delay,
// defeating the point of backoff for external-API integrations.
func (s *Step) RetryBackoffStrategy() string {
	if s.Retry == nil || s.Retry.Backoff == "" {
		return "fixed"
	}
	return s.Retry.Backoff
}

// ShouldBecome reports whether the step requests privilege escalation.
// True iff AsUser is non-empty (spec-21 collapsed become/become_user) AND
// the current process is not already running as the target user. When the
// current euid is 0 and AsUser targets root ("root" or "0"), no escalation
// is needed — short-circuits sudo invocation so presets work in minimal
// containers (ubuntu:24.04, alpine:3.21) that don't ship sudo.
func (s *Step) ShouldBecome() bool {
	if s.AsUser == "" {
		return false
	}
	if (s.AsUser == "root" || s.AsUser == "0") && os.Geteuid() == 0 {
		return false
	}
	return true
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
	Type           string      `yaml:"type" json:"type"` // "for_each" or "for_each_file"
	Item           interface{} `yaml:"item" json:"item"`
	Index          int         `yaml:"index" json:"index"`
	First          bool        `yaml:"first" json:"first"`
	Last           bool        `yaml:"last" json:"last"`
	LoopExpression string      `yaml:"loop_expression,omitempty" json:"loop_expression,omitempty"`
	Depth          int         `yaml:"depth,omitempty" json:"depth,omitempty"` // Directory depth for filetree items
}

var stepType = reflect.TypeOf(Step{})

// actionFieldIndices holds the struct field indices of all Step fields tagged
// with `action:`, computed once at package init. Declaration order determines
// DetermineActionType priority for the (invalid) case of multiple set fields.
var actionFieldIndices = func() []int {
	var idx []int
	for i := 0; i < stepType.NumField(); i++ {
		if _, ok := stepType.Field(i).Tag.Lookup("action"); ok {
			idx = append(idx, i)
		}
	}
	return idx
}()

// ActionFieldIndices returns the cached action field indices for use by
// external packages (e.g. the planner's renderActionTemplates).
func ActionFieldIndices() []int { return actionFieldIndices }

// countActions returns the number of non-nil action fields in this step.
func (s *Step) countActions() int {
	rv := reflect.ValueOf(s).Elem()
	n := 0
	for _, i := range actionFieldIndices {
		if !rv.Field(i).IsNil() {
			n++
		}
	}
	return n
}

// DetermineActionType returns the action type for this step based on which action field is populated.
// Returned strings are the modern dot-namespaced YAML keys (spec-21).
func (s *Step) DetermineActionType() string {
	rv := reflect.ValueOf(s).Elem()
	for _, i := range actionFieldIndices {
		if !rv.Field(i).IsNil() {
			return stepType.Field(i).Tag.Get("action")
		}
	}
	if s.ForEach != nil || s.ForEachFile != nil {
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
	// spec-30 transaction-shape rules. A Step with a Transaction is a
	// compound node; it cannot also carry a leaf action (the children
	// carry actions). OnRollback only makes sense paired with a
	// Transaction.
	if len(s.Transaction) > 0 && s.countActions() > 0 {
		return fmt.Errorf("Step %s: cannot combine transaction: with an action field", s.Name)
	}
	if len(s.OnRollback) > 0 && len(s.Transaction) == 0 {
		return fmt.Errorf("Step %s: on_rollback requires transaction: on the same step", s.Name)
	}
	if len(s.Transaction) > 0 && len(s.Try) > 0 {
		return fmt.Errorf("Step %s: cannot combine transaction: with try: (nest one inside the other)", s.Name)
	}
	if len(s.Transaction) > 0 {
		// Compound Step — no action discriminator required/allowed.
		return nil
	}

	// spec-23 §2 try / catch / finally rules. A Step with Try is a
	// compound node; it cannot also carry a leaf action. Catch /
	// Finally without Try is invalid (orphan branches).
	if len(s.Try) > 0 && s.countActions() > 0 {
		return fmt.Errorf("Step %s: cannot combine try: with an action field", s.Name)
	}
	if len(s.Try) == 0 && (len(s.Catch) > 0 || len(s.Finally) > 0) {
		return fmt.Errorf("Step %s: catch:/finally: requires try: on the same step", s.Name)
	}
	if len(s.Try) > 0 {
		// Compound Step — no action discriminator required/allowed.
		return nil
	}

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
		Name:              s.Name,
		When:              s.When,
		UnlessExists:      s.UnlessExists,
		UnlessCommand:     s.UnlessCommand,
		Creates:           s.Creates,
		Unless:            s.Unless,
		FileWrite:         s.FileWrite,
		FileTemplate:      s.FileTemplate,
		FileCopy:          s.FileCopy,
		FileDownload:      s.FileDownload,
		FileUnarchive:     s.FileUnarchive,
		TextLine:          s.TextLine,
		TextReplace:       s.TextReplace,
		TextInsert:        s.TextInsert,
		TextDeleteRange:   s.TextDeleteRange,
		TextPatch:         s.TextPatch,
		TextPatchINI:      s.TextPatchINI,
		TextPatchJSON:     s.TextPatchJSON,
		TextPatchYAML:     s.TextPatchYAML,
		Pkg:               s.Pkg,
		PkgRepo:           s.PkgRepo,
		PkgHold:           s.PkgHold,
		PkgUpgrade:        s.PkgUpgrade,
		PkgList:           s.PkgList,
		Tool:              s.Tool,
		OsService:         s.OsService,
		OsUser:            s.OsUser,
		OsGroup:           s.OsGroup,
		OsSSHKey:          s.OsSSHKey,
		OsCron:            s.OsCron,
		OsSysctl:          s.OsSysctl,
		OsSystemd:         s.OsSystemd,
		OsMount:           s.OsMount,
		OsFirewall:        s.OsFirewall,
		ContainerImage:    s.ContainerImage,
		Container:         s.Container,
		Cmd:               s.Cmd,
		RepoSearch:        s.RepoSearch,
		RepoTree:          s.RepoTree,
		ReadJSON:          s.ReadJSON,
		ReadYAML:          s.ReadYAML,
		RepoPatch:         s.RepoPatch,
		GitClone:          s.GitClone,
		GitCheckout:       s.GitCheckout,
		GitConfig:         s.GitConfig,
		ArtifactCapture:   s.ArtifactCapture,
		ArtifactValidate:  s.ArtifactValidate,
		Shell:             s.Shell,
		Assert:            s.Assert,
		ObservePort:       s.ObservePort,
		ObserveProcess:    s.ObserveProcess,
		ObserveHTTP:       s.ObserveHTTP,
		ObserveService:    s.ObserveService,
		ObserveCPU:        s.ObserveCPU,
		ObserveMemory:     s.ObserveMemory,
		ObserveDisk:       s.ObserveDisk,
		ObserveGPU:        s.ObserveGPU,
		ObserveLogs:       s.ObserveLogs,
		WaitPort:          s.WaitPort,
		WaitHTTP:          s.WaitHTTP,
		WaitFile:          s.WaitFile,
		WaitCommand:       s.WaitCommand,
		HTTPRequest:       s.HTTPRequest,
		Log:               s.Log,
		Use:               s.Use,
		Import:            s.Import,
		VarsLoad:          s.VarsLoad,
		Vars:              s.Vars,
		AsUser:            s.AsUser,
		Env:               s.Env,
		Cwd:               s.Cwd,
		Timeout:           s.Timeout,
		Retry:             s.Retry,
		ContinueOnError:   s.ContinueOnError,
		ChangedWhen:       s.ChangedWhen,
		FailedWhen:        s.FailedWhen,
		ForEach:           s.ForEach,
		ForEachFile:       s.ForEachFile,
		Tags:              append([]string(nil), s.Tags...),
		As:                s.As,
		OnChange:          append([]Step(nil), s.OnChange...),
		Transaction:       append([]Step(nil), s.Transaction...),
		OnRollback:        append([]Step(nil), s.OnRollback...),
		AllowIrreversible: s.AllowIrreversible,
		TxnParent:         s.TxnParent,
		TxnRole:           s.TxnRole,
		Try:               append([]Step(nil), s.Try...),
		Catch:             append([]Step(nil), s.Catch...),
		Finally:           append([]Step(nil), s.Finally...),
		TryParent:         s.TryParent,
		TryRole:           s.TryRole,
		ID:                s.ID,
		ActionType:        s.ActionType,
		Origin:            s.Origin,
		Skipped:           s.Skipped,
		LoopContext:       s.LoopContext,
		TriggeredBy:       s.TriggeredBy,
	}
}

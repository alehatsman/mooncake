package schemagen

// KnownEnums maps action.field paths to their enum values.
// Keys use spec-21 dot-namespaced action names.
var KnownEnums = map[string][]string{
	// os.service action enums
	"os.service.state": {"started", "stopped", "restarted", "reloaded"},
	"os.service.scope": {"system", "user"},

	// file.write action enums
	"file.write.state": {"file", "present", "absent", "directory", "link", "hardlink", "touch", "perms"},

	// pkg action enums
	"pkg.state": {"present", "absent", "latest"},

	// os.cron / os.sysctl state enums
	"os.cron.state":   {"present", "absent"},
	"os.sysctl.state": {"present", "absent"},

	// os.systemd scope enum (proposal-17)
	"os.systemd.scope": {"system", "user"},

	// shell.interpreter is intentionally unconstrained: any executable on
	// PATH is valid (bash, sh, zsh, pwsh, powershell, cmd, nu, ...).
}

// KnownPatterns maps field names to regex patterns for validation.
var KnownPatterns = map[string]string{
	// Duration fields
	"timeout": `^[0-9]+(ns|us|µs|ms|s|m|h)$`,
	"delay":   `^[0-9]+(ns|us|µs|ms|s|m|h)$`, // retry.delay sub-field

	// File permissions (octal)
	"mode": `^[0-7]{3,4}$`,
}

// KnownRanges maps field names to min/max constraints.
var KnownRanges = map[string]struct{ Min, Max float64 }{
	"attempts": {Min: 0, Max: 100}, // retry.attempts sub-field
}

// applyKnownValidation adds enum, pattern, and range validation to a property.
func applyKnownValidation(actionName, fieldName string, prop *Property) {
	// Check for enum values
	enumKey := actionName + "." + fieldName
	if enumValues, ok := KnownEnums[enumKey]; ok {
		prop.Enum = make([]interface{}, len(enumValues))
		for i, v := range enumValues {
			prop.Enum[i] = v
		}
	}

	// Check for pattern validation
	if pattern, ok := KnownPatterns[fieldName]; ok {
		prop.Pattern = pattern
	}

	// Check for range validation
	if propRange, ok := KnownRanges[fieldName]; ok {
		prop.Minimum = &propRange.Min
		prop.Maximum = &propRange.Max
	}
}

// EnhancedDescriptions adds detailed descriptions to properties.
var EnhancedDescriptions = map[string]map[string]string{
	"os.service": {
		"name":          "Service name (systemd: nginx, launchd: com.example.app)",
		"state":         "Desired service state",
		"enabled":       "Enable service to start on boot (systemd: enable/disable, launchd: bootstrap/bootout)",
		"scope":         "Service scope: system (default, requires root) or user (no root; launchd: ~/Library/LaunchAgents + gui/<uid> domain)",
		"daemon_reload": "Run 'systemctl daemon-reload' after unit file changes (systemd only)",
	},
	"file.write": {
		"path":  "File, directory, or symlink path (required)",
		"state": "Desired file state (file/present: file exists, absent: removed, directory: dir exists, link: symlink, hardlink: hard link, touch: update timestamp, perms: change permissions only)",
		"mode":  "File permissions (e.g., '0644', '0755')",
		"owner": "File owner (username or UID)",
		"group": "File group (groupname or GID)",
	},
	"shell": {
		"cmd":          "Shell command to execute (required)",
		"interpreter":  "Shell interpreter binary (any executable on PATH). Default: bash on Unix, powershell on Windows. On Windows, 'cmd' dispatches with /c, others with -Command.",
		"stdin":        "Input to provide to the command via stdin",
		"capture":      "Capture command output (default: true). When false, output is only streamed",
		"run_as_admin": "Windows only — assert the mooncake process is elevated; fail the step if it isn't. Does not attempt UAC. Ignored on Unix.",
		"error_action": "Windows + PowerShell only — sets $ErrorActionPreference (Stop, Continue, SilentlyContinue, ...). Default: Stop. Ignored elsewhere.",
	},
	"pkg": {
		"name":         "Package name (single package)",
		"names":        "Multiple packages to install/remove",
		"state":        "Package state (present: installed, absent: removed, latest: install or upgrade)",
		"manager":      "Package manager (auto-detected if empty: apt, dnf, yum, pacman, zypper, apk, brew, port, choco, scoop)",
		"update_cache": "Update package cache before operation (e.g., apt-get update)",
	},
}

// applyEnhancedDescription sets a detailed description if available.
func applyEnhancedDescription(actionName, fieldName string, prop *Property) {
	if actionDescs, ok := EnhancedDescriptions[actionName]; ok {
		if desc, ok := actionDescs[fieldName]; ok {
			prop.Description = desc
		}
	}
}

// FieldOverrides hand-tunes specific (action, field) pairs whose JSON Schema
// type isn't directly derivable from the Go struct (e.g. union types
// supported by custom UnmarshalYAML).
//
// Applied after the reflection-based property extraction.
var FieldOverrides = map[string]func(prop *Property){
	"package.names": func(prop *Property) {
		// Accept either a list of package names or a single template
		// expression that resolves to a list at execute time.
		prop.Type = ""
		prop.Items = nil
		prop.OneOf = []*Property{
			{Type: "array", Items: &Property{Type: "string"}},
			{Type: "string"},
		}
	},
	"os.sysctl.value": func(prop *Property) {
		// Sysctl values are stringly typed in /proc but YAML users
		// naturally write `value: 1` or `value: "1"`. Accept either.
		prop.Type = ""
		prop.OneOf = []*Property{
			{Type: "string"},
			{Type: "integer"},
			{Type: "boolean"},
		}
	},
}

// applyFieldOverride applies any registered override for (action, field).
func applyFieldOverride(actionName, fieldName string, prop *Property) {
	if override, ok := FieldOverrides[actionName+"."+fieldName]; ok {
		override(prop)
	}
}

package schemagen

// KnownEnums maps action.field paths to their enum values.
// These are extracted from the existing schema and validation logic.
var KnownEnums = map[string][]string{
	// Service action enums
	"service.state": {"started", "stopped", "restarted", "reloaded"},

	// File action enums
	"file.state": {"present", "absent", "directory", "link", "touch"},

	// Package action enums
	"package.state": {"present", "absent", "latest"},

	// Shell action enums
	"shell.interpreter": {"bash", "sh", "pwsh", "cmd"},

	// Download/File/Copy mode validation (not enum, but common pattern)
	// These will be handled as pattern validation

	// Template action enums (none specific, uses file-like patterns)
}

// KnownPatterns maps field names to regex patterns for validation.
var KnownPatterns = map[string]string{
	// Duration fields
	"timeout":     `^[0-9]+(ns|us|µs|ms|s|m|h)$`,
	"retry_delay": `^[0-9]+(ns|us|µs|ms|s|m|h)$`,

	// File permissions (octal)
	"mode": `^[0-7]{3,4}$`,
}

// KnownRanges maps field names to min/max constraints.
var KnownRanges = map[string]struct{ Min, Max float64 }{
	"retries": {Min: 0, Max: 100},
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
	"service": {
		"name":          "Service name (systemd: nginx, launchd: com.example.app)",
		"state":         "Desired service state",
		"enabled":       "Enable service to start on boot (systemd: enable/disable, launchd: bootstrap/bootout)",
		"daemon_reload": "Run 'systemctl daemon-reload' after unit file changes (systemd only)",
	},
	"file": {
		"path":  "File, directory, or symlink path (required)",
		"state": "Desired file state (present: file exists, absent: removed, directory: dir exists, link: symlink, touch: update timestamp)",
		"mode":  "File permissions (e.g., '0644', '0755')",
		"owner": "File owner (username or UID)",
		"group": "File group (groupname or GID)",
	},
	"shell": {
		"cmd":         "Shell command to execute (required)",
		"interpreter": "Shell interpreter (bash, sh, pwsh, cmd). Default: bash on Unix, pwsh on Windows",
		"stdin":       "Input to provide to the command via stdin",
		"capture":     "Capture command output (default: true). When false, output is only streamed",
	},
	"package": {
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

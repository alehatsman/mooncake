// Package register imports all action handler packages to trigger their init() functions.
// This package should be imported from cmd/mooncake.go to register all handlers.
//
// This package exists separately from internal/actions to avoid circular imports:
// - actions defines the Handler interface
// - actions/print (and other actions) implement Handler and import actions
// - register imports all action packages to trigger registration
// - cmd imports register (not actions directly)
package register

import (
	// Artifact
	_ "github.com/alehatsman/mooncake/internal/actions/artifact_capture"
	_ "github.com/alehatsman/mooncake/internal/actions/artifact_validate"

	// Container
	_ "github.com/alehatsman/mooncake/internal/actions/container_image"
	_ "github.com/alehatsman/mooncake/internal/actions/container_run"

	// File management
	_ "github.com/alehatsman/mooncake/internal/actions/copy"
	_ "github.com/alehatsman/mooncake/internal/actions/download"
	_ "github.com/alehatsman/mooncake/internal/actions/file"
	_ "github.com/alehatsman/mooncake/internal/actions/file_delete_range"
	_ "github.com/alehatsman/mooncake/internal/actions/file_insert"
	_ "github.com/alehatsman/mooncake/internal/actions/file_patch_apply"
	_ "github.com/alehatsman/mooncake/internal/actions/file_replace"
	_ "github.com/alehatsman/mooncake/internal/actions/template"
	_ "github.com/alehatsman/mooncake/internal/actions/unarchive"

	// Git
	_ "github.com/alehatsman/mooncake/internal/actions/git_checkout"
	_ "github.com/alehatsman/mooncake/internal/actions/git_clone"
	_ "github.com/alehatsman/mooncake/internal/actions/git_config"

	// Observe
	_ "github.com/alehatsman/mooncake/internal/actions/observe_cpu"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_disk"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_gpu"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_http"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_logs"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_memory"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_port"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_process"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_service"

	// OS
	_ "github.com/alehatsman/mooncake/internal/actions/os_cron"
	_ "github.com/alehatsman/mooncake/internal/actions/os_firewall"
	_ "github.com/alehatsman/mooncake/internal/actions/os_group"
	_ "github.com/alehatsman/mooncake/internal/actions/os_mount"
	_ "github.com/alehatsman/mooncake/internal/actions/os_ssh_key"
	_ "github.com/alehatsman/mooncake/internal/actions/os_sysctl"
	_ "github.com/alehatsman/mooncake/internal/actions/os_systemd"
	_ "github.com/alehatsman/mooncake/internal/actions/os_user"
	_ "github.com/alehatsman/mooncake/internal/actions/service"

	// Package management
	_ "github.com/alehatsman/mooncake/internal/actions/package"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_hold"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_list"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_repo"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_upgrade"

	// Repository
	_ "github.com/alehatsman/mooncake/internal/actions/repo_apply_patchset"
	_ "github.com/alehatsman/mooncake/internal/actions/repo_search"
	_ "github.com/alehatsman/mooncake/internal/actions/repo_tree"

	// Runtime / execution
	_ "github.com/alehatsman/mooncake/internal/actions/assert"
	_ "github.com/alehatsman/mooncake/internal/actions/command"
	_ "github.com/alehatsman/mooncake/internal/actions/preset"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	_ "github.com/alehatsman/mooncake/internal/actions/tool"

	// Text patching
	_ "github.com/alehatsman/mooncake/internal/actions/text_line"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch_ini"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch_json"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch_yaml"

	// Variables / data
	_ "github.com/alehatsman/mooncake/internal/actions/include_vars"
	_ "github.com/alehatsman/mooncake/internal/actions/print"
	_ "github.com/alehatsman/mooncake/internal/actions/read_json"
	_ "github.com/alehatsman/mooncake/internal/actions/read_yaml"
	_ "github.com/alehatsman/mooncake/internal/actions/vars"

	// Network
	_ "github.com/alehatsman/mooncake/internal/actions/http_request"

	// Wait / polling
	_ "github.com/alehatsman/mooncake/internal/actions/wait_command"
	_ "github.com/alehatsman/mooncake/internal/actions/wait_file"
	_ "github.com/alehatsman/mooncake/internal/actions/wait_http"
	_ "github.com/alehatsman/mooncake/internal/actions/wait_port"

	// Windows-only
	_ "github.com/alehatsman/mooncake/internal/actions/windows_firewall_rule"
	_ "github.com/alehatsman/mooncake/internal/actions/windows_hyperv_firewall_rule"
	_ "github.com/alehatsman/mooncake/internal/actions/windows_scheduled_task"
)

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
	// Container
	_ "github.com/alehatsman/mooncake/internal/actions/container_image"
	_ "github.com/alehatsman/mooncake/internal/actions/container_run"

	// File management
	_ "github.com/alehatsman/mooncake/internal/actions/file_copy"
	_ "github.com/alehatsman/mooncake/internal/actions/file_download"
	_ "github.com/alehatsman/mooncake/internal/actions/file_template"
	_ "github.com/alehatsman/mooncake/internal/actions/file_unarchive"
	_ "github.com/alehatsman/mooncake/internal/actions/file_write"
	_ "github.com/alehatsman/mooncake/internal/actions/text_delete_range"
	_ "github.com/alehatsman/mooncake/internal/actions/text_insert"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch"
	_ "github.com/alehatsman/mooncake/internal/actions/text_replace"

	// Git
	_ "github.com/alehatsman/mooncake/internal/actions/git_checkout"
	_ "github.com/alehatsman/mooncake/internal/actions/git_clone"
	_ "github.com/alehatsman/mooncake/internal/actions/git_config"

	// Observe
	_ "github.com/alehatsman/mooncake/internal/actions/observe_command"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_cpu"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_disk"
	_ "github.com/alehatsman/mooncake/internal/actions/observe_file"
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
	_ "github.com/alehatsman/mooncake/internal/actions/os_service"
	_ "github.com/alehatsman/mooncake/internal/actions/os_ssh_key"
	_ "github.com/alehatsman/mooncake/internal/actions/os_sysctl"
	_ "github.com/alehatsman/mooncake/internal/actions/os_systemd"
	_ "github.com/alehatsman/mooncake/internal/actions/os_user"

	// Package management
	_ "github.com/alehatsman/mooncake/internal/actions/pkg"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_hold"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_list"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_repo"
	_ "github.com/alehatsman/mooncake/internal/actions/pkg_upgrade"

	// Runtime / execution
	_ "github.com/alehatsman/mooncake/internal/actions/assert"
	_ "github.com/alehatsman/mooncake/internal/actions/cmd"
	_ "github.com/alehatsman/mooncake/internal/actions/process"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	_ "github.com/alehatsman/mooncake/internal/actions/tool"
	_ "github.com/alehatsman/mooncake/internal/actions/use"

	// Text patching
	_ "github.com/alehatsman/mooncake/internal/actions/text_line"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch_ini"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch_json"
	_ "github.com/alehatsman/mooncake/internal/actions/text_patch_yaml"

	// Variables / data
	_ "github.com/alehatsman/mooncake/internal/actions/log"
	_ "github.com/alehatsman/mooncake/internal/actions/read_json"
	_ "github.com/alehatsman/mooncake/internal/actions/read_yaml"
	_ "github.com/alehatsman/mooncake/internal/actions/vars"
	_ "github.com/alehatsman/mooncake/internal/actions/vars_load"

	// Network
	_ "github.com/alehatsman/mooncake/internal/actions/http_request"

	// Wait / polling

	// Windows-only
	_ "github.com/alehatsman/mooncake/internal/actions/windows_firewall_rule"
	_ "github.com/alehatsman/mooncake/internal/actions/windows_hyperv_firewall_rule"
	_ "github.com/alehatsman/mooncake/internal/actions/windows_registry"
	_ "github.com/alehatsman/mooncake/internal/actions/windows_scheduled_task"
)

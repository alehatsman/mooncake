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
	// Register all action handlers by importing their packages
	_ "github.com/alehatsman/mooncake/internal/actions/assert"
	_ "github.com/alehatsman/mooncake/internal/actions/command"
	_ "github.com/alehatsman/mooncake/internal/actions/copy"
	_ "github.com/alehatsman/mooncake/internal/actions/download"
	_ "github.com/alehatsman/mooncake/internal/actions/file"
	_ "github.com/alehatsman/mooncake/internal/actions/include_vars"
	_ "github.com/alehatsman/mooncake/internal/actions/package"
	_ "github.com/alehatsman/mooncake/internal/actions/preset"
	_ "github.com/alehatsman/mooncake/internal/actions/print"
	_ "github.com/alehatsman/mooncake/internal/actions/service"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	_ "github.com/alehatsman/mooncake/internal/actions/template"
	_ "github.com/alehatsman/mooncake/internal/actions/unarchive"
	_ "github.com/alehatsman/mooncake/internal/actions/vars"

	// All handlers migrated! 🎉
)

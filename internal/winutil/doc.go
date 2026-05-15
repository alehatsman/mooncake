// Package winutil builds the small set of PowerShell snippets mooncake
// needs to manage Windows-specific resources idempotently — firewall
// rules and scheduled tasks today, registry / service entries later.
//
// The package is intentionally split from the action handlers
// (internal/actions/windows_*) and the fleet bootstrap path
// (internal/fleet/bootstrap.go) because both want the same building
// blocks: the action handlers run them in-process via os/exec, the
// fleet bootstrap path runs them over SSH on a remote box. Keeping the
// renderers pure-Go-strings (no os/exec, no SSH coupling, no config
// types) means tests are dependency-free and the consumers add the
// transport.
//
// Everything in this package is build-tag-free — these files compile
// and unit-test on Linux/macOS just like any other Go code. The
// actual PowerShell execution lives at the consumer's boundary.
package winutil

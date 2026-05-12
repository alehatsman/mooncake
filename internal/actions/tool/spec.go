package tool

import "github.com/alehatsman/mooncake/internal/lockfile"

// SpecFromLockEntry rebuilds the minimal Spec needed to call
// Backend.Locate from a lockfile entry. Used by the read-only CLI
// subcommands (`mooncake tool which`, `tool list`, `tool env`) which
// don't have access to the original YAML config.
//
// Backend-specific fields beyond (backend, name, version) are not stored
// in the lockfile and are not reconstructed here — Locate doesn't need
// them. URL backends rely on the install dir; mise's Locate uses
// `mise which name --version version`.
func SpecFromLockEntry(e lockfile.Entry) Spec {
	return Spec{
		Backend: e.Backend,
		Name:    e.Name,
		Version: e.Version,
		Bin:     e.Bin,
	}
}

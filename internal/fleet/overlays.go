package fleet

import (
	"os"
	"path/filepath"
)

// ResolveVarsFiles returns the ordered list of vars-files to bundle with a
// per-peer submit. Paths are absolute on the controller and rooted under
// planDir, so they're inside the synced tree and survive the
// controller→peer path translation in Apply.
//
// Order (matches spec-48):
//
//  1. <planDir>/vars/common.yml          — always loaded if present
//  2. <planDir>/vars/by-tag/<tag>.yml    — once per tag, in peer.Tags order
//  3. <planDir>/vars/by-host/<name>.yml  — most specific, loaded last
//
// "Loaded last" follows the existing mooncake convention that later vars
// files override earlier on key collision. Files that don't exist on disk
// are silently skipped — the convention is opt-in; an operator who doesn't
// use overlays gets an empty result.
//
// The returned paths are filepath.Clean'd and absolute. Callers prepend the
// result to any explicit --vars-file args so the user's flag still wins.
func ResolveVarsFiles(planDir string, peer Peer) []string {
	return resolveOverlayCandidates(planDir, peer.Name, peer.Tags)
}

// ResolveLocalOverlays is the local-apply (spec-51) equivalent of
// ResolveVarsFiles. The local path has no peers.toml entry, so tag overlays
// are omitted — only vars/common.yml and vars/by-host/<hostname>.yml are
// loaded. Sharing resolveOverlayCandidates with the fleet path keeps the
// overlay convention single-sourced; the two callers can't drift apart on
// file-stat or ordering rules.
func ResolveLocalOverlays(planDir, hostname string) []string {
	return resolveOverlayCandidates(planDir, hostname, nil)
}

// resolveOverlayCandidates is the shared engine for both fleet and local
// overlay resolution. Pass tags=nil for the local path (no peers.toml).
// hostname may be empty to suppress the by-host candidate entirely.
func resolveOverlayCandidates(planDir, hostname string, tags []string) []string {
	var candidates []string
	candidates = append(candidates, filepath.Join(planDir, "vars", "common.yml"))
	for _, tag := range tags {
		candidates = append(candidates, filepath.Join(planDir, "vars", "by-tag", tag+".yml"))
	}
	if hostname != "" {
		candidates = append(candidates, filepath.Join(planDir, "vars", "by-host", hostname+".yml"))
	}

	out := make([]string, 0, len(candidates))
	for _, p := range candidates {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		out = append(out, filepath.Clean(p))
	}
	return out
}

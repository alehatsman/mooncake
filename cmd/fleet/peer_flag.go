package fleet

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/fleet"
)

// Fleet DX proposal-01: unify peer targeting.
//
// Every fleet subcommand selects peers with ONE flag:
//
//	--peer <selector>        # repeat for UNION; "@k=v,k2=v2" for AND group
//
// Each --peer value classifies as:
//
//	bare name           "main_pc"             → name match
//	key=value           "tag=production"      → single-key filter
//	@k=v,k2=v2          "@tag=prod,os=linux"  → AND group (within one --peer)
//
// Multiple --peer flags → UNION (any matches).
//
// The legacy --peers / --peer-filter flags were removed in this
// proposal; the project's vision doc explicitly disallows
// deprecation shims, so the surface change is one-shot. resolvePeers
// is the single entry point that turns the flag values into
// [][]filterTerm and runs peerMatchesFilters on each candidate peer.

// parsePeerFlags converts a slice of raw `--peer <selector>` values into
// the AND-group shape peerMatchesFilters already consumes. Each value
// produces exactly one group:
//
//	"main_pc"             →  [{name, main_pc}]
//	"tag=production"      →  [{tag, production}]
//	"@tag=prod,os=linux"  →  [{tag, prod}, {os, linux}]
//
// Multiple --peer values produce multiple groups (union). Empty strings
// and whitespace-only entries are skipped (yields one less group rather
// than failing — the StringSliceFlag occasionally surfaces empty tokens
// when scripts splice `--peer "$VAR"` with VAR unset).
func parsePeerFlags(args []string) ([][]filterTerm, error) {
	var groups [][]filterTerm
	for _, raw := range args {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		switch {
		case strings.HasPrefix(s, "@"):
			// @k1=v1,k2=v2 — AND group with N terms within one --peer.
			inner := strings.TrimPrefix(s, "@")
			group, err := parsePeerTerms(inner)
			if err != nil {
				return nil, fmt.Errorf("--peer %q: %w", s, err)
			}
			if len(group) > 0 {
				groups = append(groups, group)
			}
		case strings.ContainsRune(s, '='):
			// key=value — single-term group.
			term, err := parsePeerTerm(s)
			if err != nil {
				return nil, fmt.Errorf("--peer %q: %w", s, err)
			}
			groups = append(groups, []filterTerm{term})
		default:
			// Bare name → synthetic name= group; `--peer main_pc`
			// is the friendly short form of `--peer name=main_pc`.
			groups = append(groups, []filterTerm{{key: "name", value: s}})
		}
	}
	return groups, nil
}

// parsePeerTerms splits a comma-separated list of key=value into
// filterTerm entries for the AND-group form (`@k=v,k2=v2`).
func parsePeerTerms(s string) ([]filterTerm, error) {
	var out []filterTerm
	for _, raw := range strings.Split(s, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		t, err := parsePeerTerm(tok)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// peerFlagsReferenceOSKey reports whether any --peer value contains
// an `os=` predicate. Callers use this to avoid building a peer-OS
// probe cache when no os= filter is in scope.
func peerFlagsReferenceOSKey(args []string) bool {
	const needle = "os="
	for _, a := range args {
		s := strings.TrimSpace(a)
		s = strings.TrimPrefix(s, "@")
		for _, tok := range strings.Split(s, ",") {
			if strings.HasPrefix(strings.TrimSpace(tok), needle) {
				return true
			}
		}
	}
	return false
}

// parsePeerTerm parses one key=value into a filterTerm.
func parsePeerTerm(s string) (filterTerm, error) {
	eq := strings.IndexByte(s, '=')
	if eq < 0 {
		return filterTerm{}, fmt.Errorf("expected key=value, got %q", s)
	}
	k := strings.TrimSpace(s[:eq])
	v := strings.TrimSpace(s[eq+1:])
	if k == "" || v == "" {
		return filterTerm{}, fmt.Errorf("key and value must be non-empty in %q", s)
	}
	return filterTerm{key: k, value: v}, nil
}

// peerSelection is the resolved outcome of `--peer` on a fleet
// subcommand. Callers consume `Matched`; `UnknownNames` carries any
// `--peer main_pc` entries (bare name selectors) that didn't match a
// peer in peers.toml so subcommands can warn rather than silently
// no-op on a typo.
type peerSelection struct {
	Matched      []fleet.Peer
	UnknownNames []string
}

// resolvePeers is the single entry point every fleet subcommand uses
// to turn `--peer <selector>` values into a concrete `[]fleet.Peer`.
//
// Semantics:
//   - empty peerFlag → return all peers (default = entire fleet).
//   - len(peerFlag) > 0 → union of every selector group; each peer
//     matches if at least one --peer value classifies it in.
//
// `osFor` is the lazy GOOS probe for `os=` predicates; pass nil when
// no `os=` term is in scope (peerMatchesFilters drops `os=` predicates
// on nil resolver). `allPeers` is the universe to select from —
// typically `cfg.Peers` from peers.toml.
//
// `UnknownNames` reports bare-name --peer values that didn't match
// any known peer. Subcommands surface this as a warning so a typo
// (`--peer main_p` for `main_pc`) doesn't silently no-op.
func resolvePeers(
	allPeers []fleet.Peer,
	peerFlag []string,
	osFor peerOSResolver,
) (peerSelection, error) {
	if len(peerFlag) == 0 {
		matched := make([]fleet.Peer, len(allPeers))
		copy(matched, allPeers)
		return peerSelection{Matched: matched}, nil
	}
	groups, err := parsePeerFlags(peerFlag)
	if err != nil {
		return peerSelection{}, err
	}
	if err := validatePeerFilterKeys(groups); err != nil {
		return peerSelection{}, err
	}

	// Collect bare-name selectors so we can report typos. Bare names
	// produce a single-term group with key=="name"; anything else is
	// a key=value filter that doesn't have a typo notion.
	bareNames := collectBareNameSelectors(groups)
	known := make(map[string]struct{}, len(allPeers))
	for _, p := range allPeers {
		known[p.Name] = struct{}{}
	}
	var unknown []string
	for _, n := range bareNames {
		if _, ok := known[n]; !ok {
			unknown = append(unknown, n)
		}
	}

	var matched []fleet.Peer
	for _, p := range allPeers {
		if peerMatchesFilters(p, groups, osFor) {
			matched = append(matched, p)
		}
	}
	return peerSelection{Matched: matched, UnknownNames: unknown}, nil
}

// collectBareNameSelectors returns the value of every single-term
// `name=` group in groups. A bare-name --peer entry like `main_pc`
// produces exactly this shape; multi-term groups like
// `@tag=prod,name=main_pc` aren't bare names so don't qualify for
// the typo report (the user explicitly asked for an AND).
func collectBareNameSelectors(groups [][]filterTerm) []string {
	var out []string
	for _, g := range groups {
		if len(g) == 1 && g[0].key == "name" {
			out = append(out, g[0].value)
		}
	}
	return out
}

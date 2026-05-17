package fleet

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// MachineManifestName is the file that declares a machine's ordered apply
// phases. Conventionally lives at `<plan-dir>/machines/<name>/fleet.yml`
// next to the machine's vars.yml and entry plan. The filename is
// intentionally distinct from `index.yml` (the single-phase entry plan)
// so the two layouts coexist — a machine can ship a fleet.yml today and
// keep its single-peer index.yml in case operators bypass the manifest
// via `--peers` or `--from-plan`.
const MachineManifestName = "fleet.yml"

// MachineManifest is a machine's apply plan: an ordered list of phases,
// each pinning a peer + plan + optional per-phase vars/tags. Phases run
// sequentially with fail-fast semantics; subsequent phases are skipped
// after a non-zero phase result. Designed for the canonical Windows+WSL
// box layout where one physical machine exposes two agentd daemons that
// must be applied in order (Windows host → WSL guest).
type MachineManifest struct {
	// Phases run in declared order. Empty is invalid (Validate rejects).
	Phases []MachinePhase `yaml:"phases"`
}

// MachinePhase is one row of the manifest: which peer applies which plan
// for this machine, with optional per-phase vars/tags layered on top of
// the machine-wide vars stack.
type MachinePhase struct {
	// Name is a human-readable label for the phase, used in banners
	// ("phase 1/2 — windows-host"). Required.
	Name string `yaml:"name"`
	// Peer is the peers.toml [[peers]] name to drive this phase against.
	// Required.
	Peer string `yaml:"peer"`
	// Plan is the entry plan for this phase, relative to the manifest's
	// directory (i.e. `machines/<name>/`). Required.
	Plan string `yaml:"plan"`
	// Vars is an optional per-phase vars-file list, layered on top of
	// the machine-wide vars stack with the same later-wins semantics as
	// `mooncake apply -v a.yml -v b.yml`. Each entry is resolved
	// relative to the manifest's directory.
	Vars []string `yaml:"vars,omitempty"`
	// Tags optionally narrows step selection for this phase. AND'd with
	// any `--tags` passed at the command line.
	Tags []string `yaml:"tags,omitempty"`
}

// LookupMachineManifest checks whether `<planDir>/machines/<machine>/fleet.yml`
// exists. Returns (path, true, nil) when found; (path, false, nil) when the
// file doesn't exist (caller falls back to single-phase / index.yml mode);
// (_, _, err) for stat errors that aren't ErrNotExist.
//
// Path is returned absolute, suitable for passing straight to
// LoadMachineManifest.
func LookupMachineManifest(planDir, machine string) (path string, found bool, err error) {
	path = filepath.Join(planDir, "machines", machine, MachineManifestName)
	info, statErr := os.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return path, false, nil
		}
		return "", false, fmt.Errorf("stat machine manifest %s: %w", path, statErr)
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("machine manifest %s is a directory", path)
	}
	return path, true, nil
}

// LoadMachineManifest reads, parses, and validates a fleet.yml at path.
// Returns a fully-resolved manifest where each Plan / Vars entry is an
// absolute path on the controller (so callers don't need to remember the
// manifest's own location to compose paths downstream).
//
// Parsing is strict: unknown top-level or per-phase fields fail with a
// `field <name> not found in type` error rather than silently zero-
// valuing. This mirrors `internal/config/reader.go`'s plan-side
// behaviour and catches operator typos (`vrs:` for `vars:`,
// `peers:` for `peer:`) at load time instead of letting them produce
// a manifest that runs the wrong machine config. F048.
func LoadMachineManifest(path string) (*MachineManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m MachineManifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// Resolve paths relative to the manifest's own directory.
	manifestDir := filepath.Dir(path)
	for i := range m.Phases {
		p := &m.Phases[i]
		p.Plan = resolveManifestPath(manifestDir, p.Plan)
		for j, v := range p.Vars {
			p.Vars[j] = resolveManifestPath(manifestDir, v)
		}
	}
	return &m, nil
}

// resolveManifestPath joins p against the manifest directory and cleans
// the result. Absolute paths pass through unchanged so an operator can
// point a phase at a plan outside the conventional layout if needed.
func resolveManifestPath(manifestDir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(manifestDir, p))
}

// Validate returns the first structural error in m, or nil. Validation
// catches the things a stat-after-load can't (empty phases, missing
// required fields, duplicate phase names).
func (m *MachineManifest) Validate() error {
	if len(m.Phases) == 0 {
		return errors.New("machine manifest: phases list is empty")
	}
	seen := make(map[string]struct{}, len(m.Phases))
	for i, p := range m.Phases {
		if p.Name == "" {
			return fmt.Errorf("phase %d: name is empty", i)
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("duplicate phase name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
		if p.Peer == "" {
			return fmt.Errorf("phase %q: peer is empty", p.Name)
		}
		if p.Plan == "" {
			return fmt.Errorf("phase %q: plan is empty", p.Name)
		}
	}
	return nil
}

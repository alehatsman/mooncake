package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/presets"
	"github.com/alehatsman/mooncake/internal/schemagen"
)

// agent proposal-01: three MCP discovery tools so agents can
// introspect the action surface without out-of-band knowledge or
// shell access. Each is a thin wrapper over an internal API that
// already backs an existing CLI surface:
//
//   list_actions       → actions.List()             (≈ mooncake actions list)
//   describe_action    → registry + schemagen pipeline (≈ mooncake actions show)
//   list_presets       → presets.DiscoverAllPresets (in-process; no CLI equivalent — the `mooncake presets` CLI was retired in `2b7eee8e`)
//
// Returning JSON-as-text matches the rest of the MCP tool surface
// (the response envelope handles the text wrapping; handlers just
// return a JSON-stringified body).

// listActionsEntry is the per-row wire shape for the list_actions
// response. Mirrors the columns `mooncake actions list` shows so an
// agent calling list_actions sees the same data a human sees from
// the CLI. Capability bools come from the live interface satisfaction
// populated by Registry.List() (proposal-05).
type listActionsEntry struct {
	Name                  string   `json:"name"`
	Category              string   `json:"category"`
	Description           string   `json:"description,omitempty"`
	Platforms             []string `json:"platforms,omitempty"`
	RequiresSudo          bool     `json:"requires_sudo"`
	ImplementsCheck       bool     `json:"implements_check"`
	ImplementsDiff        bool     `json:"implements_diff"`
	ImplementsCost        bool     `json:"implements_cost"`
	ImplementsReverse     bool     `json:"implements_reverse"`
	ImplementsPermissions bool     `json:"implements_permissions"`
	Version               string   `json:"version,omitempty"`
}

// listActionsResult is the top-level wire shape: a sorted slice plus
// a total. Total is redundant for clients that count Actions
// themselves, but matches the proposal's documented response and
// lets a paginating future client check truncation without buffering
// the full slice.
type listActionsResult struct {
	Actions []listActionsEntry `json:"actions"`
	Total   int                `json:"total"`
}

// HandleListActions returns the full action inventory, optionally
// filtered by category. Mirrors `mooncake actions list --format json`
// — same data, different wire shape (richer JSON per-row).
func HandleListActions(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Category string `json:"category"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	category := strings.TrimSpace(params.Category)

	all := actions.List()
	out := make([]listActionsEntry, 0, len(all))
	for _, m := range all {
		if category != "" && string(m.Category) != category {
			continue
		}
		out = append(out, listActionsEntry{
			Name:                  m.Name,
			Category:              string(m.Category),
			Description:           m.Description,
			Platforms:             m.SupportedPlatforms,
			RequiresSudo:          m.RequiresSudo,
			ImplementsCheck:       m.ImplementsCheck,
			ImplementsDiff:        m.ImplementsDiff,
			ImplementsCost:        m.ImplementsCost,
			ImplementsReverse:     m.ImplementsReverse,
			ImplementsPermissions: m.ImplementsPermissions,
			Version:               m.Version,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return marshalIndent(listActionsResult{Actions: out, Total: len(out)})
}

// describeActionResult is the wire shape for describe_action. It
// reuses the schemagen Definition (with x-implements-* extensions)
// alongside a flattened metadata block so an agent doesn't need to
// re-read the extensions to know basic facts about the action.
type describeActionResult struct {
	Name         string                `json:"name"`
	Category     string                `json:"category"`
	Description  string                `json:"description,omitempty"`
	Platforms    []string              `json:"platforms,omitempty"`
	Capabilities map[string]bool       `json:"capabilities"`
	Schema       *schemagen.Definition `json:"schema"`
}

// HandleDescribeAction returns one action's typed contract: the
// schemagen Definition (parameters, required, defaults, enums) plus
// a capability summary derived from the spec-22 ABI. Mirrors the
// data `mooncake actions show <name>` renders as text.
func HandleDescribeAction(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name string `json:"name"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return "", fmt.Errorf("name parameter required")
	}

	var meta *actions.ActionMetadata
	for _, m := range actions.List() {
		if m.Name == name {
			m := m
			meta = &m
			break
		}
	}
	if meta == nil {
		return "", fmt.Errorf("unknown action %q", name)
	}

	gen := schemagen.NewGenerator(schemagen.GeneratorOptions{
		IncludeExtensions: true,
		OutputFormat:      "json",
	})
	schema, err := gen.Generate()
	if err != nil {
		return "", fmt.Errorf("generate schema: %w", err)
	}
	def, ok := schema.Definitions[name]
	if !ok {
		return "", fmt.Errorf("action %q has no schema definition (registry/schemagen drift)", name)
	}

	return marshalIndent(describeActionResult{
		Name:        meta.Name,
		Category:    string(meta.Category),
		Description: meta.Description,
		Platforms:   meta.SupportedPlatforms,
		Capabilities: map[string]bool{
			"check":       meta.ImplementsCheck,
			"diff":        meta.ImplementsDiff,
			"cost":        meta.ImplementsCost,
			"reverse":     meta.ImplementsReverse,
			"permissions": meta.ImplementsPermissions,
			"sudo":        meta.RequiresSudo,
			"dry_run":     meta.SupportsDryRun,
		},
		Schema: def,
	})
}

// listPresetsEntry is the wire shape per preset. Mirrors the
// presets.DiscoverAllPresets in-process surface so MCP clients see a
// stable record regardless of how the operator browses presets
// out-of-band (filesystem walk under `./presets/`, or future tooling).
type listPresetsEntry struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path,omitempty"`
}

type listPresetsResult struct {
	Presets []listPresetsEntry `json:"presets"`
	Total   int                `json:"total"`
}

// HandleListPresets returns every discoverable preset: built-ins,
// registry-cloned, and local. No arguments — the discovery is global.
// `describe_preset` (the per-preset parameter introspection) is
// deferred per proposal-01's "What this doesn't address" note.
func HandleListPresets(_ context.Context, _ json.RawMessage) (string, error) {
	all, err := presets.DiscoverAllPresets()
	if err != nil {
		return "", fmt.Errorf("discover presets: %w", err)
	}
	out := make([]listPresetsEntry, 0, len(all))
	for _, p := range all {
		out = append(out, listPresetsEntry{
			Name:        p.Name,
			Description: p.Description,
			Version:     p.Version,
			Source:      p.Source,
			Path:        p.Path,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return marshalIndent(listPresetsResult{Presets: out, Total: len(out)})
}

// marshalIndent is the JSON-as-text encoder shared by the discovery
// handlers. Matches HandleExplain's pattern (indent for readability,
// trailing newline trimmed).
func marshalIndent(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

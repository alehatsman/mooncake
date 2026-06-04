package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/alehatsman/mooncake/internal/fleet"
)

// HandleListPeers reads peers.toml and returns the peer list as JSON.
// Params: optional peers_file path.
func HandleListPeers(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		PeersFile string `json:"peers_file"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}

	peersPath, err := resolvePeersPath(params.PeersFile)
	if err != nil {
		return "", err
	}

	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return "", fmt.Errorf("load peers: %w", err)
	}

	type peerEntry struct {
		Name      string   `json:"name"`
		Addr      string   `json:"addr"`
		Transport string   `json:"transport"`
		Tags      []string `json:"tags,omitempty"`
		Roles     []string `json:"roles,omitempty"`
	}
	entries := make([]peerEntry, 0, len(cfg.Peers))
	for _, p := range cfg.Peers {
		entries = append(entries, peerEntry{
			Name:      p.Name,
			Addr:      p.Addr,
			Transport: string(p.Transport),
			Tags:      p.Tags,
			Roles:     p.Roles,
		})
	}

	result := map[string]interface{}{
		"peers":      entries,
		"total":      len(entries),
		"peers_file": peersPath,
	}
	return encodeJSON(result)
}

// HandleFleetRunPlan drives fleet.Orchestrator with the given config and
// peer selection, returning per-peer outcomes + summary as JSON.
// Params: config (required), peers_file, peers (name filter), vars_file, parallel.
func HandleFleetRunPlan(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Config    string   `json:"config"`
		PeersFile string   `json:"peers_file"`
		Peers     []string `json:"peers"`
		VarsFile  []string `json:"vars_file"`
		Parallel  int      `json:"parallel"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Config == "" {
		return "", fmt.Errorf("config parameter required")
	}

	peersPath, err := resolvePeersPath(params.PeersFile)
	if err != nil {
		return "", err
	}

	peerCfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return "", fmt.Errorf("load peers: %w", err)
	}
	if len(peerCfg.Peers) == 0 {
		return "", fmt.Errorf("%s", fleet.NoPeersConfiguredError(peersPath))
	}

	selected, unknown := filterPeersByName(peerCfg.Peers, params.Peers)
	if len(selected) == 0 {
		return "", fmt.Errorf("%s", fleet.NoPeersSelectedError(len(peerCfg.Peers), unknown))
	}

	var w bytes.Buffer
	cfg := &fleet.ApplyConfig{
		PlanArg:       params.Config,
		PeersPath:     peersPath,
		SelectedPeers: selected,
		UnknownPeers:  unknown,
		AllPeers:      peerCfg.Peers,
		VarsFilesRel:  params.VarsFile,
		Parallel:      params.Parallel,
		NoColor:       true,
		Writer:        io.Discard,
	}
	_ = w

	kr, runErr := fleet.NewOrchestrator(cfg).Run(ctx)

	return marshalFleetResult(kr, runErr)
}

// resolvePeersPath returns path if non-empty, otherwise fleet.DefaultPeersPath().
func resolvePeersPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	p, err := fleet.DefaultPeersPath()
	if err != nil {
		return "", fmt.Errorf("resolve peers path: %w", err)
	}
	return p, nil
}

// filterPeersByName selects peers whose Name is in the names slice.
// An empty names slice returns all peers. Unknown names are collected
// in the second return value so callers can warn or surface typos.
func filterPeersByName(all []fleet.Peer, names []string) (matched []fleet.Peer, unknown []string) {
	if len(names) == 0 {
		out := make([]fleet.Peer, len(all))
		copy(out, all)
		return out, nil
	}
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	known := make(map[string]struct{}, len(all))
	for _, p := range all {
		known[p.Name] = struct{}{}
		if _, ok := want[p.Name]; ok {
			matched = append(matched, p)
		}
	}
	for _, n := range names {
		if _, ok := known[n]; !ok {
			unknown = append(unknown, n)
		}
	}
	return
}

// marshalFleetResult converts a *fleet.KernelResult + run error to indented JSON.
func marshalFleetResult(kr *fleet.KernelResult, runErr error) (string, error) {
	type syncOut struct {
		Total      int   `json:"total"`
		Put        int   `json:"put"`
		Skipped    int   `json:"skipped"`
		BytesTotal int64 `json:"bytes_total"`
		BytesPut   int64 `json:"bytes_put"`
	}
	type peerOut struct {
		RunID       string  `json:"run_id,omitempty"`
		Status      string  `json:"status,omitempty"`
		EventsCount int     `json:"events_count"`
		Sync        syncOut `json:"sync"`
	}

	peers := map[string]peerOut{}
	if kr != nil {
		for id, pr := range kr.Peers {
			if pr == nil {
				continue
			}
			peers[id] = peerOut{
				RunID:       pr.RunID,
				Status:      pr.Status,
				EventsCount: pr.EventsCount,
				Sync: syncOut{
					Total:      pr.Sync.Total,
					Put:        pr.Sync.Put,
					Skipped:    pr.Sync.Skipped,
					BytesTotal: pr.Sync.BytesTotal,
					BytesPut:   pr.Sync.BytesPut,
				},
			}
		}
	}

	result := map[string]interface{}{
		"peers": peers,
	}
	if kr != nil {
		result["ok"] = kr.Summary.OK
		result["run_failed"] = kr.Summary.RunFailed
		result["unreachable"] = kr.Summary.Unreachable
		result["total_peers"] = kr.Summary.TotalPeers
		if len(kr.Summary.FailedNames) > 0 {
			result["failed_names"] = kr.Summary.FailedNames
		}
	}
	if runErr != nil {
		var exitErr *fleet.ExitError
		if errors.As(runErr, &exitErr) {
			result["error"] = exitErr.Message
			result["exit_code"] = exitErr.ExitCode
		} else {
			result["error"] = runErr.Error()
		}
	}

	return encodeJSON(result)
}

// encodeJSON marshals v to indented JSON string, trimming the trailing newline.
func encodeJSON(v interface{}) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

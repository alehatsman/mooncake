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

// HandleFleetCheckPlan previews what a config would do across the fleet
// without mutating any remote system. It syncs plan files to each peer then
// calls check_plan via the peer's MCP endpoint, returning per-peer
// would-change predictions alongside sync stats.
func HandleFleetCheckPlan(ctx context.Context, args json.RawMessage) (string, error) {
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

	controllerID, err := fleet.EnsureControllerID()
	if err != nil {
		return "", fmt.Errorf("controller id: %w", err)
	}

	planAbs, planDir, err := fleet.ResolvePlanPath(params.Config)
	if err != nil {
		return "", fmt.Errorf("resolve plan: %w", err)
	}

	varsAbs := fleet.ResolveVarsFilesAbs(planDir, params.VarsFile)

	type peerCheckOut struct {
		Status      string          `json:"status"`
		WouldChange *int            `json:"would_change,omitempty"`
		Check       json.RawMessage `json:"check,omitempty"`
		Sync        struct {
			Total      int   `json:"total"`
			Put        int   `json:"put"`
			Skipped    int   `json:"skipped"`
			BytesTotal int64 `json:"bytes_total"`
		} `json:"sync"`
		Error string `json:"error,omitempty"`
	}

	sem := make(chan struct{}, max(params.Parallel, len(selected)))
	if params.Parallel > 0 {
		sem = make(chan struct{}, params.Parallel)
	}

	type indexed struct {
		i   int
		out peerCheckOut
	}
	ch := make(chan indexed, len(selected))

	for i, p := range selected {
		go func(i int, p fleet.Peer) {
			sem <- struct{}{}
			defer func() { <-sem }()

			client := fleet.NewTransportClient(p)
			overlayVars := fleet.ResolveVarsFiles(planDir, p)
			peerVars := append(append([]string{}, overlayVars...), varsAbs...)

			r := fleet.Check(ctx, fleet.CheckOptions{
				PeerName:     p.Name,
				Peer:         client,
				PlanDir:      planDir,
				PlanPath:     planAbs,
				VarsFiles:    peerVars,
				ControllerID: controllerID,
			})

			out := peerCheckOut{}
			out.Sync.Total = r.Sync.Total
			out.Sync.Put = r.Sync.Put
			out.Sync.Skipped = r.Sync.Skipped
			out.Sync.BytesTotal = r.Sync.BytesTotal

			if r.Error != nil {
				out.Status = "error"
				out.Error = r.Error.Error()
			} else {
				out.Status = "ok"
				raw := json.RawMessage(r.Raw)
				out.Check = raw
				// Extract would_change count for the summary field.
				var checkSummary struct {
					Inspections []struct {
						WouldChange bool `json:"would_change"`
					} `json:"inspections"`
				}
				if json.Unmarshal([]byte(r.Raw), &checkSummary) == nil {
					n := 0
					for _, ins := range checkSummary.Inspections {
						if ins.WouldChange {
							n++
						}
					}
					out.WouldChange = &n
				}
			}
			ch <- indexed{i: i, out: out}
		}(i, p)
	}

	peers := make(map[string]peerCheckOut, len(selected))
	for range selected {
		item := <-ch
		peers[selected[item.i].Name] = item.out
	}

	warnUnknown := unknown
	result := map[string]interface{}{
		"peers":       peers,
		"total_peers": len(selected),
	}
	if len(warnUnknown) > 0 {
		result["unknown_peers"] = warnUnknown
	}
	return encodeJSON(result)
}

package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// CheckOptions configures one peer's check cycle: sync the plan-dir, then
// call check_plan via the peer's MCP endpoint. Mirrors ApplyOptions.
type CheckOptions struct {
	PeerName     string
	Peer         *transport.Client
	PlanDir      string
	PlanPath     string
	VarsFiles    []string
	Tags         []string
	Names        []string
	ControllerID string
	MaxSyncBytes int64
}

// CheckResult is the per-peer outcome of Check.
type CheckResult struct {
	PeerName string
	Sync     SyncStats
	// Raw is the JSON text returned by the peer's check_plan MCP tool.
	// Callers unmarshal it into whatever shape they need.
	Raw string
	// Error is non-nil when the check could not be completed (unreachable,
	// sync failure, MCP error).
	Error error
}

// Check runs sync → check_plan (via MCP) against one peer and returns the
// raw JSON payload from check_plan alongside sync stats. It does not mutate
// the remote system.
//
// Steps:
//  1. Walk PlanDir, enforcing MaxSyncBytes.
//  2. GetVersion to learn the peer's SyncedRoot.
//  3. SyncTo (same as Apply — check still needs the files present).
//  4. Translate PlanPath to the peer-side absolute path.
//  5. Call check_plan via POST /v1/mcp.
func Check(ctx context.Context, opts CheckOptions) CheckResult {
	result := CheckResult{PeerName: opts.PeerName}

	scope, err := ScopeFor(opts.ControllerID, opts.PlanDir)
	if err != nil {
		result.Error = fmt.Errorf("scope: %w", err)
		return result
	}

	entries, _, err := Walk(opts.PlanDir, opts.MaxSyncBytes)
	if err != nil {
		result.Error = fmt.Errorf("walk plan-dir: %w", err)
		return result
	}

	ver, err := opts.Peer.GetVersion(ctx)
	if err != nil {
		result.Error = fmt.Errorf("version probe: %w", err)
		return result
	}

	syncStats, err := SyncTo(ctx, opts.Peer, entries, scope)
	result.Sync = syncStats
	if err != nil {
		result.Error = fmt.Errorf("sync: %w", err)
		return result
	}

	planRel, err := filepath.Rel(opts.PlanDir, opts.PlanPath)
	if err != nil {
		result.Error = fmt.Errorf("rel plan_path: %w", err)
		return result
	}
	peerPlanPath := PeerPath(ver.SyncedRoot, scope, filepath.ToSlash(planRel))

	args, err := json.Marshal(map[string]string{"config": peerPlanPath})
	if err != nil {
		result.Error = fmt.Errorf("marshal check_plan args: %w", err)
		return result
	}

	raw, err := opts.Peer.CallMCPTool(ctx, "check_plan", args)
	if err != nil {
		result.Error = err
		return result
	}
	result.Raw = raw
	return result
}

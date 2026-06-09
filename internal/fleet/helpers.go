package fleet

import (
	"fmt"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// ResolvePlanPath converts planArg to an absolute path and derives planDir
// from it. It does not handle the machine-manifest convention — it is the
// simple file-path case used by MCP callers that already have a concrete YAML
// path (as opposed to a bare machine name).
func ResolvePlanPath(planArg string) (planAbs, planDir string, err error) {
	planAbs, err = filepath.Abs(planArg)
	if err != nil {
		return "", "", fmt.Errorf("resolve plan path: %w", err)
	}
	planDir = filepath.Dir(planAbs)
	return
}

// NewTransportClient builds a transport.Client for a peer.
func NewTransportClient(p Peer) *transport.Client {
	return transport.New(p.Name, p.Addr, p.Token)
}

// ResolveVarsFilesAbs converts a slice of relative or absolute varsfile paths
// into absolute paths anchored at planDir.
func ResolveVarsFilesAbs(planDir string, varsRel []string) []string {
	out := make([]string, 0, len(varsRel))
	for _, v := range varsRel {
		if !filepath.IsAbs(v) {
			v = filepath.Join(planDir, v)
		}
		out = append(out, filepath.Clean(v))
	}
	return out
}

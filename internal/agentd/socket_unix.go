//go:build !windows

package agentd

import "os"

// chmodSocket tightens the unix socket file to 0600 (user-only). On Unix
// this is the primary gate that keeps non-owner processes off the local
// admin endpoint — the socket is created with the umask, which can be
// permissive, so we tighten explicitly.
func chmodSocket(path string) error {
	return os.Chmod(path, 0o600)
}

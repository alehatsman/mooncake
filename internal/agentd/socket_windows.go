//go:build windows

package agentd

// chmodSocket is a no-op on Windows. AF_UNIX sockets on Windows pick up
// the ACLs of their containing directory; the daemon places the socket
// under %LOCALAPPDATA%\Mooncake\ which is already user-private by default.
// Calling os.Chmod here would silently no-op (Windows ignores POSIX mode
// bits beyond a read-only toggle), so we make that explicit rather than
// pretending the call did something.
func chmodSocket(_ string) error {
	return nil
}

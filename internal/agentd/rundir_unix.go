//go:build !windows && !darwin

package agentd

// systemRunDir is the volatile runtime directory that holds the system-mode
// agentd socket. Linux and the BSDs put it at /run (or symlink /var/run to
// it), which is tmpfs and root-writable, so the socket dir can be created on
// demand at daemon start.
const systemRunDir = "/run"

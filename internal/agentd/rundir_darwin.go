//go:build darwin

package agentd

// macOS has no /run, and since Big Sur the root filesystem is a read-only
// signed system volume — so `mkdir /run` fails with EROFS and no amount of
// privilege helps. /var/run (a symlink to /private/var/run on the writable
// data volume) is the platform's equivalent and is root-writable.
//
// Getting this wrong is silent from the outside: the launchd plist installs
// fine, the job loads, then the process exits 1 on every spawn and KeepAlive
// respawns it forever. See issue #49.
const systemRunDir = "/var/run"

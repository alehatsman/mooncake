package agentd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Self-shutdown endpoint. Allows `mooncake fleet shutdown <peer>` to
// power the host off via agentd. The handler replies 202 BEFORE the
// real shutdown command runs; the operator polls /v1/version (or just
// observes the missing peer) to confirm.
//
// Wire shape:
//
//	POST /v1/self/shutdown   body={"force":bool}
//	     → 202 {"daemon_pid":N,"scheduled_in_sec":1}
//	     → 409 {"error":"runs_in_flight",...} if a run is active and
//	            force=false
//
// Forwards the actual exec to the OS-specific shutdownCommand func
// (self_shutdown_unix.go / self_shutdown_windows.go) one second after
// the response is flushed.

// shutdownMu serialises self-shutdown attempts AND guards the package-
// level shutdownArmed / shutdownDelay / shutdownExec hooks. Only one
// shutdown can be scheduled per daemon lifetime; once the goroutine
// has launched, further calls return 409. Tests also use SetShutdown*
// to mutate the hooks, so every read/write goes through the mutex —
// the post-response goroutine snapshots a local copy under the lock
// before sleeping so test cleanup can't race with the exec.
var shutdownMu sync.Mutex

// shutdownArmed records whether the post-response goroutine has been
// scheduled. Guarded by shutdownMu.
var shutdownArmed bool

// shutdownDelay is the gap between writing the 202 response and the
// real exec. Lets the client read the response body before the
// network goes away. Guarded by shutdownMu; tests override via
// SetShutdownDelay.
var shutdownDelay = 1 * time.Second

// SetShutdownDelay overrides the post-response delay before exec. Test
// hook. Returns the previous value so tests can restore it.
func SetShutdownDelay(d time.Duration) time.Duration {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	prev := shutdownDelay
	shutdownDelay = d
	return prev
}

// shutdownExec is the function called by the post-response goroutine
// to actually power off the host. Guarded by shutdownMu; tests override
// via SetShutdownExec.
var shutdownExec = realShutdownExec

// SetShutdownExec replaces the shutdown-exec function. Test hook;
// returns the previous value so tests can restore it.
func SetShutdownExec(fn func() error) func() error {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	prev := shutdownExec
	shutdownExec = fn
	return prev
}

type shutdownRequest struct {
	Force bool `json:"force,omitempty"`
}

type shutdownResponse struct {
	DaemonPID      int `json:"daemon_pid"`
	ScheduledInSec int `json:"scheduled_in_sec"`
}

func (s *Server) selfShutdownHandler(w http.ResponseWriter, r *http.Request) {
	var req shutdownRequest
	// Empty body is fine — force defaults to false. Tolerate it.
	if r.ContentLength > 0 {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14))
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
	}

	shutdownMu.Lock()
	defer shutdownMu.Unlock()

	if shutdownArmed {
		writeError(w, http.StatusConflict, "shutdown_in_progress",
			"a shutdown has already been scheduled on this daemon")
		return
	}

	_, running := s.worker.Stats()
	if running > 0 && !req.Force {
		writeError(w, http.StatusConflict, "runs_in_flight",
			fmt.Sprintf("%d run(s) in flight; pass force=true to shutdown anyway", running))
		return
	}

	// Snapshot the hook values under the lock so the goroutine can
	// run them without racing against SetShutdown* calls from test
	// cleanup. After this block, the goroutine is the sole owner of
	// its local copies — Set* calls only affect future shutdowns.
	delay := shutdownDelay
	exec := shutdownExec
	scheduledSec := int(delay / time.Second)
	if scheduledSec < 1 {
		scheduledSec = 1
	}

	shutdownArmed = true

	writeJSON(w, http.StatusAccepted, shutdownResponse{
		DaemonPID:      os.Getpid(),
		ScheduledInSec: scheduledSec,
	})

	go func() {
		time.Sleep(delay)
		if err := exec(); err != nil {
			s.log.Error("shutdown exec failed", "err", err)
			// Re-arm so a follow-up POST can retry — the host didn't
			// actually go down.
			shutdownMu.Lock()
			shutdownArmed = false
			shutdownMu.Unlock()
		}
	}()
}

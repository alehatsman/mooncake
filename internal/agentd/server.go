package agentd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/discovery"
	"github.com/alehatsman/mooncake/internal/mcp"
)

// Server is an HTTP daemon exposing mooncake over `/v1/...`. It listens
// on a unix socket (filesystem-perm-gated) when `cfg.SocketPath` is set
// and/or on a TCP address (bearer-auth-gated) when `cfg.BindAddr` is set.
// At least one of the two must be configured; Validate() enforces this.
type Server struct {
	cfg       Config
	log       *slog.Logger
	version   string
	hostname  string
	startedAt time.Time

	mcp    *mcp.Server
	store  *Store
	worker *Worker

	unixSrv *http.Server
	unixLn  net.Listener
	tcpSrv  *http.Server
	tcpLn   net.Listener
}

func New(cfg Config, log *slog.Logger, version string) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}

	store, err := NewStore(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	if changed, err := store.Reconcile(os.Getpid()); err != nil {
		log.Error("reconcile previous-daemon runs", "err", err)
	} else if len(changed) > 0 {
		log.Info("reconciled interrupted runs from previous daemon", "count", len(changed), "ids", changed)
	}

	// The MCP server's stdio reader/writer are unused here — DispatchBytes
	// handles each request directly. Use io.Discard for the writer so any
	// stray writes don't leak; nil for the reader since Serve() is never
	// called on this instance.
	mcpSrv := mcp.New(nil, io.Discard)
	mcp.RegisterAllTools(mcpSrv)

	worker := NewWorker(store, log)

	// Cache hostname at startup. os.Hostname() is cheap, but caching makes
	// the version handler trivial and dodges a rare-but-possible error path
	// inside a request handler.
	hn, err := os.Hostname()
	if err != nil {
		log.Warn("read hostname", "err", err)
		hn = ""
	}

	return &Server{
		cfg:       cfg,
		log:       log,
		version:   version,
		hostname:  hn,
		startedAt: time.Now(),
		mcp:       mcpSrv,
		store:     store,
		worker:    worker,
	}, nil
}

// Serve binds the configured listeners and serves HTTP until ctx is
// canceled. The unix socket is opened only when cfg.SocketPath is set;
// the TCP listener is opened only when cfg.BindAddr is set. Both are
// optional individually but at least one must be configured.
//
// The caller is responsible for cancelling ctx (typically on SIGTERM/SIGINT)
// and then waiting for Serve to return.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.claimSocket(); err != nil {
		return err
	}
	go s.worker.Run()

	if s.cfg.SocketPath != "" {
		unixLn, err := net.Listen("unix", s.cfg.SocketPath)
		if err != nil {
			return fmt.Errorf("listen %s: %w", s.cfg.SocketPath, err)
		}
		// chmod(0o600) is a documented no-op on Windows. Skip it there
		// so the build is honest about what we rely on; %LOCALAPPDATA%
		// already has user-private ACLs by default.
		if err := chmodSocket(s.cfg.SocketPath); err != nil {
			_ = unixLn.Close()
			return fmt.Errorf("chmod socket: %w", err)
		}
		s.unixLn = unixLn
		s.unixSrv = &http.Server{
			Handler:           s.buildHandler(false),
			ReadHeaderTimeout: 10 * time.Second,
		}
	}

	if s.cfg.BindAddr != "" {
		tcpLn, err := net.Listen("tcp", s.cfg.BindAddr)
		if err != nil {
			if s.unixLn != nil {
				_ = s.unixLn.Close()
				_ = os.Remove(s.cfg.SocketPath)
			}
			return fmt.Errorf("listen %s: %w", s.cfg.BindAddr, err)
		}
		s.tcpLn = tcpLn
		s.tcpSrv = &http.Server{
			Handler:           s.buildHandler(true),
			ReadHeaderTimeout: 10 * time.Second,
		}
	}

	s.log.Info("agentd listening",
		"socket", s.cfg.SocketPath,
		"bind", s.cfg.BindAddr,
		"state_dir", s.cfg.StateDir,
		"synced_root", s.cfg.SyncedRoot(),
		"system_mode", s.cfg.SystemMode,
		"hostname", s.hostname,
		"pid", os.Getpid(),
	)

	// Spec-45 §Task 1 — mDNS advertise. Only when TCP is bound (no
	// point advertising a unix-socket-only daemon over the LAN) and
	// AdvertiseMDNS is on. Runs for the lifetime of ctx and shuts down
	// cleanly when ctx is canceled; errors here are non-fatal (the
	// daemon stays useful even if advertising fails — operators can
	// fall back to peers.toml).
	if s.cfg.BindAddr != "" && s.cfg.AdvertiseMDNS {
		port, err := tcpPortFromBindAddr(s.cfg.BindAddr)
		if err != nil {
			s.log.Warn("mDNS advertise disabled: cannot parse bind addr", "bind", s.cfg.BindAddr, "err", err)
		} else {
			instance := s.cfg.AdvertiseName
			if instance == "" {
				instance = trimHostnameFirstLabel(s.hostname)
			}
			s.log.Info("agentd advertising via mDNS",
				"service", discovery.MDNSServiceType,
				"instance", instance,
				"port", port,
			)
			go func() {
				err := discovery.Advertise(ctx, discovery.AdvertiseOptions{
					InstanceName: instance,
					Port:         port,
					Version:      s.version,
					Hostname:     trimHostnameFirstLabel(s.hostname),
					SystemMode:   s.cfg.SystemMode,
				})
				if err != nil && !errors.Is(err, context.Canceled) {
					s.log.Warn("mDNS advertise exited with error", "err", err)
				}
			}()
		}
	}

	errCh := make(chan error, 2)
	if s.unixSrv != nil {
		go func() {
			err := s.unixSrv.Serve(s.unixLn)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("unix serve: %w", err)
				return
			}
			errCh <- nil
		}()
	}
	if s.tcpSrv != nil {
		go func() {
			err := s.tcpSrv.Serve(s.tcpLn)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("tcp serve: %w", err)
				return
			}
			errCh <- nil
		}()
	}

	select {
	case <-ctx.Done():
		return s.shutdown()
	case err := <-errCh:
		// One listener errored — bring everything down.
		_ = s.shutdown()
		return err
	}
}

// tcpPortFromBindAddr extracts the integer port from a "host:port"
// bind address. Used by the mDNS advertise goroutine which needs the
// port in numeric form for the SRV record.
func tcpPortFromBindAddr(addr string) (int, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, fmt.Errorf("non-integer port %q: %w", p, err)
	}
	return port, nil
}

// trimHostnameFirstLabel strips the trailing `.local` (or any other
// trailing DNS label) so the advertised name matches the operator's
// mental model. macOS's os.Hostname() reports `MacBook-Air.local`; the
// operator wants `MacBook-Air` in peers.toml and in `dns-sd` output.
func trimHostnameFirstLabel(h string) string {
	if i := strings.Index(h, "."); i >= 0 {
		return h[:i]
	}
	return h
}

// buildHandler composes the middleware stack for a listener. requireAuth=true
// is used on the TCP listener; the unix socket relies on filesystem perms
// (0600) instead.
func (s *Server) buildHandler(requireAuth bool) http.Handler {
	h := http.Handler(s.routes())
	h = accessLogMiddleware(s.log)(h)
	h = recoverMiddleware(s.log)(h)
	if requireAuth {
		h = bearerAuthMiddleware(s.cfg.Token)(h)
	}
	// requestIDMiddleware outermost so panics and access logs carry the id.
	h = requestIDMiddleware(h)
	return h
}

func (s *Server) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.log.Info("agentd shutting down")

	var errs []error
	if s.unixSrv != nil {
		if err := s.unixSrv.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("unix shutdown: %w", err))
		}
	}
	if s.tcpSrv != nil {
		if err := s.tcpSrv.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("tcp shutdown: %w", err))
		}
	}
	// Drain in-flight runs. v1 has no cancellation, so this may block until
	// the current plan finishes.
	s.worker.Shutdown()
	if s.cfg.SocketPath != "" {
		_ = os.Remove(s.cfg.SocketPath)
	}
	return errors.Join(errs...)
}

// claimSocket implements the standard "is anyone home?" dance: if the socket
// path already exists, try to dial it; if dial succeeds, another daemon owns
// it. Otherwise the file is stale and we remove it. No-op in TCP-only mode.
func (s *Server) claimSocket() error {
	if s.cfg.SocketPath == "" {
		return nil
	}
	info, err := os.Stat(s.cfg.SocketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket", s.cfg.SocketPath)
	}
	conn, err := net.DialTimeout("unix", s.cfg.SocketPath, 250*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("another mooncake agentd is already listening on %s", s.cfg.SocketPath)
	}
	if err := os.Remove(s.cfg.SocketPath); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", healthHandler)
	mux.HandleFunc("GET /v1/version", s.versionHandler)
	mux.HandleFunc("GET /v1/facts", factsHandler)
	mux.HandleFunc("GET /v1/metrics", metricsHandler)
	mux.HandleFunc("POST /v1/mcp", s.mcpHandler)
	mux.HandleFunc("POST /v1/runs", s.submitRunHandler)
	mux.HandleFunc("GET /v1/runs", s.listRunsHandler)
	mux.HandleFunc("GET /v1/runs/{id}", s.getRunHandler)
	mux.HandleFunc("GET /v1/runs/{id}/result", s.runResultHandler)
	mux.HandleFunc("GET /v1/runs/{id}/events", s.runEventsHandler)
	mux.HandleFunc("PUT /v1/files", s.putFileHandler)
	mux.HandleFunc("HEAD /v1/files", s.headFileHandler)
	mux.HandleFunc("PUT /v1/self/binary", s.selfBinaryHandler)
	mux.HandleFunc("POST /v1/self/replace", s.selfReplaceHandler)
	mux.HandleFunc("GET /v1/self/mac", s.selfMACHandler)
	mux.HandleFunc("POST /v1/self/shutdown", s.selfShutdownHandler)
	mux.HandleFunc("/", notFoundHandler)
	return mux
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("%s %s", r.Method, r.URL.Path))
}

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
	"time"

	"github.com/alehatsman/mooncake/internal/mcp"
)

// Server is a unix-socket HTTP daemon exposing mooncake over `/v1/...`.
type Server struct {
	cfg       Config
	log       *slog.Logger
	version   string
	startedAt time.Time

	mcp      *mcp.Server
	store    *Store
	worker   *Worker
	httpSrv  *http.Server
	listener net.Listener
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

	return &Server{
		cfg:       cfg,
		log:       log,
		version:   version,
		startedAt: time.Now(),
		mcp:       mcpSrv,
		store:     store,
		worker:    worker,
	}, nil
}

// Serve binds the unix socket and serves HTTP until ctx is canceled. The
// caller is responsible for cancelling ctx (typically on SIGTERM/SIGINT) and
// then waiting for Serve to return.
func (s *Server) Serve(ctx context.Context) error {
	if err := s.claimSocket(); err != nil {
		return err
	}
	go s.worker.Run()

	ln, err := net.Listen("unix", s.cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.SocketPath, err)
	}
	if err := os.Chmod(s.cfg.SocketPath, 0o600); err != nil {
		_ = ln.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}
	s.listener = ln

	mux := s.routes()
	handler := requestIDMiddleware(
		recoverMiddleware(s.log)(
			accessLogMiddleware(s.log)(mux),
		),
	)

	s.httpSrv = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	s.log.Info("agentd listening",
		"socket", s.cfg.SocketPath,
		"state_dir", s.cfg.StateDir,
		"system_mode", s.cfg.SystemMode,
		"pid", os.Getpid(),
	)

	errCh := make(chan error, 1)
	go func() {
		err := s.httpSrv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		return s.shutdown()
	case err := <-errCh:
		return err
	}
}

func (s *Server) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.log.Info("agentd shutting down")
	err := s.httpSrv.Shutdown(shutdownCtx)
	// Drain in-flight runs. v1 has no cancellation, so this may block until
	// the current plan finishes.
	s.worker.Shutdown()
	_ = os.Remove(s.cfg.SocketPath)
	return err
}

// claimSocket implements the standard "is anyone home?" dance: if the socket
// path already exists, try to dial it; if dial succeeds, another daemon owns
// it. Otherwise the file is stale and we remove it.
func (s *Server) claimSocket() error {
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
	mux.HandleFunc("GET /v1/runs/{id}/events", s.runEventsHandler)
	mux.HandleFunc("/", notFoundHandler)
	return mux
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("%s %s", r.Method, r.URL.Path))
}

package agentd

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxKey int

const ctxKeyRequestID ctxKey = iota

const requestIDHeader = "X-Request-ID"

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func requestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Flush delegates to the underlying writer so SSE handlers can do
// `w.(http.Flusher)` even when wrapped by access-log middleware.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func accessLogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			log.Info("http",
				"request_id", requestIDFrom(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// bearerAuthMiddleware rejects requests whose Authorization header does not
// match `Bearer <token>` exactly.
//
// F029: both sides are SHA-256'd to a fixed-size 32-byte digest before
// ConstantTimeCompare, so the comparison is truly constant-time regardless
// of the attacker's input length. subtle.ConstantTimeCompare alone
// short-circuits on a length mismatch — that leaks token length via
// response timing. Hashing erases the leak; SHA-256 of "Bearer "+32 chars
// is in the microsecond range and cheaper than the cost of the surrounding
// HTTP machinery.
//
// Used on the TCP listener only. The unix socket relies on filesystem
// permissions (0600) for access control.
func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
	expectedHash := sha256.Sum256([]byte("Bearer " + token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotHash := sha256.Sum256([]byte(r.Header.Get("Authorization")))
			if subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="mooncake-agentd"`)
				writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func recoverMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic",
						"request_id", requestIDFrom(r.Context()),
						"path", r.URL.Path,
						"panic", p,
						"stack", string(debug.Stack()),
					)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

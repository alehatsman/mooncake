package agentd

// F029: bearerAuthMiddleware must SHA-256 both sides before
// subtle.ConstantTimeCompare so the comparison is constant-time
// regardless of input length. Without hashing, a length mismatch
// short-circuits ConstantTimeCompare and the response time leaks
// token length via a timing side-channel.
//
// These tests cover (1) correctness: the right token still passes,
// wrong / short / long tokens are rejected; and (2) a structural
// regression guard that the source-level pattern is the hashed one.
// A future refactor that drops the hash will trip the structural
// guard immediately.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const f029Token = "ed5b96a8c4ce42e9b1ca28f80f1d7d10"

func newAuthHandler() http.Handler {
	mw := bearerAuthMiddleware(f029Token)
	return mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

// TestBearerAuth_Correctness exercises the middleware end-to-end.
// Pre-fix all of these passed too — F029 is timing-side-channel, not a
// correctness bug. The test pins behavior so the structural change
// can't accidentally break authentication.
func TestBearerAuth_Correctness(t *testing.T) {
	h := newAuthHandler()
	cases := []struct {
		name     string
		header   string
		wantCode int
	}{
		{"valid", "Bearer " + f029Token, http.StatusOK},
		{"missing", "", http.StatusUnauthorized},
		{"empty bearer", "Bearer ", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + f029Token, http.StatusUnauthorized},
		{"shorter wrong token", "Bearer ed5b", http.StatusUnauthorized},
		{"longer wrong token", "Bearer " + f029Token + "extra", http.StatusUnauthorized},
		{"right length wrong content", "Bearer 11111111111111111111111111111111", http.StatusUnauthorized},
		{"prefix-only", "Bearer", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Errorf("Authorization=%q → status=%d, want %d", c.header, rec.Code, c.wantCode)
			}
		})
	}
}

// TestBearerAuth_StructuralGuard is the F029 regression guard. The
// middleware source must SHA-256 the inputs before comparison; if a
// future refactor goes back to comparing raw byte slices, the length
// side-channel comes back and ConstantTimeCompare's docstring promise
// no longer matches the comment claiming constant-time response.
//
// Source-level audit: middleware.go must import crypto/sha256, must
// build expectedHash from sha256.Sum256, and must not invoke
// ConstantTimeCompare on a non-hash byte slice.
func TestBearerAuth_StructuralGuard(t *testing.T) {
	body, err := os.ReadFile("middleware.go")
	if err != nil {
		t.Fatalf("read middleware.go: %v", err)
	}
	src := string(body)
	for _, required := range []string{
		`"crypto/sha256"`,
		`sha256.Sum256`,
		`ConstantTimeCompare(gotHash[:], expectedHash[:])`,
	} {
		if !strings.Contains(src, required) {
			t.Errorf("middleware.go missing F029 invariant %q", required)
		}
	}
	// The pre-F029 pattern compared raw byte slices — flag any
	// regression that reintroduces it.
	if strings.Contains(src, "ConstantTimeCompare(got, expected)") ||
		strings.Contains(src, "ConstantTimeCompare(expected, got)") {
		t.Error("middleware.go still compares un-hashed byte slices — F029 regressed (length side-channel)")
	}
}

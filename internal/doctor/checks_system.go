package doctor

import (
	"crypto/x509"
	"fmt"
	"runtime"

	"github.com/alehatsman/mooncake/internal/facts"
)

// checkFacts confirms facts collection works and surfaces a one-line
// summary of the detected host. facts.Collect is cached (sync.Once) so
// this call is a hash-lookup after the first invocation.
type checkFacts struct{}

func (checkFacts) Section() string { return "system" }
func (checkFacts) Name() string    { return "facts" }
func (checkFacts) Run(_ Context) Result {
	r := Result{Section: "system", Name: "facts"}
	f := facts.Collect()
	if f == nil {
		r.Status = StatusError
		r.Message = "facts collection returned nil"
		r.Fix = "report a bug: https://github.com/alehatsman/mooncake/issues"
		return r
	}
	r.Status = StatusOK
	r.Message = fmt.Sprintf("os=%s arch=%s distribution=%s package_manager=%s",
		valueOr(f.OS, "?"), valueOr(f.Arch, "?"),
		valueOr(f.Distribution, "-"), valueOr(f.PackageManager, "-"))
	r.Detail = "Use `mooncake facts` for the full list"
	return r
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// checkTLSTrust verifies that the host has a usable trust store for
// outbound HTTPS. On minimal containers (notably vanilla ubuntu:24.04
// without `ca-certificates`) x509.SystemCertPool returns an empty
// pool, and `file.download` / `git.clone` over HTTPS fails with
// "tls: failed to verify certificate". This check flags the gap
// before a step trips it (MT-21).
type checkTLSTrust struct{}

// systemCertPool is indirected through a package-level var so unit
// tests can inject a hermetic stub (empty pool, error, populated)
// without poking the real system.
var systemCertPool = x509.SystemCertPool

func (checkTLSTrust) Section() string { return "system" }
func (checkTLSTrust) Name() string    { return "tls-trust" }
func (checkTLSTrust) Run(_ Context) Result {
	r := Result{Section: "system", Name: "tls-trust"}

	// Windows has its own cert store mechanism that x509.SystemCertPool
	// doesn't load (returns an empty pool by design). The OS handles
	// TLS via SChannel separately, so an "empty" pool here isn't the
	// signal it is on Linux/macOS. Skip with info instead of warning.
	if runtime.GOOS == "windows" {
		r.Status = StatusInfo
		r.Message = "trust store managed by Windows; mooncake delegates to the OS"
		return r
	}

	pool, err := systemCertPool()
	if err != nil {
		r.Status = StatusWarning
		r.Message = fmt.Sprintf("failed to read system trust store: %v", err)
		r.Fix = "install the ca-certificates package (apt-get install ca-certificates, apk add ca-certificates, etc.)"
		return r
	}
	if pool == nil {
		r.Status = StatusWarning
		r.Message = "system trust store is empty"
		r.Fix = "install the ca-certificates package (apt-get install ca-certificates, apk add ca-certificates, etc.) — HTTPS downloads will fail with \"x509: certificate signed by unknown authority\""
		return r
	}
	subjects := pool.Subjects() //nolint:staticcheck // Subjects() returns []byte slices; we only want the count
	if len(subjects) == 0 {
		r.Status = StatusWarning
		r.Message = "system trust store is empty (0 root CAs)"
		r.Fix = "install the ca-certificates package (apt-get install ca-certificates, apk add ca-certificates, etc.) — HTTPS downloads will fail with \"x509: certificate signed by unknown authority\""
		return r
	}

	r.Status = StatusOK
	r.Message = fmt.Sprintf("system trust store loaded (%d root CAs)", len(subjects))
	return r
}

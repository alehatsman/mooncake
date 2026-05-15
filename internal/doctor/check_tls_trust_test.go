package doctor

import (
	"crypto/x509"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// MT-21: doctor's TLS-trust check must flag an empty system trust
// store. On vanilla containers without `ca-certificates` installed,
// x509.SystemCertPool returns an empty pool — and every later
// file.download / git.clone fails with "x509: certificate signed by
// unknown authority". The check pre-emptively surfaces the gap.

func TestCheckTLSTrust_EmptyPoolWarnsWithFix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses SChannel; the system pool is empty by design")
	}
	orig := systemCertPool
	systemCertPool = func() (*x509.CertPool, error) {
		return x509.NewCertPool(), nil
	}
	defer func() { systemCertPool = orig }()

	r := checkTLSTrust{}.Run(Context{})
	if r.Status != StatusWarning {
		t.Errorf("status = %v, want Warning", r.Status)
	}
	if !strings.Contains(r.Message, "empty") {
		t.Errorf("message should mention 'empty'; got %q", r.Message)
	}
	if !strings.Contains(r.Fix, "ca-certificates") {
		t.Errorf("fix should name ca-certificates; got %q", r.Fix)
	}
}

func TestCheckTLSTrust_PoolReadFailureWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows skips read")
	}
	orig := systemCertPool
	systemCertPool = func() (*x509.CertPool, error) {
		return nil, fmt.Errorf("permission denied: /etc/ssl/certs")
	}
	defer func() { systemCertPool = orig }()

	r := checkTLSTrust{}.Run(Context{})
	if r.Status != StatusWarning {
		t.Errorf("status = %v, want Warning", r.Status)
	}
	if !strings.Contains(r.Message, "permission denied") {
		t.Errorf("message should surface the underlying error; got %q", r.Message)
	}
}

func TestCheckTLSTrust_PopulatedPoolIsOK(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows skips read")
	}
	// A minimal self-signed cert exercises the populated branch.
	pemData := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(pemData)) {
		t.Fatal("failed to seed pool with test cert")
	}
	orig := systemCertPool
	systemCertPool = func() (*x509.CertPool, error) { return pool, nil }
	defer func() { systemCertPool = orig }()

	r := checkTLSTrust{}.Run(Context{})
	if r.Status != StatusOK {
		t.Errorf("status = %v, want OK", r.Status)
	}
	if !strings.Contains(r.Message, "trust store loaded") {
		t.Errorf("message should confirm load; got %q", r.Message)
	}
}

func TestCheckTLSTrust_WindowsSkipsWithInfo(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only meaningful on Windows")
	}
	r := checkTLSTrust{}.Run(Context{})
	if r.Status != StatusInfo {
		t.Errorf("Windows path should report Info; got %v", r.Status)
	}
}

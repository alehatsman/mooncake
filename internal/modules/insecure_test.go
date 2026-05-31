package modules

import "testing"

func TestReference_CloneURLWithScheme(t *testing.T) {
	r := Reference{Host: "127.0.0.1:8080", Owner: "owner", Repo: "go-quality", Version: "v0.1.0"}
	if got, want := r.CloneURLWithScheme("http"), "http://127.0.0.1:8080/owner/go-quality.git"; got != want {
		t.Errorf("http scheme = %q, want %q", got, want)
	}
	// CloneURL stays the https default.
	if got, want := r.CloneURL(), "https://127.0.0.1:8080/owner/go-quality.git"; got != want {
		t.Errorf("default = %q, want %q", got, want)
	}
}

func TestFetcher_cloneURLFor(t *testing.T) {
	ref := Reference{Host: "127.0.0.1:8080", Owner: "o", Repo: "r", Version: "v1"}
	github := Reference{Host: "github.com", Owner: "o", Repo: "r", Version: "v1"}

	// Default: https for every host.
	if got := (&Fetcher{}).cloneURLFor(ref); got != "https://127.0.0.1:8080/o/r.git" {
		t.Errorf("default scheme = %q, want https", got)
	}

	// InsecureHosts field opts a specific host:port into http; others stay https.
	f := &Fetcher{InsecureHosts: []string{"127.0.0.1:8080"}}
	if got, want := f.cloneURLFor(ref), "http://127.0.0.1:8080/o/r.git"; got != want {
		t.Errorf("insecure host = %q, want %q", got, want)
	}
	if got, want := f.cloneURLFor(github), "https://github.com/o/r.git"; got != want {
		t.Errorf("non-insecure host = %q, want %q (https)", got, want)
	}

	// CloneURL override (tests) wins over scheme selection.
	override := &Fetcher{
		InsecureHosts: []string{"127.0.0.1:8080"},
		CloneURL:      func(Reference) string { return "file:///tmp/bare" },
	}
	if got := override.cloneURLFor(ref); got != "file:///tmp/bare" {
		t.Errorf("override = %q, want file:///tmp/bare", got)
	}
}

func TestFetcher_insecureHostFromEnv(t *testing.T) {
	t.Setenv("MOONCAKE_MODULE_INSECURE", "localhost:9000, 127.0.0.1:8080 ")
	f := &Fetcher{}
	// Both entries (whitespace-trimmed) are honored.
	if !f.insecureHost("127.0.0.1:8080") {
		t.Error("expected 127.0.0.1:8080 trusted via env")
	}
	if !f.insecureHost("localhost:9000") {
		t.Error("expected localhost:9000 trusted via env")
	}
	if f.insecureHost("github.com") {
		t.Error("github.com must not be trusted")
	}
	if f.insecureHost("") {
		t.Error("empty host must never be trusted")
	}
}

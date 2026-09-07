package modules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolver_InlineRemote(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0")
	cacheRoot := t.TempDir()
	r := &Resolver{
		Fetcher: &Fetcher{
			Root:     cacheRoot,
			CloneURL: func(_ Reference) string { return "file://" + bare },
		},
	}
	got, err := r.Resolve(context.Background(), "github.com/owner/testmod@v1.0.0")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(cacheRoot, "github.com/owner/testmod@v1.0.0/components/install.yml")
	if got.ComponentPath != want {
		t.Errorf("ComponentPath = %q, want %q", got.ComponentPath, want)
	}
}

func TestResolver_Alias_Default(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0")
	cacheRoot := t.TempDir()
	r := &Resolver{
		Fetcher: &Fetcher{
			Root:     cacheRoot,
			CloneURL: func(_ Reference) string { return "file://" + bare },
		},
		Modules: map[string]string{
			"testmod": "github.com/owner/testmod@v1.0.0",
		},
	}
	got, err := r.Resolve(context.Background(), "testmod")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.HasSuffix(got.ComponentPath, "components/install.yml") {
		t.Errorf("ComponentPath = %q, want suffix components/install.yml", got.ComponentPath)
	}
}

func TestResolver_Alias_UnknownExport(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0")
	cacheRoot := t.TempDir()
	r := &Resolver{
		Fetcher: &Fetcher{
			Root:     cacheRoot,
			CloneURL: func(_ Reference) string { return "file://" + bare },
		},
		Modules: map[string]string{
			"testmod": "github.com/owner/testmod@v1.0.0",
		},
	}
	_, err := r.Resolve(context.Background(), "testmod/missing")
	if err == nil {
		t.Fatal("expected error for unknown export")
	}
	if !strings.Contains(err.Error(), `no export "missing"`) {
		t.Errorf("error = %q", err.Error())
	}
}

func TestResolver_UnknownAlias(t *testing.T) {
	r := NewResolver(map[string]string{})
	_, err := r.Resolve(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
	if !strings.Contains(err.Error(), `unknown module alias "nope"`) {
		t.Errorf("error = %q", err.Error())
	}
}

func TestResolver_BadInlineRef(t *testing.T) {
	r := NewResolver(nil)
	_, err := r.Resolve(context.Background(), "github.com/x@")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJoinSafe(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "tmp", "modroot")
	cases := []struct {
		name string
		rel  string
		ok   bool
	}{
		{"plain subdir", "sub/pkg", true},
		{"leading slash stripped", "/sub/pkg", true},
		{"dot-prefixed within", "sub/../pkg", true},
		{"parent escape", "../../../etc", false},
		{"single parent escape", "..", false},
		{"escape via segment", "sub/../../outside", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, err := joinSafe(base, c.rel)
			if (err == nil) != c.ok {
				t.Fatalf("joinSafe(%q, %q) err=%v, want ok=%v", base, c.rel, err, c.ok)
			}
		})
	}
}

// A resolver carrying a lockfile must verify the fetched tree before it reads
// index.yml — an unverified module must not get its manifest parsed, let alone
// its components run.
func TestResolver_VerifiesAgainstLock(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0")
	cacheRoot := t.TempDir()
	newFetcher := func() *Fetcher {
		return &Fetcher{
			Root:     cacheRoot,
			CloneURL: func(_ Reference) string { return "file://" + bare },
		}
	}
	const refStr = "github.com/owner/testmod@v1.0.0"
	key := "github.com/owner/testmod@v1.0.0"

	// Fetch once to learn the real hash.
	ref, err := ParseReference(refStr)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := newFetcher().Fetch(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	resetHashCache()
	good, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no lock resolves as before", func(t *testing.T) {
		r := &Resolver{Fetcher: newFetcher()}
		if _, err := r.Resolve(context.Background(), refStr); err != nil {
			t.Errorf("resolve without a lock failed: %v", err)
		}
	})

	t.Run("matching lock resolves", func(t *testing.T) {
		lock := &Lock{}
		lock.Set(LockEntry{Ref: key, Hash: good})
		r := &Resolver{Fetcher: newFetcher(), Lock: lock}
		if _, err := r.Resolve(context.Background(), refStr); err != nil {
			t.Errorf("resolve with a matching lock failed: %v", err)
		}
	})

	t.Run("mismatched lock aborts resolution", func(t *testing.T) {
		lock := &Lock{}
		lock.Set(LockEntry{Ref: key, Hash: "h1:not-the-hash"})
		r := &Resolver{Fetcher: newFetcher(), Lock: lock}
		_, err := r.Resolve(context.Background(), refStr)
		if err == nil {
			t.Fatal("resolve succeeded despite a hash mismatch")
		}
		if !strings.Contains(err.Error(), "failed verification") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("subpath reference verifies against the repo hash", func(t *testing.T) {
		// The subpath does not change the cache dir, so it must not change
		// the lock key or the hash.
		lock := &Lock{}
		lock.Set(LockEntry{Ref: key, Hash: good})
		r := &Resolver{Fetcher: newFetcher(), Lock: lock}
		// components/ has no index.yml, so this fails at LoadIndex — but it
		// must get PAST verification to do so.
		_, err := r.Resolve(context.Background(), "github.com/owner/testmod/components@v1.0.0")
		if err != nil && strings.Contains(err.Error(), "failed verification") {
			t.Errorf("subpath ref failed verification; hash should cover the repo: %v", err)
		}
	})
}

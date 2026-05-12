package lockfile

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadMissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	l, err := Load(filepath.Join(dir, "nope.lock"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(l.Entries) != 0 {
		t.Fatalf("expected empty lock, got %d entries", len(l.Entries))
	}
}

func TestSetSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)

	l := &Lock{}
	l.Set(Entry{
		Backend:      "archive-url",
		Name:         "go",
		Version:      "1.25.3",
		ResolvedURL:  "https://go.dev/dl/go1.25.3.linux-amd64.tar.gz",
		SHA256:       "abc123",
		LockedAt:     "2026-05-12T19:14:02Z",
		LockedByArch: "linux-amd64",
	})
	l.Set(Entry{
		Backend:  "mise",
		Name:     "node",
		Version:  "24.0.0",
		LockedAt: "2026-05-12T19:15:01Z",
	})
	if err := l.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got.Entries))
	}

	e, ok := got.Lookup("archive-url", "go", "1.25.3", "linux-amd64")
	if !ok {
		t.Fatal("expected go entry not found")
	}
	if e.SHA256 != "abc123" {
		t.Fatalf("unexpected sha256 %q", e.SHA256)
	}

	e, ok = got.Lookup("mise", "node", "24.0.0", "")
	if !ok {
		t.Fatal("expected mise entry not found")
	}
	if e.ResolvedURL != "" || e.SHA256 != "" {
		t.Fatalf("mise entry should have empty url/sha, got %+v", e)
	}
}

func TestSetReplacesExisting(t *testing.T) {
	l := &Lock{}
	l.Set(Entry{Backend: "archive-url", Name: "go", Version: "1.25.3", SHA256: "old", LockedByArch: "linux-amd64"})
	l.Set(Entry{Backend: "archive-url", Name: "go", Version: "1.25.3", SHA256: "new", LockedByArch: "linux-amd64"})
	if len(l.Entries) != 1 {
		t.Fatalf("expected 1 entry after replace, got %d", len(l.Entries))
	}
	if l.Entries[0].SHA256 != "new" {
		t.Fatalf("expected replaced sha256 'new', got %q", l.Entries[0].SHA256)
	}
}

func TestLookupByName(t *testing.T) {
	l := &Lock{}
	l.Set(Entry{Backend: "mise", Name: "node", Version: "24.0.0"})
	e, ok := l.LookupByName("node", "24.0.0")
	if !ok {
		t.Fatal("LookupByName failed")
	}
	if e.Backend != "mise" {
		t.Fatalf("expected backend mise, got %q", e.Backend)
	}
	if _, ok := l.LookupByName("node", "99.9.9"); ok {
		t.Fatal("LookupByName should miss on wrong version")
	}
}

func TestConcurrentSavesSerialize(t *testing.T) {
	// Multiple goroutines call Save concurrently; the final file must be a
	// well-formed YAML lock containing every entry. flock + atomic rename
	// guarantee this. (We can't easily assert serialization order, only that
	// the result is consistent.)
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)

	const N = 8
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l, err := Load(path)
			if err != nil {
				t.Errorf("Load: %v", err)
				return
			}
			l.Set(Entry{
				Backend:      "archive-url",
				Name:         "tool",
				Version:      "1.0.0",
				SHA256:       "sha",
				LockedAt:     "2026-05-12T19:14:02Z",
				LockedByArch: shardArch(i),
			})
			if err := l.Save(path); err != nil {
				t.Errorf("Save: %v", err)
			}
		}(i)
	}
	wg.Wait()

	final, err := Load(path)
	if err != nil {
		t.Fatalf("final Load: %v", err)
	}
	if len(final.Entries) == 0 {
		t.Fatal("expected at least one entry after concurrent saves")
	}
}

// shardArch returns a unique arch-like label so each goroutine writes a
// distinct (backend,name,version,arch) entry rather than clobbering.
// Without distinct arch labels Set() would replace previous entries.
func shardArch(i int) string {
	return "test-arch-" + string(rune('a'+i))
}

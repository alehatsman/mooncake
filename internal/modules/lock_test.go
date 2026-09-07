package modules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLockFilename_IsNotTheToolLockfile(t *testing.T) {
	// internal/lockfile owns "mooncake.lock" for tool installs. Sharing the
	// name would make the two lockfiles clobber each other on save.
	if LockFilename == "mooncake.lock" {
		t.Fatal("module lockfile must not collide with the tool lockfile name")
	}
	if LockFilename != "mooncake.modules.lock" {
		t.Errorf("LockFilename = %q, want mooncake.modules.lock", LockFilename)
	}
}

func TestLockKey_DropsSubpath(t *testing.T) {
	withSub := Reference{
		Host: "github.com", Owner: "o", Repo: "r",
		Subpath: "components/db", Version: "v1.0.0",
	}
	without := Reference{Host: "github.com", Owner: "o", Repo: "r", Version: "v1.0.0"}

	if LockKey(withSub) != LockKey(without) {
		t.Errorf("subpath leaked into the lock key: %q vs %q",
			LockKey(withSub), LockKey(without))
	}
	if got, want := LockKey(without), "github.com/o/r@v1.0.0"; got != want {
		t.Errorf("LockKey = %q, want %q", got, want)
	}
}

func TestLock_SaveIsSortedAndDeterministic(t *testing.T) {
	NowRFC3339 = func() string { return "2026-09-07T00:00:00Z" }
	t.Cleanup(func() { NowRFC3339 = defaultNowRFC3339 })

	dir := t.TempDir()
	path := filepath.Join(dir, LockFilename)

	l := &Lock{Version: LockVersion}
	// Insert out of order; the file must come out sorted.
	for _, ref := range []string{"z.com/o/r@v1", "a.com/o/r@v1", "m.com/o/r@v1"} {
		l.Set(LockEntry{Ref: ref, Hash: "h1:x", LockedAt: NowRFC3339()})
	}
	if err := l.SaveLock(path); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	order := []string{"a.com/o/r@v1", "m.com/o/r@v1", "z.com/o/r@v1"}
	prev := -1
	for _, ref := range order {
		i := strings.Index(string(first), ref)
		if i < 0 {
			t.Fatalf("%s missing from lockfile:\n%s", ref, first)
		}
		if i < prev {
			t.Errorf("entries not sorted by ref:\n%s", first)
		}
		prev = i
	}

	// Reload and re-save: byte-identical, so a tidy that changes nothing
	// produces no diff.
	reloaded, err := LoadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	again := filepath.Join(dir, "again.lock")
	if err := reloaded.SaveLock(again); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(again)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("save is not byte-deterministic:\n--- first ---\n%s\n--- second ---\n%s",
			first, second)
	}
}

func TestLock_SaveDoesNotLeaveTempFiles(t *testing.T) {
	dir := t.TempDir()
	l := &Lock{Version: LockVersion}
	l.Set(LockEntry{Ref: "a.com/o/r@v1", Hash: "h1:x"})
	if err := l.SaveLock(filepath.Join(dir, LockFilename)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestLoadLock_MissingFileIsEmptyNotError(t *testing.T) {
	l, err := LoadLock(filepath.Join(t.TempDir(), LockFilename))
	if err != nil {
		t.Fatalf("missing lockfile should not error: %v", err)
	}
	if !l.Empty() {
		t.Error("missing lockfile should load as empty")
	}
}

func TestLoadLock_RejectsCorruptAndFutureVersions(t *testing.T) {
	t.Run("corrupt yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), LockFilename)
		if err := os.WriteFile(path, []byte("modules: [oh: no: yes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLock(path); err == nil {
			t.Error("corrupt lockfile loaded without error")
		}
	})

	t.Run("future version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), LockFilename)
		if err := os.WriteFile(path, []byte("version: 99\nmodules: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadLock(path)
		if err == nil {
			t.Fatal("a newer lock version loaded without error")
		}
		if !strings.Contains(err.Error(), "upgrade mooncake") {
			t.Errorf("error should tell the operator what to do, got: %v", err)
		}
	})
}

func TestLock_KeepDropsUndeclared(t *testing.T) {
	l := &Lock{Version: LockVersion}
	l.Set(LockEntry{Ref: "a.com/o/r@v1", Hash: "h1:a"})
	l.Set(LockEntry{Ref: "b.com/o/r@v1", Hash: "h1:b"})

	l.Keep(map[string]bool{"a.com/o/r@v1": true})

	if got := l.Refs(); len(got) != 1 || got[0] != "a.com/o/r@v1" {
		t.Errorf("Keep left %v, want only a.com/o/r@v1", got)
	}
}

func TestLock_SetReplacesRatherThanDuplicates(t *testing.T) {
	l := &Lock{Version: LockVersion}
	l.Set(LockEntry{Ref: "a.com/o/r@v1", Hash: "h1:old"})
	l.Set(LockEntry{Ref: "a.com/o/r@v1", Hash: "h1:new"})

	if got := l.Refs(); len(got) != 1 {
		t.Fatalf("Set duplicated an entry: %v", got)
	}
	e, _ := l.LookupLock("a.com/o/r@v1")
	if e.Hash != "h1:new" {
		t.Errorf("Set did not replace: hash = %q", e.Hash)
	}
}

func TestVerifyDir(t *testing.T) {
	dir := writeTree(t, map[string]string{"index.yml": "name: m\n"})
	resetHashCache()
	hash, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	const key = "github.com/o/r@v1.0.0"

	t.Run("empty lock verifies nothing", func(t *testing.T) {
		if err := (&Lock{}).VerifyDir(key, dir); err != nil {
			t.Errorf("empty lock should be a no-op, got: %v", err)
		}
	})

	t.Run("nil lock verifies nothing", func(t *testing.T) {
		var l *Lock
		if err := l.VerifyDir(key, dir); err != nil {
			t.Errorf("nil lock should be a no-op, got: %v", err)
		}
	})

	t.Run("matching hash passes", func(t *testing.T) {
		l := &Lock{}
		l.Set(LockEntry{Ref: key, Hash: hash})
		if err := l.VerifyDir(key, dir); err != nil {
			t.Errorf("matching hash failed verification: %v", err)
		}
	})

	t.Run("mismatched hash fails with both hashes", func(t *testing.T) {
		l := &Lock{}
		l.Set(LockEntry{Ref: key, Hash: "h1:deadbeef"})
		err := l.VerifyDir(key, dir)
		if err == nil {
			t.Fatal("mismatched hash passed verification")
		}
		msg := err.Error()
		if !strings.Contains(msg, "h1:deadbeef") || !strings.Contains(msg, hash) {
			t.Errorf("error should name both hashes, got: %v", err)
		}
	})

	// A non-empty lockfile that omits the module about to run has drifted from
	// the playbook. Trusting the unpinned one is the hole the lockfile closes.
	t.Run("unpinned ref in a non-empty lock fails", func(t *testing.T) {
		l := &Lock{}
		l.Set(LockEntry{Ref: "other.com/o/r@v1", Hash: "h1:x"})
		err := l.VerifyDir(key, dir)
		if err == nil {
			t.Fatal("unpinned module passed verification against a non-empty lock")
		}
		if !strings.Contains(err.Error(), "mod tidy") {
			t.Errorf("error should point at `mooncake mod tidy`, got: %v", err)
		}
	})

	t.Run("broken lock fails every verification", func(t *testing.T) {
		sentinel := errors.New("lockfile is unreadable")
		err := BrokenLock(sentinel).VerifyDir(key, dir)
		if !errors.Is(err, sentinel) {
			t.Errorf("broken lock should surface its load error, got: %v", err)
		}
	})
}

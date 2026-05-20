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

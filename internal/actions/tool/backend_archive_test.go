package tool

import (
	"context"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestArchiveURLValidate(t *testing.T) {
	b, err := Get(BackendArchiveURL)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		tool    *config.Tool
		wantErr bool
	}{
		{"ok with https + checksum", &config.Tool{URL: "https://example.com/go.tar.gz", Checksum: "sha256:abc"}, false},
		{"ok with https + no checksum (TOFU)", &config.Tool{URL: "https://example.com/go.tar.gz"}, false},
		{"reject http + no checksum", &config.Tool{URL: "http://example.com/go.tar.gz"}, true},
		{"missing url", &config.Tool{}, true},
		{"reject github fields", &config.Tool{URL: "https://x", Repo: "a/b"}, true},
		{"reject mise fields", &config.Tool{URL: "https://x", MiseTool: "node"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := b.Validate(tc.tool)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestArchiveURLPlanRendersTemplate(t *testing.T) {
	b, err := Get(BackendArchiveURL)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Backend:         BackendArchiveURL,
		Name:            "go",
		Version:         "1.25.3",
		URL:             "https://go.dev/dl/go{{ version }}.{{ os }}-{{ arch }}.tar.gz",
		StripComponents: 1,
		Bin:             "bin/go",
	}
	facts := FactSnapshot{OS: "linux", Arch: "amd64"}
	plan, err := b.Plan(context.Background(), spec, facts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := "https://go.dev/dl/go1.25.3.linux-amd64.tar.gz"
	if plan.URL != want {
		t.Errorf("URL: got %q, want %q", plan.URL, want)
	}
	if !plan.UseSharedPipeline {
		t.Error("UseSharedPipeline must be true for archive-url")
	}
	if plan.StripComponents != 1 {
		t.Errorf("StripComponents=%d, want 1", plan.StripComponents)
	}
}

func TestRenderURLWhitespaceVariants(t *testing.T) {
	got := renderURL("https://x/{{version}}/{{ os }}/{{arch}}.tar.gz", "1.0.0", FactSnapshot{OS: "darwin", Arch: "arm64"})
	want := "https://x/1.0.0/darwin/arm64.tar.gz"
	if got != want {
		t.Errorf("renderURL = %q, want %q", got, want)
	}
}

package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions/tool/store"
	"github.com/alehatsman/mooncake/internal/config"
)

func init() {
	register(&archiveURLBackend{})
}

// archiveURLBackend implements Backend for the `archive-url` source: a
// templated download URL + optional checksum. Templates against
// {{ version }}, {{ os }}, {{ arch }} (plain string replace; pongo2 is
// not required for this small surface).
type archiveURLBackend struct{}

func (archiveURLBackend) Name() string { return BackendArchiveURL }

func (archiveURLBackend) Validate(t *config.Tool) error {
	if t.URL == "" {
		return fmt.Errorf("archive-url backend: url is required")
	}
	// HTTPS required for TOFU (no inline checksum).
	if t.Checksum == "" && !strings.HasPrefix(t.URL, "https://") {
		return fmt.Errorf("archive-url backend: url must be https when checksum is omitted (TOFU requires authenticated transport)")
	}
	// Forbid mise/github-release fields under archive-url.
	if t.Repo != "" || t.Asset != "" || t.Tag != "" {
		return fmt.Errorf("archive-url backend: repo/asset/tag are github-release fields")
	}
	if t.MiseTool != "" || len(t.Env) > 0 {
		return fmt.Errorf("archive-url backend: mise_tool/env are mise fields")
	}
	return nil
}

func (archiveURLBackend) Plan(_ context.Context, spec Spec, facts FactSnapshot) (Plan, error) {
	url := renderURL(spec.URL, spec.Version, facts)
	return Plan{
		URL:               url,
		Checksum:          spec.InlineChecksum,
		StripComponents:   spec.StripComponents,
		BinRel:            spec.Bin,
		UseSharedPipeline: true,
	}, nil
}

// Install is a no-op for archive-url; the shared pipeline does the work.
func (archiveURLBackend) Install(_ context.Context, _ Spec, _ Plan, _ string) error {
	return nil
}

func (archiveURLBackend) Locate(_ context.Context, spec Spec, installDir string) (string, error) {
	return store.LocateInInstallDir(spec.Bin, installDir)
}

// renderURL substitutes {{ version }}, {{ os }}, {{ arch }} (and
// whitespace variants) into urlTmpl. Avoids pulling pongo2 into the
// backend for a 3-variable replace.
func renderURL(urlTmpl, version string, facts FactSnapshot) string {
	r := strings.NewReplacer(
		"{{ version }}", version,
		"{{version}}", version,
		"{{ os }}", facts.OS,
		"{{os}}", facts.OS,
		"{{ arch }}", facts.Arch,
		"{{arch}}", facts.Arch,
	)
	return r.Replace(urlTmpl)
}

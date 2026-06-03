// Package selfbuild implements `mooncake selfbuild` — cross-compile the
// mooncake binary for every fleet target platform into the local
// ~/.mooncake/bin store, so `fleet bootstrap` can ship the right artefact
// to any peer without a published release.
package selfbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/binstore"
)

// target is one GOOS/GOARCH the store carries.
type target struct{ os, arch string }

// defaultTargets is the fleet matrix: the platforms mooncake peers run on.
var defaultTargets = []target{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

// Command returns the `mooncake selfbuild` CLI command.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "selfbuild",
		Usage: "Cross-compile mooncake for every target into ~/.mooncake/bin",
		Description: "Builds mooncake for each fleet target platform (linux/darwin/windows × " +
			"amd64/arm64) and writes them to the ~/.mooncake/bin store as " +
			"mooncake_<os>_<arch>[.exe]. fleet bootstrap resolves the right binary from " +
			"this store by detected target platform. Must run inside the mooncake source " +
			"tree (needs the Go toolchain). Use --only to build a subset.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "only",
				Usage: "Comma-separated os/arch list to build (e.g. windows/amd64,linux/arm64); default all",
			},
			&cli.StringFlag{
				Name:  "source",
				Usage: "Path to the mooncake module root (default: search upward from cwd)",
			},
		},
		Action: run,
	}
}

func run(c *cli.Context) error {
	targets, err := selectTargets(c.String("only"))
	if err != nil {
		return cli.Exit(err.Error(), 2)
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return cli.Exit("go toolchain not found on PATH — selfbuild cross-compiles from source", 1)
	}

	root, err := moduleRoot(c.String("source"))
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	dir, err := binstore.Dir()
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return cli.Exit(fmt.Sprintf("create store %s: %v", dir, err), 1)
	}

	w := c.App.Writer
	fmt.Fprintf(w, "building %d target(s) from %s → %s\n", len(targets), root, dir)
	for _, t := range targets {
		out, err := binstore.Path(t.os, t.arch)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		cmd := exec.CommandContext(c.Context, goBin, "build", "-o", out, "./cmd")
		cmd.Dir = root
		// CGO off → static, reproducible cross-compiles; mooncake has no
		// cgo dependency (the sqlite_fts5 caveat is dex, not mooncake).
		cmd.Env = append(os.Environ(),
			"GOOS="+t.os, "GOARCH="+t.arch, "CGO_ENABLED=0")
		if combined, err := cmd.CombinedOutput(); err != nil {
			return cli.Exit(fmt.Sprintf("build %s/%s failed: %v\n%s",
				t.os, t.arch, err, strings.TrimSpace(string(combined))), 1)
		}
		fmt.Fprintf(w, "  ✓ %s/%s → %s\n", t.os, t.arch, filepath.Base(out))
	}
	fmt.Fprintf(w, "done — store populated at %s\n", dir)
	return nil
}

// selectTargets returns the matrix to build: all by default, or the parsed
// --only subset (os/arch tokens).
func selectTargets(only string) ([]target, error) {
	if strings.TrimSpace(only) == "" {
		return defaultTargets, nil
	}
	var out []target
	for _, tok := range strings.Split(only, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		parts := strings.SplitN(tok, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --only target %q (want os/arch)", tok)
		}
		out = append(out, target{parts[0], parts[1]})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--only parsed to no targets")
	}
	return out, nil
}

// moduleRoot resolves the mooncake module root. With an explicit path it
// validates it; otherwise it searches upward from cwd for a go.mod whose
// module path is mooncake's.
func moduleRoot(explicit string) (string, error) {
	const wantModule = "module github.com/alehatsman/mooncake"
	check := func(dir string) bool {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			return false
		}
		for _, ln := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(ln) == wantModule {
				return true
			}
		}
		return false
	}
	if explicit != "" {
		if check(explicit) {
			return explicit, nil
		}
		return "", fmt.Errorf("--source %q is not the mooncake module root (no matching go.mod)", explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if check(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside the mooncake source tree (no go.mod for github.com/alehatsman/mooncake found above cwd); run from the repo or pass --source")
		}
		dir = parent
	}
}

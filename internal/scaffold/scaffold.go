// Package scaffold implements `mooncake init`: it copies an embedded template
// into the user's working directory and renders each file through the
// mooncake template engine with detected facts as variables, so the
// generated playbook picks the right package manager and OS gates out of
// the box.
package scaffold

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Template names. Keep this list in sync with the templates/ subdirectories
// shipped via go:embed below. The order here also drives the order of
// --list-templates output.
const (
	TemplateDotfiles     = "dotfiles"
	TemplateServer       = "server"
	TemplateEmpty        = "empty"
	TemplateAgentSandbox = "agent-sandbox"
)

var Templates = []string{TemplateDotfiles, TemplateServer, TemplateEmpty, TemplateAgentSandbox}

var templateDescriptions = map[string]string{
	TemplateDotfiles:     "Dotfiles / dev box — recommended for solo devs",
	TemplateServer:       "Server — Linux service host",
	TemplateEmpty:        "Empty playbook — start from scratch",
	TemplateAgentSandbox: "Agent sandbox — no shell, only typed actions",
}

//go:embed templates/*
var templatesFS embed.FS

// gitignoreSection is the appendable .gitignore block. Idempotent: if
// .gitignore already contains the leading marker line, Scaffold leaves it
// alone.
const gitignoreSection = `# Mooncake — local state and plan artifacts
.mooncake/
*.plan.json
*.plan.yaml
`
const gitignoreMarker = "# Mooncake — local state and plan artifacts"

// Options controls how Scaffold operates. All zero values are valid; the
// CLI layer fills these from urfave/cli flags.
type Options struct {
	Template       string
	NonInteractive bool
	Force          bool
	Dir            string
	NoVars         bool
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
}

// ListTemplates prints the four embedded templates with one-line
// descriptions in catalogue order.
func ListTemplates(out io.Writer) error {
	for _, name := range Templates {
		fmt.Fprintf(out, "%-15s %s\n", name, templateDescriptions[name])
	}
	return nil
}

// Scaffold renders the chosen template into opts.Dir (defaulting to cwd).
// It refuses to overwrite top-level files unless opts.Force is set, except
// for .gitignore which is append-or-noop. The function is the single
// public entry point used by cmd/init.go.
func Scaffold(opts Options) error {
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve dir: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", absDir, err)
	}

	tplName, err := resolveTemplate(opts)
	if err != nil {
		return err
	}

	// Template files are copied verbatim — `{{ os }}`, `{{ home }}`,
	// `{{ apt_available }}`, etc. must remain mooncake template placeholders
	// that the planner resolves at apply time. Substituting them here
	// would freeze the scaffold to the host that ran `init`.
	//
	// We deliberately don't call facts.Collect() at scaffold time: it shells
	// out and breaks the <500ms wall-time budget (spec-39 AC12). The
	// printed summary uses only runtime.GOOS/GOARCH, which are free.
	files, err := listTemplateFiles(tplName)
	if err != nil {
		return err
	}

	created := []string{}
	for _, rel := range files {
		dst, skip := mapTemplatePath(rel, opts)
		if skip {
			continue
		}
		dstAbs := filepath.Join(absDir, dst)

		body, readErr := fs.ReadFile(templatesFS, rel)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", rel, readErr)
		}

		writeMode := writeReplace
		if filepath.Base(dst) == ".gitignore" {
			writeMode = writeGitignore
		}

		wrote, err := writeOne(dstAbs, body, opts.Force, writeMode)
		if err != nil {
			return err
		}
		if wrote {
			created = append(created, dst)
		}
	}

	// Always create the .mooncake/ state dir even if no template file
	// references it — keeps the project layout consistent.
	stateDir := filepath.Join(absDir, ".mooncake")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", stateDir, err)
	}

	printSummary(out, tplName, created)
	return nil
}

// resolveTemplate picks a template name from opts, prompting the user when
// --non-interactive is not set and no --template was provided.
func resolveTemplate(opts Options) (string, error) {
	if opts.Template != "" {
		if _, ok := templateDescriptions[opts.Template]; !ok {
			return "", fmt.Errorf("unknown template %q (run `mooncake init --list-templates`)", opts.Template)
		}
		return opts.Template, nil
	}
	if opts.NonInteractive {
		return "", errors.New("--non-interactive requires --template <name> (run `mooncake init --list-templates`)")
	}
	return promptForTemplate(opts.Stdin, opts.Stdout)
}

// promptForTemplate asks the user which scaffold they want via a tiny
// numbered prompt. Stdin must be a TTY in practice; if it isn't, the read
// returns EOF and we surface a clear remediation.
func promptForTemplate(stdin io.Reader, stdout io.Writer) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	fmt.Fprintln(stdout, "What are you setting up?")
	for i, name := range Templates {
		fmt.Fprintf(stdout, "  %d) %-15s %s\n", i+1, name, templateDescriptions[name])
	}
	fmt.Fprint(stdout, "> ")
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("`mooncake init` needs a TTY for interactive use. Run with `--non-interactive --template <name>` instead")
	}
	trimmed := strings.TrimSpace(line)
	for i, name := range Templates {
		if trimmed == fmt.Sprint(i+1) || trimmed == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("invalid selection %q", trimmed)
}

// listTemplateFiles returns the relative paths (under templates/) of every
// file in the chosen template, sorted for deterministic output.
func listTemplateFiles(tplName string) ([]string, error) {
	root := "templates/" + tplName
	var out []string
	err := fs.WalkDir(templatesFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk template %s: %w", tplName, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("template %q has no files (build issue?)", tplName)
	}
	sort.Strings(out)
	return out, nil
}

// mapTemplatePath converts an embedded path like "templates/empty/gitignore"
// to a destination relative path. Returns skip=true when the file should
// not be written (e.g. mooncake.vars.yml under --no-vars).
func mapTemplatePath(rel string, opts Options) (dst string, skip bool) {
	// Strip "templates/<name>/" prefix.
	parts := strings.SplitN(rel, "/", 3)
	if len(parts) != 3 {
		return rel, true
	}
	leaf := parts[2]
	if leaf == "gitignore" {
		leaf = ".gitignore"
	}
	if opts.NoVars && leaf == "mooncake.vars.yml" {
		return "", true
	}
	return leaf, false
}

type writeMode int

const (
	writeReplace writeMode = iota
	writeGitignore
)

// writeOne writes data to dst atomically (tmp + rename). For .gitignore the
// mode is append-or-noop: if a file already exists and lacks the mooncake
// section, the section is appended; if the section is already present,
// the call is a no-op. Returns wrote=true when the on-disk content
// changed.
func writeOne(dst string, data []byte, force bool, mode writeMode) (bool, error) {
	if mode == writeGitignore {
		return writeGitignoreSection(dst, data)
	}
	if _, err := os.Stat(dst); err == nil && !force {
		return false, fmt.Errorf("%s already exists. Use --force to overwrite, or --dir <path> to scaffold elsewhere", filepath.Base(dst))
	}
	if err := atomicWrite(dst, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// writeGitignoreSection appends gitignoreSection to dst unless the marker
// is already present. If dst doesn't exist, it's created from data.
func writeGitignoreSection(dst string, data []byte) (bool, error) {
	existing, err := os.ReadFile(dst)
	if errors.Is(err, os.ErrNotExist) {
		return writeReplaceFile(dst, data)
	}
	if err != nil {
		return false, err
	}
	if strings.Contains(string(existing), gitignoreMarker) {
		return false, nil // already configured
	}
	appended := string(existing)
	if !strings.HasSuffix(appended, "\n") {
		appended += "\n"
	}
	appended += "\n" + gitignoreSection
	if err := atomicWrite(dst, []byte(appended), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func writeReplaceFile(dst string, data []byte) (bool, error) {
	if err := atomicWrite(dst, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// atomicWrite writes data to a sibling temp file and renames into place.
// Avoids a partial-write window if the process is interrupted.
func atomicWrite(dst string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func printSummary(out io.Writer, tpl string, created []string) {
	if len(created) == 0 {
		fmt.Fprintln(out, "Nothing to do (all template files already present).")
		fmt.Fprintln(out, "Run `mooncake plan` to preview, or `mooncake init --force` to overwrite.")
		return
	}

	fmt.Fprintf(out, "Target: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(out, "Created (template: %s):\n", tpl)
	for _, p := range created {
		fmt.Fprintf(out, "  %s\n", p)
	}
	fmt.Fprintln(out, "  .mooncake/")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Next:")
	fmt.Fprintln(out, "  mooncake plan          # preview what will run")
	fmt.Fprintln(out, "  mooncake apply         # run it")
	fmt.Fprintln(out, "  mooncake presets list  # browse 330+ built-in workflows")
}

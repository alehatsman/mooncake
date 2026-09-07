package mod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/modules"
)

// Verification states reported by `mod verify` and `mod list`.
const (
	statusOK       = "ok"
	statusMismatch = "mismatch"
	statusMissing  = "missing"
)

func modTidyCommand() *cli.Command {
	return &cli.Command{
		Name:  "tidy",
		Usage: "Fetch every declared module and pin its hash in " + modules.LockFilename,
		Description: `Reads the playbook's modules: block, fetches each reference,
and writes its content hash to ` + modules.LockFilename + ` next to the playbook.
Entries for modules no longer declared are dropped.

A Git tag is a mutable pointer — the same @v1.2.0 can resolve to different trees
on two machines. The lockfile pins the tree behind the tag, and any run that
finds a lockfile verifies against it.`,
		Flags: []cli.Flag{
			playbookFlag(),
			cmdutil.FormatFlag(),
		},
		Action: runModTidy,
	}
}

func modVerifyCommand() *cli.Command {
	return &cli.Command{
		Name:  "verify",
		Usage: "Re-hash every locked module and report drift",
		Description: `Re-hashes each module pinned in ` + modules.LockFilename + ` from the
local cache and compares it against the pin. Never clones: a module that is not
cached is reported as missing, not fetched.

Exits non-zero if any module is not ok.`,
		Flags: []cli.Flag{
			playbookFlag(),
			cmdutil.FormatFlag(),
		},
		Action: runModVerify,
	}
}

func modListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List the modules a playbook declares, with cache and lock state",
		Description: `Lists each alias in the playbook's modules: block with its
reference, whether it is present in the local cache, and whether it is pinned in
` + modules.LockFilename + `.

This is the playbook's view. For what is in the cache regardless of any
playbook, use ` + "`mooncake mod cache list`" + `.`,
		Flags: []cli.Flag{
			playbookFlag(),
			cmdutil.FormatFlag(),
		},
		Action: runModList,
	}
}

func playbookFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "playbook",
		Usage: "Path to the mooncake.yml to read (default: ./mooncake.yml)",
	}
}

// playbookPath resolves --playbook, defaulting to ./mooncake.yml.
func playbookPath(c *cli.Context) string {
	if p := c.String("playbook"); p != "" {
		return p
	}
	return "mooncake.yml"
}

// lockPathFor returns the lockfile path that belongs to a playbook: always a
// sibling of it, never an upward search. Writing must be unambiguous even when
// an ancestor directory happens to have its own lockfile.
func lockPathFor(playbook string) string {
	return filepath.Join(filepath.Dir(playbook), modules.LockFilename)
}

// playbookModules reads the `modules:` block out of a playbook.
func playbookModules(playbook string) (map[string]config.ModuleBinding, error) {
	parsed, err := config.ReadConfig(playbook)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", playbook, err)
	}
	return parsed.Modules, nil
}

// sortedAliases returns the aliases of mods in a stable order, so text output
// and JSON output are both reproducible.
func sortedAliases(mods map[string]config.ModuleBinding) []string {
	out := make([]string, 0, len(mods))
	for alias := range mods {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

// tidyResult is the JSON shape of `mod tidy --format json`.
type tidyResult struct {
	Lockfile string      `json:"lockfile"`
	Pinned   []tidyEntry `json:"pinned"`
	Removed  []string    `json:"removed"`
}

type tidyEntry struct {
	Alias string `json:"alias"`
	Ref   string `json:"ref"`
	Hash  string `json:"hash"`
}

func runModTidy(c *cli.Context) error {
	asJSON, err := cmdutil.WantJSON(c)
	if err != nil {
		return err
	}
	playbook := playbookPath(c)
	mods, err := playbookModules(playbook)
	if err != nil {
		return err
	}

	lockPath := lockPathFor(playbook)
	lock, err := modules.LoadLock(lockPath)
	if err != nil {
		return err
	}
	before := lock.Refs()

	fetcher := newCLIFetcher()
	keep := make(map[string]bool, len(mods))
	pinned := make([]tidyEntry, 0, len(mods))

	// Several aliases can point at one cache dir (same repo, different
	// subpaths). Hash each dir once and reuse — HashDir memoizes, but the
	// dedupe also keeps the reported list honest about what got pinned.
	hashes := map[string]string{}

	for _, alias := range sortedAliases(mods) {
		ref, err := modules.ParseReference(mods[alias].Source)
		if err != nil {
			return fmt.Errorf("modules[%q] = %q: %w", alias, mods[alias].Source, err)
		}
		key := modules.LockKey(ref)
		keep[key] = true

		hash, done := hashes[key]
		if !done {
			dir, err := fetcher.Fetch(context.Background(), ref)
			if err != nil {
				return err
			}
			hash, err = modules.HashDir(dir)
			if err != nil {
				return err
			}
			hashes[key] = hash

			// Keep the existing timestamp when the hash is unchanged. Stamping
			// a fresh one every run would make `mod tidy` produce a diff on a
			// no-op, which defeats the point of committing the lockfile.
			lockedAt := modules.NowRFC3339()
			if prev, ok := lock.LookupLock(key); ok && prev.Hash == hash && prev.LockedAt != "" {
				lockedAt = prev.LockedAt
			}
			lock.Set(modules.LockEntry{
				Ref:      key,
				Hash:     hash,
				LockedAt: lockedAt,
			})
		}
		pinned = append(pinned, tidyEntry{Alias: alias, Ref: key, Hash: hash})
	}

	lock.Keep(keep)
	if err := lock.SaveLock(lockPath); err != nil {
		return err
	}

	removed := make([]string, 0)
	for _, ref := range before {
		if !keep[ref] {
			removed = append(removed, ref)
		}
	}

	if asJSON {
		return cmdutil.EmitJSON(tidyResult{
			Lockfile: lockPath, Pinned: pinned, Removed: removed,
		})
	}
	for _, e := range pinned {
		fmt.Printf("pinned %-20s %s\n       %s\n", e.Alias, e.Ref, e.Hash)
	}
	for _, ref := range removed {
		fmt.Printf("removed %s (no longer declared)\n", ref)
	}
	if len(pinned) == 0 && len(removed) == 0 {
		fmt.Printf("%s declares no modules; %s left empty\n", playbook, modules.LockFilename)
		return nil
	}
	fmt.Printf("wrote %s\n", lockPath)
	return nil
}

// verifyResult is the JSON shape of `mod verify --format json`.
type verifyResult struct {
	Lockfile string        `json:"lockfile"`
	Modules  []verifyEntry `json:"modules"`
	OK       bool          `json:"ok"`
}

type verifyEntry struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
	Locked string `json:"locked"`
	Actual string `json:"actual,omitempty"`
}

func runModVerify(c *cli.Context) error {
	asJSON, err := cmdutil.WantJSON(c)
	if err != nil {
		return err
	}
	playbook := playbookPath(c)
	lockPath := lockPathFor(playbook)

	if _, err := os.Stat(lockPath); err != nil {
		return fmt.Errorf("no %s next to %s; run `mooncake mod tidy` first",
			modules.LockFilename, playbook)
	}
	lock, err := modules.LoadLock(lockPath)
	if err != nil {
		return err
	}

	fetcher := newCLIFetcher()
	entries := make([]verifyEntry, 0, len(lock.Refs()))
	allOK := true

	for _, key := range lock.Refs() {
		locked, _ := lock.LookupLock(key)
		e := verifyEntry{Ref: key, Locked: locked.Hash}

		ref, err := modules.ParseReference(key)
		if err != nil {
			return fmt.Errorf("%s: entry %q: %w", lockPath, key, err)
		}
		// Cache-only: verify reports on what is here, it does not go fetch
		// what is missing. Fetching would mask the very drift being checked.
		dir, err := fetcher.FetchCached(context.Background(), ref)
		if err != nil {
			e.Status = statusMissing
			allOK = false
			entries = append(entries, e)
			continue
		}
		actual, err := modules.HashDir(dir)
		if err != nil {
			return err
		}
		e.Actual = actual
		if actual == locked.Hash {
			e.Status = statusOK
		} else {
			e.Status = statusMismatch
			allOK = false
		}
		entries = append(entries, e)
	}

	if asJSON {
		if err := cmdutil.EmitJSON(verifyResult{
			Lockfile: lockPath, Modules: entries, OK: allOK,
		}); err != nil {
			return err
		}
	} else {
		for _, e := range entries {
			switch e.Status {
			case statusOK:
				fmt.Printf("ok       %s\n", e.Ref)
			case statusMissing:
				fmt.Printf("missing  %s (not in local cache)\n", e.Ref)
			default:
				fmt.Printf("MISMATCH %s\n  locked:  %s\n  on disk: %s\n",
					e.Ref, e.Locked, e.Actual)
			}
		}
		if len(entries) == 0 {
			fmt.Printf("%s pins no modules\n", lockPath)
		}
	}

	if !allOK {
		return cli.Exit("", 1)
	}
	return nil
}

// listResult is the JSON shape of `mod list --format json`.
type listResult struct {
	Playbook string      `json:"playbook"`
	Lockfile string      `json:"lockfile"`
	Modules  []listEntry `json:"modules"`
}

type listEntry struct {
	Alias  string `json:"alias"`
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Cached bool   `json:"cached"`
	Locked bool   `json:"locked"`
}

func runModList(c *cli.Context) error {
	asJSON, err := cmdutil.WantJSON(c)
	if err != nil {
		return err
	}
	playbook := playbookPath(c)
	mods, err := playbookModules(playbook)
	if err != nil {
		return err
	}
	lockPath := lockPathFor(playbook)
	lock, err := modules.LoadLock(lockPath)
	if err != nil {
		return err
	}

	fetcher := newCLIFetcher()
	entries := make([]listEntry, 0, len(mods))
	for _, alias := range sortedAliases(mods) {
		source := mods[alias].Source
		e := listEntry{Alias: alias, Source: source}
		ref, err := modules.ParseReference(source)
		if err != nil {
			return fmt.Errorf("modules[%q] = %q: %w", alias, source, err)
		}
		e.Ref = modules.LockKey(ref)
		if _, err := fetcher.FetchCached(context.Background(), ref); err == nil {
			e.Cached = true
		}
		_, e.Locked = lock.LookupLock(e.Ref)
		entries = append(entries, e)
	}

	if asJSON {
		return cmdutil.EmitJSON(listResult{
			Playbook: playbook, Lockfile: lockPath, Modules: entries,
		})
	}
	if len(entries) == 0 {
		fmt.Printf("%s declares no modules\n", playbook)
		return nil
	}
	for _, e := range entries {
		fmt.Printf("%-20s %s\n  cached=%-5v locked=%v\n",
			e.Alias, e.Source, e.Cached, e.Locked)
	}
	return nil
}

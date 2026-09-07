package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// checkModuleCache reports the Git-native module cache: where it is, how
// many modules are resolved into it, and whether it's readable.
//
// This replaced a "component search paths" check that outlived its subject.
// The in-tree component library was retired in favour of module distribution,
// but doctor kept listing four component directories and calling a stale
// symlink "21 components across 4 search path(s)" — teaching operators a
// concept the tool no longer has (#175).
type checkModuleCache struct{}

func (checkModuleCache) Section() string { return "modules" }
func (checkModuleCache) Name() string    { return "cache" }
func (checkModuleCache) Run(ctx Context) Result {
	r := Result{Section: "modules", Name: "cache"}

	root := ctx.ModuleCacheRoot
	if root == "" {
		r.Status = StatusInfo
		r.Message = "module cache location could not be resolved"
		r.Fix = "set $MOONCAKE_MODULE_CACHE, or ensure $HOME is set so the default ~/.cache/mooncake/modules resolves"
		return r
	}

	info, err := os.Stat(root)
	switch {
	case os.IsNotExist(err):
		// Not an error: nothing has been fetched yet.
		r.Status = StatusInfo
		r.Message = "no modules cached yet"
		r.Detail = root + " — not created"
		r.Fix = "run `mooncake mod add <host>/<owner>/<repo>@<tag>` to fetch one"
		return r
	case err != nil:
		r.Status = StatusWarning
		r.Message = "module cache unreadable"
		r.Detail = fmt.Sprintf("%s — %v", root, err)
		return r
	case !info.IsDir():
		r.Status = StatusWarning
		r.Message = "module cache path is not a directory"
		r.Detail = root
		return r
	}

	mods := cachedModules(root)
	r.Status = StatusOK
	r.Detail = root
	if len(mods) == 0 {
		r.Message = "module cache is empty"
		r.Fix = "run `mooncake mod add <host>/<owner>/<repo>@<tag>` to fetch one"
		return r
	}
	r.Message = fmt.Sprintf("%d module(s) cached", len(mods))
	r.Detail = root + "\n" + strings.Join(mods, "\n")
	return r
}

// cachedModules returns "<host>/<owner>/<repo>@<version>" for every module
// resolved under root. Mirrors `mooncake mod cache list`; the three-level
// walk is the cache's on-disk shape.
func cachedModules(root string) []string {
	var out []string
	hosts, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, host := range hosts {
		if !host.IsDir() {
			continue
		}
		owners, err := os.ReadDir(filepath.Join(root, host.Name()))
		if err != nil {
			continue
		}
		for _, owner := range owners {
			if !owner.IsDir() {
				continue
			}
			repos, err := os.ReadDir(filepath.Join(root, host.Name(), owner.Name()))
			if err != nil {
				continue
			}
			for _, repo := range repos {
				if !repo.IsDir() {
					continue
				}
				out = append(out, fmt.Sprintf("%s/%s/%s",
					host.Name(), owner.Name(), repo.Name()))
			}
		}
	}
	return out
}

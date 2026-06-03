package config

import (
	"os"
	"path/filepath"
)

// TasksSearchPaths is the ordered list of dedicated tasks-file
// candidates, checked before falling back to the apply-config search.
// `mooncake task` prefers these; `mooncake apply` does not look at
// them.
var TasksSearchPaths = []string{
	"tasks.yml",
	"tasks.yaml",
}

// DiscoverTasksConfig locates the file `mooncake task` should read.
//
// Resolution order, rooted at dir:
//  1. The first existing regular file from TasksSearchPaths.
//  2. The first existing regular file from SearchPaths whose parsed
//     config defines at least one task.
//
// Returns *ErrNoConfigFound when nothing matches either set. When a
// dedicated tasks file is chosen and an apply-config file in the same
// dir ALSO defines tasks, the apply-config path is returned as
// shadowed so the CLI can warn (the dedicated file wins by design).
//
// The shadow check is best-effort: a malformed mooncake.yml is treated
// as "no tasks defined" for shadowing purposes — surfacing parse
// errors here would mask the real diagnostic the user gets when they
// run `mooncake apply`, which is the path that uses that file.
func DiscoverTasksConfig(dir string) (path, shadowed string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}

	// Dedicated tasks file wins. Check shadowing against the apply
	// search paths so the user knows their tasks: in mooncake.yml is
	// being ignored.
	for _, rel := range TasksSearchPaths {
		candidate := filepath.Join(abs, rel)
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		shadowed = findShadowedApplyConfig(abs)
		return candidate, shadowed, nil
	}

	// Fall back to the apply-config search path. The file must define
	// at least one task to be considered.
	for _, rel := range SearchPaths {
		candidate := filepath.Join(abs, rel)
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if fileDefinesTasks(candidate) {
			return candidate, "", nil
		}
	}

	return "", "", &ErrNoConfigFound{Dir: abs}
}

// findShadowedApplyConfig returns the absolute path of the first
// existing apply-config candidate in dir that defines tasks. Empty
// when no such file exists or none of them define tasks. Used to
// warn the user that their mooncake.yml's tasks: are being suppressed
// by a dedicated tasks.yml.
func findShadowedApplyConfig(dir string) string {
	for _, rel := range SearchPaths {
		candidate := filepath.Join(dir, rel)
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if fileDefinesTasks(candidate) {
			return candidate
		}
	}
	return ""
}

// fileDefinesTasks reports whether the YAML file at path parses to a
// config that contains at least one entry under `tasks:`. Returns false
// on any error — the caller treats "couldn't decide" as "no tasks" so
// shadow detection doesn't surface unrelated parse errors.
func fileDefinesTasks(path string) bool {
	parsed, _, err := ReadConfigWithValidation(path, nil)
	if err != nil || parsed == nil {
		return false
	}
	return len(parsed.Tasks) > 0
}

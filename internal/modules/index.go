package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Index represents a module's index.yml manifest. It declares the module's
// identity and maps export names to component file paths relative to the
// module root.
//
//	name: postgres
//	exports:
//	  default: components/install.yml
//	  backup:  components/backup.yml
type Index struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Exports     map[string]string `yaml:"exports"`
}

// LoadIndex reads and parses moduleRoot/index.yml. moduleRoot is the directory
// holding the manifest (typically a cache directory like
// ~/.cache/mooncake/modules/<host>/<owner>/<repo>@<version>/).
func LoadIndex(moduleRoot string) (*Index, error) {
	path := filepath.Join(moduleRoot, "index.yml")
	data, err := os.ReadFile(path) // #nosec G304 -- moduleRoot is validated by caller (resolver/CLI)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("module has no index.yml at root (%s)", moduleRoot)
		}
		return nil, fmt.Errorf("read index.yml: %w", err)
	}
	var idx Index
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index.yml: %w", err)
	}
	if idx.Name == "" {
		return nil, fmt.Errorf("index.yml missing required field `name`")
	}
	if len(idx.Exports) == 0 {
		return nil, fmt.Errorf("index.yml has empty `exports` map")
	}
	return &idx, nil
}

// ResolveExport looks up an export name and returns the absolute path to the
// component file. moduleRoot is the directory the index was loaded from.
//
//	export == "" or "default" → uses the "default" export entry
//	otherwise                 → uses the entry named exactly `export`
//
// The returned path is verified to exist on disk before returning.
func (idx *Index) ResolveExport(moduleRoot, export string) (string, error) {
	if export == "" {
		export = "default"
	}
	rel, ok := idx.Exports[export]
	if !ok {
		return "", fmt.Errorf("module %s has no export %q; available: %s",
			idx.Name, export, strings.Join(idx.exportNames(), ", "))
	}
	abs := filepath.Join(moduleRoot, rel)
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("export %q points to %s which does not exist", export, rel)
		}
		return "", fmt.Errorf("stat exported component %s: %w", rel, err)
	}
	return abs, nil
}

// exportNames returns the export names in sorted order for stable error output.
func (idx *Index) exportNames() []string {
	names := make([]string, 0, len(idx.Exports))
	for k := range idx.Exports {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

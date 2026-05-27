package doctor

import (
	"fmt"
	"os"
	"strings"
)

// checkPresetPaths inspects each preset search path. Each path is listed
// as a line of detail; the overall result is warning only if NO path
// resolves to any preset at all — mooncake can run without presets.
type checkPresetPaths struct{}

func (checkPresetPaths) Section() string { return "presets" }
func (checkPresetPaths) Name() string    { return "search-paths" }
func (checkPresetPaths) Run(ctx Context) Result {
	r := Result{Section: "presets", Name: "search-paths"}
	var lines []string
	total := 0
	for _, p := range ctx.PresetPaths {
		info, err := os.Stat(p)
		switch {
		case os.IsNotExist(err):
			lines = append(lines, fmt.Sprintf("%s — not found", p))
		case err != nil:
			lines = append(lines, fmt.Sprintf("%s — error: %v", p, err))
		case !info.IsDir():
			lines = append(lines, fmt.Sprintf("%s — not a directory", p))
		default:
			n := countPresets(p)
			total += n
			lines = append(lines, fmt.Sprintf("%s — %d presets", p, n))
		}
	}
	r.Detail = strings.Join(lines, "\n")
	if total == 0 {
		r.Status = StatusWarning
		r.Message = "no presets found in any search path"
		r.Fix = "populate at least one preset search path — clone github.com/mooncake/presets, or vendor components under ./presets/ in your project"
		return r
	}
	r.Status = StatusOK
	r.Message = fmt.Sprintf("%d presets across %d search path(s)", total, len(ctx.PresetPaths))
	return r
}

// countPresets returns the number of *.yml files or subdirectory presets
// (preset.yml inside) found one level under dir. Cheap heuristic — the
// loader's authoritative count would require walking deeper than this
// snapshot needs.
func countPresets(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			// dir-shaped preset: <name>/preset.yml
			if _, err := os.Stat(dir + "/" + e.Name() + "/preset.yml"); err == nil {
				n++
			}
			continue
		}
		if strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
			n++
		}
	}
	return n
}

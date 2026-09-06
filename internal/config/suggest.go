package config

import (
	"reflect"
	"strings"
	"sync"

	"github.com/alehatsman/mooncake/internal/utils"
)

// Unknown-field errors used to end at "unknown field `file.wrte`", leaving
// the operator to diff their YAML against the docs by eye — even though the
// full set of legal keys is right there in the Step struct's yaml tags. A
// one-character typo now names its own fix (#174).

// maxSuggestDistance is the edit-distance ceiling for a suggestion. Two
// covers the common slips (transposition, a dropped or doubled letter,
// a wrong character) without volunteering nonsense for a field name the
// operator invented wholesale.
const maxSuggestDistance = 2

// renamedFields maps retired spellings to their current names. Edit
// distance can't reach these — `become` is nowhere near `as_user` — but
// they're exactly the ones an operator carries in from Ansible muscle
// memory or from an old playbook, so they get an exact answer instead of
// a shrug. `mooncake init --template server` itself shipped `become:`
// for months after the rename (#171).
var renamedFields = map[string]string{
	"become":        "as_user",
	"become_user":   "as_user",
	"register":      "as",
	"with_items":    "for_each",
	"with_filetree": "for_each_file",
	"parameters":    "props",
	"loop":          "for_each",
}

var (
	fieldNamesOnce  sync.Once
	stepFields      []string
	runConfigFields []string
)

// yamlFieldNames reflects over a struct's yaml tags and returns the wire
// names, skipping "-" and embedded/planner-internal entries.
func yamlFieldNames(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		name := tag
		if i := strings.Index(name, ","); i >= 0 {
			name = name[:i]
		}
		if name == "" || name == "-" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func initFieldNames() {
	stepFields = yamlFieldNames(reflect.TypeOf(Step{}))
	runConfigFields = yamlFieldNames(reflect.TypeOf(RunConfig{}))
}

// candidatesForType returns the legal field names for the Go type named in
// a yaml.v3 strict-mode error ("... not found in type config.Step").
func candidatesForType(typeName string) []string {
	fieldNamesOnce.Do(initFieldNames)
	switch {
	case strings.HasSuffix(typeName, "RunConfig"):
		return runConfigFields
	case strings.HasSuffix(typeName, "Step"):
		return stepFields
	default:
		// Unknown container type — offering Step's vocabulary is still
		// better than nothing, since nested action structs are the only
		// other case and they're always reached through a Step.
		return stepFields
	}
}

// suggestField returns the closest candidate to unknown within
// maxSuggestDistance, or "" when nothing is close enough. Ties break
// toward the shorter name, then alphabetically, so the result is stable
// across runs and platforms.
func suggestField(unknown string, candidates []string) string {
	if unknown == "" {
		return ""
	}
	lower := strings.ToLower(unknown)
	if renamed, ok := renamedFields[lower]; ok {
		return renamed
	}
	best, bestDist := "", maxSuggestDistance+1
	for _, c := range candidates {
		d := utils.Levenshtein(lower, strings.ToLower(c))
		if d > maxSuggestDistance || d > bestDist {
			continue
		}
		if d < bestDist || len(c) < len(best) || (len(c) == len(best) && c < best) {
			best, bestDist = c, d
		}
	}
	return best
}

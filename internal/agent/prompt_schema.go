package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

// BuildSchemaChunk renders the action vocabulary section of the system
// prompt from the embedded schema.json. New actions added to schema.json
// automatically appear here — no edit to internal/agent/prompt*.go needed.
//
// Layout:
//
//	A Mooncake config is an array of steps. ...
//
//	ACTIONS (grouped by category):
//	  [category]
//	  - action.name — description
//	    required: field (type), ...
//	    optional: field, ...
//
//	UNIVERSAL STEP FIELDS:
//	  - field (type): description
func BuildSchemaChunk() (string, error) {
	return buildSchemaChunkFromJSON(config.SchemaJSON())
}

func buildSchemaChunkFromJSON(raw []byte) (string, error) {
	var root struct {
		Definitions map[string]schemaDef `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("parse schema.json: %w", err)
	}
	step, ok := root.Definitions["step"]
	if !ok {
		return "", fmt.Errorf("schema missing 'step' definition")
	}

	actions, universals := partitionStepProperties(step, root.Definitions)
	return renderChunk(actions, universals), nil
}

type schemaDef struct {
	Type        string                `json:"type"`
	Description string                `json:"description"`
	Properties  map[string]schemaProp `json:"properties"`
	Required    []string              `json:"required"`
	Category    string                `json:"x-category"`
}

type schemaProp struct {
	Type        any         `json:"type"`
	Description string      `json:"description"`
	Ref         string      `json:"$ref"`
	Items       *schemaProp `json:"items"`
	Enum        []any       `json:"enum"`
}

type actionEntry struct {
	Name        string
	Category    string
	Description string
	Required    []string
	Optional    []string
	Types       map[string]string
}

type universalEntry struct {
	Name        string
	Type        string
	Description string
}

func partitionStepProperties(step schemaDef, defs map[string]schemaDef) ([]actionEntry, []universalEntry) {
	var actions []actionEntry
	var universals []universalEntry

	for name, prop := range step.Properties {
		if prop.Ref != "" {
			defName := strings.TrimPrefix(prop.Ref, "#/definitions/")
			def, ok := defs[defName]
			if !ok {
				continue
			}
			// Use the step-property description if present (it's the
			// surface-level summary), else fall back to the definition's.
			desc := prop.Description
			if desc == "" {
				desc = def.Description
			}
			actions = append(actions, buildActionEntry(name, desc, def))
		} else {
			universals = append(universals, universalEntry{
				Name:        name,
				Type:        formatPropType(prop),
				Description: prop.Description,
			})
		}
	}

	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Category != actions[j].Category {
			return actions[i].Category < actions[j].Category
		}
		return actions[i].Name < actions[j].Name
	})
	sort.Slice(universals, func(i, j int) bool {
		return universals[i].Name < universals[j].Name
	})
	return actions, universals
}

func buildActionEntry(name, desc string, def schemaDef) actionEntry {
	required := append([]string(nil), def.Required...)
	sort.Strings(required)

	requiredSet := make(map[string]bool, len(required))
	for _, r := range required {
		requiredSet[r] = true
	}

	types := make(map[string]string, len(def.Properties))
	var optional []string
	for propName, prop := range def.Properties {
		types[propName] = formatPropType(prop)
		if !requiredSet[propName] {
			optional = append(optional, propName)
		}
	}
	sort.Strings(optional)

	return actionEntry{
		Name:        name,
		Category:    def.Category,
		Description: desc,
		Required:    required,
		Optional:    optional,
		Types:       types,
	}
}

func formatPropType(p schemaProp) string {
	base := jsonTypeString(p.Type)
	if base == "array" && p.Items != nil {
		inner := jsonTypeString(p.Items.Type)
		if inner != "" {
			return "array of " + inner
		}
		return "array"
	}
	if base == "" && p.Ref != "" {
		return strings.TrimPrefix(p.Ref, "#/definitions/")
	}
	return base
}

func jsonTypeString(t any) string {
	switch v := t.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " or ")
	}
	return ""
}

func renderChunk(actions []actionEntry, universals []universalEntry) string {
	var b strings.Builder

	// Format-neutral: the preamble dictates the wire format (compact
	// JSON, proposal-08 #13). This chunk describes structure only, so it
	// must not name a serialization — a stray "YAML" here contradicts the
	// "Output ONLY a compact JSON array" instruction and is exactly the
	// mixed signal that trips up smaller local models.
	b.WriteString("A Mooncake config is an array of steps. Each step has:\n")
	b.WriteString("- Optional 'name' field (string)\n")
	b.WriteString("- Exactly ONE action from the ACTIONS list below\n")
	b.WriteString("- Optional universal modifiers from UNIVERSAL STEP FIELDS\n\n")

	b.WriteString("ACTIONS (grouped by category):\n")
	var currentCategory string
	for _, a := range actions {
		cat := a.Category
		if cat == "" {
			cat = "other"
		}
		if cat != currentCategory {
			currentCategory = cat
			fmt.Fprintf(&b, "\n  [%s]\n", cat)
		}
		fmt.Fprintf(&b, "  - %s", a.Name)
		if a.Description != "" {
			fmt.Fprintf(&b, " — %s", a.Description)
		}
		b.WriteString("\n")
		if len(a.Required) > 0 {
			fmt.Fprintf(&b, "      required: %s\n", joinFieldsWithTypes(a.Required, a.Types))
		}
		if len(a.Optional) > 0 {
			fmt.Fprintf(&b, "      optional: %s\n", strings.Join(a.Optional, ", "))
		}
	}

	b.WriteString("\nUNIVERSAL STEP FIELDS:\n")
	for _, u := range universals {
		typeStr := u.Type
		if typeStr == "" {
			typeStr = "value"
		}
		fmt.Fprintf(&b, "  - %s (%s)", u.Name, typeStr)
		if u.Description != "" {
			fmt.Fprintf(&b, ": %s", u.Description)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func joinFieldsWithTypes(names []string, types map[string]string) string {
	parts := make([]string, 0, len(names))
	for _, n := range names {
		t := types[n]
		if t == "" {
			parts = append(parts, n)
		} else {
			parts = append(parts, fmt.Sprintf("%s (%s)", n, t))
		}
	}
	return strings.Join(parts, ", ")
}

package config

// secret_tag.go implements the spec-23 §3 YAML pre-pass that turns
// `!secret <ref>` scalars into sentinel-marker strings before the
// regular yaml.Node decode runs.
//
// Why pre-pass rather than custom UnmarshalYAML on each action struct:
// secrets can appear in any string field (FileWrite.Content,
// PkgRepo.Key, ShellAction.Env, etc.) and rewriting every action
// struct to accept a typed SecretOrString union would be a sprawling
// edit. The marker-in-string approach keeps action structs untouched —
// the executor pre-dispatch resolver replaces markers with resolved
// values just before the handler runs.
//
// Side effect of this design: plan output JSON contains the marker
// string by default. The plan command (cmd/mooncake.go) runs a post-
// marshal redaction pass that rewrites markers to `"!secret <ref>"`
// for human reading. See security.SentinelPrefix.

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/security"
)

// substituteSecretTags walks the yaml.Node tree and rewrites any scalar
// node tagged `!secret` into a regular scalar whose value is the sentinel
// marker `SentinelPrefix + ref`. The downstream decode flows into action
// struct string fields naturally without knowing the value came from a
// secret tag.
//
// Idempotent: if the value already has the marker prefix, it's left
// alone. Invalid tag usage (non-scalar, empty value) leaves the node
// untouched so the schema validator can surface a useful error on it
// later.
func substituteSecretTags(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode && node.Tag == "!secret" {
		ref := strings.TrimSpace(node.Value)
		if ref != "" && !strings.HasPrefix(node.Value, security.SentinelPrefix) {
			// Rewrite. Tag becomes empty so the rest of the decoder treats
			// it as a normal string scalar.
			node.Tag = "!!str"
			node.Value = security.SentinelPrefix + ref
		}
		return
	}
	for i := range node.Content {
		substituteSecretTags(node.Content[i])
	}
}

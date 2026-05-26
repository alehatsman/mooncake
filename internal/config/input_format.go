package config

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// DetectInputFormat returns "json" if the first non-whitespace byte is '{' or
// '[', "yaml" otherwise. Empty input returns "yaml" so existing empty-file
// diagnostics (MT-73) keep firing at the call site.
func DetectInputFormat(data []byte) string {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[':
			return "json"
		default:
			return "yaml"
		}
	}
	return "yaml"
}

// DecodeAuto decodes data into dst. If data sniffs as JSON, it is unmarshaled
// into a generic any, re-encoded as YAML, and decoded via yaml.Unmarshal so
// existing yaml:"..." struct tags keep working without a parallel json tag set.
func DecodeAuto(data []byte, dst any) error {
	if DetectInputFormat(data) == "json" {
		var generic any
		if err := json.Unmarshal(data, &generic); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
		buf, err := yaml.Marshal(generic)
		if err != nil {
			return fmt.Errorf("re-encode JSON as YAML: %w", err)
		}
		return yaml.Unmarshal(buf, dst)
	}
	return yaml.Unmarshal(data, dst)
}

// DecodeAutoStrict is DecodeAuto but rejects unknown fields. JSON input is
// re-encoded as YAML and then decoded with KnownFields(true) so the strict
// contract (MT-83 / MT-44 "additionalProperties: false") is identical for
// both formats — no need to maintain a parallel json-tag set on every Step.
func DecodeAutoStrict(data []byte, dst any) error {
	if DetectInputFormat(data) == "json" {
		var generic any
		if err := json.Unmarshal(data, &generic); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
		buf, err := yaml.Marshal(generic)
		if err != nil {
			return fmt.Errorf("re-encode JSON as YAML: %w", err)
		}
		dec := yaml.NewDecoder(bytes.NewReader(buf))
		dec.KnownFields(true)
		return dec.Decode(dst)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(dst)
}

// DecodeAutoNode decodes data into a yaml.Node tree. JSON input is
// round-tripped through yaml.Marshal so the resulting node looks
// indistinguishable from a YAML-sourced node — downstream code (location map,
// secret-tag substitution, strict revalidation) works unchanged. Source line
// numbers for JSON inputs point into the re-encoded YAML buffer, not the
// user's JSON file.
func DecodeAutoNode(data []byte) (*yaml.Node, error) {
	if DetectInputFormat(data) == "json" {
		var generic any
		if err := json.Unmarshal(data, &generic); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		buf, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("re-encode JSON as YAML: %w", err)
		}
		var root yaml.Node
		if err := yaml.Unmarshal(buf, &root); err != nil {
			return nil, err
		}
		return &root, nil
	}
	var root yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	return &root, nil
}

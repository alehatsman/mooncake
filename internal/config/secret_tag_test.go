package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/security"
)

// TestSubstituteSecretTags_ScalarRewrite asserts the pre-pass turns a
// scalar carrying the !secret tag into a regular string scalar with the
// sentinel-marker value. Downstream decode then treats it as a normal
// string and the executor's resolver swaps it at apply time.
func TestSubstituteSecretTags_ScalarRewrite(t *testing.T) {
	const src = `key: !secret env:APP_TOKEN`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	substituteSecretTags(&root)

	// Decode into a map and assert the value is the marker form.
	var got map[string]string
	if err := root.Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := security.SentinelPrefix + "env:APP_TOKEN"
	if got["key"] != want {
		t.Errorf("got %q, want %q", got["key"], want)
	}
}

// TestSubstituteSecretTags_NestedInsideMap walks past a map and a
// sequence to find a tagged scalar. The recursive walker should still
// rewrite it.
func TestSubstituteSecretTags_NestedInsideMap(t *testing.T) {
	const src = `
outer:
  inner_list:
    - normal
    - !secret env:DEEP_TOKEN
    - also-normal
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	substituteSecretTags(&root)

	var got struct {
		Outer struct {
			InnerList []string `yaml:"inner_list"`
		} `yaml:"outer"`
	}
	if err := root.Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Outer.InnerList) != 3 {
		t.Fatalf("inner_list len = %d, want 3", len(got.Outer.InnerList))
	}
	if got.Outer.InnerList[0] != "normal" {
		t.Errorf("[0] = %q, want 'normal'", got.Outer.InnerList[0])
	}
	if !security.IsMarker(got.Outer.InnerList[1]) {
		t.Errorf("[1] = %q, want marker", got.Outer.InnerList[1])
	}
	if security.MarkerRef(got.Outer.InnerList[1]) != "env:DEEP_TOKEN" {
		t.Errorf("[1] ref = %q, want 'env:DEEP_TOKEN'",
			security.MarkerRef(got.Outer.InnerList[1]))
	}
}

// TestSubstituteSecretTags_LeavesUntagged ensures plain strings without
// the !secret tag flow through unchanged. Regression guard for any
// future widening of the walker.
func TestSubstituteSecretTags_LeavesUntagged(t *testing.T) {
	const src = `
key1: plain-value
key2: !!str also-plain
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	substituteSecretTags(&root)

	var got map[string]string
	if err := root.Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["key1"] != "plain-value" {
		t.Errorf("key1 mutated: %q", got["key1"])
	}
	if got["key2"] != "also-plain" {
		t.Errorf("key2 mutated: %q", got["key2"])
	}
	if strings.Contains(got["key1"], security.SentinelPrefix) {
		t.Errorf("key1 unexpectedly contains marker: %q", got["key1"])
	}
}

// TestSubstituteSecretTags_EmptyValueLeftAlone — defensive case. A
// tagged scalar with an empty value is malformed; we leave it to the
// schema validator to surface the error rather than silently rewriting
// it to an empty marker.
func TestSubstituteSecretTags_EmptyValueLeftAlone(t *testing.T) {
	const src = `key: !secret`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	substituteSecretTags(&root)

	// Find the value node manually since Decode would coerce to string.
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		t.Fatal("unexpected root shape")
	}
	mapNode := root.Content[0]
	if mapNode.Kind != yaml.MappingNode || len(mapNode.Content) != 2 {
		t.Fatal("unexpected map shape")
	}
	valNode := mapNode.Content[1]
	if valNode.Tag != "!secret" {
		t.Errorf("expected tag to remain !secret, got %q", valNode.Tag)
	}
}

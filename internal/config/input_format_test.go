package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDetectInputFormat(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"empty", "", "yaml"},
		{"whitespace only", "   \n\t  ", "yaml"},
		{"yaml list", "- shell: echo hi", "yaml"},
		{"yaml map", "steps:\n  - shell:", "yaml"},
		{"yaml comment first", "# header\n- shell: x", "yaml"},
		{"json object", `{"steps":[]}`, "json"},
		{"json array", `[{"shell":"echo"}]`, "json"},
		{"json with leading ws", "  \n\t{\"a\":1}", "json"},
		{"json with newline before brace", "\n[1]", "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectInputFormat([]byte(tt.data))
			if got != tt.want {
				t.Fatalf("DetectInputFormat(%q) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestDecodeAuto_YAMLPath(t *testing.T) {
	type sample struct {
		Name  string   `yaml:"name"`
		Items []string `yaml:"items"`
	}
	src := "name: a\nitems:\n  - x\n  - y\n"
	var got sample
	if err := DecodeAuto([]byte(src), &got); err != nil {
		t.Fatalf("DecodeAuto YAML: %v", err)
	}
	if got.Name != "a" || len(got.Items) != 2 || got.Items[0] != "x" || got.Items[1] != "y" {
		t.Fatalf("unexpected decode result: %+v", got)
	}
}

func TestDecodeAuto_JSONPath(t *testing.T) {
	type sample struct {
		Name  string   `yaml:"name"`
		Items []string `yaml:"items"`
	}
	src := `{"name":"a","items":["x","y"]}`
	var got sample
	if err := DecodeAuto([]byte(src), &got); err != nil {
		t.Fatalf("DecodeAuto JSON: %v", err)
	}
	if got.Name != "a" || len(got.Items) != 2 || got.Items[0] != "x" || got.Items[1] != "y" {
		t.Fatalf("unexpected decode result: %+v", got)
	}
}

func TestDecodeAuto_JSONIntoMap(t *testing.T) {
	src := `{"k1":"v1","k2":42}`
	got := make(map[string]any)
	if err := DecodeAuto([]byte(src), &got); err != nil {
		t.Fatalf("DecodeAuto JSON map: %v", err)
	}
	if got["k1"] != "v1" {
		t.Fatalf("k1: got %v", got["k1"])
	}
	// JSON numbers round-trip through yaml as int when integral
	if v, ok := got["k2"].(int); !ok || v != 42 {
		t.Fatalf("k2: got %v (%T)", got["k2"], got["k2"])
	}
}

func TestDecodeAutoNode_YAMLPath(t *testing.T) {
	src := "- shell: echo hi\n"
	node, err := DecodeAutoNode([]byte(src))
	if err != nil {
		t.Fatalf("DecodeAutoNode YAML: %v", err)
	}
	if node.Kind != yaml.DocumentNode {
		t.Fatalf("expected document node, got kind %v", node.Kind)
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.SequenceNode {
		t.Fatalf("expected top-level sequence, got %+v", node.Content)
	}
}

func TestDecodeAutoNode_JSONPath(t *testing.T) {
	src := `[{"shell":"echo hi"}]`
	node, err := DecodeAutoNode([]byte(src))
	if err != nil {
		t.Fatalf("DecodeAutoNode JSON: %v", err)
	}
	if node.Kind != yaml.DocumentNode {
		t.Fatalf("expected document node, got kind %v", node.Kind)
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.SequenceNode {
		t.Fatalf("expected top-level sequence, got %+v", node.Content)
	}
	if len(node.Content[0].Content) != 1 || node.Content[0].Content[0].Kind != yaml.MappingNode {
		t.Fatalf("expected one mapping in sequence, got %+v", node.Content[0].Content)
	}
}

func TestDecodeAuto_InvalidJSON(t *testing.T) {
	src := `{"unterminated":`
	var dst any
	if err := DecodeAuto([]byte(src), &dst); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

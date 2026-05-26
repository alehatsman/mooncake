package fleet

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet"
)

func TestParsePeerFlags_BareName(t *testing.T) {
	groups, err := parsePeerFlags([]string{"main_pc"})
	if err != nil {
		t.Fatalf("parsePeerFlags: %v", err)
	}
	if len(groups) != 1 || len(groups[0]) != 1 {
		t.Fatalf("expected 1 group with 1 term, got %v", groups)
	}
	if groups[0][0].key != "name" || groups[0][0].value != "main_pc" {
		t.Errorf("bare name should map to {name, main_pc}; got %+v", groups[0][0])
	}
}

func TestParsePeerFlags_KeyValue(t *testing.T) {
	groups, err := parsePeerFlags([]string{"tag=production"})
	if err != nil {
		t.Fatalf("parsePeerFlags: %v", err)
	}
	if len(groups) != 1 || len(groups[0]) != 1 {
		t.Fatalf("expected 1 group with 1 term, got %v", groups)
	}
	if groups[0][0].key != "tag" || groups[0][0].value != "production" {
		t.Errorf("got %+v", groups[0][0])
	}
}

func TestParsePeerFlags_AndGroup(t *testing.T) {
	groups, err := parsePeerFlags([]string{"@tag=prod,os=linux"})
	if err != nil {
		t.Fatalf("parsePeerFlags: %v", err)
	}
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("expected 1 group with 2 terms; got %v", groups)
	}
	if groups[0][0].key != "tag" || groups[0][0].value != "prod" {
		t.Errorf("first term: %+v", groups[0][0])
	}
	if groups[0][1].key != "os" || groups[0][1].value != "linux" {
		t.Errorf("second term: %+v", groups[0][1])
	}
}

func TestParsePeerFlags_UnionAcrossFlags(t *testing.T) {
	groups, err := parsePeerFlags([]string{"main_pc", "tag=gpu"})
	if err != nil {
		t.Fatalf("parsePeerFlags: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (union); got %v", groups)
	}
	if groups[0][0].key != "name" || groups[1][0].key != "tag" {
		t.Errorf("union order should be preserved; got %v", groups)
	}
}

func TestParsePeerFlags_EmptyAndWhitespace(t *testing.T) {
	groups, err := parsePeerFlags([]string{"", "   ", "main_pc"})
	if err != nil {
		t.Fatalf("parsePeerFlags: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (blanks dropped); got %v", groups)
	}
}

func TestParsePeerFlags_BadKeyValue(t *testing.T) {
	for _, bad := range []string{"=value", "key=", "@=v", "@k=,k2="} {
		if _, err := parsePeerFlags([]string{bad}); err == nil {
			t.Errorf("parsePeerFlags(%q) should error", bad)
		}
	}
}

func TestParsePeerFlags_AndGroupCommaWhitespace(t *testing.T) {
	groups, err := parsePeerFlags([]string{"@tag=prod, , os=linux"})
	if err != nil {
		t.Fatalf("parsePeerFlags: %v", err)
	}
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("expected one 2-term group; got %v", groups)
	}
}

func mkPeers() []fleet.Peer {
	return []fleet.Peer{
		{Name: "main_pc", Tags: []string{"production", "gpu"}, Roles: []string{"workstation"}},
		{Name: "laptop", Tags: []string{"production"}, Roles: []string{"laptop"}},
		{Name: "test_box", Tags: []string{"staging"}, Roles: []string{"workstation"}},
	}
}

func TestResolvePeers_BareName(t *testing.T) {
	sel, err := resolvePeers(mkPeers(), []string{"main_pc"}, nil)
	if err != nil {
		t.Fatalf("resolvePeers: %v", err)
	}
	if len(sel.Matched) != 1 || sel.Matched[0].Name != "main_pc" {
		t.Errorf("expected exactly main_pc; got %+v", sel.Matched)
	}
}

func TestResolvePeers_TagFilter(t *testing.T) {
	sel, err := resolvePeers(mkPeers(), []string{"tag=production"}, nil)
	if err != nil {
		t.Fatalf("resolvePeers: %v", err)
	}
	if len(sel.Matched) != 2 {
		t.Fatalf("expected 2 (production tag); got %+v", sel.Matched)
	}
}

func TestResolvePeers_UnionNameAndTag(t *testing.T) {
	sel, err := resolvePeers(mkPeers(), []string{"test_box", "tag=gpu"}, nil)
	if err != nil {
		t.Fatalf("resolvePeers: %v", err)
	}
	if len(sel.Matched) != 2 {
		t.Fatalf("expected 2 (test_box + main_pc); got %+v", sel.Matched)
	}
	names := map[string]bool{}
	for _, p := range sel.Matched {
		names[p.Name] = true
	}
	if !names["test_box"] || !names["main_pc"] {
		t.Errorf("expected both test_box and main_pc; got %+v", names)
	}
}

func TestResolvePeers_AndGroup(t *testing.T) {
	sel, err := resolvePeers(mkPeers(), []string{"@tag=production,role=workstation"}, nil)
	if err != nil {
		t.Fatalf("resolvePeers: %v", err)
	}
	if len(sel.Matched) != 1 || sel.Matched[0].Name != "main_pc" {
		t.Errorf("expected only main_pc; got %+v", sel.Matched)
	}
}

func TestResolvePeers_UnknownNameSurfaces(t *testing.T) {
	sel, err := resolvePeers(mkPeers(), []string{"main_pc", "nosuch"}, nil)
	if err != nil {
		t.Fatalf("resolvePeers: %v", err)
	}
	if len(sel.UnknownNames) != 1 || sel.UnknownNames[0] != "nosuch" {
		t.Errorf("expected unknown name reported; got %+v", sel.UnknownNames)
	}
}

func TestResolvePeers_EmptyReturnsAll(t *testing.T) {
	sel, err := resolvePeers(mkPeers(), nil, nil)
	if err != nil {
		t.Fatalf("resolvePeers: %v", err)
	}
	if len(sel.Matched) != 3 {
		t.Errorf("no --peer → all peers; got %+v", sel.Matched)
	}
}

func TestResolvePeers_BadFilterKey(t *testing.T) {
	if _, err := resolvePeers(mkPeers(), []string{"unknownkey=x"}, nil); err == nil {
		t.Error("expected error for unknown filter key")
	}
}

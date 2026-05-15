package artifact_capture

import (
	"testing"
)

// TestMT24_UserVarsOnly_StripsFacts is a regression test for manual-test
// #24 (2026-05-15): the planner pre-merges system facts into Scope.User
// for template lookup convenience, so dumping that map verbatim into the
// artifact's initial_vars produced 100+ entries of cpu_flags, kernel info,
// distro details, etc. — drowning the playbook-supplied vars and bloating
// changes.json. userVarsOnly subtracts the live facts keyset to leave
// only what the user actually set.
func TestMT24_UserVarsOnly_StripsFacts(t *testing.T) {
	// Mix of (a) a real user var, (b) a known fact key, (c) something
	// custom that shadows a fact name.
	user := map[string]interface{}{
		"my_app_version": "1.2.3", // user-set
		"arch":           "amd64", // fact, must be filtered
		"cpu_cores":      32,      // fact, must be filtered
	}
	got := userVarsOnly(user)
	if got == nil {
		t.Fatal("got nil; want map with my_app_version")
	}
	if got["my_app_version"] != "1.2.3" {
		t.Errorf("my_app_version = %v, want 1.2.3", got["my_app_version"])
	}
	if _, present := got["arch"]; present {
		t.Errorf("arch (fact) should have been filtered, got %v", got["arch"])
	}
	if _, present := got["cpu_cores"]; present {
		t.Errorf("cpu_cores (fact) should have been filtered")
	}
}

func TestMT24_UserVarsOnly_NilOnEmpty(t *testing.T) {
	if got := userVarsOnly(nil); got != nil {
		t.Errorf("userVarsOnly(nil) = %v, want nil", got)
	}
	if got := userVarsOnly(map[string]interface{}{}); got != nil {
		t.Errorf("userVarsOnly(empty) = %v, want nil", got)
	}
}

func TestMT24_UserVarsOnly_NilWhenAllFactsFiltered(t *testing.T) {
	// If the only entries in scope.User are facts, the resulting map
	// should be nil so the JSON marshaller drops initial_vars entirely
	// (it has json:"initial_vars,omitempty").
	user := map[string]interface{}{
		"arch":      "amd64",
		"cpu_cores": 32,
	}
	if got := userVarsOnly(user); got != nil {
		t.Errorf("expected nil when all keys are facts, got %v", got)
	}
}

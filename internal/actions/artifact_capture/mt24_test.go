package artifact_capture

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
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

// MT-24: the headline reproducer was "Captured 0 file changes" after
// inner steps did mutate the filesystem. Root cause: ChannelPublisher
// dispatches OnEvent on a per-subscriber goroutine, so events emitted
// while inner steps run could still be queued when artifact_capture
// reads tracker.GetFileChanges(). The handler now calls
// EventPublisher.Flush() before reading. This test pins that
// contract: publish + Flush + read sees every event.
func TestMT24_TrackerCapturesEventsAfterFlush(t *testing.T) {
	pub := events.NewPublisher()
	defer pub.Close()

	tracker := newFileChangeTracker()
	pub.Subscribe(tracker)

	// Simulate what file.write emits on a creation: EventFileCreated
	// with FileOperationData.
	for _, p := range []string{"/tmp/a.txt", "/tmp/b.txt", "/tmp/c.txt"} {
		pub.Publish(events.Event{
			Type: events.EventFileCreated,
			Data: events.FileOperationData{Path: p, SizeBytes: 4},
		})
	}
	// A single update + a template-render to cover the other two
	// branches in OnEvent.
	pub.Publish(events.Event{
		Type: events.EventFileUpdated,
		Data: events.FileOperationData{Path: "/tmp/a.txt", SizeBytes: 8},
	})
	pub.Publish(events.Event{
		Type: events.EventTemplateRender,
		Data: events.TemplateRenderData{DestPath: "/tmp/rendered.txt", SizeBytes: 16},
	})

	// Without Flush, the tracker (running on a per-subscriber goroutine)
	// may not yet have seen these events — exactly the original bug.
	pub.Flush()

	got := tracker.GetFileChanges()
	if len(got) != 5 {
		t.Fatalf("tracker recorded %d changes after Flush, want 5", len(got))
	}

	// Sanity-check the operation strings so the JSON consumer can
	// distinguish created/updated/template.
	wantOps := map[string]int{"created": 3, "updated": 1, "template": 1}
	gotOps := map[string]int{}
	for _, c := range got {
		gotOps[c.Operation]++
	}
	for op, want := range wantOps {
		if gotOps[op] != want {
			t.Errorf("op %q count = %d, want %d", op, gotOps[op], want)
		}
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

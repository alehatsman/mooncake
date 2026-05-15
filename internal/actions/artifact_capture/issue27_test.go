package artifact_capture

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/events"
)

// TestIssue27_TrackerCarriesSizeBeforeChecksumsAndContent pins the new
// wire-up: when file.write emits an EventFileCreated/Updated with the
// pre/post fields populated, the tracker must copy them into FileChange
// so EnhanceFileChange can produce a DetailedFileChange with non-empty
// SizeBefore / SizeAfter / SizeDelta / ChecksumBefore / ChecksumAfter
// and (with capture_content) ContentBefore / ContentAfter.
//
// Pre-fix this test failed three ways:
//  1. SizeBefore was absent on the event entirely (no struct field)
//  2. ChecksumBefore/After were empty because file.write didn't compute them
//  3. ContentBefore/After were absent on the event entirely
//
// The combination produced changes.json entries that were missing the
// before/after byte/checksum/content fields and a summary that always
// reported total_size_delta:0.
func TestIssue27_TrackerCarriesSizeBeforeChecksumsAndContent(t *testing.T) {
	pub := events.NewPublisher()
	defer pub.Close()

	tracker := newFileChangeTracker()
	pub.Subscribe(tracker)

	oldBytes := []byte("old\n")
	newBytes := []byte("UPDATED\n")
	oldSum := sha256.Sum256(oldBytes)
	newSum := sha256.Sum256(newBytes)

	pub.Publish(events.Event{
		Type: events.EventFileUpdated,
		Data: events.FileOperationData{
			Path:           "/tmp/will-update",
			SizeBytes:      int64(len(newBytes)),
			SizeBefore:     int64(len(oldBytes)),
			ChecksumBefore: hex.EncodeToString(oldSum[:]),
			ChecksumAfter:  hex.EncodeToString(newSum[:]),
			ContentBefore:  oldBytes,
			ContentAfter:   newBytes,
		},
	})

	createBytes := []byte("NEW\n")
	createSum := sha256.Sum256(createBytes)
	pub.Publish(events.Event{
		Type: events.EventFileCreated,
		Data: events.FileOperationData{
			Path:          "/tmp/will-create",
			SizeBytes:     int64(len(createBytes)),
			SizeBefore:    0,
			ChecksumAfter: hex.EncodeToString(createSum[:]),
			ContentAfter:  createBytes,
		},
	})

	pub.Flush()

	got := tracker.GetFileChanges()
	if len(got) != 2 {
		t.Fatalf("tracker recorded %d changes, want 2", len(got))
	}

	// updated: pre-fix had SizeBefore=0 and empty checksums; post-fix
	// the tracker must propagate them.
	updated := find(got, "/tmp/will-update")
	if updated == nil {
		t.Fatal("missing /tmp/will-update in tracked changes")
	}
	if updated.SizeBefore != int64(len(oldBytes)) {
		t.Errorf("updated.SizeBefore = %d, want %d", updated.SizeBefore, len(oldBytes))
	}
	if updated.SizeBytes != int64(len(newBytes)) {
		t.Errorf("updated.SizeBytes = %d, want %d", updated.SizeBytes, len(newBytes))
	}
	if updated.ChecksumBefore == "" || updated.ChecksumAfter == "" {
		t.Errorf("updated checksums must be non-empty; before=%q after=%q",
			updated.ChecksumBefore, updated.ChecksumAfter)
	}
	if updated.ChecksumBefore == updated.ChecksumAfter {
		t.Error("before/after checksums should differ for an actual content change")
	}
	if string(updated.ContentBefore) != string(oldBytes) {
		t.Errorf("updated.ContentBefore = %q, want %q", updated.ContentBefore, oldBytes)
	}
	if string(updated.ContentAfter) != string(newBytes) {
		t.Errorf("updated.ContentAfter = %q, want %q", updated.ContentAfter, newBytes)
	}

	// created: SizeBefore=0 by definition; ContentBefore=nil; checksum_after set.
	created := find(got, "/tmp/will-create")
	if created == nil {
		t.Fatal("missing /tmp/will-create in tracked changes")
	}
	if created.SizeBefore != 0 {
		t.Errorf("created.SizeBefore = %d, want 0", created.SizeBefore)
	}
	if created.ChecksumAfter == "" {
		t.Error("created.ChecksumAfter must be non-empty")
	}
	if created.ContentBefore != nil {
		t.Errorf("created.ContentBefore should be nil for new files; got %q", created.ContentBefore)
	}
}

// TestIssue27_EnhanceComputesSizeDelta locks down that EnhanceFileChange
// derives SizeDelta = SizeAfter - SizeBefore. Pre-fix SizeDelta was
// always 0 because (a) FileChange had no SizeBefore field and (b) the
// arithmetic was missing — together producing the bug's headline
// "summary.total_size_delta: 0 despite 12 bytes of writes."
func TestIssue27_EnhanceComputesSizeDelta(t *testing.T) {
	cases := []struct {
		name      string
		before    int64
		after     int64
		wantDelta int64
	}{
		{"created", 0, 4, 4},
		{"grew", 4, 8, 4},
		{"shrank", 8, 4, -4},
		{"unchanged", 4, 4, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := &artifacts.FileChange{
				Path:       "/tmp/x",
				Operation:  "updated",
				SizeBytes:  tc.after,
				SizeBefore: tc.before,
			}
			d := artifacts.EnhanceFileChange(fc, "", "")
			if d.SizeBefore != tc.before {
				t.Errorf("SizeBefore = %d, want %d", d.SizeBefore, tc.before)
			}
			if d.SizeAfter != tc.after {
				t.Errorf("SizeAfter = %d, want %d", d.SizeAfter, tc.after)
			}
			if d.SizeDelta != tc.wantDelta {
				t.Errorf("SizeDelta = %d, want %d", d.SizeDelta, tc.wantDelta)
			}
		})
	}
}

func find(changes []artifacts.FileChange, path string) *artifacts.FileChange {
	for i := range changes {
		if changes[i].Path == path {
			return &changes[i]
		}
	}
	return nil
}

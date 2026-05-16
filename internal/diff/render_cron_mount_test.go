package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// ------ helpers --------------------------------------------------------------

func cronDiff(op actions.Operation, payload *actions.CronDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: payload.Name,
			Attributes: map[string]string{"kind": "os.cron"},
		},
		Operation: op,
		After:     payload,
	}
}

func mountDiff(op actions.Operation, payload *actions.MountDiff) *actions.Diff {
	return &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: payload.Dest,
			Attributes: map[string]string{"kind": "os.mount"},
		},
		Operation: op,
		After:     payload,
	}
}

// ------ cron -----------------------------------------------------------------

func TestLookup_CronDiff_AddWithSchedule(t *testing.T) {
	r := Lookup(cronDiff(actions.OpCreate, &actions.CronDiff{
		Name:     "backup",
		State:    "present",
		User:     "deploy",
		Schedule: "0 3 * * *",
		Command:  "/usr/local/bin/backup.sh",
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "cron" {
		t.Errorf("Kind = %q, want cron", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  cron add backup [user=deploy]: 0 3 * * * /usr/local/bin/backup.sh\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_CronDiff_AddBare(t *testing.T) {
	// No user → no [user=...] tag. No command → ends at schedule.
	r := Lookup(cronDiff(actions.OpCreate, &actions.CronDiff{
		Name:     "tick",
		State:    "present",
		Schedule: "@daily",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  cron add tick: @daily\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_CronDiff_RemoveIdentityOnly(t *testing.T) {
	r := Lookup(cronDiff(actions.OpDelete, &actions.CronDiff{
		Name:  "backup",
		State: "absent",
		// Schedule/Command present but suppressed for delete.
		Schedule: "0 3 * * *",
		Command:  "x",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  cron remove backup\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_CronDiff_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.cron"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"name":     "backup",
			"state":    "present",
			"schedule": "@daily",
			"command":  "x",
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded cron diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  cron add backup: @daily x\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_CronDiff_WrongAttribute(t *testing.T) {
	d := cronDiff(actions.OpCreate, &actions.CronDiff{Name: "x"})
	d.Resource.Attributes["kind"] = "os.user"
	if r := Lookup(d); r != nil && r.Kind() == "cron" {
		t.Errorf("cron renderer matched on kind=os.user: %+v", r)
	}
}

func TestLookup_CronDiff_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.cron"},
		},
		After: map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty After: %+v", r)
	}
}

// ------ mount ----------------------------------------------------------------

func TestLookup_MountDiff_AddDeviceWithOptions(t *testing.T) {
	r := Lookup(mountDiff(actions.OpCreate, &actions.MountDiff{
		Src:     "/dev/sdb",
		Dest:    "/mnt/data",
		FSType:  "ext4",
		State:   "mounted",
		Options: []string{"defaults", "noatime"},
	}))
	if r == nil {
		t.Fatal("Lookup returned nil")
	}
	if r.Kind() != "mount" {
		t.Errorf("Kind = %q, want mount", r.Kind())
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, FormatText); err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "  mount add /dev/sdb /mnt/data ext4 (defaults,noatime)\n"
	if got := buf.String(); got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLookup_MountDiff_AddUUID(t *testing.T) {
	r := Lookup(mountDiff(actions.OpCreate, &actions.MountDiff{
		Src:    "UUID=abcd-1234",
		Dest:   "/var/log",
		FSType: "xfs",
		State:  "mounted",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  mount add UUID=abcd-1234 /var/log xfs\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_MountDiff_Unmount(t *testing.T) {
	// State=unmounted maps to OpUpdate at the handler; renderer
	// surfaces "unmount" as the user-facing verb. Show identity only,
	// no src/fstype clutter.
	r := Lookup(mountDiff(actions.OpUpdate, &actions.MountDiff{
		Src:    "/dev/sdb",
		Dest:   "/mnt/data",
		FSType: "ext4",
		State:  "unmounted",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  mount unmount /mnt/data\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_MountDiff_Remove(t *testing.T) {
	r := Lookup(mountDiff(actions.OpDelete, &actions.MountDiff{
		Dest:  "/mnt/data",
		State: "absent",
	}))
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); got != "  mount remove /mnt/data\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_MountDiff_JSONDecoded(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.mount"},
		},
		Operation: actions.OpCreate,
		After: map[string]interface{}{
			"src":     "tmpfs",
			"dest":    "/tmp/scratch",
			"fstype":  "tmpfs",
			"state":   "mounted",
			"options": []interface{}{"size=1G"},
		},
	}
	r := Lookup(d)
	if r == nil {
		t.Fatal("Lookup returned nil for json-decoded mount diff")
	}
	var buf bytes.Buffer
	_ = r.Render(&buf, FormatText)
	if got := buf.String(); !strings.Contains(got, "tmpfs /tmp/scratch tmpfs") ||
		!strings.Contains(got, "size=1G") {
		t.Errorf("Render = %q", got)
	}
}

func TestLookup_MountDiff_WrongAttribute(t *testing.T) {
	d := mountDiff(actions.OpCreate, &actions.MountDiff{Dest: "/x"})
	d.Resource.Attributes["kind"] = "os.cron"
	if r := Lookup(d); r != nil && r.Kind() == "mount" {
		t.Errorf("mount renderer matched on kind=os.cron: %+v", r)
	}
}

func TestLookup_MountDiff_EmptyAfter(t *testing.T) {
	d := &actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Attributes: map[string]string{"kind": "os.mount"},
		},
		After: map[string]interface{}{},
	}
	if r := Lookup(d); r != nil {
		t.Errorf("Lookup matched on empty After: %+v", r)
	}
}

func TestCronOperationVerb(t *testing.T) {
	cases := []struct {
		op   actions.Operation
		want string
	}{
		{actions.OpCreate, "add"},
		{actions.OpUpdate, "update"},
		{actions.OpDelete, "remove"},
		{actions.OpNoop, "noop"},
		{actions.Operation("weird"), "change"},
	}
	for _, tc := range cases {
		if got := cronOperationVerb(tc.op); got != tc.want {
			t.Errorf("cronOperationVerb(%q) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

func TestMountOperationVerb(t *testing.T) {
	cases := []struct {
		op    actions.Operation
		state string
		want  string
	}{
		{actions.OpCreate, "mounted", "add"},
		{actions.OpUpdate, "unmounted", "unmount"},
		{actions.OpUpdate, "mounted", "update"},
		{actions.OpDelete, "absent", "remove"},
		{actions.OpNoop, "mounted", "noop"},
		{actions.Operation("weird"), "x", "change"},
	}
	for _, tc := range cases {
		if got := mountOperationVerb(tc.op, tc.state); got != tc.want {
			t.Errorf("mountOperationVerb(%q,%q) = %q, want %q",
				tc.op, tc.state, got, tc.want)
		}
	}
}

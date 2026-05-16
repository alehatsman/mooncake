package template

import (
	"strings"
	"testing"
	"time"
)

func TestStrftimeFormat_Codes(t *testing.T) {
	// Fixed reference time: 2026-05-17 14:09:07 in a deterministic
	// non-UTC zone so %Z / %z are exercised meaningfully.
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	ref := time.Date(2026, time.May, 17, 14, 9, 7, 0, loc)

	cases := []struct {
		format string
		want   string
	}{
		{"%Y", "2026"},
		{"%y", "26"},
		{"%m", "05"},
		{"%d", "17"},
		{"%H", "14"},
		{"%I", "02"},
		{"%M", "09"},
		{"%S", "07"},
		{"%p", "PM"},
		{"%j", "137"},
		{"%a", "Sun"},
		{"%A", "Sunday"},
		{"%b", "May"},
		{"%B", "May"},
		{"%F", "2026-05-17"},
		{"%T", "14:09:07"},
		{"%R", "14:09"},
		{"%%", "%"},
		{"%Y%m%d_%H%M%S", "20260517_140907"},
		{"backup-%Y-%m-%d.tar.gz", "backup-2026-05-17.tar.gz"},
		{"plain text", "plain text"},
	}

	for _, c := range cases {
		got, err := strftimeFormat(ref, c.format)
		if err != nil {
			t.Errorf("strftimeFormat(%q) error = %v", c.format, err)
			continue
		}
		if got != c.want {
			t.Errorf("strftimeFormat(%q) = %q, want %q", c.format, got, c.want)
		}
	}
}

func TestStrftimeFormat_ZoneCodes(t *testing.T) {
	// %Z and %z depend on the zone. Use a fixed offset so the test is
	// deterministic across runners.
	loc := time.FixedZone("UTC+02:00", 2*60*60)
	ref := time.Date(2026, 5, 17, 0, 0, 0, 0, loc)

	got, err := strftimeFormat(ref, "%z")
	if err != nil {
		t.Fatalf("strftimeFormat(%%z): %v", err)
	}
	if got != "+0200" {
		t.Errorf("%%z = %q, want %q", got, "+0200")
	}

	// %Z just calls t.Zone() which returns the zone name on a
	// fixed-offset zone — accept any non-empty value.
	got, err = strftimeFormat(ref, "%Z")
	if err != nil || got == "" {
		t.Errorf("%%Z = %q, err=%v; want non-empty", got, err)
	}
}

func TestStrftimeFormat_AMHourBoundaries(t *testing.T) {
	// %I uses 12-hour clock: hour 0 → 12, hour 12 → 12, hour 13 → 01.
	cases := []struct {
		hour  int
		wantI string
		wantP string
	}{
		{0, "12", "AM"},
		{1, "01", "AM"},
		{11, "11", "AM"},
		{12, "12", "PM"},
		{13, "01", "PM"},
		{23, "11", "PM"},
	}
	for _, c := range cases {
		ref := time.Date(2026, 1, 1, c.hour, 0, 0, 0, time.UTC)
		got, err := strftimeFormat(ref, "%I %p")
		if err != nil {
			t.Errorf("hour=%d: error %v", c.hour, err)
			continue
		}
		want := c.wantI + " " + c.wantP
		if got != want {
			t.Errorf("hour=%d: got %q, want %q", c.hour, got, want)
		}
	}
}

func TestStrftimeFormat_Errors(t *testing.T) {
	ref := time.Date(2026, 5, 17, 14, 9, 7, 0, time.UTC)

	if _, err := strftimeFormat(ref, "%Q"); err == nil {
		t.Error("expected error for unknown code %Q; got nil")
	} else if !strings.Contains(err.Error(), "unknown format code") {
		t.Errorf("error = %q; want \"unknown format code\"", err)
	}

	if _, err := strftimeFormat(ref, "trailing %"); err == nil {
		t.Error("expected error for trailing %; got nil")
	}
}

func TestStrftimeFilter_RendersThroughPongo2(t *testing.T) {
	r, err := NewPongo2Renderer()
	if err != nil {
		t.Fatalf("NewPongo2Renderer: %v", err)
	}

	ref := time.Date(2026, 5, 17, 14, 9, 7, 0, time.UTC)
	vars := map[string]interface{}{"apply_started_at": ref}

	got, err := r.Render(`{{ apply_started_at | strftime:"%Y%m%d_%H%M%S" }}`, vars)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "20260517_140907" {
		t.Errorf("Render result = %q, want %q", got, "20260517_140907")
	}
}

func TestStrftimeFilter_UnknownCodeFailsRender(t *testing.T) {
	r, err := NewPongo2Renderer()
	if err != nil {
		t.Fatalf("NewPongo2Renderer: %v", err)
	}

	ref := time.Date(2026, 5, 17, 14, 9, 7, 0, time.UTC)
	vars := map[string]interface{}{"apply_started_at": ref}

	// Per proposal-09 design note: unknown codes must fail loudly so a
	// typo doesn't silently pass through (the original {% now %} bug).
	_, err = r.Render(`{{ apply_started_at | strftime:"%Q" }}`, vars)
	if err == nil {
		t.Fatal("expected render error for unknown strftime code %Q; got nil")
	}
}

func TestStrftimeFilter_NonTimeInputFails(t *testing.T) {
	r, err := NewPongo2Renderer()
	if err != nil {
		t.Fatalf("NewPongo2Renderer: %v", err)
	}
	vars := map[string]interface{}{"not_a_time": "hello"}
	_, err = r.Render(`{{ not_a_time | strftime:"%Y" }}`, vars)
	if err == nil {
		t.Fatal("expected render error for non-time input; got nil")
	}
}

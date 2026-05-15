package filter

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/plan"
)

// TestMT19_UnmatchedTagsError covers manual-test #19 (2026-05-15):
// `--tags deplly` (typo of `deploy`) silently degraded to "untagged
// steps only" with a green recap, so users thought their deploy ran
// when it didn't. The check fires on the cross-product of filter and
// plan-tag inventory.
func TestMT19_UnmatchedTagsError(t *testing.T) {
	stepDeploy := config.Step{Tags: []string{"deploy"}}
	stepBuild := config.Step{Tags: []string{"build"}}
	stepUntagged := config.Step{}
	plain := &plan.Plan{Steps: []config.Step{stepDeploy, stepBuild, stepUntagged}}
	allUntagged := &plan.Plan{Steps: []config.Step{stepUntagged, stepUntagged}}

	cases := []struct {
		name      string
		filter    []string
		plan      *plan.Plan
		wantEmpty bool
		mustMatch []string
	}{
		{"empty filter is fine", nil, plain, true, nil},
		{"matching tag passes", []string{"deploy"}, plain, true, nil},
		{"typo errors with suggestion", []string{"deplly"}, plain, false, []string{"no steps matched tags: deplly", "Did you mean: deploy", "(available: build, deploy)"}},
		{"unknown tag errors", []string{"prod"}, plain, false, []string{"no steps matched tags: prod"}},
		{"untagged-only plan stays quiet", []string{"deploy"}, allUntagged, true, nil},
		{"nil plan stays quiet", []string{"deploy"}, nil, true, nil},
		{"at least one filter tag matching is enough", []string{"deplly", "build"}, plain, true, nil},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := UnmatchedTagsError(c.filter, c.plan)
			if c.wantEmpty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if got == "" {
				t.Errorf("expected non-empty error")
				return
			}
			for _, sub := range c.mustMatch {
				if !strings.Contains(got, sub) {
					t.Errorf("error %q missing substring %q", got, sub)
				}
			}
		})
	}
}

func TestMT19_CollectStepTags_WalksNestedSteps(t *testing.T) {
	plan := []config.Step{
		{Tags: []string{"a"}},
		{Transaction: []config.Step{{Tags: []string{"b"}}}},
		{Try: []config.Step{{Tags: []string{"c"}}}},
		{OnChange: []config.Step{{Tags: []string{"d"}}}},
		{OnRollback: []config.Step{{Tags: []string{"e"}}}},
	}
	tags := collectStepTags(plan)
	got := map[string]bool{}
	for _, t := range tags {
		got[t] = true
	}
	for _, want := range []string{"a", "b", "c", "d", "e"} {
		if !got[want] {
			t.Errorf("expected tag %q to be collected, got %v", want, tags)
		}
	}
}

func TestMT19_Levenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"deploy", "deplly", 1},
		{"deploy", "deploy", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

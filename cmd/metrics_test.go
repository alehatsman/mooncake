package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseMetricsFields(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"single", []string{"cpu_usage_pct"}, []string{"cpu_usage_pct"}},
		{"comma_in_one_flag", []string{"cpu_usage_pct,load_avg_1m"}, []string{"cpu_usage_pct", "load_avg_1m"}},
		{"repeated_flags", []string{"cpu_usage_pct", "load_avg_1m"}, []string{"cpu_usage_pct", "load_avg_1m"}},
		{"mixed", []string{"cpu_usage_pct,load_avg_1m", "gpus_metrics"}, []string{"cpu_usage_pct", "load_avg_1m", "gpus_metrics"}},
		{"whitespace_trimmed", []string{" cpu_usage_pct , load_avg_1m "}, []string{"cpu_usage_pct", "load_avg_1m"}},
		{"empty_segments_dropped", []string{"cpu_usage_pct,,load_avg_1m"}, []string{"cpu_usage_pct", "load_avg_1m"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMetricsFields(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFormatCollectedAtRFC3339(t *testing.T) {
	ts := time.Date(2026, 5, 12, 21, 30, 45, 0, time.UTC)
	out := formatCollectedAt(map[string]time.Time{"cpu_usage_pct": ts})
	want := "2026-05-12T21:30:45Z"
	if out["cpu_usage_pct"] != want {
		t.Errorf("got %q, want %q", out["cpu_usage_pct"], want)
	}
}

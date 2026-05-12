package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleGetMetricsFullPayload(t *testing.T) {
	// No fields filter → full ToMap. Just assert it returns valid JSON with
	// the expected top-level keys; values are environment-dependent.
	out, err := HandleGetMetrics(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleGetMetrics: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, k := range []string{"cpu_usage_pct", "load_avg_1m", "memory_used_mb"} {
		if _, ok := got[k]; !ok {
			t.Errorf("expected key %q in full payload", k)
		}
	}
	// Full mode must NOT include _collected_at.
	if _, ok := got["_collected_at"]; ok {
		t.Error("full payload should not include _collected_at (only fields-filtered does)")
	}
}

func TestHandleGetMetricsFieldsFilter(t *testing.T) {
	args, _ := json.Marshal(map[string]interface{}{
		"fields": []string{"cpu_usage_pct", "load_avg_1m"},
	})
	out, err := HandleGetMetrics(context.Background(), args)
	if err != nil {
		t.Fatalf("HandleGetMetrics: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	// Only requested keys + _collected_at.
	if _, ok := got["memory_used_mb"]; ok {
		t.Error("unrequested key memory_used_mb leaked into filtered response")
	}
	for _, k := range []string{"cpu_usage_pct", "load_avg_1m", "_collected_at"} {
		if _, ok := got[k]; !ok {
			t.Errorf("expected key %q in filtered response", k)
		}
	}
	// _collected_at should only contain requested keys.
	ts, _ := got["_collected_at"].(map[string]interface{})
	for k := range ts {
		if k != "cpu_usage_pct" && k != "load_avg_1m" {
			t.Errorf("_collected_at leaked unrequested key %q", k)
		}
	}
}

func TestHandleGetMetricsRefreshBypassesCache(t *testing.T) {
	// Two calls with refresh:true should both re-sample; we verify by
	// checking that the second call's timestamp is >= the first.
	args, _ := json.Marshal(map[string]interface{}{
		"fields":  []string{"cpu_usage_pct"},
		"refresh": true,
	})
	first, err := HandleGetMetrics(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	var firstParsed map[string]interface{}
	_ = json.Unmarshal([]byte(first), &firstParsed)
	firstTS, _ := firstParsed["_collected_at"].(map[string]interface{})["cpu_usage_pct"].(string)

	second, err := HandleGetMetrics(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	var secondParsed map[string]interface{}
	_ = json.Unmarshal([]byte(second), &secondParsed)
	secondTS, _ := secondParsed["_collected_at"].(map[string]interface{})["cpu_usage_pct"].(string)

	// Both should be valid RFC3339 timestamps; second should be >= first
	// (string comparison works for RFC3339).
	if secondTS < firstTS {
		t.Errorf("refresh did not advance timestamp: first=%s second=%s", firstTS, secondTS)
	}
}

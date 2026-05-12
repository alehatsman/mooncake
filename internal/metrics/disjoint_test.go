package metrics_test

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/metrics"
)

// TestFactsMetricsKeysDisjoint guards the contract that facts.ToMap() and
// metrics.ToMap() never share keys. Templates and `when:` expressions read
// from a merged namespace at run start (executor.AddGlobalVariables), so
// overlap would mean a metrics value silently shadows a fact (or vice versa)
// — debugging that would be a nightmare.
func TestFactsMetricsKeysDisjoint(t *testing.T) {
	factsMap := (&facts.Facts{}).ToMap()
	metricsMap := (&metrics.Metrics{}).ToMap()

	for k := range metricsMap {
		if _, collides := factsMap[k]; collides {
			t.Errorf("key %q is in both facts.ToMap() and metrics.ToMap() — must be disjoint", k)
		}
	}
}

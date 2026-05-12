package metrics

import (
	"sync"
	"time"
)

var (
	cacheMu      sync.Mutex
	cachedM      *Metrics
	sampledAt    = map[string]time.Time{} // collector name → last successful sample
	forceRefresh bool
)

// Collect returns the current Metrics with per-collector TTL refresh. If
// fields is non-nil, only collectors whose Outputs() intersect fields are
// invoked (and only if past their TTL). If fields is nil, every registered
// collector is considered.
//
// collectedAt maps each requested ToMap key to the timestamp of its
// collector's last successful sample — exposed to CLI/MCP so callers see
// freshness without leaking TTL internals.
func Collect(fields []string) (m *Metrics, collectedAt map[string]time.Time, err error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cachedM == nil {
		cachedM = &Metrics{}
	}

	wantedKey := func(k string) bool {
		if fields == nil {
			return true
		}
		for _, f := range fields {
			if f == k {
				return true
			}
		}
		return false
	}

	now := time.Now()
	var firstErr error

	for _, c := range collectors {
		if !collectorWanted(c, wantedKey) {
			continue
		}
		last, ok := sampledAt[c.Name()]
		if !forceRefresh && ok && now.Sub(last) < c.TTL() {
			continue // fresh
		}
		if cerr := c.Collect(cachedM); cerr != nil {
			if firstErr == nil {
				firstErr = cerr
			}
			continue // do not mark sampledAt — retry next call
		}
		sampledAt[c.Name()] = time.Now()
	}
	forceRefresh = false

	collectedAt = buildCollectedAt(fields)
	return cachedM, collectedAt, firstErr
}

func collectorWanted(c Collector, wanted func(string) bool) bool {
	for _, out := range c.Outputs() {
		if wanted(out) {
			return true
		}
	}
	return false
}

func buildCollectedAt(fields []string) map[string]time.Time {
	out := map[string]time.Time{}
	for _, c := range collectors {
		ts, ok := sampledAt[c.Name()]
		if !ok {
			continue
		}
		for _, k := range c.Outputs() {
			if fields != nil && !containsString(fields, k) {
				continue
			}
			out[k] = ts
		}
	}
	return out
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// Refresh forces re-collection of every collector on the next Collect call,
// bypassing TTL once. The flag clears after the next Collect.
func Refresh() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	forceRefresh = true
}

// ClearCache discards the cached Metrics and timestamps. Intended for tests.
func ClearCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedM = nil
	sampledAt = map[string]time.Time{}
	forceRefresh = false
}

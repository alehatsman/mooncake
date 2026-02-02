package facts

import "sync"

var (
	cachedFacts *Facts
	cacheOnce   sync.Once
)

// Collect gathers system facts with per-process caching.
// Facts are collected only once per execution and cached in memory.
func Collect() *Facts {
	cacheOnce.Do(func() {
		cachedFacts = collectUncached()
	})
	return cachedFacts
}

// ClearCache forces re-collection on next Collect() call.
// This is primarily intended for testing purposes.
func ClearCache() {
	cacheOnce = sync.Once{}
	cachedFacts = nil
}

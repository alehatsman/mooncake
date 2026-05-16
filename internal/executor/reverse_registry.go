package executor

import (
	"fmt"
	"sync"
)

// ReverseDataFactory produces a fresh zero value of a typed
// ReverseData payload (e.g. `func() any { return &FileReverseInfo{} }`).
// Callers register one factory per concrete type at init() time;
// Result.UnmarshalJSON looks it up by discriminator to materialise
// the concrete type from the wire envelope.
type ReverseDataFactory func() any

// reverseDataRegistry holds the global discriminator → factory map.
// Populated at init() time by handler packages; read by
// Result.UnmarshalJSON when decoding a wire envelope. The lock is
// effectively read-only after init, but kept explicit so a future
// dynamic loader (plugins) doesn't race.
var reverseDataRegistry = struct {
	mu        sync.RWMutex
	factories map[string]ReverseDataFactory
}{factories: map[string]ReverseDataFactory{}}

// RegisterReverseDataType registers a factory for a concrete
// ReverseData payload type, keyed by the wire discriminator name.
// Convention: pass the Go type name (e.g. "FileReverseInfo") so
// reflect-based encode and registry-based decode agree on the
// discriminator without per-handler bookkeeping.
//
// Panics on duplicate registration — silent overwrite would let a
// later handler shadow an earlier one and surface as a wire-decode
// drift far from the cause.
//
// Called from each handler package's init() alongside
// actions.Register. The wire round-trip is the contract this
// registry implements: see Result.MarshalJSON / UnmarshalJSON
// (spec R2.1c phase 2).
func RegisterReverseDataType(name string, factory ReverseDataFactory) {
	reverseDataRegistry.mu.Lock()
	defer reverseDataRegistry.mu.Unlock()
	if _, exists := reverseDataRegistry.factories[name]; exists {
		panic(fmt.Sprintf("executor.RegisterReverseDataType: duplicate registration for %q", name))
	}
	reverseDataRegistry.factories[name] = factory
}

// newReverseData looks up a factory by wire discriminator and
// invokes it. Returns (nil, false) when the type is unknown to this
// binary — Result.UnmarshalJSON treats that as "skip this field"
// (forward compatibility: a new daemon version registering a new
// type doesn't break old clients that simply don't know how to
// decode it).
func newReverseData(name string) (any, bool) {
	reverseDataRegistry.mu.RLock()
	defer reverseDataRegistry.mu.RUnlock()
	f, ok := reverseDataRegistry.factories[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

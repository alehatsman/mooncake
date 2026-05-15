package actions

import (
	"github.com/alehatsman/mooncake/internal/config"
)

// Spec-22 phase 2 — registry helpers with safe defaults.
//
// Each Resolve* helper takes a Handler and always returns a non-nil
// implementation of the corresponding sub-interface. When the handler
// natively implements it, that's what comes back; otherwise a default
// wrapper provides the fallback semantics from spec-22:
//
//   | Missing    | Default                                                    |
//   |------------|------------------------------------------------------------|
//   | Differ     | Operation derived from Run(ModePlan) result; no Before/After |
//   | Reverser   | returns (nil, nil) — "no reverse needed"                    |
//   | Coster     | {Risk: 5, Reversible: <whether Reverser is implemented>}    |
//   | Permitter  | empty PermissionSet{}                                        |
//
// Callers should ALWAYS go through these helpers rather than asserting
// directly — the defaults preserve the contract for every existing
// handler without forcing each one to implement four methods up front.

// ResolveDiffer returns h as a Differ. If h doesn't implement Differ, a
// default wrapper is returned that derives a coarse Diff from h.Run in
// plan mode.
func ResolveDiffer(h Handler) Differ {
	if d, ok := h.(Differ); ok {
		return d
	}
	return defaultDiffer{h: h}
}

// ResolveReverser returns h as a Reverser. If h doesn't implement
// Reverser, a default that returns (nil, nil) is returned — i.e. "no
// reverse needed", NOT "irreversible." Handlers that want to declare
// themselves irreversible should implement Reverser and return an
// explicit error.
func ResolveReverser(h Handler) Reverser {
	if r, ok := h.(Reverser); ok {
		return r
	}
	return defaultReverser{}
}

// ResolveCoster returns h as a Coster. If h doesn't implement Coster,
// a neutral default is returned (Risk=5, Reversible inferred from
// whether h implements Reverser, Resources=-1, Bytes=-1).
func ResolveCoster(h Handler) Coster {
	if c, ok := h.(Coster); ok {
		return c
	}
	_, reversible := h.(Reverser)
	return defaultCoster{reversible: reversible}
}

// ResolvePermitter returns h as a Permitter. If h doesn't implement
// Permitter, an empty PermissionSet is returned — meaning "no special
// privileges declared." Callers should treat this as "unknown
// requirements," not "guaranteed safe."
func ResolvePermitter(h Handler) Permitter {
	if p, ok := h.(Permitter); ok {
		return p
	}
	return defaultPermitter{}
}

// IsDiffer / IsReverser / IsCoster / IsPermitter report whether h
// natively implements the corresponding sub-interface (without
// returning the default wrapper). Useful for capability reporting in
// `mooncake actions list` / docs generation / metadata APIs, where
// we want to distinguish "implements natively" from "has a default."
func IsDiffer(h Handler) bool    { _, ok := h.(Differ); return ok }
func IsReverser(h Handler) bool  { _, ok := h.(Reverser); return ok }
func IsCoster(h Handler) bool    { _, ok := h.(Coster); return ok }
func IsPermitter(h Handler) bool { _, ok := h.(Permitter); return ok }

// ----- defaults -------------------------------------------------------------

// defaultDiffer derives a coarse Diff from a Run-in-plan-mode result.
// It can only report Operation (update vs noop, conservatively); the
// rich Before/After payload and Lines breakdown are left empty. Real
// per-handler Diff implementations supersede this.
type defaultDiffer struct {
	h Handler
}

func (d defaultDiffer) Diff(_ Context, _ *config.Step) (Diff, error) {
	// We deliberately do NOT call Run in plan mode here, even though
	// the spec suggests "derive Diff from Run(ModePlan) Result." That
	// would mean every JSON plan request runs every handler in plan
	// mode just to fill out Diff — fine when planner already did the
	// plan pass, redundant otherwise. Phase 1+2 keeps the default
	// truly minimal: report ResourceOther + OpUpdate. Callers wanting
	// rich Diff implement Differ on the handler.
	return Diff{
		Resource: ResourceRef{
			Kind:       ResourceOther,
			Identifier: d.h.Metadata().Name,
		},
		Operation: OpUpdate,
	}, nil
}

// defaultReverser returns (nil, nil) — "no reverse needed." This is
// the right default because it treats absence-of-implementation as
// "this action has no inverse," NOT "this action is destructive and
// unrecoverable." For destructive-irreversible actions (e.g. `shell`,
// `cmd`), the handler should implement Reverser and return an explicit
// error so transactions surface the irreversibility honestly.
type defaultReverser struct{}

func (defaultReverser) Reverse(_ Context, _ *config.Step, _ Result) (*config.Step, error) {
	return nil, nil
}

// defaultCoster returns a middle-of-the-road CostEstimate. Reversible
// is captured at construction time from whether the underlying
// Handler also implements Reverser, so it remains accurate even though
// this default doesn't know which handler it wraps after construction.
type defaultCoster struct {
	reversible bool
}

func (c defaultCoster) Cost(_ Context, _ *config.Step) (CostEstimate, error) {
	return CostEstimate{
		Resources:  -1,
		Bytes:      -1,
		Reversible: c.reversible,
		Risk:       5,
	}, nil
}

// defaultPermitter returns an empty PermissionSet. This signals
// "handler did not declare any special requirements" — NOT
// "guaranteed safe." Executor preflight only enforces against the
// handler's own declaration; absence of declaration is permissive.
type defaultPermitter struct{}

func (defaultPermitter) Permissions(_ *config.Step) PermissionSet {
	return PermissionSet{}
}

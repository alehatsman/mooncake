package service

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// The Reverse() function works on synthetic ReverseData regardless
// of the host OS — only the apply-time capture is platform-specific.
// These tests construct OsServiceReverseInfo directly so they exercise
// the Reverse logic on any CI host, no skips needed.

func TestReverse_Linux_RestoresPriorActiveAndEnabled(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:             "nginx",
		Platform:         "linux",
		PriorActive:      true,
		PriorEnabled:     true,
		HadStateIntent:   true,
		HadEnabledIntent: true,
	}
	enabled := false
	step := &config.Step{OsService: &config.ServiceAction{
		Name:    "nginx",
		State:   ServiceStateStopped,
		Enabled: &enabled,
	}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsService == nil {
		t.Fatal("Reverse must return an os.service step")
	}
	if rev.OsService.State != ServiceStateStarted {
		t.Errorf("State = %s, want started (prior was active)", rev.OsService.State)
	}
	if rev.OsService.Enabled == nil || *rev.OsService.Enabled != true {
		t.Errorf("Enabled = %v, want *true (prior was enabled)", rev.OsService.Enabled)
	}
}

func TestReverse_Linux_PriorInactiveBecomesStopped(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:           "nginx",
		Platform:       "linux",
		PriorActive:    false,
		HadStateIntent: true,
	}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStarted}}

	rev, _ := h.Reverse(nil, step, r)
	if rev == nil || rev.OsService.State != ServiceStateStopped {
		t.Errorf("State = %v, want stopped (prior was inactive)", rev)
	}
}

// TestReverse_EmptyPlatformTreatedAsLinux pins backward compat: pre-
// darwin ReverseInfo payloads (captured before this commit landed)
// have no Platform tag — Reverse must still honor the Linux-shaped
// State reverse rather than treat them as "unknown platform".
func TestReverse_EmptyPlatformTreatedAsLinux(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:           "nginx",
		Platform:       "", // explicit empty — pre-darwin payload
		PriorActive:    true,
		HadStateIntent: true,
	}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStopped}}

	rev, _ := h.Reverse(nil, step, r)
	if rev == nil || rev.OsService.State != ServiceStateStarted {
		t.Errorf("State = %v, want started (empty platform treated as linux)", rev)
	}
}

// TestReverse_Windows_HonorsEnabledButNotState pins the windows
// (SCM) policy: like darwin, only the Enabled axis (StartType
// Automatic vs Disabled) inverts cleanly. Running/Stopped is
// transient on a host where StartType=Automatic auto-starts the
// service across reboots — a State reverse would over-promise.
func TestReverse_Windows_HonorsEnabledButNotState(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:             "MSSQL",
		Platform:         "windows",
		PriorEnabled:     true,
		HadStateIntent:   true,
		HadEnabledIntent: true,
	}
	enabled := false
	step := &config.Step{OsService: &config.ServiceAction{
		Name:    "MSSQL",
		State:   ServiceStateStopped,
		Enabled: &enabled,
	}}
	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsService == nil {
		t.Fatal("Reverse must return an os.service step")
	}
	if rev.OsService.State != "" {
		t.Errorf("windows: State must stay empty (SCM doesn't reverse State cleanly); got %q", rev.OsService.State)
	}
	if rev.OsService.Enabled == nil || *rev.OsService.Enabled != true {
		t.Errorf("windows: Enabled must reflect prior=true; got %v", rev.OsService.Enabled)
	}
}

// TestReverse_Darwin_HonorsEnabledButNotState pins the launchd policy:
// only the Enabled axis (load/unload) has clean inverse semantics; the
// State axis (started/stopped) doesn't map to launchd's
// transient-running vs. persistent-loaded distinction. Reverse on
// darwin emits Enabled when HadEnabledIntent and intentionally leaves
// State empty even when HadStateIntent is true.
func TestReverse_Darwin_HonorsEnabledButNotState(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:             "com.example.daemon",
		Platform:         "darwin",
		PriorEnabled:     true,
		HadStateIntent:   true, // apply pinned state...
		HadEnabledIntent: true, // ...and enabled
	}
	enabled := false
	step := &config.Step{OsService: &config.ServiceAction{
		Name:    "com.example.daemon",
		State:   ServiceStateStopped,
		Enabled: &enabled,
	}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsService == nil {
		t.Fatal("Reverse must return an os.service step")
	}
	if rev.OsService.State != "" {
		t.Errorf("darwin: State must stay empty (launchd doesn't reverse State cleanly); got %q", rev.OsService.State)
	}
	if rev.OsService.Enabled == nil || *rev.OsService.Enabled != true {
		t.Errorf("darwin: Enabled must reflect prior=true; got %v", rev.OsService.Enabled)
	}
}

// TestReverse_Darwin_StateOnlyApplyIsNoop catches the corner case
// where apply set State but no Enabled. On linux that's a valid
// state-only reverse; on darwin the State axis is skipped, so the
// inverse step has neither field set → return (nil, nil) so the
// transaction layer treats it as a noop.
func TestReverse_Darwin_StateOnlyApplyIsNoop(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:           "com.example.daemon",
		Platform:       "darwin",
		PriorActive:    true,
		HadStateIntent: true, // apply only pinned state
	}
	step := &config.Step{OsService: &config.ServiceAction{
		Name:  "com.example.daemon",
		State: ServiceStateStopped,
	}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev != nil {
		t.Errorf("darwin state-only apply must reverse to noop; got %+v", rev)
	}
}

func TestReverse_NoIntentReturnsNoop(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsServiceReverseInfo{
		Name:     "nginx",
		Platform: "linux",
		// No HadStateIntent or HadEnabledIntent — apply didn't
		// manage lifecycle.
	}
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx"}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse must not error on lifecycle-noop apply; got %v", err)
	}
	if rev != nil {
		t.Errorf("Reverse must return nil step when neither State nor Enabled is set on inverse; got %+v", rev)
	}
}

// TestReverse_NoReverseDataIsNoop covers both:
//   - apply was a noop (nothing was captured)
//   - apply ran on a platform that doesn't capture (windows today —
//     handleWindowsService is a stub, so runApply skips the capture
//     branch and ReverseData stays nil).
//
// Both should return (nil, nil) — no error, no reverse step.
func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	step := &config.Step{OsService: &config.ServiceAction{Name: "nginx", State: ServiceStateStarted}}

	rev, err := h.Reverse(nil, step, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if rev != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", rev)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := &Handler{}
	_, err := h.Reverse(nil, nil, nil)
	if err == nil {
		t.Fatal("Reverse must error on nil step")
	}
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := &Handler{}
	r := executor.NewResult()
	r.ReverseData = "wrong"
	_, err := h.Reverse(nil, &config.Step{OsService: &config.ServiceAction{Name: "nginx"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}

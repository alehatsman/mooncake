package package_handler

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_AlwaysSudoAndNetwork locks in the spec-22 contract
// for pkg: BOTH Sudo and Network are unconditionally true. The action
// type is the indicator — pkg always mutates system state via a root-
// privileged package manager AND fetches from remote repos. Even
// degenerate inputs (nil step, empty Names, state=absent) must keep
// the flags on so a policy layer that gates these can't be sidestepped
// by a malformed step.
func TestPermissions_AlwaysSudoAndNetwork(t *testing.T) {
	h := &Handler{}
	cases := []*config.Step{
		nil,
		{},
		{Pkg: &config.Package{}},
		{Pkg: &config.Package{Name: "vim"}},
		{Pkg: &config.Package{Name: "vim", State: "absent"}},
		{Pkg: &config.Package{Names: []string{"a", "b", "c"}}},
	}
	for i, step := range cases {
		got := h.Permissions(step)
		if !got.Sudo {
			t.Errorf("case %d: Sudo = false, must be true regardless of step shape", i)
		}
		if !got.Network {
			t.Errorf("case %d: Network = false, must be true regardless of step shape", i)
		}
	}
}

// TestPermissions_NoFilesystemWriteOrBinaries — pkg deliberately
// leaves FilesystemWrite + RequiredBinaries empty. FilesystemWrite
// because installs go to system-managed paths that don't map to a
// step field. RequiredBinaries because the handler auto-detects the
// manager; pinning a specific binary on PATH would break detection.
// Locked in so a future refactor doesn't accidentally add a write
// target or a hard binary dep.
func TestPermissions_NoFilesystemWriteOrBinaries(t *testing.T) {
	h := &Handler{}
	got := h.Permissions(&config.Step{Pkg: &config.Package{Name: "vim"}})
	if len(got.FilesystemWrite) != 0 {
		t.Errorf("FilesystemWrite = %v, want empty (pkg uses system-managed paths)", got.FilesystemWrite)
	}
	if len(got.RequiredBinaries) != 0 {
		t.Errorf("RequiredBinaries = %v, want empty (handler auto-detects manager)", got.RequiredBinaries)
	}
}

// TestPermissions_RegisteredAsPermitter catches a future refactor
// that narrows the receiver style and accidentally breaks the
// interface.
func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	if !actions.IsPermitter(&Handler{}) {
		t.Error("*Handler should satisfy actions.Permitter")
	}
}

package service

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPermissions_AlwaysSudo locks in the spec-22 contract for
// os.service: Sudo is unconditionally true. Every supported backend
// (systemd, launchd, Windows SCM) needs elevated privileges to
// start/stop/enable units OR mutate unit-file contents. Even a
// status-only check requires the right to introspect via the
// managing daemon. Degenerate inputs (nil step, missing Name) must
// still report Sudo=true so a policy layer can't be sidestepped.
func TestPermissions_AlwaysSudo(t *testing.T) {
	h := &Handler{}
	cases := []*config.Step{
		nil,
		{},
		{OsService: &config.ServiceAction{}},
		{OsService: &config.ServiceAction{Name: "ssh"}},
		{OsService: &config.ServiceAction{Name: "ssh", State: "started"}},
		{OsService: &config.ServiceAction{Name: "x.service", State: "stopped"}},
	}
	for i, step := range cases {
		got := h.Permissions(step)
		if !got.Sudo {
			t.Errorf("case %d: Sudo = false, must be true regardless of step shape", i)
		}
	}
}

// TestPermissions_NoNetworkOrWrites — os.service operations are all
// local: no Network, no FilesystemWrite. Unit-file mutations go to
// system-managed dirs (/etc/systemd/system, ~/Library/LaunchAgents,
// etc.) that aren't addressed via a step.OsService.Path. RequiredBinaries
// stays empty so cross-platform backend detection isn't pinned to one
// specific tool.
func TestPermissions_NoNetworkOrWrites(t *testing.T) {
	h := &Handler{}
	got := h.Permissions(&config.Step{OsService: &config.ServiceAction{Name: "ssh"}})
	if got.Network {
		t.Error("Network = true, want false (os.service is local)")
	}
	if len(got.FilesystemWrite) != 0 {
		t.Errorf("FilesystemWrite = %v, want empty (system-managed paths)", got.FilesystemWrite)
	}
	if len(got.RequiredBinaries) != 0 {
		t.Errorf("RequiredBinaries = %v, want empty (cross-platform backend detection)", got.RequiredBinaries)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	if !actions.IsPermitter(&Handler{}) {
		t.Error("*Handler should satisfy actions.Permitter")
	}
}

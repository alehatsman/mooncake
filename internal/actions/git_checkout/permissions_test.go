package git_checkout

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_NoNetwork(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x", Ref: "v1"}})
	if ps.Network {
		t.Errorf("Network must be false for git.checkout; got %+v", ps)
	}
}

func TestPermissions_RequiresGitBinary(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x", Ref: "v1"}})
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "git" {
		t.Errorf("RequiredBinaries = %v, want [git]", ps.RequiredBinaries)
	}
}

func TestPermissions_SudoOnSystemDest(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitCheckout: &config.GitCheckout{Dest: "/opt/app", Ref: "v1"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true for dest under /opt; got %+v", ps)
	}
}

func TestPermissions_FilesystemWriteIsDest(t *testing.T) {
	h := Handler{}
	dest := "/tmp/myrepo"
	ps := h.Permissions(&config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: "v1"}})
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != dest {
		t.Errorf("FilesystemWrite = %v, want [%s]", ps.FilesystemWrite, dest)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

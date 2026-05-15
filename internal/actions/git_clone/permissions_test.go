package git_clone

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_NetworkAlwaysTrue(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitClone: &config.GitClone{Repo: "https://x", Dest: "/tmp/x"}})
	if !ps.Network {
		t.Fatalf("Network must be true for git.clone; got %+v", ps)
	}
}

func TestPermissions_RequiresGitBinary(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/tmp/x"}})
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "git" {
		t.Errorf("RequiredBinaries = %v, want [git]", ps.RequiredBinaries)
	}
}

func TestPermissions_SudoOnSystemDest(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/etc/app"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true for dest under /etc; got %+v", ps)
	}
}

func TestPermissions_NoSudoOnUserDest(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitClone: &config.GitClone{Repo: "x", Dest: "/tmp/myrepo"}})
	if ps.Sudo {
		t.Errorf("Sudo must be false for dest under /tmp; got %+v", ps)
	}
}

func TestPermissions_FilesystemWriteIsDest(t *testing.T) {
	h := Handler{}
	dest := "/opt/myapp"
	ps := h.Permissions(&config.Step{GitClone: &config.GitClone{Repo: "x", Dest: dest}})
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != dest {
		t.Errorf("FilesystemWrite = %v, want [%s]", ps.FilesystemWrite, dest)
	}
}

func TestPermissions_NilStepReturnsBase(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(nil)
	if !ps.Network {
		t.Errorf("Network must be true even for nil step; got %+v", ps)
	}
	if ps.Sudo {
		t.Errorf("Sudo must be false when step is nil; got %+v", ps)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

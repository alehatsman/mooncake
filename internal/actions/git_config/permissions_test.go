package git_config

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_NoNetwork(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"user.email": "x"}}})
	if ps.Network {
		t.Errorf("Network must be false for git.config; got %+v", ps)
	}
}

func TestPermissions_RequiresGitBinary(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"user.email": "x"}}})
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "git" {
		t.Errorf("RequiredBinaries = %v, want [git]", ps.RequiredBinaries)
	}
}

func TestPermissions_GlobalScopeNoSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"user.email": "x"}}})
	if ps.Sudo {
		t.Errorf("Sudo must be false for global scope; got %+v", ps)
	}
}

func TestPermissions_SystemScopeNeedsSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitConfig: &config.GitConfig{Scope: "system", Set: map[string]string{"user.email": "x"}}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true for system scope; got %+v", ps)
	}
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != "/etc/gitconfig" {
		t.Errorf("FilesystemWrite = %v, want [/etc/gitconfig]", ps.FilesystemWrite)
	}
}

func TestPermissions_LocalScopeUserRepoNoSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  "/tmp/myrepo",
		Set:   map[string]string{"core.autocrlf": "false"},
	}})
	if ps.Sudo {
		t.Errorf("Sudo must be false for local scope under /tmp; got %+v", ps)
	}
}

func TestPermissions_LocalScopeSystemRepoNeedsSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  "/opt/shared/repo",
		Set:   map[string]string{"core.autocrlf": "false"},
	}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true for local scope under /opt; got %+v", ps)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

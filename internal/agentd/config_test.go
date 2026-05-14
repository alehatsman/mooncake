package agentd

import (
	"strings"
	"testing"
)

// TestValidate_RequiresAtLeastOneListener locks in the new contract:
// unix-only, TCP-only, and both-at-once are valid; neither is not.
// Spec-49 §"Validate".
func TestValidate_RequiresAtLeastOneListener(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string // substring; "" means no error
	}{
		{
			name: "unix-only",
			cfg: Config{
				SocketPath: "/tmp/a.sock",
				StateDir:   "/tmp/state",
			},
		},
		{
			name: "tcp-only with token",
			cfg: Config{
				BindAddr: "127.0.0.1:7878",
				StateDir: "/tmp/state",
				Token:    "abc",
			},
		},
		{
			name: "both unix and tcp",
			cfg: Config{
				SocketPath: "/tmp/a.sock",
				BindAddr:   "127.0.0.1:7878",
				StateDir:   "/tmp/state",
				Token:      "abc",
			},
		},
		{
			name: "neither listener configured",
			cfg: Config{
				StateDir: "/tmp/state",
			},
			wantErr: "at least one of socket_path or bind_addr",
		},
		{
			name: "tcp without token",
			cfg: Config{
				BindAddr: "127.0.0.1:7878",
				StateDir: "/tmp/state",
			},
			wantErr: "token is empty",
		},
		{
			name: "missing state_dir",
			cfg: Config{
				SocketPath: "/tmp/a.sock",
			},
			wantErr: "state_dir is empty",
		},
		{
			name: "relative socket path",
			cfg: Config{
				SocketPath: "relative.sock",
				StateDir:   "/tmp/state",
			},
			wantErr: "must be absolute",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("want nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

// TestEnsureDirs_SkipsSocketDirInTCPOnlyMode — when SocketPath is empty,
// EnsureDirs must NOT try to MkdirAll(filepath.Dir("") == "."). Without
// this guard the daemon would silently create a "./" directory chain in
// the daemon's CWD, which is wrong on every platform.
func TestEnsureDirs_SkipsSocketDirInTCPOnlyMode(t *testing.T) {
	tmp := t.TempDir()
	cfg := Config{
		BindAddr: "127.0.0.1:0",
		StateDir: tmp + "/state",
		Token:    "abc",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
}

// TestDefault_ReturnsPopulatedConfig sanity-checks the platform-tagged
// helpers: Default(false) must return a fully populated config (all
// listener+state+token paths set, MaxSyncBytes != 0).
func TestDefault_ReturnsPopulatedConfig(t *testing.T) {
	cfg, err := Default(false)
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if cfg.SocketPath == "" {
		t.Error("SocketPath empty")
	}
	if cfg.StateDir == "" {
		t.Error("StateDir empty")
	}
	if cfg.TokenPath == "" {
		t.Error("TokenPath empty")
	}
	if cfg.MaxSyncBytes == 0 {
		t.Error("MaxSyncBytes zero")
	}
}

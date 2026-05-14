package transport

import (
	"strings"
	"testing"
)

func TestParseSSHTarget(t *testing.T) {
	cases := []struct {
		in           string
		wantUser     string
		wantHost     string
		wantPort     int
		wantErrSubst string
	}{
		{"aleh@192.168.1.68", "aleh", "192.168.1.68", 0, ""},
		{"aleh@192.168.1.68:2222", "aleh", "192.168.1.68", 2222, ""},
		{"host", "", "host", 0, ""},
		{"host:22", "", "host", 22, ""},
		{"user@", "", "", 0, "missing host"},
		{"", "", "", 0, "empty"},
		{"host:not-a-port", "", "", 0, "invalid port"},
		{"user@host:0", "", "", 0, "invalid port"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseSSHTarget(tc.in)
			if tc.wantErrSubst != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubst) {
					t.Fatalf("want err containing %q, got %v", tc.wantErrSubst, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.User != tc.wantUser || got.Host != tc.wantHost || got.Port != tc.wantPort {
				t.Errorf("got %+v, want user=%q host=%q port=%d",
					got, tc.wantUser, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestSSHTarget_String(t *testing.T) {
	cases := []struct {
		t    SSHTarget
		want string
	}{
		{SSHTarget{User: "u", Host: "h"}, "u@h"},
		{SSHTarget{User: "", Host: "h"}, "h"},
		{SSHTarget{Host: "h", Port: 22}, "h"}, // port not in String()
	}
	for _, tc := range cases {
		if got := tc.t.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.t, got, tc.want)
		}
	}
}

// TestParseUnameOutput drives DetectPlatform's pure-function half against
// the real combinations that surface from `uname -s` and `uname -m` on
// Linux + macOS. Cheap to test and the place a regression is most likely.
func TestParseUnameOutput(t *testing.T) {
	cases := []struct {
		in           string
		wantOS       string
		wantArch     string
		wantErrSubst string
	}{
		{"Linux\nx86_64", "linux", "amd64", ""},
		{"Linux\naarch64", "linux", "arm64", ""},
		{"linux\narm64", "linux", "arm64", ""},
		{"Darwin\narm64", "darwin", "arm64", ""},
		{"Darwin\nx86_64", "darwin", "amd64", ""},
		{"FreeBSD\namd64", "", "", "unsupported OS"},
		{"Linux\nmips", "", "", "unsupported arch"},
		{"Linux", "", "", "expected 2 lines"},
	}
	for _, tc := range cases {
		t.Run(strings.ReplaceAll(tc.in, "\n", "+"), func(t *testing.T) {
			osN, arch, err := parseUnameOutput(tc.in)
			if tc.wantErrSubst != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubst) {
					t.Fatalf("want err containing %q, got %v", tc.wantErrSubst, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if osN != tc.wantOS || arch != tc.wantArch {
				t.Errorf("got os=%q arch=%q, want os=%q arch=%q",
					osN, arch, tc.wantOS, tc.wantArch)
			}
		})
	}
}

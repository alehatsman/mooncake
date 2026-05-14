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

func TestSSHRunner_BuildsCorrectArgs(t *testing.T) {
	r := NewSSHRunner(SSHTarget{User: "aleh", Host: "h", Port: 2222})
	args := r.sshArgs(false)
	if !contains(args, "-p") || !contains(args, "2222") {
		t.Errorf("ssh args missing port flag: %v", args)
	}
	scpArgs := r.sshArgs(true)
	if !contains(scpArgs, "-P") || !contains(scpArgs, "2222") {
		t.Errorf("scp args missing port flag: %v", scpArgs)
	}
	if !contains(args, "BatchMode=yes") {
		t.Errorf("expected BatchMode=yes in args: %v", args)
	}
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

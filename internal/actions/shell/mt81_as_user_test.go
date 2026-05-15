//go:build !windows

package shell

import (
	"context"
	"os/exec"
	"os/user"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestMT81_AsUser_SudoMissing asserts that when as_user is set but
// sudo isn't installed, the user sees a targeted error pointing at
// the fix (install sudo / run as target user). Pre-fix, the error
// was the generic "command failed with exit code 1" surfaced from
// the failed sudo exec.
func TestMT81_AsUser_SudoMissing(t *testing.T) {
	origLook := lookPathFunc
	defer func() { lookPathFunc = origLook }()
	lookPathFunc = func(string) (string, error) { return "", exec.ErrNotFound }

	h := &Handler{}
	ctx := newMockExecutionContext()
	step := &config.Step{
		Shell:  &config.ShellAction{Cmd: "id"},
		AsUser: "alice",
	}

	_, err := h.buildCommand(context.Background(), ctx, step, "id")
	if err == nil {
		t.Fatal("expected error when sudo missing + as_user set")
	}
	for _, want := range []string{"as_user: alice", "sudo not on PATH", "apt-get install sudo"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to contain %q, got %q", want, err.Error())
		}
	}
}

// TestMT81_AsUser_UserMissing asserts that when as_user names a user
// who doesn't exist, the user sees a targeted error instead of the
// generic post-fork failure from sudo -u.
func TestMT81_AsUser_UserMissing(t *testing.T) {
	origLook := lookPathFunc
	origUser := userLookup
	defer func() {
		lookPathFunc = origLook
		userLookup = origUser
	}()
	lookPathFunc = func(string) (string, error) { return "/usr/bin/sudo", nil }
	userLookup = func(name string) (*user.User, error) {
		return nil, user.UnknownUserError(name)
	}

	h := &Handler{}
	ctx := newMockExecutionContext()
	step := &config.Step{
		Shell:  &config.ShellAction{Cmd: "id"},
		AsUser: "nobodyxyz",
	}

	_, err := h.buildCommand(context.Background(), ctx, step, "id")
	if err == nil {
		t.Fatal("expected error when as_user names nonexistent user")
	}
	for _, want := range []string{"as_user: nobodyxyz", "user does not exist", "os.user"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to contain %q, got %q", want, err.Error())
		}
	}
}

// TestMT81_AsUser_Happy asserts the preflight doesn't break the
// good path: when sudo is present AND the user exists, buildCommand
// produces a valid command.
func TestMT81_AsUser_Happy(t *testing.T) {
	origLook := lookPathFunc
	origUser := userLookup
	defer func() {
		lookPathFunc = origLook
		userLookup = origUser
	}()
	lookPathFunc = func(string) (string, error) { return "/usr/bin/sudo", nil }
	userLookup = func(name string) (*user.User, error) {
		return &user.User{Username: name, Uid: "1000"}, nil
	}

	h := &Handler{}
	ctx := newMockExecutionContext()
	step := &config.Step{
		Shell:  &config.ShellAction{Cmd: "id"},
		AsUser: "alice",
	}

	cmd, err := h.buildCommand(context.Background(), ctx, step, "id")
	if err != nil {
		t.Fatalf("buildCommand unexpected error: %v", err)
	}
	if cmd == nil || !strings.HasSuffix(cmd.Path, "sudo") {
		t.Errorf("expected sudo command, got %+v", cmd)
	}
}

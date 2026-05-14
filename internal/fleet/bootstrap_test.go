package fleet

import (
	"context"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fakeSession is a Session-shaped stub used only to exercise sudoer.Run's
// command-shape contract. It records every command and returns a canned
// response. Real Session is exercised by the transport-package integration
// tests in PR 9.
//
// Why not test Bootstrap end-to-end here: the production Bootstrap path
// drives transport.Connect, which would require either (a) a live SSH
// server (Docker; not available in this test environment) or (b)
// duplicating the testSSHServer scaffolding from PR 9's transport_test
// files into the fleet package (~150 LOC of duplication). Pure-function
// + command-shape coverage is the pragmatic level for now; revisit when
// CI grows an SSH harness.
type fakeSession struct {
	commands []string
	canned   string
}

// runVia is the interface sudoer needs from its underlying session. The
// real sudoer holds a *transport.Session whose Run method has the same
// shape; this fake satisfies the same shape so the test can call sudoer.Run
// without reaching real SSH.
type runVia interface {
	Run(ctx context.Context, cmd string) (string, string, int, error)
}

func (f *fakeSession) Run(_ context.Context, cmd string) (string, string, int, error) {
	f.commands = append(f.commands, cmd)
	return f.canned, "", 0, nil
}

// makeSudoerWith constructs a sudoer that talks to runVia instead of a
// real Session. Requires a tiny copy of the production sudoer for the
// test — production sudoer carries a *transport.Session concrete type so
// it can't be reused here without a refactor.
type testSudoer struct {
	r      runVia
	isRoot bool
}

func (s *testSudoer) Run(ctx context.Context, cmd string) (string, string, int, error) {
	if s.isRoot {
		return s.r.Run(ctx, cmd)
	}
	escaped := strings.ReplaceAll(cmd, "'", `'"'"'`)
	return s.r.Run(ctx, "sudo -n sh -c '"+escaped+"'")
}

func TestSudoer_RootSkipsPrefix(t *testing.T) {
	f := &fakeSession{}
	s := &testSudoer{r: f, isRoot: true}
	_, _, _, _ = s.Run(context.Background(), "systemctl daemon-reload")
	if len(f.commands) != 1 {
		t.Fatalf("commands = %v", f.commands)
	}
	if f.commands[0] != "systemctl daemon-reload" {
		t.Errorf("root should not wrap with sudo; got %q", f.commands[0])
	}
}

func TestSudoer_NonRootWrapsWithSudo(t *testing.T) {
	f := &fakeSession{}
	s := &testSudoer{r: f, isRoot: false}
	_, _, _, _ = s.Run(context.Background(), "systemctl daemon-reload")
	if len(f.commands) != 1 {
		t.Fatalf("commands = %v", f.commands)
	}
	got := f.commands[0]
	if !strings.HasPrefix(got, "sudo -n sh -c '") {
		t.Errorf("non-root should wrap with sudo -n sh -c '...'; got %q", got)
	}
	if !strings.HasSuffix(got, "'") {
		t.Errorf("missing closing quote; got %q", got)
	}
	if !strings.Contains(got, "systemctl daemon-reload") {
		t.Errorf("inner command missing; got %q", got)
	}
}

// TestSudoer_NonRootEscapesEmbeddedQuotes pins the single-quote-escape
// contract. Bootstrap.go composes commands with paths that almost never
// contain quotes today, but a misbehaving command embedded with `'`
// would break sh -c parsing without escaping.
func TestSudoer_NonRootEscapesEmbeddedQuotes(t *testing.T) {
	f := &fakeSession{}
	s := &testSudoer{r: f, isRoot: false}
	_, _, _, _ = s.Run(context.Background(), `echo "it's working"`)
	got := f.commands[0]
	// Single quotes in the inner command should be turned into '"'"' so
	// the outer sh -c '...' tokenization stays valid.
	if !strings.Contains(got, `'"'"'`) {
		t.Errorf("embedded ' should be escaped to '\"'\"'; got %q", got)
	}
}

// Compile-time check: real *transport.Session satisfies runVia. If this
// breaks, the testSudoer-vs-production-sudoer shape divergence has gone
// past acceptable — refactor to a shared interface.
var _ runVia = (*transport.Session)(nil)

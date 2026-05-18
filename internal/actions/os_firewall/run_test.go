//nolint:revive // package name follows action convention
package os_firewall

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

// stub captures every ufw invocation the action triggers and lets
// the test inject pre-canned status output. All package-level hooks
// route through it for the duration of the test.
type stub struct {
	statusOut string
	statusErr error
	hasUFW    bool
	calls     [][]string
	runErr    error
}

func newStub(t *testing.T) *stub {
	t.Helper()
	s := &stub{hasUFW: true}
	origRun, origStatus, origLook := ufwRun, ufwStatus, ufwLookPath
	ufwRun = func(_ *security.Privileged, args ...string) error {
		cp := append([]string(nil), args...)
		s.calls = append(s.calls, cp)
		return s.runErr
	}
	ufwStatus = func(_ *security.Privileged) (string, error) { return s.statusOut, s.statusErr }
	ufwLookPath = func() (string, error) {
		if !s.hasUFW {
			return "", errors.New("ufw not installed")
		}
		return "/usr/sbin/ufw", nil
	}
	t.Cleanup(func() { ufwRun, ufwStatus, ufwLookPath = origRun, origStatus, origLook })
	return s
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func runErr(t *testing.T, plan bool, step *config.Step) error {
	t.Helper()
	_, err := (&Handler{}).Run(newCtx(t, plan), step)
	return err
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"both rule and rules", &config.Step{OsFirewall: &config.OsFirewall{
			Rule:  &config.FirewallRule{Port: 22},
			Rules: []config.FirewallRule{{Port: 80}},
		}}, true},
		{"neither rule nor rules", &config.Step{OsFirewall: &config.OsFirewall{}}, true},
		{"bad state", &config.Step{OsFirewall: &config.OsFirewall{
			State: "queued", Rule: &config.FirewallRule{Port: 22},
		}}, true},
		{"bad backend", &config.Step{OsFirewall: &config.OsFirewall{
			Backend: "iptables", Rule: &config.FirewallRule{Port: 22},
		}}, true},
		{"port out of range", &config.Step{OsFirewall: &config.OsFirewall{
			Rule: &config.FirewallRule{Port: 99999},
		}}, true},
		{"port zero", &config.Step{OsFirewall: &config.OsFirewall{
			Rule: &config.FirewallRule{Port: 0},
		}}, true},
		{"bad protocol", &config.Step{OsFirewall: &config.OsFirewall{
			Rule: &config.FirewallRule{Port: 22, Protocol: "ipx"},
		}}, true},
		{"bad action", &config.Step{OsFirewall: &config.OsFirewall{
			Rule: &config.FirewallRule{Port: 22, Action: "block"},
		}}, true},
		{"ok single", &config.Step{OsFirewall: &config.OsFirewall{
			Rule: &config.FirewallRule{Port: 22},
		}}, false},
		{"ok multi", &config.Step{OsFirewall: &config.OsFirewall{
			Rules: []config.FirewallRule{
				{Port: 22, Comment: "ssh"},
				{Port: 80, Comment: "http"},
			},
		}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestApply_AddsMissingRule(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = "Status: active\n\n     To                         Action      From\n     --                         ------      ----\n"
	step := &config.Step{OsFirewall: &config.OsFirewall{
		Rule: &config.FirewallRule{Port: 22, Comment: "ssh"},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected add; reason=%q", r.Reason)
	}
	if len(st.calls) != 1 {
		t.Fatalf("expected 1 ufw call; got %d (%v)", len(st.calls), st.calls)
	}
	got := strings.Join(st.calls[0], " ")
	want := "allow port 22 proto tcp comment ssh"
	if got != want {
		t.Errorf("call mismatch\n got %q\nwant %q", got, want)
	}
}

func TestApply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere                   # ssh
`
	step := &config.Step{OsFirewall: &config.OsFirewall{
		Rule: &config.FirewallRule{Port: 22, Comment: "ssh"},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("rule already present should be noop; reason=%q", r.Reason)
	}
	if len(st.calls) != 0 {
		t.Errorf("expected zero ufw mutating calls; got %v", st.calls)
	}
}

func TestApply_AbsentRemovesPresentRule(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = `Status: active

[ 1] 22/tcp                     ALLOW IN    Anywhere                   # ssh
`
	step := &config.Step{OsFirewall: &config.OsFirewall{
		State: "absent",
		Rule:  &config.FirewallRule{Port: 22},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected removal; reason=%q", r.Reason)
	}
	if len(st.calls) != 1 || st.calls[0][0] != "delete" {
		t.Errorf("expected one delete call; got %v", st.calls)
	}
}

func TestApply_AbsentNoopWhenMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = "Status: active\n\n     To                         Action      From\n     --                         ------      ----\n"
	step := &config.Step{OsFirewall: &config.OsFirewall{
		State: "absent",
		Rule:  &config.FirewallRule{Port: 22},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("absent + missing should be noop; reason=%q", r.Reason)
	}
}

func TestApply_MultiRulePartialAdd(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = `Status: active

[ 1] 22/tcp                     ALLOW IN    Anywhere                   # ssh
`
	step := &config.Step{OsFirewall: &config.OsFirewall{
		Rules: []config.FirewallRule{
			{Port: 22, Comment: "ssh"},
			{Port: 80, Comment: "http"},
			{Port: 443, Comment: "https"},
		},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change (2 new rules); reason=%q", r.Reason)
	}
	if len(st.calls) != 2 {
		t.Errorf("expected 2 add calls; got %d (%v)", len(st.calls), st.calls)
	}
}

func TestApply_RuleWithCIDRSource(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = ""
	step := &config.Step{OsFirewall: &config.OsFirewall{
		Rule: &config.FirewallRule{Port: 22, From: "10.0.0.0/8"},
	}}
	_ = mustRun(t, false, step)
	got := strings.Join(st.calls[0], " ")
	want := "allow from 10.0.0.0/8 to any port 22 proto tcp"
	if got != want {
		t.Errorf("CIDR source call\n got %q\nwant %q", got, want)
	}
}

func TestPlan_DoesNotInvokeUFW(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = ""
	step := &config.Step{OsFirewall: &config.OsFirewall{
		Rule: &config.FirewallRule{Port: 22},
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should set WouldChange; reason=%q", r.Reason)
	}
	if len(st.calls) != 0 {
		t.Errorf("plan must not call ufw; got %v", st.calls)
	}
}

func TestBackend_AutoErrorsWhenUFWMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.hasUFW = false
	step := &config.Step{OsFirewall: &config.OsFirewall{
		Rule: &config.FirewallRule{Port: 22},
	}}
	err := runErr(t, false, step)
	if err == nil {
		t.Fatal("expected error when ufw not installed and backend=auto")
	}
	if !strings.Contains(err.Error(), "no supported backend") {
		t.Errorf("expected clear backend-missing message; got %v", err)
	}
}

func TestParseStatusLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want rule
		ok   bool
	}{
		{
			"basic allow",
			"[ 1] 22/tcp                     ALLOW IN    Anywhere                   # ssh",
			rule{Port: 22, Protocol: "tcp", Action: "allow", From: "any", Comment: "ssh"},
			true,
		},
		{
			"deny no comment",
			"[ 2] 23/tcp                     DENY IN     Anywhere",
			rule{Port: 23, Protocol: "tcp", Action: "deny", From: "any"},
			true,
		},
		{
			"explicit CIDR",
			"[ 3] 22/tcp                     ALLOW IN    10.0.0.0/8                 # vpn",
			rule{Port: 22, Protocol: "tcp", Action: "allow", From: "10.0.0.0/8", Comment: "vpn"},
			true,
		},
		{
			"header line",
			"     To                         Action      From",
			rule{},
			false,
		},
		{
			"empty line",
			"",
			rule{},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseStatusLine(c.in)
			if ok != c.ok {
				t.Fatalf("ok=%v want %v", ok, c.ok)
			}
			if ok && got != c.want {
				t.Errorf("parse mismatch\n got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestReadCurrent_CollapsesIPv6Duplicates(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	st := newStub(t)
	st.statusOut = `Status: active

[ 1] 22/tcp                     ALLOW IN    Anywhere                   # ssh
[ 2] 22/tcp (v6)                ALLOW IN    Anywhere (v6)              # ssh
`
	rules, err := readCurrent(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Both entries collapse to one logical rule because the parser
	// rejects "(v6)" suffixes that don't match a clean port/proto and
	// the dedup key is identical for the IPv4 line.
	allow22 := 0
	for _, r := range rules {
		if r.Port == 22 && r.Action == "allow" {
			allow22++
		}
	}
	if allow22 != 1 {
		t.Errorf("expected v4/v6 collapse to single rule; got %d entries: %+v", allow22, rules)
	}
}

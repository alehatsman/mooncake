package winutil

import (
	"strings"
	"testing"
)

func TestRule_Validate_RequiresName(t *testing.T) {
	if err := (Rule{}).Validate(); err == nil {
		t.Fatal("expected error for empty DisplayName")
	}
}

func TestRule_Validate_RejectsQuotesInName(t *testing.T) {
	for _, name := range []string{`has "double"`, `has 'single'`, `mix"both'`} {
		if err := (Rule{DisplayName: name}).Validate(); err == nil {
			t.Errorf("expected error for name %q", name)
		}
	}
}

func TestRule_Validate_RejectsBadEnums(t *testing.T) {
	cases := []struct {
		name string
		r    Rule
	}{
		{"direction", Rule{DisplayName: "x", Direction: Direction("sideways")}},
		{"protocol", Rule{DisplayName: "x", Protocol: Protocol("xyz")}},
		{"action", Rule{DisplayName: "x", Action: Action("maybe")}},
		{"profile", Rule{DisplayName: "x", Profiles: []Profile{Profile("home")}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.r.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRule_Validate_RejectsICMPWithPorts(t *testing.T) {
	r := Rule{
		DisplayName: "x",
		Protocol:    ProtoICMPv4,
		LocalPorts:  []string{"7878"},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error: ICMP + ports")
	}
}

func TestValidatePortSpec(t *testing.T) {
	good := []string{"7878", "any", "Any", "1-65535", "80-443"}
	for _, p := range good {
		if err := validatePortSpec(p); err != nil {
			t.Errorf("expected %q OK, got %v", p, err)
		}
	}
	bad := []string{"", "abc", "80-abc", "abc-80", "-80", "80-", "123456"}
	for _, p := range bad {
		if err := validatePortSpec(p); err == nil {
			t.Errorf("expected %q to fail validation", p)
		}
	}
}

func TestRenderCreate_DefaultsApplied(t *testing.T) {
	got, err := RenderCreate(Rule{
		DisplayName: "Mooncake Agentd",
		LocalPorts:  []string{"7879"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`New-NetFirewallRule`,
		`-DisplayName 'Mooncake Agentd'`,
		`-Direction Inbound`,
		`-Protocol TCP`,
		`-LocalPort 7879`,
		`-Action Allow`,
		`-Profile Domain,Private`,
		`| Out-Null`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered command missing %q:\n%s", want, got)
		}
	}
	// No -Enabled emitted when default (true).
	if strings.Contains(got, "-Enabled") {
		t.Errorf("expected no -Enabled in default render:\n%s", got)
	}
}

func TestRenderCreate_DisabledExplicit(t *testing.T) {
	f := false
	got, err := RenderCreate(Rule{DisplayName: "x", LocalPorts: []string{"80"}, Enabled: &f})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "-Enabled False") {
		t.Errorf("expected -Enabled False, got:\n%s", got)
	}
}

func TestRenderCreate_AnyProtocolSkipsProtocolArg(t *testing.T) {
	got, err := RenderCreate(Rule{DisplayName: "x", Protocol: ProtoAny})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "-Protocol") {
		t.Errorf("expected no -Protocol for ProtoAny, got:\n%s", got)
	}
}

func TestRenderCreate_AnyProfileShortCircuits(t *testing.T) {
	got, err := RenderCreate(Rule{
		DisplayName: "x",
		Profiles:    []Profile{ProfPrivate, ProfAny, ProfDomain},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "-Profile Any") {
		t.Errorf("expected -Profile Any to win, got:\n%s", got)
	}
	if strings.Contains(got, "Private") || strings.Contains(got, "Domain") {
		t.Errorf("Any should suppress others, got:\n%s", got)
	}
}

func TestRenderCreate_SortsPortsAndProfiles(t *testing.T) {
	a, err := RenderCreate(Rule{DisplayName: "x", LocalPorts: []string{"8080", "80", "443"}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderCreate(Rule{DisplayName: "x", LocalPorts: []string{"443", "80", "8080"}})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("port-order should not affect rendering:\na: %s\nb: %s", a, b)
	}
}

func TestRenderCreate_EscapesQuotesInDescription(t *testing.T) {
	got, err := RenderCreate(Rule{
		DisplayName: "x",
		Description: "it's complicated",
		LocalPorts:  []string{"80"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `'it''s complicated'`) {
		t.Errorf("expected doubled-quote escape, got:\n%s", got)
	}
}

func TestRenderCreate_FailsOnInvalidRule(t *testing.T) {
	_, err := RenderCreate(Rule{})
	if err == nil {
		t.Fatal("expected error for invalid rule")
	}
}

func TestRenderEnsure_WrapsWithGetGuard(t *testing.T) {
	got, err := RenderEnsure(Rule{DisplayName: "Mooncake Agentd", LocalPorts: []string{"7879"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`if (-not (Get-NetFirewallRule -DisplayName 'Mooncake Agentd' -ErrorAction SilentlyContinue))`,
		`New-NetFirewallRule`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

func TestRenderQuery_Shape(t *testing.T) {
	got := RenderQuery("Mooncake Agentd")
	for _, want := range []string{
		`Get-NetFirewallRule -DisplayName 'Mooncake Agentd'`,
		`Get-NetFirewallPortFilter`,
		`DisplayName  = $_.DisplayName`,
		`Protocol     = "$($portFilter.Protocol)"`,
		`Enabled      = "$($_.Enabled)"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("query render missing %q:\n%s", want, got)
		}
	}
}

func TestRenderDelete_HappyPath(t *testing.T) {
	got := RenderDelete("My Rule")
	if !strings.Contains(got, "Remove-NetFirewallRule -DisplayName 'My Rule'") {
		t.Errorf("got: %s", got)
	}
	if !strings.Contains(got, "-ErrorAction SilentlyContinue") {
		t.Errorf("expected silent-on-missing, got: %s", got)
	}
}

func TestPsStringLiteral(t *testing.T) {
	cases := map[string]string{
		"":          "''",
		"plain":     "'plain'",
		"has space": "'has space'",
		`it's`:      `'it''s'`,
		`'leading`:  `'''leading'`,
		`trailing'`: `'trailing'''`,
	}
	for in, want := range cases {
		if got := psStringLiteral(in); got != want {
			t.Errorf("psStringLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}

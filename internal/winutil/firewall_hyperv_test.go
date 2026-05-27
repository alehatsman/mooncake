package winutil

import (
	"strings"
	"testing"
)

func TestHyperVRule_Validate_RequiresName(t *testing.T) {
	if err := (HyperVRule{VMCreatorID: WSLDefaultVMCreatorID}).Validate(); err == nil {
		t.Fatal("expected error for empty DisplayName")
	}
}

func TestHyperVRule_Validate_RequiresVMCreatorID(t *testing.T) {
	if err := (HyperVRule{DisplayName: "x"}).Validate(); err == nil {
		t.Fatal("expected error for empty VMCreatorID")
	}
}

func TestHyperVRule_Validate_RejectsQuotesInVMCreatorID(t *testing.T) {
	r := HyperVRule{DisplayName: "x", VMCreatorID: `{has-"quotes"}`}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for quoted VMCreatorID")
	}
}

func TestHyperVRule_Validate_RejectsBadEnums(t *testing.T) {
	base := HyperVRule{DisplayName: "x", VMCreatorID: WSLDefaultVMCreatorID}
	cases := []struct {
		name string
		mut  func(*HyperVRule)
	}{
		{"direction", func(r *HyperVRule) { r.Direction = Direction("sideways") }},
		{"protocol", func(r *HyperVRule) { r.Protocol = Protocol("xyz") }},
		{"action", func(r *HyperVRule) { r.Action = Action("maybe") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mut(&r)
			if err := r.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestHyperVRule_Validate_RejectsICMPWithPorts(t *testing.T) {
	r := HyperVRule{
		DisplayName: "x",
		VMCreatorID: WSLDefaultVMCreatorID,
		Protocol:    ProtoICMPv4,
		LocalPorts:  []string{"7878"},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error: ICMP + ports")
	}
}

func TestRenderHyperVCreate_DefaultsApplied(t *testing.T) {
	got, err := RenderHyperVCreate(HyperVRule{
		DisplayName: "WSL2 SSH",
		VMCreatorID: WSLDefaultVMCreatorID,
		LocalPorts:  []string{"2222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`New-NetFirewallHyperVRule`,
		`-DisplayName 'WSL2 SSH'`,
		`-VMCreatorID '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}'`,
		`-Direction Inbound`,
		`-Protocol TCP`,
		`-LocalPorts 2222`,
		`-Action Allow`,
		`| Out-Null`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered command missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "-Profile") {
		t.Errorf("Hyper-V rules do not take -Profile:\n%s", got)
	}
	if strings.Contains(got, "-Enabled") {
		t.Errorf("expected no -Enabled in default render:\n%s", got)
	}
}

func TestRenderHyperVCreate_OmitsDescription(t *testing.T) {
	// New-NetFirewallHyperVRule does not accept -Description (unlike
	// New-NetFirewallRule). The YAML field exists for human readers
	// but must never leak into the rendered cmdlet — regression
	// guard from the first apply against a real Windows host.
	got, err := RenderHyperVCreate(HyperVRule{
		DisplayName: "WSL2 SSH",
		Description: "should not appear",
		VMCreatorID: WSLDefaultVMCreatorID,
		LocalPorts:  []string{"2222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "-Description") {
		t.Errorf("must not emit -Description; got:\n%s", got)
	}
	if strings.Contains(got, "should not appear") {
		t.Errorf("description text leaked into render:\n%s", got)
	}
}

func TestRenderHyperVCreate_DisabledExplicit(t *testing.T) {
	f := false
	got, err := RenderHyperVCreate(HyperVRule{
		DisplayName: "x",
		VMCreatorID: WSLDefaultVMCreatorID,
		LocalPorts:  []string{"80"},
		Enabled:     &f,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "-Enabled False") {
		t.Errorf("expected -Enabled False, got:\n%s", got)
	}
}

func TestRenderHyperVCreate_AnyProtocolSkipsProtocolArg(t *testing.T) {
	got, err := RenderHyperVCreate(HyperVRule{
		DisplayName: "x",
		VMCreatorID: WSLDefaultVMCreatorID,
		Protocol:    ProtoAny,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "-Protocol") {
		t.Errorf("expected no -Protocol for ProtoAny, got:\n%s", got)
	}
}

func TestRenderHyperVCreate_SortsPorts(t *testing.T) {
	a, err := RenderHyperVCreate(HyperVRule{
		DisplayName: "x", VMCreatorID: WSLDefaultVMCreatorID,
		LocalPorts: []string{"8080", "80", "443"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderHyperVCreate(HyperVRule{
		DisplayName: "x", VMCreatorID: WSLDefaultVMCreatorID,
		LocalPorts: []string{"443", "80", "8080"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("port-order should not affect rendering:\na: %s\nb: %s", a, b)
	}
}

func TestRenderHyperVCreate_FailsOnInvalidRule(t *testing.T) {
	if _, err := RenderHyperVCreate(HyperVRule{}); err == nil {
		t.Fatal("expected error for invalid rule")
	}
}

func TestRenderHyperVEnsure_WrapsWithGetGuard(t *testing.T) {
	got, err := RenderHyperVEnsure(HyperVRule{
		DisplayName: "WSL2 SSH",
		VMCreatorID: WSLDefaultVMCreatorID,
		LocalPorts:  []string{"2222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`if (-not (Get-NetFirewallHyperVRule -VMCreatorID '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}'`,
		`Where-Object { $_.DisplayName -eq 'WSL2 SSH' }`,
		`New-NetFirewallHyperVRule`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

func TestRenderHyperVQuery_Shape(t *testing.T) {
	got := RenderHyperVQuery("WSL2 SSH", WSLDefaultVMCreatorID)
	for _, want := range []string{
		`Get-NetFirewallHyperVRule -VMCreatorID '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}'`,
		`Where-Object { $_.DisplayName -eq 'WSL2 SSH' }`,
		`DisplayName  = $_.DisplayName`,
		`VMCreatorID  = "$($_.VMCreatorID)"`,
		`Enabled      = "$($_.Enabled)"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("query render missing %q:\n%s", want, got)
		}
	}
}

func TestRenderHyperVDelete_HappyPath(t *testing.T) {
	got := RenderHyperVDelete("My Rule", WSLDefaultVMCreatorID)
	for _, want := range []string{
		`Get-NetFirewallHyperVRule -VMCreatorID '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}'`,
		`Where-Object { $_.DisplayName -eq 'My Rule' }`,
		`Remove-NetFirewallHyperVRule -ErrorAction SilentlyContinue`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("delete render missing %q:\n%s", want, got)
		}
	}
}

func TestRenderHyperVResolveVMCreator_Shape(t *testing.T) {
	got := RenderHyperVResolveVMCreator("WSL", WSLDefaultVMCreatorID)
	for _, want := range []string{
		`Get-NetFirewallHyperVVMCreator -ErrorAction SilentlyContinue`,
		`Where-Object { $_.Name -match 'WSL' }`,
		`Select-Object -First 1 -ExpandProperty VMCreatorID`,
		`if (-not $wslId) { $wslId = '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}' }`,
		`$wslId`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("resolver render missing %q:\n%s", want, got)
		}
	}
}

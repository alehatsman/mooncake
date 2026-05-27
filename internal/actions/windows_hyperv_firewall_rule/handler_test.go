package windows_hyperv_firewall_rule

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/winutil"
)

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil config", &config.Step{}, true},
		{"present without name", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{State: "present"},
		}, true},
		{"absent without name", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{State: "absent"},
		}, true},
		{"bad state", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{Name: "x", State: "maybe"},
		}, true},
		{"happy present (auto vm_creator_id)", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
				Name: "WSL2 SSH", VMCreatorID: "auto", LocalPort: []string{"2222"},
			},
		}, false},
		{"happy present (literal vm_creator_id)", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
				Name: "WSL2 SSH", VMCreatorID: winutil.WSLDefaultVMCreatorID, LocalPort: []string{"2222"},
			},
		}, false},
		{"happy present (empty vm_creator_id = auto)", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
				Name: "WSL2 SSH", LocalPort: []string{"2222"},
			},
		}, false},
		{"happy absent", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
				Name: "WSL2 SSH", State: "absent",
			},
		}, false},
		{"bad protocol", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
				Name: "x", VMCreatorID: "auto", Protocol: "xyz",
			},
		}, true},
		{"icmp with ports rejected", &config.Step{
			WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
				Name: "x", VMCreatorID: "auto", Protocol: "icmpv4", LocalPort: []string{"7"},
			},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := h.Validate(tc.step)
			if (err != nil) != tc.wantErr {
				t.Errorf("got err=%v, want wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestToRule_FieldMapping(t *testing.T) {
	en := true
	got := toRule(&config.WindowsHyperVFirewallRule{
		Name:        "WSL2 SSH",
		Description: "fleet peer",
		VMCreatorID: winutil.WSLDefaultVMCreatorID,
		Direction:   "Inbound",
		Protocol:    "TCP",
		LocalPort:   []string{"2222"},
		Action:      "Allow",
		Enabled:     &en,
	})
	if got.DisplayName != "WSL2 SSH" {
		t.Errorf("name: %q", got.DisplayName)
	}
	if got.VMCreatorID != winutil.WSLDefaultVMCreatorID {
		t.Errorf("vmid: %q", got.VMCreatorID)
	}
	if got.Direction != winutil.DirInbound {
		t.Errorf("direction: %q", got.Direction)
	}
	if got.Protocol != winutil.ProtoTCP {
		t.Errorf("protocol: %q", got.Protocol)
	}
	if got.Action != winutil.ActionAllow {
		t.Errorf("action: %q", got.Action)
	}
}

func TestResolveVMCreatorID(t *testing.T) {
	prev := runPS
	defer func() { runPS = prev }()

	t.Run("literal GUID passes through", func(t *testing.T) {
		called := false
		runPS = func(string) (string, error) {
			called = true
			return "", nil
		}
		got, err := resolveVMCreatorID("{ABCDEF01-2345-6789-ABCD-EF0123456789}")
		if err != nil {
			t.Fatal(err)
		}
		if got != "{ABCDEF01-2345-6789-ABCD-EF0123456789}" {
			t.Errorf("got %q", got)
		}
		if called {
			t.Error("PowerShell should not be invoked for literal GUID")
		}
	})

	t.Run("auto triggers resolver", func(t *testing.T) {
		captured := ""
		runPS = func(s string) (string, error) {
			captured = s
			return winutil.WSLDefaultVMCreatorID + "\n", nil
		}
		got, err := resolveVMCreatorID("auto")
		if err != nil {
			t.Fatal(err)
		}
		if got != winutil.WSLDefaultVMCreatorID {
			t.Errorf("got %q", got)
		}
		if !strings.Contains(captured, "Get-NetFirewallHyperVVMCreator") {
			t.Errorf("expected resolver script, got: %s", captured)
		}
	})

	t.Run("empty also triggers resolver", func(t *testing.T) {
		runPS = func(string) (string, error) {
			return winutil.WSLDefaultVMCreatorID, nil
		}
		got, err := resolveVMCreatorID("")
		if err != nil {
			t.Fatal(err)
		}
		if got != winutil.WSLDefaultVMCreatorID {
			t.Errorf("got %q", got)
		}
	})

	t.Run("resolver empty output is an error", func(t *testing.T) {
		runPS = func(string) (string, error) {
			return "   \n", nil
		}
		if _, err := resolveVMCreatorID("auto"); err == nil {
			t.Error("expected error on empty resolver output")
		}
	})
}

func TestObservedRule_Equals(t *testing.T) {
	desired := winutil.HyperVRule{
		DisplayName: "WSL2 SSH",
		VMCreatorID: winutil.WSLDefaultVMCreatorID,
		Direction:   winutil.DirInbound,
		Protocol:    winutil.ProtoTCP,
		LocalPorts:  []string{"2222"},
		Action:      winutil.ActionAllow,
	}

	t.Run("exact match", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "WSL2 SSH",
			VMCreatorID: winutil.WSLDefaultVMCreatorID,
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"2222"},
			Action:      "allow",
			Enabled:     true,
		}
		if !obs.Equals(desired) {
			t.Errorf("expected match")
		}
	})

	t.Run("VMCreatorID compared case-insensitively", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "WSL2 SSH",
			VMCreatorID: strings.ToLower(winutil.WSLDefaultVMCreatorID),
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"2222"},
			Action:      "allow",
			Enabled:     true,
		}
		if !obs.Equals(desired) {
			t.Errorf("expected match (case-insensitive GUID)")
		}
	})

	t.Run("vmid drift", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "WSL2 SSH",
			VMCreatorID: "{00000000-0000-0000-0000-000000000000}",
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"2222"},
			Action:      "allow",
			Enabled:     true,
		}
		if obs.Equals(desired) {
			t.Errorf("expected mismatch on vmid")
		}
	})

	t.Run("port drift", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "WSL2 SSH",
			VMCreatorID: winutil.WSLDefaultVMCreatorID,
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"22"},
			Action:      "allow",
			Enabled:     true,
		}
		if obs.Equals(desired) {
			t.Errorf("expected mismatch on port")
		}
	})

	t.Run("disabled drift", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "WSL2 SSH",
			VMCreatorID: winutil.WSLDefaultVMCreatorID,
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"2222"},
			Action:      "allow",
			Enabled:     false,
		}
		if obs.Equals(desired) {
			t.Errorf("expected mismatch on enabled")
		}
	})
}

func TestStringSliceEqualUnsorted(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"b"}, false},
	}
	for _, tc := range cases {
		if got := stringSliceEqualUnsorted(tc.a, tc.b); got != tc.want {
			t.Errorf("eq(%v,%v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormaliseObservedPorts(t *testing.T) {
	if got := normaliseObservedPorts([]string{"Any"}); got != nil {
		t.Errorf("Any should normalise to nil, got %v", got)
	}
	if got := normaliseObservedPorts([]string{"2222", " 8080 "}); len(got) != 2 || got[1] != "8080" {
		t.Errorf("trim should work, got %v", got)
	}
}

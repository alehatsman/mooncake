package windows_firewall_rule

import (
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
			WindowsFirewallRule: &config.WindowsFirewallRule{State: "present"},
		}, true},
		{"absent without name", &config.Step{
			WindowsFirewallRule: &config.WindowsFirewallRule{State: "absent"},
		}, true},
		{"bad state", &config.Step{
			WindowsFirewallRule: &config.WindowsFirewallRule{Name: "x", State: "maybe"},
		}, true},
		{"happy present", &config.Step{
			WindowsFirewallRule: &config.WindowsFirewallRule{Name: "Mooncake", LocalPort: []string{"7879"}},
		}, false},
		{"happy absent", &config.Step{
			WindowsFirewallRule: &config.WindowsFirewallRule{Name: "Mooncake", State: "absent"},
		}, false},
		{"bad protocol", &config.Step{
			WindowsFirewallRule: &config.WindowsFirewallRule{Name: "x", Protocol: "xyz"},
		}, true},
		{"icmp with ports rejected", &config.Step{
			WindowsFirewallRule: &config.WindowsFirewallRule{Name: "x", Protocol: "icmpv4", LocalPort: []string{"7"}},
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
	got, err := toRule(&config.WindowsFirewallRule{
		Name:        "Mooncake Agentd",
		Description: "fleet peer",
		Direction:   "Inbound",
		Protocol:    "TCP",
		LocalPort:   []string{"7879"},
		Action:      "Allow",
		Profile:     []string{"Domain", "Private"},
		Enabled:     &en,
	})
	if err != nil {
		t.Fatal(err)
	}
	// All YAML inputs should be lowercase-normalised through the
	// winutil enums.
	if got.DisplayName != "Mooncake Agentd" {
		t.Errorf("name: %q", got.DisplayName)
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
	if len(got.Profiles) != 2 || got.Profiles[0] != winutil.ProfDomain {
		t.Errorf("profiles: %v", got.Profiles)
	}
}

func TestObservedRule_Equals(t *testing.T) {
	desired := winutil.Rule{
		DisplayName: "Mooncake",
		Direction:   winutil.DirInbound,
		Protocol:    winutil.ProtoTCP,
		LocalPorts:  []string{"7879"},
		Action:      winutil.ActionAllow,
		Profiles:    []winutil.Profile{winutil.ProfDomain, winutil.ProfPrivate},
	}

	t.Run("exact match", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "Mooncake",
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"7879"},
			Action:      "allow",
			Profiles:    []string{"domain", "private"},
			Enabled:     true,
		}
		if !obs.Equals(desired) {
			t.Errorf("expected match")
		}
	})

	t.Run("profile order doesn't matter", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "Mooncake",
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"7879"},
			Action:      "allow",
			Profiles:    []string{"private", "domain"}, // reversed
			Enabled:     true,
		}
		if !obs.Equals(desired) {
			t.Errorf("expected match (order-insensitive)")
		}
	})

	t.Run("port drift", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "Mooncake",
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"7878"}, // wrong
			Action:      "allow",
			Profiles:    []string{"domain", "private"},
			Enabled:     true,
		}
		if obs.Equals(desired) {
			t.Errorf("expected mismatch on port")
		}
	})

	t.Run("disabled drift", func(t *testing.T) {
		obs := &observedRule{
			DisplayName: "Mooncake",
			Direction:   "inbound",
			Protocol:    "tcp",
			LocalPorts:  []string{"7879"},
			Action:      "allow",
			Profiles:    []string{"domain", "private"},
			Enabled:     false, // drift
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
		{[]string{}, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a", "a"}, []string{"a"}, false},
		{[]string{"a"}, []string{"b"}, false},
	}
	for _, tc := range cases {
		if got := stringSliceEqualUnsorted(tc.a, tc.b); got != tc.want {
			t.Errorf("eq(%v,%v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseProfileMask(t *testing.T) {
	cases := map[string][]string{
		"":                      nil,
		"Domain":                {"domain"},
		"Domain, Private":       {"domain", "private"},
		"Any":                   {"any"},
		"Domain,Private,Public": {"domain", "private", "public"},
	}
	for in, want := range cases {
		got := parseProfileMask(in)
		if !stringSliceEqualUnsorted(got, want) {
			t.Errorf("parseProfileMask(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNormaliseObservedPorts(t *testing.T) {
	if got := normaliseObservedPorts([]string{"Any"}); got != nil {
		t.Errorf("Any should normalise to nil, got %v", got)
	}
	if got := normaliseObservedPorts([]string{"7878", " 8080 "}); len(got) != 2 || got[1] != "8080" {
		t.Errorf("trim should work, got %v", got)
	}
}

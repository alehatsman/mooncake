package windows_scheduled_task

import (
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/winutil"
)

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}
	bootBoot := &config.WindowsScheduledTaskTrigger{Type: "boot"}
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil config", &config.Step{}, true},
		{"missing name", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{Trigger: bootBoot, Actions: []config.WindowsScheduledTaskAction{{Execute: "x"}}},
		}, true},
		{"present without trigger", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{Name: "x", Actions: []config.WindowsScheduledTaskAction{{Execute: "x"}}},
		}, true},
		{"trigger AND triggers", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{
				Name:     "x",
				Trigger:  bootBoot,
				Triggers: []config.WindowsScheduledTaskTrigger{*bootBoot},
				Actions:  []config.WindowsScheduledTaskAction{{Execute: "x"}},
			},
		}, true},
		{"no actions", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{Name: "x", Trigger: bootBoot},
		}, true},
		{"happy present", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{
				Name:    "Mooncake-Agentd-Autostart",
				Trigger: bootBoot,
				Actions: []config.WindowsScheduledTaskAction{{Execute: `C:\foo\bar.exe`}},
				Principal: &config.WindowsScheduledTaskPrincipal{
					User: `DESKTOP-X\aleh`,
				},
			},
		}, false},
		{"happy absent", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{Name: "x", State: "absent"},
		}, false},
		{"bad state", &config.Step{
			WindowsScheduledTask: &config.WindowsScheduledTask{Name: "x", State: "maybe"},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := h.Validate(tc.step)
			if (err != nil) != tc.wantErr {
				t.Errorf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestToTask_TriggerSugar(t *testing.T) {
	// Trigger (singular) should be lifted into Triggers (plural).
	tk, err := toTask(&config.WindowsScheduledTask{
		Name:    "x",
		Trigger: &config.WindowsScheduledTaskTrigger{Type: "boot"},
		Actions: []config.WindowsScheduledTaskAction{{Execute: "x"}},
		Principal: &config.WindowsScheduledTaskPrincipal{User: `X\u`, LogonType: "s4u"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tk.Triggers) != 1 || tk.Triggers[0].Type != winutil.TrigBoot {
		t.Fatalf("triggers: %+v", tk.Triggers)
	}
	if tk.Principal.LogonType != winutil.LogonS4U {
		t.Errorf("logon type: %q", tk.Principal.LogonType)
	}
}

func TestToTask_RepetitionTriggerInterval(t *testing.T) {
	tk, err := toTask(&config.WindowsScheduledTask{
		Name: "Keepalive",
		Triggers: []config.WindowsScheduledTaskTrigger{{
			Type:     "repetition",
			Interval: "5m",
		}},
		Actions: []config.WindowsScheduledTaskAction{{Execute: "x"}},
		Principal: &config.WindowsScheduledTaskPrincipal{User: `X\u`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tk.Triggers[0].RepetitionInterval != 5*time.Minute {
		t.Errorf("interval: %v", tk.Triggers[0].RepetitionInterval)
	}
}

func TestToTask_RepetitionISO8601(t *testing.T) {
	tk, err := toTask(&config.WindowsScheduledTask{
		Name: "x",
		Triggers: []config.WindowsScheduledTaskTrigger{{
			Type:     "repetition",
			Interval: "PT15M",
		}},
		Actions:   []config.WindowsScheduledTaskAction{{Execute: "x"}},
		Principal: &config.WindowsScheduledTaskPrincipal{User: "u"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tk.Triggers[0].RepetitionInterval != 15*time.Minute {
		t.Errorf("interval ISO parse: %v", tk.Triggers[0].RepetitionInterval)
	}
}

func TestParseISO8601(t *testing.T) {
	cases := map[string]time.Duration{
		"PT5M":   5 * time.Minute,
		"PT1H":   time.Hour,
		"PT1H30M": time.Hour + 30*time.Minute,
		"P1D":     24 * time.Hour,
		"P1DT2H":  26 * time.Hour,
		"PT45S":   45 * time.Second,
	}
	for in, want := range cases {
		got, err := parseISO8601(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%q: got %v, want %v", in, got, want)
		}
	}
}

func TestTitleCase_MappingTable(t *testing.T) {
	cases := map[string]string{
		"s4u":             "S4U",
		"interactive":     "Interactive",
		"ignore_new":      "IgnoreNew",
		"stop_existing":   "StopExisting",
		"":                "",
		"already":         "Already",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

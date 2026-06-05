package windows_scheduled_task

import (
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
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
		Name:      "x",
		Trigger:   &config.WindowsScheduledTaskTrigger{Type: "boot"},
		Actions:   []config.WindowsScheduledTaskAction{{Execute: "x"}},
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
		Actions:   []config.WindowsScheduledTaskAction{{Execute: "x"}},
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
		"PT5M":    5 * time.Minute,
		"PT1H":    time.Hour,
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
		"s4u":           "S4U",
		"interactive":   "Interactive",
		"ignore_new":    "IgnoreNew",
		"stop_existing": "StopExisting",
		"":              "",
		"already":       "Already",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderTemplates covers issue #24: the action must render its own
// string fields. A `{{ var }}` in an action's arguments/execute (and the
// principal + triggers) must resolve, not reach Task Scheduler verbatim.
func TestRenderTemplates(t *testing.T) {
	ctx := actions.NewTestContext(actions.TestContextConfig{
		Vars: map[string]interface{}{
			"wsl_distro": "Ubuntu-24.04",
			"username":   `MINIPC\aleha`,
			"wsl_exe":    `C:\Windows\System32\wsl.exe`,
			"poll":       "5m",
		},
	})

	in := &config.WindowsScheduledTask{
		Name:        "WSL2-SSH-Keepalive",
		Description: "keepalive for {{ wsl_distro }}",
		Triggers: []config.WindowsScheduledTaskTrigger{
			{Type: "repetition", Interval: "{{ poll }}"},
		},
		Actions: []config.WindowsScheduledTaskAction{
			{Execute: "{{ wsl_exe }}", Arguments: "-d {{ wsl_distro }} -u root -- systemctl start ssh"},
		},
		Principal: &config.WindowsScheduledTaskPrincipal{User: "{{ username }}"},
	}

	out, err := renderTemplates(ctx, in)
	if err != nil {
		t.Fatalf("renderTemplates: %v", err)
	}

	if got, want := out.Actions[0].Arguments, "-d Ubuntu-24.04 -u root -- systemctl start ssh"; got != want {
		t.Errorf("arguments = %q, want %q", got, want)
	}
	if got, want := out.Actions[0].Execute, `C:\Windows\System32\wsl.exe`; got != want {
		t.Errorf("execute = %q, want %q", got, want)
	}
	if got, want := out.Principal.User, `MINIPC\aleha`; got != want {
		t.Errorf("principal.user = %q, want %q", got, want)
	}
	if got, want := out.Triggers[0].Interval, "5m"; got != want {
		t.Errorf("trigger.interval = %q, want %q", got, want)
	}
	if got, want := out.Description, "keepalive for Ubuntu-24.04"; got != want {
		t.Errorf("description = %q, want %q", got, want)
	}

	// Original step must be untouched (plan mode may re-dispatch it).
	if in.Actions[0].Arguments != "-d {{ wsl_distro }} -u root -- systemctl start ssh" {
		t.Errorf("input mutated: %q", in.Actions[0].Arguments)
	}
	if in.Principal.User != "{{ username }}" {
		t.Errorf("input principal mutated: %q", in.Principal.User)
	}
}
